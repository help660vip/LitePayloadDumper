package main

import (
	"context"

	"github.com/help660vip/LitePayloadDumper/core"
)

type PartitionItem = core.PartitionItem
type PackageDetails = core.PackageDetails
type ExtractionProgress = core.ExtractionProgress
type InspectionProgress = core.InspectionProgress
type DeviceInfo = core.DeviceInfo
type packageMode = core.PackageMode

const (
	packageModePayload = core.PackageModePayload
	packageModeZIP     = core.PackageModeZIP
	packageModeTGZ     = core.PackageModeTGZ
)

func inspectPackage(path string) (*PackageDetails, error) {
	return core.InspectPackage(path)
}

func inspectPackageContext(ctx context.Context, input string, onProgress func(InspectionProgress)) (*PackageDetails, error) {
	return core.InspectPackageContext(ctx, input, onProgress)
}

func extractPackage(ctx context.Context, path, outputDir, sourceDir string, partitions []string, concurrency int, onProgress func(ExtractionProgress)) error {
	return core.ExtractPackage(ctx, path, outputDir, sourceDir, partitions, concurrency, onProgress)
}

func extractPackageWithItems(ctx context.Context, path, outputDir, sourceDir string, partitions []string, archiveItems []PartitionItem, concurrency int, onProgress func(ExtractionProgress)) error {
	return core.ExtractPackageWithItems(ctx, path, outputDir, sourceDir, partitions, archiveItems, concurrency, onProgress)
}

func isRemoteInput(input string) bool {
	return core.IsRemoteInput(input)
}

func isTGZInput(input string) bool {
	return core.IsTGZInput(input)
}
