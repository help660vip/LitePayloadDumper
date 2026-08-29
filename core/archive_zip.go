package core

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

var invalidArchiveName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

type zipImageCandidate struct {
	entry *zip.File
	base  string
}

func zipPartitionItems(zr *zip.Reader) []PartitionItem {
	if zr == nil {
		return nil
	}
	var candidates []zipImageCandidate
	counts := make(map[string]int)
	for _, entry := range zr.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if entry.FileInfo().IsDir() || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > math.MaxInt64 || !strings.EqualFold(path.Ext(name), ".img") {
			continue
		}
		base := sanitizeArchivePartitionName(strings.TrimSuffix(path.Base(name), path.Ext(name)))
		if base == "" {
			continue
		}
		candidates = append(candidates, zipImageCandidate{entry: entry, base: base})
		counts[strings.ToLower(base)]++
	}
	used := make(map[string]int)
	items := make([]PartitionItem, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.base
		if counts[strings.ToLower(name)] > 1 {
			parent := sanitizeArchivePartitionName(path.Base(path.Dir(strings.ReplaceAll(candidate.entry.Name, "\\", "/"))))
			if parent != "" && parent != "." {
				name = parent + "_" + name
			}
		}
		key := strings.ToLower(name)
		used[key]++
		if used[key] > 1 {
			name = fmt.Sprintf("%s_%d", name, used[key])
		}
		item := PartitionItem{Name: name, Size: candidate.entry.UncompressedSize64, Operations: 1, ArchivePath: candidate.entry.Name}
		if candidate.entry.Method != zip.Store && candidate.entry.Method != zip.Deflate {
			item.UnsupportedOps = []string{fmt.Sprintf("ZIP method %d", candidate.entry.Method)}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func sanitizeArchivePartitionName(name string) string {
	name = invalidArchiveName.ReplaceAllString(strings.TrimSpace(name), "_")
	name = strings.Trim(name, "._-")
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func extractZIPImages(ctx context.Context, zr *zip.Reader, outputDir string, partitions []string, concurrency int, onProgress func(ExtractionProgress)) error {
	items := zipPartitionItems(zr)
	byName := make(map[string]PartitionItem, len(items))
	entries := make(map[string]*zip.File, len(zr.File))
	for _, entry := range zr.File {
		entries[entry.Name] = entry
	}
	for _, item := range items {
		byName[item.Name] = item
	}
	selected := make([]PartitionItem, 0, len(partitions))
	var totalBytes uint64
	for _, name := range partitions {
		item, ok := byName[name]
		if !ok {
			return fmt.Errorf("ZIP 中未找到镜像分区 %q", name)
		}
		if len(item.UnsupportedOps) > 0 {
			return fmt.Errorf("%s 不支持：%s", name, strings.Join(item.UnsupportedOps, ", "))
		}
		selected = append(selected, item)
		totalBytes += item.Size
	}

	completedBytes := make(map[string]uint64, len(selected))
	completedItems := make(map[string]bool, len(selected))
	var progressMu sync.Mutex
	report := func(item PartitionItem, bytesDone uint64, done bool, failure error) {
		progressMu.Lock()
		completedBytes[item.Name] = bytesDone
		if done && failure == nil {
			completedItems[item.Name] = true
		}
		var overallBytes uint64
		for _, count := range completedBytes {
			overallBytes += count
		}
		completed := len(completedItems)
		progressMu.Unlock()
		if onProgress != nil {
			onProgress(ExtractionProgress{
				Partition:      item.Name,
				CompletedOps:   boolInt(done && failure == nil),
				PartitionOps:   1,
				OverallDone:    completed,
				OverallTotal:   len(selected),
				BytesDone:      bytesDone,
				BytesTotal:     item.Size,
				OverallBytes:   overallBytes,
				OverallSize:    totalBytes,
				Stage:          "解压 ZIP",
				PartitionDone:  done,
				PartitionError: failure,
			})
		}
	}

	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(selected) {
		concurrency = len(selected)
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)
	for _, item := range selected {
		item := item
		entry := entries[item.ArchivePath]
		if entry == nil {
			return fmt.Errorf("ZIP 条目不存在：%s", item.ArchivePath)
		}
		group.Go(func() (err error) {
			report(item, 0, false, nil)
			outputPath := filepath.Join(outputDir, item.Name+".img")
			defer func() {
				if err != nil {
					_ = os.Remove(outputPath)
					report(item, 0, true, err)
				}
			}()
			reader, err := entry.Open()
			if err != nil {
				return fmt.Errorf("打开 ZIP 条目 %s 失败：%w", item.ArchivePath, err)
			}
			defer reader.Close()
			output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("创建 %s 失败：%w", outputPath, err)
			}
			var written uint64
			buffer := make([]byte, 256<<10)
			for {
				if err := groupCtx.Err(); err != nil {
					output.Close()
					return err
				}
				count, readErr := reader.Read(buffer)
				if count > 0 {
					writeCount, writeErr := output.Write(buffer[:count])
					written += uint64(writeCount)
					report(item, written, false, nil)
					if writeErr != nil {
						output.Close()
						return fmt.Errorf("写入 %s 失败：%w", outputPath, writeErr)
					}
					if writeCount != count {
						output.Close()
						return io.ErrShortWrite
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					output.Close()
					return fmt.Errorf("解压 ZIP 条目 %s 失败：%w", item.ArchivePath, readErr)
				}
			}
			if err := output.Close(); err != nil {
				return fmt.Errorf("关闭 %s 失败：%w", outputPath, err)
			}
			if written != item.Size {
				return fmt.Errorf("%s 大小不符：得到 %d，预期 %d", item.Name, written, item.Size)
			}
			report(item, written, true, nil)
			return nil
		})
	}
	return group.Wait()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
