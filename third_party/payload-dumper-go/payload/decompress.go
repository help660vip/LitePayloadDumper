package payload

import (
	"compress/bzip2"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type readCloser struct {
	io.Reader
	closeFn func() error
}

func (rc readCloser) Close() error {
	if rc.closeFn != nil {
		return rc.closeFn()
	}
	return nil
}

func newXZReader(r io.Reader) (io.ReadCloser, error) {
	d, err := xz.NewReader(r)
	if err != nil {
		return nil, err
	}
	return readCloser{Reader: d}, nil
}

func newBzip2Reader(r io.Reader) io.ReadCloser {
	return readCloser{Reader: bzip2.NewReader(r)}
}

func newZstdReader(r io.Reader) (io.ReadCloser, error) {
	zr, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true))
	if err != nil {
		return nil, err
	}
	return readCloser{Reader: zr, closeFn: func() error {
		zr.Close()
		return nil
	}}, nil
}
