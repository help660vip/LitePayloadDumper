package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const remoteBlockSize int64 = 4 << 20
const remoteCacheBlocks = 12

type remoteBlock struct {
	ready chan struct{}
	data  []byte
	err   error
	used  uint64
}

type httpReaderAt struct {
	ctx       context.Context
	url       string
	size      int64
	client    *http.Client
	transport *http.Transport

	mu      sync.Mutex
	blocks  map[int64]*remoteBlock
	useTick uint64
}

func newHTTPReaderAt(ctx context.Context, address string) (*httpReaderAt, error) {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          24,
		MaxIdleConnsPerHost:   12,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	reader := &httpReaderAt{
		ctx:       ctx,
		url:       address,
		client:    &http.Client{Transport: transport},
		transport: transport,
		blocks:    make(map[int64]*remoteBlock),
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("在线链接无效：%w", err)
	}
	request.Header.Set("Range", "bytes=0-0")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "PayloadExtractor/1.0")
	response, err := reader.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("无法连接在线固件：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("服务器不支持 HTTP Range（返回 %s），无法按需提取；请先下载到本地", response.Status)
	}
	size, err := parseContentRangeSize(response.Header.Get("Content-Range"))
	if err != nil || size <= 0 {
		return nil, fmt.Errorf("服务器返回的 Content-Range 无效：%q", response.Header.Get("Content-Range"))
	}
	reader.size = size
	return reader, nil
}

func parseContentRangeSize(header string) (int64, error) {
	slash := strings.LastIndexByte(header, '/')
	if slash < 0 || slash == len(header)-1 {
		return 0, fmt.Errorf("missing total size")
	}
	return strconv.ParseInt(strings.TrimSpace(header[slash+1:]), 10, 64)
}

func (reader *httpReaderAt) Size() int64 { return reader.size }

func (reader *httpReaderAt) Close() {
	if reader.transport != nil {
		reader.transport.CloseIdleConnections()
	}
}

func (reader *httpReaderAt) ReadAt(buffer []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	if offset >= reader.size {
		return 0, io.EOF
	}
	written := 0
	for written < len(buffer) && offset < reader.size {
		blockIndex := offset / remoteBlockSize
		block, err := reader.getBlock(blockIndex)
		if err != nil {
			return written, err
		}
		inside := int(offset - blockIndex*remoteBlockSize)
		if inside >= len(block) {
			break
		}
		count := copy(buffer[written:], block[inside:])
		written += count
		offset += int64(count)
	}
	if written < len(buffer) {
		return written, io.EOF
	}
	return written, nil
}

func (reader *httpReaderAt) getBlock(index int64) ([]byte, error) {
	reader.mu.Lock()
	reader.useTick++
	if existing, ok := reader.blocks[index]; ok {
		existing.used = reader.useTick
		ready := existing.ready
		reader.mu.Unlock()
		select {
		case <-ready:
			return existing.data, existing.err
		case <-reader.ctx.Done():
			return nil, reader.ctx.Err()
		}
	}
	entry := &remoteBlock{ready: make(chan struct{}), used: reader.useTick}
	reader.blocks[index] = entry
	reader.mu.Unlock()

	entry.data, entry.err = reader.fetchBlock(index)
	reader.mu.Lock()
	close(entry.ready)
	reader.evictLocked(index)
	reader.mu.Unlock()
	return entry.data, entry.err
}

func (reader *httpReaderAt) fetchBlock(index int64) ([]byte, error) {
	start := index * remoteBlockSize
	end := start + remoteBlockSize - 1
	if end >= reader.size {
		end = reader.size - 1
	}
	request, err := http.NewRequestWithContext(reader.ctx, http.MethodGet, reader.url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "PayloadExtractor/1.0")
	response, err := reader.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("Range %d-%d 返回 %s", start, end, response.Status)
	}
	expected := end - start + 1
	data, err := io.ReadAll(io.LimitReader(response.Body, expected+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != expected {
		return nil, fmt.Errorf("Range %d-%d 长度错误：得到 %d，预期 %d", start, end, len(data), expected)
	}
	return data, nil
}

func (reader *httpReaderAt) evictLocked(keep int64) {
	for len(reader.blocks) > remoteCacheBlocks {
		var oldestIndex int64
		var oldestUsed uint64 = ^uint64(0)
		found := false
		for index, entry := range reader.blocks {
			if index == keep {
				continue
			}
			select {
			case <-entry.ready:
				if entry.used < oldestUsed {
					oldestIndex, oldestUsed, found = index, entry.used, true
				}
			default:
			}
		}
		if !found {
			return
		}
		delete(reader.blocks, oldestIndex)
	}
}
