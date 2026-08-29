package core

import "context"

type PackageMode = packageMode

const (
	PackageModePayload = packageModePayload
	PackageModeZIP     = packageModeZIP
	PackageModeTGZ     = packageModeTGZ
)

func InspectPackage(path string) (*PackageDetails, error) {
	return inspectPackage(path)
}

func InspectPackageContext(ctx context.Context, input string, onProgress func(InspectionProgress)) (*PackageDetails, error) {
	return inspectPackageContext(ctx, input, onProgress)
}

func ExtractPackage(ctx context.Context, path, outputDir, sourceDir string, partitions []string, concurrency int, onProgress func(ExtractionProgress)) error {
	return extractPackage(ctx, path, outputDir, sourceDir, partitions, concurrency, onProgress)
}

func ExtractPackageWithItems(ctx context.Context, path, outputDir, sourceDir string, partitions []string, archiveItems []PartitionItem, concurrency int, onProgress func(ExtractionProgress)) error {
	return extractPackageWithItems(ctx, path, outputDir, sourceDir, partitions, archiveItems, concurrency, onProgress)
}

func IsRemoteInput(input string) bool {
	return isRemoteInput(input)
}

func IsTGZInput(input string) bool {
	return isTGZInput(input)
}

func ApplyArchiveFilenameMetadata(info *DeviceInfo, input string) {
	applyArchiveFilenameMetadata(info, input)
}
