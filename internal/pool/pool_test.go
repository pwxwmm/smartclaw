package pool

import (
	"bytes"
	"encoding/json"
	"testing"
)

type benchPayload struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

var payload100 = benchPayload{ID: 42, Message: "hello world from the streaming path", Detail: "some extra detail that makes this a bit longer than usual"}
var payload4K benchPayload

func init() {
	buf := make([]byte, 4000)
	for i := range buf {
		buf[i] = byte('a' + i%26)
	}
	payload4K = benchPayload{ID: 99, Message: string(buf), Detail: string(buf[:1000])}
}

func BenchmarkBufferPooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := GetBuffer()
		buf.WriteString("hello world this is a streaming response")
		buf.WriteByte('\n')
		_ = buf.String()
		PutBuffer(buf)
	}
}

func BenchmarkBufferUnpooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		buf.WriteString("hello world this is a streaming response")
		buf.WriteByte('\n')
		_ = buf.String()
	}
}

func BenchmarkByteSlicePooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sl := GetByteSlice(4096)
		sl = append(sl, "some data to write into the slice"...)
		_ = len(sl)
		PutByteSlice(sl)
	}
}

func BenchmarkByteSliceUnpooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sl := make([]byte, 0, 4096)
		sl = append(sl, "some data to write into the slice"...)
		_ = len(sl)
	}
}

func BenchmarkJSONEncoderPooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pe := GetJSONEncoder(nil)
		_ = pe.Encode(payload100)
		_ = pe.Bytes()
		PutJSONEncoder(pe)
	}
}

func BenchmarkJSONEncoderUnpooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		_ = enc.Encode(payload100)
	}
}

func BenchmarkJSONMarshalPooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pe := GetJSONEncoder(nil)
		_ = pe.Encode(payload100)
		_ = len(pe.Bytes())
		PutJSONEncoder(pe)
	}
}

func BenchmarkJSONMarshalUnpooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(payload100)
	}
}

func BenchmarkJSONMarshal4KPooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pe := GetJSONEncoder(nil)
		_ = pe.Encode(payload4K)
		_ = len(pe.Bytes())
		PutJSONEncoder(pe)
	}
}

func BenchmarkJSONMarshal4KUnpooled(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(payload4K)
	}
}

func BenchmarkByteSlicePooled16K(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sl := GetByteSlice(16384)
		sl = append(sl, "some data"...)
		_ = len(sl)
		PutByteSlice(sl)
	}
}

func BenchmarkByteSliceUnpooled16K(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sl := make([]byte, 0, 16384)
		sl = append(sl, "some data"...)
		_ = len(sl)
	}
}

func TestGetBufferReturnsResetBuffer(t *testing.T) {
	buf := GetBuffer()
	buf.WriteString("hello")
	if buf.Len() != 5 {
		t.Fatalf("expected len 5, got %d", buf.Len())
	}
	PutBuffer(buf)

	buf2 := GetBuffer()
	if buf2.Len() != 0 {
		t.Fatalf("expected reset buffer with len 0, got %d", buf2.Len())
	}
	PutBuffer(buf2)
}

func TestGetByteSliceReturnsEmptySlice(t *testing.T) {
	sl := GetByteSlice(4096)
	if len(sl) != 0 {
		t.Fatalf("expected len 0, got %d", len(sl))
	}
	if cap(sl) < 4096 {
		t.Fatalf("expected cap >= 4096, got %d", cap(sl))
	}
	PutByteSlice(sl)
}

func TestGetByteSliceOversizedHint(t *testing.T) {
	sl := GetByteSlice(100000)
	if cap(sl) < 100000 {
		t.Fatalf("expected cap >= 100000, got %d", cap(sl))
	}
}

func TestPutByteSliceDiscardsNonClass(t *testing.T) {
	sl := make([]byte, 0, 5000)
	PutByteSlice(sl)
}

func TestPooledEncoderBytes(t *testing.T) {
	pe := GetJSONEncoder(nil)
	if err := pe.Encode(payload100); err != nil {
		t.Fatal(err)
	}
	b := pe.Bytes()
	if len(b) == 0 {
		t.Fatal("expected non-empty bytes")
	}
	if b[len(b)-1] == '\n' {
		t.Fatal("Bytes() should strip trailing newline")
	}
	PutJSONEncoder(pe)
}

func TestPooledEncoderFlush(t *testing.T) {
	var buf bytes.Buffer
	pe := GetJSONEncoder(&buf)
	if err := pe.Encode(payload100); err != nil {
		t.Fatal(err)
	}
	if err := pe.Flush(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected data flushed to writer")
	}
	PutJSONEncoder(pe)
}

func TestPutBufferDiscardsOversized(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, maxBufferSize+1))
	PutBuffer(buf)
}

func TestPutJSONEncoderDiscardsOversized(t *testing.T) {
	pe := GetJSONEncoder(nil)
	pe.buf.Grow(maxBufferSize + 1)
	PutJSONEncoder(pe)
}
