package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type InspectionProgress struct {
	Stage string
	Done  uint64
	Total uint64
}

type archiveMetadataDocument struct {
	name string
	data []byte
}

type contextCountingReader struct {
	ctx    context.Context
	reader io.Reader
	done   uint64
	total  uint64
	stage  string
	onRead func(InspectionProgress)
	last   time.Time
}

func (reader *contextCountingReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if len(buffer) > 256<<10 {
		buffer = buffer[:256<<10]
	}
	count, err := reader.reader.Read(buffer)
	reader.done += uint64(count)
	if reader.onRead != nil && (reader.last.IsZero() || time.Since(reader.last) >= 100*time.Millisecond || (reader.total > 0 && reader.done >= reader.total)) {
		reader.last = time.Now()
		reader.onRead(InspectionProgress{Stage: reader.stage, Done: reader.done, Total: reader.total})
	}
	return count, err
}

func cacheRemoteTGZ(ctx context.Context, address string, onProgress func(InspectionProgress)) (cachePath string, size int64, err error) {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", 0, fmt.Errorf("在线 TGZ 链接无效：%w", err)
	}
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "LitePayloadDumper/1.0")
	response, err := client.Do(request)
	if err != nil {
		return "", 0, fmt.Errorf("连接在线 TGZ 失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("下载在线 TGZ 返回 %s", response.Status)
	}
	size = response.ContentLength
	file, err := os.CreateTemp("", "LitePayloadDumper-*.tgz")
	if err != nil {
		return "", 0, fmt.Errorf("创建 TGZ 临时缓存失败：%w", err)
	}
	cachePath = file.Name()
	tempPath := cachePath
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(tempPath)
			cachePath = ""
		}
	}()
	buffer := make([]byte, 1<<20)
	var written uint64
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			writeCount, writeErr := file.Write(buffer[:count])
			written += uint64(writeCount)
			if onProgress != nil {
				total := uint64(0)
				if size > 0 {
					total = uint64(size)
				}
				onProgress(InspectionProgress{Stage: "正在缓存远程 TGZ", Done: written, Total: total})
			}
			if writeErr != nil {
				return "", 0, fmt.Errorf("写入 TGZ 临时缓存失败：%w", writeErr)
			}
			if writeCount != count {
				return "", 0, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", 0, fmt.Errorf("下载远程 TGZ 失败：%w", readErr)
		}
	}
	if size > 0 && written != uint64(size) {
		return "", 0, fmt.Errorf("TGZ 缓存大小不符：得到 %d，预期 %d", written, size)
	}
	return cachePath, int64(written), nil
}

func inspectTGZPackage(ctx context.Context, filename string, size int64, onProgress func(InspectionProgress)) (*PackageDetails, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开 TGZ 失败：%w", err)
	}
	defer file.Close()
	counting := &contextCountingReader{ctx: ctx, reader: file, total: uint64(size), stage: "正在扫描 TGZ 文件目录", onRead: onProgress}
	gzipReader, err := gzip.NewReader(counting)
	if err != nil {
		return nil, fmt.Errorf("读取 TGZ gzip 数据失败：%w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var imagePaths []string
	imageSizes := make(map[string]uint64)
	var documents []archiveMetadataDocument
	remainingMetadata := int64(20 << 20)
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("扫描 TGZ 目录失败：%w", nextErr)
		}
		entries++
		name := strings.ReplaceAll(header.Name, "\\", "/")
		regular := header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA
		if regular && header.Size > 0 && strings.EqualFold(path.Ext(name), ".img") {
			imagePaths = append(imagePaths, name)
			imageSizes[name] = uint64(header.Size)
		}
		lowerName := strings.ToLower(name)
		if regular && remainingMetadata > 0 && header.Size > 0 && header.Size <= 4<<20 && isMetadataCandidate(lowerName) {
			limit := header.Size
			if limit > remainingMetadata {
				limit = remainingMetadata
			}
			data, readErr := io.ReadAll(io.LimitReader(tarReader, limit))
			if readErr == nil {
				documents = append(documents, archiveMetadataDocument{name: name, data: data})
				remainingMetadata -= int64(len(data))
			}
		}
	}
	items := tarPartitionItems(imagePaths, imageSizes)
	info := inspectArchiveMetadataDocuments("线刷 TGZ", documents)
	info.PartitionCount = len(items)
	return &PackageDetails{
		Path: filename, FileSize: size, Mode: packageModeTGZ, ArchiveEntries: entries,
		Partitions: items, Info: info,
	}, nil
}

func tarPartitionItems(paths []string, sizes map[string]uint64) []PartitionItem {
	counts := make(map[string]int)
	bases := make(map[string]string, len(paths))
	for _, entryPath := range paths {
		base := sanitizeArchivePartitionName(strings.TrimSuffix(path.Base(entryPath), path.Ext(entryPath)))
		bases[entryPath] = base
		counts[strings.ToLower(base)]++
	}
	used := make(map[string]int)
	items := make([]PartitionItem, 0, len(paths))
	for _, entryPath := range paths {
		name := bases[entryPath]
		if name == "" {
			continue
		}
		if counts[strings.ToLower(name)] > 1 {
			parent := sanitizeArchivePartitionName(path.Base(path.Dir(entryPath)))
			if parent != "" && parent != "." {
				name = parent + "_" + name
			}
		}
		key := strings.ToLower(name)
		used[key]++
		if used[key] > 1 {
			name = fmt.Sprintf("%s_%d", name, used[key])
		}
		items = append(items, PartitionItem{Name: name, Size: sizes[entryPath], Operations: 1, ArchivePath: entryPath})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func inspectArchiveMetadataDocuments(packageType string, documents []archiveMetadataDocument) DeviceInfo {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, document := range documents {
		entry, err := writer.Create(document.name)
		if err != nil {
			continue
		}
		_, _ = entry.Write(document.data)
	}
	if writer.Close() != nil {
		return DeviceInfo{PackageType: packageType}
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return DeviceInfo{PackageType: packageType}
	}
	info := inspectZipMetadata(reader)
	info.PackageType = packageType
	return info
}

func extractTGZImages(ctx context.Context, filename, outputDir string, partitions []string, suppliedItems []PartitionItem, onProgress func(ExtractionProgress)) error {
	items := suppliedItems
	if len(items) == 0 {
		stat, err := os.Stat(filename)
		if err != nil {
			return err
		}
		details, err := inspectTGZPackage(ctx, filename, stat.Size(), nil)
		if err != nil {
			return err
		}
		items = details.Partitions
	}
	byName := make(map[string]PartitionItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}
	selected := make(map[string]PartitionItem, len(partitions))
	var totalBytes uint64
	for _, name := range partitions {
		item, ok := byName[name]
		if !ok {
			return fmt.Errorf("TGZ 中未找到镜像分区 %q", name)
		}
		selected[item.ArchivePath] = item
		totalBytes += item.Size
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	var progressMu sync.Mutex
	completedBytes := make(map[string]uint64, len(selected))
	completedItems := make(map[string]bool, len(selected))
	report := func(item PartitionItem, bytesDone uint64, stage string, done bool, failure error) {
		progressMu.Lock()
		completedBytes[item.Name] = bytesDone
		if done && failure == nil {
			completedItems[item.Name] = true
		}
		var overall uint64
		for _, count := range completedBytes {
			overall += count
		}
		completed := len(completedItems)
		progressMu.Unlock()
		if onProgress != nil {
			onProgress(ExtractionProgress{Partition: item.Name, CompletedOps: boolInt(done && failure == nil), PartitionOps: 1,
				OverallDone: completed, OverallTotal: len(selected), BytesDone: bytesDone, BytesTotal: item.Size,
				OverallBytes: overall, OverallSize: totalBytes, Stage: stage, PartitionDone: done, PartitionError: failure})
		}
	}
	var first PartitionItem
	for _, item := range selected {
		first = item
		break
	}
	counting := &contextCountingReader{ctx: ctx, reader: file, total: uint64(stat.Size()), stage: "扫描 TGZ"}
	counting.onRead = func(InspectionProgress) { report(first, completedBytes[first.Name], "扫描 TGZ", false, nil) }
	gzipReader, err := gzip.NewReader(counting)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	remaining := len(selected)
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("扫描 TGZ 失败：%w", nextErr)
		}
		item, wanted := selected[strings.ReplaceAll(header.Name, "\\", "/")]
		if !wanted {
			continue
		}
		outputPath := filepath.Join(outputDir, item.Name+".img")
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		var written uint64
		buffer := make([]byte, 256<<10)
		for written < item.Size {
			if err := ctx.Err(); err != nil {
				output.Close()
				_ = os.Remove(outputPath)
				return err
			}
			want := len(buffer)
			if left := item.Size - written; uint64(want) > left {
				want = int(left)
			}
			count, readErr := tarReader.Read(buffer[:want])
			if count > 0 {
				writeCount, writeErr := output.Write(buffer[:count])
				written += uint64(writeCount)
				report(item, written, "解压 TGZ", false, nil)
				if writeErr != nil || writeCount != count {
					output.Close()
					_ = os.Remove(outputPath)
					if writeErr != nil {
						return writeErr
					}
					return io.ErrShortWrite
				}
			}
			if readErr != nil && readErr != io.EOF {
				output.Close()
				_ = os.Remove(outputPath)
				return readErr
			}
			if readErr == io.EOF && written < item.Size {
				output.Close()
				_ = os.Remove(outputPath)
				return io.ErrUnexpectedEOF
			}
		}
		if err := output.Close(); err != nil {
			_ = os.Remove(outputPath)
			return err
		}
		report(item, written, "完成", true, nil)
		delete(selected, item.ArchivePath)
		remaining--
	}
	if remaining != 0 {
		return fmt.Errorf("TGZ 扫描结束，仍有 %d 个所选镜像未找到", remaining)
	}
	return nil
}
