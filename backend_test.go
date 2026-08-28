package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	update "github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"google.golang.org/protobuf/proto"
)

func pointer[T any](value T) *T { return &value }

func syntheticPayload(t *testing.T) ([]byte, []byte) {
	t.Helper()
	image := bytes.Repeat([]byte{0x5a}, 4096)
	imageHash := sha256.Sum256(image)
	opType := update.InstallOperation_REPLACE
	manifest := &update.DeltaArchiveManifest{
		BlockSize:          pointer(uint32(4096)),
		MinorVersion:       pointer(uint32(0)),
		SecurityPatchLevel: pointer("2026-01-01"),
		Partitions: []*update.PartitionUpdate{{
			PartitionName: pointer("boot"),
			NewPartitionInfo: &update.PartitionInfo{
				Size: pointer(uint64(len(image))),
				Hash: imageHash[:],
			},
			Operations: []*update.InstallOperation{{
				Type:           &opType,
				DataOffset:     pointer(uint64(0)),
				DataLength:     pointer(uint64(len(image))),
				DataSha256Hash: imageHash[:],
				DstExtents: []*update.Extent{{
					StartBlock: pointer(uint64(0)),
					NumBlocks:  pointer(uint64(1)),
				}},
			}},
		}},
	}
	manifestBytes, err := proto.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var payload bytes.Buffer
	payload.WriteString("CrAU")
	if err := binary.Write(&payload, binary.BigEndian, uint64(2)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint64(len(manifestBytes))); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	payload.Write(manifestBytes)
	payload.Write(image)
	return payload.Bytes(), image
}

func syntheticPayloadWithUnusedPartition(t *testing.T, unusedSize int) ([]byte, []byte) {
	t.Helper()
	boot := bytes.Repeat([]byte{0x5a}, 4096)
	unused := bytes.Repeat([]byte{0xa5}, unusedSize)
	bootHash := sha256.Sum256(boot)
	opType := update.InstallOperation_REPLACE
	manifest := &update.DeltaArchiveManifest{
		BlockSize:    pointer(uint32(4096)),
		MinorVersion: pointer(uint32(0)),
		Partitions: []*update.PartitionUpdate{
			{
				PartitionName: pointer("boot"),
				NewPartitionInfo: &update.PartitionInfo{
					Size: pointer(uint64(len(boot))),
					Hash: bootHash[:],
				},
				Operations: []*update.InstallOperation{{
					Type:           &opType,
					DataOffset:     pointer(uint64(0)),
					DataLength:     pointer(uint64(len(boot))),
					DataSha256Hash: bootHash[:],
					DstExtents: []*update.Extent{{
						StartBlock: pointer(uint64(0)),
						NumBlocks:  pointer(uint64(1)),
					}},
				}},
			},
			{
				PartitionName: pointer("vendor"),
				NewPartitionInfo: &update.PartitionInfo{
					Size: pointer(uint64(len(unused))),
				},
				Operations: []*update.InstallOperation{{
					Type:       &opType,
					DataOffset: pointer(uint64(len(boot))),
					DataLength: pointer(uint64(len(unused))),
					DstExtents: []*update.Extent{{
						StartBlock: pointer(uint64(0)),
						NumBlocks:  pointer(uint64(unusedSize / 4096)),
					}},
				}},
			},
		},
	}
	manifestBytes, err := proto.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var payload bytes.Buffer
	payload.WriteString("CrAU")
	if err := binary.Write(&payload, binary.BigEndian, uint64(2)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint64(len(manifestBytes))); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint32(0)); err != nil {
		t.Fatal(err)
	}
	payload.Write(manifestBytes)
	payload.Write(boot)
	payload.Write(unused)
	return payload.Bytes(), boot
}

func syntheticOTA(t *testing.T, payloadBytes []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	payloadHeader := &zip.FileHeader{Name: "payload.bin", Method: zip.Store}
	payloadEntry, err := zw.CreateHeader(payloadHeader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := payloadEntry.Write(payloadBytes); err != nil {
		t.Fatal(err)
	}
	metadataEntry, err := zw.Create("META-INF/com/android/metadata")
	if err != nil {
		t.Fatal(err)
	}
	metadata := "post-build=xiaomi/fuxi/fuxi:13/TKQ1.221114.001/V14.0.32.0.TMCCNXM:user/release-keys\n" +
		"ro.product.marketname=Xiaomi 13\n" +
		"ota_version=V14.0.32.0.TMCCNXM\n" +
		"post-sdk-level=33\n" +
		"post-security-patch-level=2023-08-01\n"
	if _, err := metadataEntry.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

type rangeRecorder struct {
	mu         sync.Mutex
	bytesSent  int64
	fullReads  int
	maxRequest int64
}

func (recorder *rangeRecorder) reset() {
	recorder.mu.Lock()
	recorder.bytesSent = 0
	recorder.fullReads = 0
	recorder.maxRequest = 0
	recorder.mu.Unlock()
}

func (recorder *rangeRecorder) snapshot() (int64, int, int64) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.bytesSent, recorder.fullReads, recorder.maxRequest
}

func newRangeServer(t *testing.T, content []byte, recorder *rangeRecorder) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		rangeHeader := request.Header.Get("Range")
		if !strings.HasPrefix(rangeHeader, "bytes=") || strings.Contains(rangeHeader, ",") {
			recorder.mu.Lock()
			recorder.fullReads++
			recorder.mu.Unlock()
			http.Error(w, "Range required", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(rangeHeader, "bytes="), "-", 2)
		start, startErr := strconv.ParseInt(parts[0], 10, 64)
		end, endErr := strconv.ParseInt(parts[1], 10, 64)
		if startErr != nil || endErr != nil || start < 0 || end < start || end >= int64(len(content)) {
			http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		length := end - start + 1
		recorder.mu.Lock()
		recorder.bytesSent += length
		if length > recorder.maxRequest {
			recorder.maxRequest = length
		}
		recorder.mu.Unlock()
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
}

func newSlowRangeServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		rangeHeader := request.Header.Get("Range")
		parts := strings.SplitN(strings.TrimPrefix(rangeHeader, "bytes="), "-", 2)
		if len(parts) != 2 {
			http.Error(w, "Range required", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start, startErr := strconv.ParseInt(parts[0], 10, 64)
		end, endErr := strconv.ParseInt(parts[1], 10, 64)
		if startErr != nil || endErr != nil || start < 0 || end < start || end >= int64(len(content)) {
			http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		length := end - start + 1
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		if length == 1 {
			_, _ = w.Write(content[start : end+1])
			return
		}
		flusher, _ := w.(http.Flusher)
		for offset := start; offset <= end; {
			select {
			case <-request.Context().Done():
				return
			case <-time.After(20 * time.Millisecond):
			}
			chunkEnd := offset + (64 << 10) - 1
			if chunkEnd > end {
				chunkEnd = end
			}
			if _, err := w.Write(content[offset : chunkEnd+1]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			offset = chunkEnd + 1
		}
	}))
}

func TestInspectAndExtractOTA(t *testing.T) {
	payloadBytes, expectedImage := syntheticPayload(t)
	tempDir := t.TempDir()
	otaPath := filepath.Join(tempDir, "firmware.zip")
	file, err := os.Create(otaPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	payloadHeader := &zip.FileHeader{Name: "payload.bin", Method: zip.Store}
	payloadEntry, err := zw.CreateHeader(payloadHeader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := payloadEntry.Write(payloadBytes); err != nil {
		t.Fatal(err)
	}
	metadataEntry, err := zw.Create("META-INF/com/android/metadata")
	if err != nil {
		t.Fatal(err)
	}
	metadata := "post-build=oneplus/PGZ110/PGZ110:15/AP3A.240905.015/1600:user/release-keys\n" +
		"ro.product.marketname=一加ACE竞速版\n" +
		"ota_version=PGZ110_15.0.0.1600(CN01)\n" +
		"post-sdk-level=35\n" +
		"post-security-patch-level=2026-01-01\n"
	if _, err := metadataEntry.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	details, err := inspectPackage(otaPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Partitions) != 1 || details.Partitions[0].Name != "boot" {
		t.Fatalf("unexpected partitions: %#v", details.Partitions)
	}
	if details.Info.Model != "一加ACE竞速版" || details.Info.Device != "PGZ110" || details.Info.Android != "15" {
		t.Fatalf("unexpected device info: %#v", details.Info)
	}
	if details.Info.SystemVersion != "PGZ110_15.0.0.1600(CN01)" || details.Info.SecurityPatch != "2026-01-01" {
		t.Fatalf("unexpected version info: %#v", details.Info)
	}

	outputDir := filepath.Join(tempDir, "out")
	if err := extractPackage(context.Background(), otaPath, outputDir, "", []string{"boot"}, 1, nil); err != nil {
		t.Fatal(err)
	}
	actualImage, err := os.ReadFile(filepath.Join(outputDir, "boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actualImage, expectedImage) {
		t.Fatalf("boot.img mismatch: got %d bytes, want %d", len(actualImage), len(expectedImage))
	}
}

func TestOnlineExtractionUsesOnlyRanges(t *testing.T) {
	payloadBytes, expectedBoot := syntheticPayloadWithUnusedPartition(t, 16<<20)
	otaBytes := syntheticOTA(t, payloadBytes)
	recorder := &rangeRecorder{}
	server := newRangeServer(t, otaBytes, recorder)
	defer server.Close()

	details, err := inspectPackage(server.URL + "/firmware.zip")
	if err != nil {
		t.Fatal(err)
	}
	if !details.Remote || len(details.Partitions) != 2 {
		t.Fatalf("unexpected online details: %#v", details)
	}
	bytesSent, fullReads, maxRequest := recorder.snapshot()
	if fullReads != 0 || bytesSent >= int64(len(otaBytes)) || maxRequest > remoteBlockSize {
		t.Fatalf("inspection did not stay selective: sent=%d file=%d full=%d max=%d", bytesSent, len(otaBytes), fullReads, maxRequest)
	}

	recorder.reset()
	outputDir := filepath.Join(t.TempDir(), "out")
	if err := extractPackage(context.Background(), server.URL+"/firmware.zip", outputDir, "", []string{"boot"}, 2, nil); err != nil {
		t.Fatal(err)
	}
	actualBoot, err := os.ReadFile(filepath.Join(outputDir, "boot.img"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actualBoot, expectedBoot) {
		t.Fatal("online boot.img mismatch")
	}
	bytesSent, fullReads, maxRequest = recorder.snapshot()
	if fullReads != 0 || bytesSent >= int64(len(otaBytes)) || maxRequest > remoteBlockSize {
		t.Fatalf("extraction did not stay selective: sent=%d file=%d full=%d max=%d", bytesSent, len(otaBytes), fullReads, maxRequest)
	}
}

func TestOnlineOTAFromEnvironment(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("ONLINE_OTA_URL"))
	if address == "" {
		t.Skip("ONLINE_OTA_URL is not set")
	}
	details, err := inspectPackage(address)
	if err != nil {
		t.Fatal(err)
	}
	if !details.Remote || len(details.Partitions) == 0 {
		t.Fatalf("unexpected live OTA details: %#v", details)
	}
	t.Logf("model=%q device=%q android=%q version=%q patch=%q partitions=%d", details.Info.Model, details.Info.Device, details.Info.Android, details.Info.SystemVersion, details.Info.SecurityPatch, len(details.Partitions))
}

func TestProgressAndCancellation(t *testing.T) {
	payloadBytes, _ := syntheticPayloadWithUnusedPartition(t, 16<<20)
	otaBytes := syntheticOTA(t, payloadBytes)
	otaPath := filepath.Join(t.TempDir(), "cancel-test.zip")
	if err := os.WriteFile(otaPath, otaBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "out")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sawByteProgress bool
	started := time.Now()
	err := extractPackage(ctx, otaPath, outputDir, "", []string{"vendor"}, 1, func(progress ExtractionProgress) {
		if progress.BytesDone > 0 {
			sawByteProgress = true
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if !sawByteProgress {
		t.Fatal("no byte-level progress was reported")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cancellation took too long: %s", elapsed)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "vendor.img")); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete image was not removed: %v", statErr)
	}
}

func TestOnlineCancellationWhileDownloading(t *testing.T) {
	payloadBytes, _ := syntheticPayloadWithUnusedPartition(t, 16<<20)
	otaBytes := syntheticOTA(t, payloadBytes)
	server := newSlowRangeServer(t, otaBytes)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(120*time.Millisecond, cancel)
	started := time.Now()
	err := extractPackage(ctx, server.URL+"/firmware.zip", t.TempDir(), "", []string{"boot"}, 1, nil)
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected online context cancellation, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("online cancellation took too long: %s", elapsed)
	}
}

func TestOnlineOTAExtractionFromEnvironment(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("ONLINE_OTA_URL"))
	if address == "" {
		t.Skip("ONLINE_OTA_URL is not set")
	}
	partition := strings.TrimSpace(os.Getenv("ONLINE_OTA_PARTITION"))
	if partition == "" {
		partition = "init_boot"
	}
	details, err := inspectPackage(address)
	if err != nil {
		t.Fatal(err)
	}
	var expectedSize uint64
	found := false
	for _, item := range details.Partitions {
		if item.Name == partition {
			expectedSize = item.Size
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("partition %q not found", partition)
	}
	outputDir := t.TempDir()
	var sawProgress bool
	if err := extractPackage(context.Background(), address, outputDir, "", []string{partition}, 1, func(progress ExtractionProgress) {
		if progress.BytesDone > 0 {
			sawProgress = true
		}
	}); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(outputDir, partition+".img")
	stat, err := os.Stat(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if expectedSize > 0 && uint64(stat.Size()) != expectedSize {
		t.Fatalf("image size mismatch: got %d, want %d", stat.Size(), expectedSize)
	}
	if !sawProgress {
		t.Fatal("no byte-level progress was reported")
	}
	t.Logf("extracted %s (%d bytes) with byte-level progress", partition, stat.Size())
}
