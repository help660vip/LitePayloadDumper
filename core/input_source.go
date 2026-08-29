package core

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/ssut/payload-dumper-go/payload"
)

type packageSource struct {
	mode      packageMode
	payload   *payload.Payload
	zipReader *zip.Reader
	size      int64
	remote    *httpReaderAt
	zipCloser *zip.ReadCloser
}

type packageMode string

const (
	packageModePayload packageMode = "payload"
	packageModeZIP     packageMode = "zip"
	packageModeTGZ     packageMode = "tgz"
)

func (source *packageSource) Close() {
	if source.payload != nil {
		source.payload.Close()
	}
	if source.zipCloser != nil {
		source.zipCloser.Close()
	}
	if source.remote != nil {
		source.remote.Close()
	}
}

func isRemoteInput(input string) bool {
	parsed, err := url.Parse(strings.TrimSpace(input))
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func openPackageSource(ctx context.Context, input string) (*packageSource, error) {
	input = strings.TrimSpace(input)
	if isRemoteInput(input) {
		return openRemotePackageSource(ctx, input)
	}
	stat, err := os.Stat(input)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件：%w", err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("请选择 OTA ZIP 或 payload.bin 文件")
	}
	file, err := os.Open(input)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件头：%w", err)
	}
	var magic [4]byte
	_, magicErr := io.ReadFull(file, magic[:])
	file.Close()
	if magicErr != nil {
		return nil, fmt.Errorf("文件过小或已损坏：%w", magicErr)
	}
	if string(magic[:]) == "CrAU" {
		pl, err := payload.Open(input)
		if err != nil {
			return nil, fmt.Errorf("不是有效的 payload.bin：%w", err)
		}
		return &packageSource{mode: packageModePayload, payload: pl, size: stat.Size()}, nil
	}
	if magic[0] == 'P' && magic[1] == 'K' {
		zr, err := zip.OpenReader(input)
		if err != nil {
			return nil, fmt.Errorf("读取 ZIP 目录失败：%w", err)
		}
		source := &packageSource{mode: packageModeZIP, size: stat.Size(), zipCloser: zr, zipReader: &zr.Reader}
		if findPayloadEntry(&zr.Reader) == nil {
			return source, nil
		}
		pl, err := payload.Open(input)
		if err != nil {
			zr.Close()
			return nil, fmt.Errorf("读取 ZIP 中的 payload.bin 失败：%w", err)
		}
		source.mode = packageModePayload
		source.payload = pl
		return source, nil
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return &packageSource{mode: packageModeTGZ, size: stat.Size()}, nil
	}
	return nil, fmt.Errorf("不是有效的 OTA ZIP、线刷 ZIP、TGZ 或 payload.bin")
}

func openRemotePackageSource(ctx context.Context, address string) (*packageSource, error) {
	reader, err := newHTTPReaderAt(ctx, address)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*packageSource, error) {
		reader.Close()
		return nil, err
	}
	var magic [4]byte
	if _, err := reader.ReadAt(magic[:], 0); err != nil {
		return fail(fmt.Errorf("读取在线固件头失败：%w", err))
	}
	if string(magic[:]) == "CrAU" {
		pl, err := payload.NewFromReaderAt(reader, reader.Size())
		if err != nil {
			return fail(fmt.Errorf("解析在线 payload.bin 失败：%w", err))
		}
		return &packageSource{mode: packageModePayload, payload: pl, size: reader.Size(), remote: reader}, nil
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return fail(fmt.Errorf("远程 TGZ 需要先建立一次本地临时缓存后解析"))
	}
	if magic[0] != 'P' || magic[1] != 'K' {
		return fail(fmt.Errorf("在线文件不是有效的 OTA ZIP 或 payload.bin"))
	}
	zr, err := zip.NewReader(reader, reader.Size())
	if err != nil {
		return fail(fmt.Errorf("读取在线 ZIP 目录失败：%w", err))
	}
	entry := findPayloadEntry(zr)
	if entry == nil {
		return &packageSource{mode: packageModeZIP, zipReader: zr, size: reader.Size(), remote: reader}, nil
	}
	if entry.Method != zip.Store {
		return fail(fmt.Errorf("在线 OTA 中的 payload.bin 使用了 ZIP 二次压缩，无法只下载所选分区；程序不会自动下载整个固件，请改用本地文件"))
	}
	if entry.UncompressedSize64 > math.MaxInt64 {
		return fail(fmt.Errorf("payload.bin 尺寸超出支持范围"))
	}
	offset, err := entry.DataOffset()
	if err != nil {
		return fail(fmt.Errorf("定位在线 payload.bin 失败：%w", err))
	}
	length := int64(entry.UncompressedSize64)
	section := io.NewSectionReader(reader, offset, length)
	pl, err := payload.NewFromReaderAt(section, length)
	if err != nil {
		return fail(fmt.Errorf("解析在线 payload.bin 清单失败：%w", err))
	}
	return &packageSource{mode: packageModePayload, payload: pl, zipReader: zr, size: reader.Size(), remote: reader}, nil
}

func isTGZInput(input string) bool {
	name := strings.ToLower(strings.TrimSpace(input))
	if parsed, err := url.Parse(name); err == nil && parsed.Path != "" {
		name = parsed.Path
	}
	return strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar.gz")
}

func findPayloadEntry(zr *zip.Reader) *zip.File {
	var fallback *zip.File
	for _, entry := range zr.File {
		if entry.UncompressedSize64 == 0 {
			continue
		}
		if entry.Name == "payload.bin" {
			return entry
		}
		if fallback == nil && path.Base(strings.ReplaceAll(entry.Name, "\\", "/")) == "payload.bin" {
			fallback = entry
		}
	}
	return fallback
}
