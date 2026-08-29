package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func syntheticFastbootZIP(t *testing.T, largeUnused int) ([]byte, map[string][]byte) {
	t.Helper()
	images := map[string][]byte{
		"boot":        bytes.Repeat([]byte{0x42}, 1<<20),
		"vendor_boot": bytes.Repeat([]byte{0x73}, largeUnused),
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	metadata, err := writer.Create("build.prop")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = metadata.Write([]byte("ro.product.marketname=Fastboot Test Phone\nro.product.device=test_device\nro.build.version.release=15\nro.build.version.security_patch=2026-08-01\nro.build.display.id=TEST.1\n"))
	boot, err := writer.Create("RADIO/boot.img")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = boot.Write(images["boot"])
	vendorHeader := &zip.FileHeader{Name: "RADIO/vendor_boot.img", Method: zip.Store}
	vendor, err := writer.CreateHeader(vendorHeader)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = vendor.Write(images["vendor_boot"])
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), images
}

func syntheticFastbootTGZ(t *testing.T) ([]byte, map[string][]byte) {
	t.Helper()
	images := map[string][]byte{
		"boot":      bytes.Repeat([]byte{0x31}, 1<<20),
		"init_boot": bytes.Repeat([]byte{0x62}, 2<<20),
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	documents := map[string][]byte{
		"fuxi_images/build.prop": []byte("ro.product.marketname=Xiaomi Test\nro.product.device=fuxi\nro.build.version.release=13\nro.build.version.security_patch=2023-08-01\nro.build.display.id=V14.TEST\n"),
	}
	for name, data := range documents {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		_, _ = tarWriter.Write(data)
	}
	for _, name := range []string{"boot", "init_boot"} {
		data := images[name]
		entryName := "fuxi_images/images/" + name + ".img"
		if err := tarWriter.WriteHeader(&tar.Header{Name: entryName, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		_, _ = tarWriter.Write(data)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), images
}

func TestGenericZIPInspectAndExtract(t *testing.T) {
	archive, images := syntheticFastbootZIP(t, 2<<20)
	archivePath := filepath.Join(t.TempDir(), "fastboot.zip")
	if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
		t.Fatal(err)
	}
	details, err := inspectPackage(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if details.Mode != packageModeZIP || details.ArchiveEntries != 3 || len(details.Partitions) != 2 {
		t.Fatalf("unexpected ZIP details: %#v", details)
	}
	if details.Info.Device != "test_device" || details.Info.Android != "15" || details.Info.PackageType != "线刷 ZIP" {
		t.Fatalf("unexpected ZIP metadata: %#v", details.Info)
	}
	outputDir := t.TempDir()
	if err := extractPackageWithItems(context.Background(), details.Path, outputDir, "", []string{"boot"}, details.Partitions, 4, nil); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(outputDir, "boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, images["boot"]) {
		t.Fatal("generic ZIP boot.img mismatch")
	}
}

func TestGenericRemoteZIPUsesSelectedRanges(t *testing.T) {
	archive, images := syntheticFastbootZIP(t, 16<<20)
	recorder := &rangeRecorder{}
	server := newRangeServer(t, archive, recorder)
	defer server.Close()
	details, err := inspectPackage(server.URL + "/fastboot.zip")
	if err != nil {
		t.Fatal(err)
	}
	if details.Mode != packageModeZIP || !details.Remote {
		t.Fatalf("unexpected remote ZIP mode: %#v", details)
	}
	bytesSent, fullReads, maxRequest := recorder.snapshot()
	if fullReads != 0 || bytesSent >= int64(len(archive)) || maxRequest > remoteBlockSize {
		t.Fatalf("remote ZIP inspection was not selective: sent=%d file=%d full=%d max=%d", bytesSent, len(archive), fullReads, maxRequest)
	}
	recorder.reset()
	outputDir := t.TempDir()
	if err := extractPackageWithItems(context.Background(), details.Path, outputDir, "", []string{"boot"}, details.Partitions, 2, nil); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(outputDir, "boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, images["boot"]) {
		t.Fatal("remote generic ZIP boot.img mismatch")
	}
	bytesSent, fullReads, maxRequest = recorder.snapshot()
	if fullReads != 0 || bytesSent >= int64(len(archive)) || maxRequest > remoteBlockSize {
		t.Fatalf("remote ZIP extraction was not selective: sent=%d file=%d full=%d max=%d", bytesSent, len(archive), fullReads, maxRequest)
	}
}

func TestTGZInspectAndExtract(t *testing.T) {
	archive, images := syntheticFastbootTGZ(t)
	archivePath := filepath.Join(t.TempDir(), "fastboot.tgz")
	if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
		t.Fatal(err)
	}
	details, err := inspectPackage(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if details.Mode != packageModeTGZ || len(details.Partitions) != 2 || details.Info.Device != "fuxi" || details.Info.PackageType != "线刷 TGZ" {
		t.Fatalf("unexpected TGZ details: %#v", details)
	}
	outputDir := t.TempDir()
	if err := extractPackageWithItems(context.Background(), details.Path, outputDir, "", []string{"init_boot"}, details.Partitions, 64, nil); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(outputDir, "init_boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, images["init_boot"]) {
		t.Fatal("TGZ init_boot.img mismatch")
	}
}

func TestFastbootFilenameMetadataFallback(t *testing.T) {
	info := DeviceInfo{}
	applyArchiveFilenameMetadata(&info, "https://cdn.example/fuxi_images_V14.0.31.0.TMCCNXM_20230802.0000.00_13.0_cn_hash.tgz")
	if info.Device != "fuxi" || info.SystemVersion != "V14.0.31.0.TMCCNXM" || info.Android != "13.0" || info.BuildDate != "2023-08-02" {
		t.Fatalf("unexpected filename metadata: %#v", info)
	}
}

func TestGenericRemoteZIPCancellation(t *testing.T) {
	archive, _ := syntheticFastbootZIP(t, 16<<20)
	server := newSlowRangeServer(t, archive)
	defer server.Close()
	details, err := inspectPackage(server.URL + "/fastboot.zip")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(120*time.Millisecond, cancel)
	started := time.Now()
	err = extractPackageWithItems(ctx, details.Path, t.TempDir(), "", []string{"vendor_boot"}, details.Partitions, 1, nil)
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ZIP cancellation, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("ZIP cancellation took too long: %s", elapsed)
	}
}

func TestRemoteTGZCancellationCleansCache(t *testing.T) {
	archive, _ := syntheticFastbootTGZ(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", stringLength(len(archive)*64))
		flusher, _ := writer.(http.Flusher)
		for index := 0; index < 64; index++ {
			select {
			case <-request.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
			_, _ = writer.Write(archive)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "LitePayloadDumper-*.tgz"))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	_, err := inspectPackageContext(ctx, server.URL+"/fastboot.tgz", nil)
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected TGZ cancellation, got %v", err)
	}
	after, _ := filepath.Glob(filepath.Join(os.TempDir(), "LitePayloadDumper-*.tgz"))
	if len(after) != len(before) {
		t.Fatalf("TGZ cancellation left a cache file: before=%v after=%v", before, after)
	}
}

func TestRemoteTGZCachesOnceAndCleansUp(t *testing.T) {
	archive, images := syntheticFastbootTGZ(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.Header().Set("Content-Length", stringLength(len(archive)))
		_, _ = writer.Write(archive)
	}))
	defer server.Close()
	var sawCacheProgress bool
	details, err := inspectPackageContext(context.Background(), server.URL+"/fastboot.tgz", func(progress InspectionProgress) {
		if progress.Stage == "正在缓存远程 TGZ" && progress.Done > 0 {
			sawCacheProgress = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	cachePath := details.TempPath
	if requests.Load() != 1 || !details.Remote || cachePath == "" || !sawCacheProgress {
		t.Fatalf("unexpected remote TGZ cache details: requests=%d details=%#v progress=%v", requests.Load(), details, sawCacheProgress)
	}
	outputDir := t.TempDir()
	if err := extractPackageWithItems(context.Background(), details.Path, outputDir, "", []string{"boot"}, details.Partitions, 4, nil); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("TGZ extraction downloaded again: requests=%d", requests.Load())
	}
	actual, err := os.ReadFile(filepath.Join(outputDir, "boot.img"))
	if err != nil || !bytes.Equal(actual, images["boot"]) {
		t.Fatalf("remote TGZ boot mismatch: %v", err)
	}
	details.Cleanup()
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("TGZ cache was not removed: %v", err)
	}
}

func TestGenericRemoteZIPFromEnvironment(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("GENERIC_ZIP_URL"))
	if address == "" {
		t.Skip("GENERIC_ZIP_URL is not set")
	}
	details, err := inspectPackage(address)
	if err != nil {
		t.Fatal(err)
	}
	if details.Mode != packageModeZIP || len(details.Partitions) == 0 {
		t.Fatalf("unexpected generic ZIP details: %#v", details)
	}
	partition := strings.TrimSpace(os.Getenv("GENERIC_ZIP_PARTITION"))
	if partition == "" {
		partition = "vbmeta"
		for _, item := range details.Partitions {
			if item.Name == "RADIO_vbmeta" {
				partition = item.Name
				break
			}
		}
	}
	found := false
	for _, item := range details.Partitions {
		if item.Name == partition {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("partition %q not found", partition)
	}
	outputDir := t.TempDir()
	if err := extractPackageWithItems(context.Background(), details.Path, outputDir, "", []string{partition}, details.Partitions, 1, nil); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(filepath.Join(outputDir, partition+".img"))
	if err != nil || stat.Size() == 0 {
		t.Fatalf("generic ZIP extraction failed: %v", err)
	}
	t.Logf("entries=%d partitions=%d model=%q device=%q android=%q extracted=%s(%d)", details.ArchiveEntries, len(details.Partitions), details.Info.Model, details.Info.Device, details.Info.Android, partition, stat.Size())
}

func TestRemoteTGZPrefixFromEnvironment(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("TGZ_PREFIX_URL"))
	if address == "" {
		t.Skip("TGZ_PREFIX_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Range", "bytes=0-4194303")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		t.Fatalf("TGZ server did not honor Range: %s", response.Status)
	}
	prefix, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(prefix) > 4<<20 {
		t.Fatal("TGZ prefix response exceeded requested Range")
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(prefix))
	if err != nil {
		t.Fatalf("remote file is not gzip: %v", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var names []string
	for len(names) < 8 {
		header, nextErr := tarReader.Next()
		if nextErr != nil {
			break
		}
		names = append(names, header.Name)
	}
	if len(names) == 0 {
		t.Fatal("remote gzip prefix did not contain a TAR header")
	}
	t.Logf("validated remote TGZ prefix with entries: %s", strings.Join(names, ", "))
}

func stringLength(length int) string {
	return fmt.Sprintf("%d", length)
}
