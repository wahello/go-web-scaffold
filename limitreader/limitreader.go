package limitreader

import (
	"fmt"
	"io"
)

// NewReader factory
func NewReader(r io.Reader, limit int) *Reader {
	return &Reader{
		r:             r,
		left:          limit,
		originalLimit: limit,
	}
}

// Reader io.Reader that limit content length
type Reader struct {
	r             io.Reader
	left          int
	originalLimit int
}

// Read implements io.Reader
func (lr *Reader) Read(p []byte) (n int, err error) {
	if lr.left < 0 {
		return 0, fmt.Errorf("stream bigger than threshold %d bytes", lr.originalLimit)
	}
	if len(p) > lr.left {
		p = p[0:lr.left]
	}
	n, err = lr.r.Read(p)
	lr.left -= n
	return
}

// ReadCloser io.ReadCloser that limit content length
type ReadCloser struct {
	io.ReadCloser
	*Reader
}

// NewReadCloser factory
func NewReadCloser(r io.ReadCloser, left int) *ReadCloser {
	return &ReadCloser{
		ReadCloser: r,
		Reader:     NewReader(r, left),
	}
}

// Read implements io.ReadCloser
func (lr *ReadCloser) Read(p []byte) (n int, err error) {
	return lr.Reader.Read(p)
}
