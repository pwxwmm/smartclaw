// Package pool provides reusable sync.Pool definitions for hot-path
// allocations in streaming, JSON encoding, and byte-slice operations.
package pool

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

const maxBufferSize = 1 << 20 // 1MB — discard buffers larger than this

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GetBuffer returns a *bytes.Buffer from the pool, ready for use.
func GetBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer returns a *bytes.Buffer to the pool.
// Buffers with capacity > 1MB are discarded to prevent unbounded growth.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	if buf.Cap() > maxBufferSize {
		return
	}
	bufferPool.Put(buf)
}

var (
	byteSlice4K  sync.Pool
	byteSlice16K sync.Pool
	byteSlice64K sync.Pool
)

func init() {
	byteSlice4K.New = func() any { b := make([]byte, 0, 4096); return &b }
	byteSlice16K.New = func() any { b := make([]byte, 0, 16384); return &b }
	byteSlice64K.New = func() any { b := make([]byte, 0, 65536); return &b }
}

// sizeClasses lists the supported byte-slice capacities in ascending order.
var sizeClasses = []int{4096, 16384, 65536}

// pickSizeClass returns the smallest size class >= sizeHint.
// Returns 0 if sizeHint exceeds the largest class.
func pickSizeClass(sizeHint int) int {
	for _, sc := range sizeClasses {
		if sc >= sizeHint {
			return sc
		}
	}
	return 0
}

// GetByteSlice returns a []byte from the pool with len == 0 and cap >= sizeHint.
// If sizeHint exceeds the largest size class (64K), a fresh slice is allocated.
func GetByteSlice(sizeHint int) []byte {
	class := pickSizeClass(sizeHint)
	if class == 0 {
		return make([]byte, 0, sizeHint)
	}
	var p *sync.Pool
	switch class {
	case 4096:
		p = &byteSlice4K
	case 16384:
		p = &byteSlice16K
	default:
		p = &byteSlice64K
	}
	b := p.Get().(*[]byte)
	return (*b)[:0]
}

// PutByteSlice returns a []byte to the pool.
// Slices whose capacity does not exactly match a size class are discarded
// to prevent returning grown slices to the wrong class.
func PutByteSlice(b []byte) {
	if b == nil {
		return
	}
	c := cap(b)
	b = b[:0]
	switch c {
	case 4096:
		byteSlice4K.Put(&b)
	case 16384:
		byteSlice16K.Put(&b)
	case 65536:
		byteSlice64K.Put(&b)
	}
}

type PooledEncoder struct {
	buf    *bytes.Buffer
	enc    *json.Encoder
	target io.Writer
}

var encoderPool = sync.Pool{
	New: func() any {
		buf := new(bytes.Buffer)
		return &PooledEncoder{
			buf: buf,
			enc: json.NewEncoder(buf),
		}
	},
}

// GetJSONEncoder returns a PooledEncoder from the pool.
// The target writer w receives encoded data when Flush is called.
func GetJSONEncoder(w io.Writer) *PooledEncoder {
	pe := encoderPool.Get().(*PooledEncoder)
	pe.buf.Reset()
	pe.target = w
	return pe
}

// Encode marshals v as JSON into the internal buffer.
// It appends a trailing newline (json.Encoder behavior).
func (pe *PooledEncoder) Encode(v any) error {
	return pe.enc.Encode(v)
}

// Bytes returns the encoded JSON bytes without the trailing newline
// added by Encode. The returned slice is valid only until the next
// call to PutJSONEncoder.
func (pe *PooledEncoder) Bytes() []byte {
	b := pe.buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b
}

// Flush writes the buffer contents to the target writer and resets
// the buffer so the encoder can be reused for another object.
func (pe *PooledEncoder) Flush() error {
	if pe.target == nil {
		return nil
	}
	_, err := pe.target.Write(pe.buf.Bytes())
	pe.buf.Reset()
	return err
}

// Len returns the number of bytes in the buffer.
func (pe *PooledEncoder) Len() int {
	return pe.buf.Len()
}

// WriteTo writes the buffer contents to w and resets the buffer.
func (pe *PooledEncoder) WriteTo(w io.Writer) (int64, error) {
	n, err := pe.buf.WriteTo(w)
	return n, err
}

// PutJSONEncoder returns a PooledEncoder to the pool.
// Encoders whose internal buffer exceeds 1MB are discarded.
func PutJSONEncoder(pe *PooledEncoder) {
	if pe == nil {
		return
	}
	pe.buf.Reset()
	pe.target = nil
	if pe.buf.Cap() > maxBufferSize {
		return
	}
	encoderPool.Put(pe)
}
