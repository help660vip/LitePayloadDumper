package payload

import (
	"context"
	"io"
)

const cancellationIOChunk = 256 << 10

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if len(buffer) > cancellationIOChunk {
		buffer = buffer[:cancellationIOChunk]
	}
	return reader.r.Read(buffer)
}

type contextProgressWriter struct {
	ctx     context.Context
	w       io.Writer
	onWrite func(int64)
}

func (writer *contextProgressWriter) Write(buffer []byte) (int, error) {
	total := 0
	for len(buffer) > 0 {
		if err := writer.ctx.Err(); err != nil {
			return total, err
		}
		chunk := buffer
		if len(chunk) > cancellationIOChunk {
			chunk = chunk[:cancellationIOChunk]
		}
		n, err := writer.w.Write(chunk)
		total += n
		if n > 0 && writer.onWrite != nil {
			writer.onWrite(int64(n))
		}
		if err != nil {
			return total, err
		}
		if n != len(chunk) {
			return total, io.ErrShortWrite
		}
		buffer = buffer[n:]
	}
	return total, nil
}
