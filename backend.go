package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/ssut/payload-dumper-go/payload"
)

type PartitionItem struct {
	Name           string
	Size           uint64
	Operations     int
	NeedsSource    bool
	UnsupportedOps []string
}

type PackageDetails struct {
	Path       string
	FileSize   int64
	Remote     bool
	Partitions []PartitionItem
	Info       DeviceInfo
}

type ExtractionProgress struct {
	Partition      string
	CompletedOps   int
	PartitionOps   int
	OverallDone    int
	OverallTotal   int
	BytesDone      uint64
	BytesTotal     uint64
	OverallBytes   uint64
	OverallSize    uint64
	Stage          string
	PartitionDone  bool
	PartitionError error
}

var safePartitionName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")

func inspectPackage(path string) (*PackageDetails, error) {
	source, err := openPackageSource(context.Background(), path)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	pl := source.payload
	metadataInfo := inspectZipMetadata(source.zipReader)

	parts := pl.Partitions()
	items := make([]PartitionItem, 0, len(parts))
	for _, p := range parts {
		if !safePartitionName.MatchString(p.Name) || p.Name == "." || p.Name == ".." {
			return nil, fmt.Errorf("Payload 包含不安全的分区名：%q", p.Name)
		}
		items = append(items, PartitionItem{
			Name:           p.Name,
			Size:           p.Size,
			Operations:     p.TotalOperations,
			NeedsSource:    p.NeedsSource,
			UnsupportedOps: append([]string(nil), p.UnsupportedOperations...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	metadataInfo.PayloadVersion = pl.Header().Version
	metadataInfo.MinorVersion = pl.MinorVersion()
	metadataInfo.IsDelta = pl.IsDelta()
	metadataInfo.PartitionCount = len(items)
	if metadataInfo.SecurityPatch == "" {
		metadataInfo.SecurityPatch = pl.Manifest().GetSecurityPatchLevel()
	}
	if metadataInfo.BuildDate == "" && pl.Manifest().GetMaxTimestamp() > 0 {
		metadataInfo.BuildDate = formatUnixDate(pl.Manifest().GetMaxTimestamp())
	}

	return &PackageDetails{
		Path:       path,
		FileSize:   source.size,
		Remote:     isRemoteInput(path),
		Partitions: items,
		Info:       metadataInfo,
	}, nil
}

func extractPackage(
	ctx context.Context,
	path string,
	outputDir string,
	sourceDir string,
	partitions []string,
	concurrency int,
	onProgress func(ExtractionProgress),
) error {
	if len(partitions) == 0 {
		return fmt.Errorf("请至少勾选一个分区")
	}
	if outputDir == "" {
		return fmt.Errorf("请选择保存目录")
	}
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("保存目录无效：%w", err)
	}
	if err := os.MkdirAll(absOutput, 0o755); err != nil {
		return fmt.Errorf("无法创建保存目录：%w", err)
	}

	source, err := openPackageSource(ctx, path)
	if err != nil {
		return fmt.Errorf("重新打开 Payload 失败：%w", err)
	}
	defer source.Close()
	pl := source.payload

	selected := make(map[string]bool, len(partitions))
	for _, name := range partitions {
		selected[name] = true
	}
	totalOps := 0
	var totalBytes uint64
	for _, p := range pl.Partitions() {
		if selected[p.Name] {
			totalOps += p.TotalOperations
			totalBytes += p.Size
		}
	}
	if totalOps == 0 {
		totalOps = len(partitions)
	}

	completed := make(map[string]int, len(partitions))
	completedBytes := make(map[string]uint64, len(partitions))
	var progressMu sync.Mutex
	progress := func(ev payload.ProgressEvent) {
		progressMu.Lock()
		completed[ev.Partition] = ev.CompletedOps
		completedBytes[ev.Partition] = ev.BytesDone
		overall := 0
		for _, n := range completed {
			overall += n
		}
		if overall > totalOps {
			overall = totalOps
		}
		var overallBytes uint64
		for _, n := range completedBytes {
			overallBytes += n
		}
		if totalBytes > 0 && overallBytes > totalBytes {
			overallBytes = totalBytes
		}
		update := ExtractionProgress{
			Partition:      ev.Partition,
			CompletedOps:   ev.CompletedOps,
			PartitionOps:   ev.TotalOps,
			OverallDone:    overall,
			OverallTotal:   totalOps,
			BytesDone:      ev.BytesDone,
			BytesTotal:     ev.BytesTotal,
			OverallBytes:   overallBytes,
			OverallSize:    totalBytes,
			Stage:          ev.Stage,
			PartitionDone:  ev.Done,
			PartitionError: ev.Err,
		}
		progressMu.Unlock()
		if onProgress != nil {
			onProgress(update)
		}
	}

	if concurrency < 1 {
		concurrency = 1
	}
	return pl.Extract(ctx, payload.ExtractOptions{
		OutputDir:   absOutput,
		Partitions:  partitions,
		Concurrency: concurrency,
		SourceDir:   sourceDir,
		OnProgress:  progress,
	})
}
