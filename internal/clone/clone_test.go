package clone

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// fileDev adapts an *os.File to Verifiable for tests.
type fileDev struct {
	*os.File
	drops atomic.Int32
}

func (f *fileDev) DropCache() error { f.drops.Add(1); return nil }

func makeFile(t *testing.T, name string, data []byte) *os.File {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func randomData(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCopySizes(t *testing.T) {
	sizes := []int{0, 1, 16<<20 + 1, 100 << 20}
	for _, size := range sizes {
		for _, headLast := range []bool{false, true} {
			data := randomData(t, size)
			src := makeFile(t, "src", data)
			dst := makeFile(t, "dst", make([]byte, size))
			var written atomic.Int64
			sum, err := Copy(context.Background(), src, dst, int64(size), Options{HeadLast: headLast},
				func(n int64) { written.Add(n) })
			if err != nil {
				t.Fatalf("size %d headLast %v: %v", size, headLast, err)
			}
			want := sha256.Sum256(data)
			if !bytes.Equal(sum, want[:]) {
				t.Fatalf("size %d headLast %v: wrong hash", size, headLast)
			}
			got, _ := os.ReadFile(dst.Name())
			if !bytes.Equal(got, data) {
				t.Fatalf("size %d headLast %v: content differs", size, headLast)
			}
			if written.Load() != int64(size) {
				t.Fatalf("size %d: progress %d", size, written.Load())
			}
		}
	}
}

// headOrderWriter records whether offset 0 was written last.
type headOrderWriter struct {
	*os.File
	lastOff int64
}

func (w *headOrderWriter) WriteAt(p []byte, off int64) (int, error) {
	w.lastOff = off
	return w.File.WriteAt(p, off)
}

func TestHeadLastWritesHeadAtTheEnd(t *testing.T) {
	size := 40 << 20
	data := randomData(t, size)
	src := makeFile(t, "src", data)
	dst := &headOrderWriter{File: makeFile(t, "dst", make([]byte, size)), lastOff: -1}
	if _, err := Copy(context.Background(), src, dst, int64(size), Options{HeadLast: true}, nil); err != nil {
		t.Fatal(err)
	}
	if dst.lastOff != 0 {
		t.Fatalf("last write at %d, want 0", dst.lastOff)
	}
}

// cancelingReader cancels the context after `after` reads and counts reads.
type cancelingReader struct {
	*os.File
	after  int32
	reads  atomic.Int32
	cancel context.CancelFunc
}

func (r *cancelingReader) ReadAt(p []byte, off int64) (int, error) {
	if r.reads.Add(1) == r.after {
		r.cancel()
	}
	return r.File.ReadAt(p, off)
}

func TestCopyCancel(t *testing.T) {
	size := 160 << 20 // 10 blocks
	src := makeFile(t, "src", randomData(t, size))
	dst := makeFile(t, "dst", make([]byte, size))
	ctx, cancel := context.WithCancel(context.Background())
	r := &cancelingReader{File: src, after: 3, cancel: cancel}
	_, err := Copy(ctx, r, dst, int64(size), Options{}, nil)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if got := r.reads.Load(); got > 3 {
		t.Fatalf("%d reads, want reading to stop at the canceled block", got)
	}
}

type failingReader struct{ failAt int64 }

func (r failingReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.failAt {
		return 0, errors.New("disk unplugged")
	}
	return len(p), nil
}

type failingWriter struct{ failAt int64 }

func (w failingWriter) WriteAt(p []byte, off int64) (int, error) {
	if off >= w.failAt {
		return 0, errors.New("device full")
	}
	return len(p), nil
}

func TestCopyReadError(t *testing.T) {
	_, err := Copy(context.Background(), failingReader{failAt: 32 << 20}, failingWriter{failAt: 1 << 40}, 64<<20, Options{}, nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("disk unplugged")) {
		t.Fatalf("err = %v", err)
	}
}

func TestCopyWriteError(t *testing.T) {
	_, err := Copy(context.Background(), failingReader{failAt: 1 << 40}, failingWriter{failAt: 16 << 20}, 64<<20, Options{}, nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("device full")) {
		t.Fatalf("err = %v", err)
	}
}

func TestCopyShortSource(t *testing.T) {
	src := makeFile(t, "src", randomData(t, 1000))
	dst := makeFile(t, "dst", nil)
	if _, err := Copy(context.Background(), src, dst, 2000, Options{}, nil); err == nil {
		t.Fatal("expected error when source is smaller than size")
	}
}

func TestVerify(t *testing.T) {
	size := 20 << 20
	data := randomData(t, size)
	sum := sha256.Sum256(data)
	dst := &fileDev{File: makeFile(t, "dst", data)}

	if err := Verify(context.Background(), dst, int64(size), sum[:], nil); err != nil {
		t.Fatalf("verify of identical data: %v", err)
	}
	if dst.drops.Load() != 1 {
		t.Fatal("Verify must drop the cache before re-reading")
	}

	if _, err := dst.WriteAt([]byte{data[size/2] ^ 0xFF}, int64(size/2)); err != nil {
		t.Fatal(err)
	}
	if err := Verify(context.Background(), dst, int64(size), sum[:], nil); !errors.Is(err, ErrVerifyMismatch) {
		t.Fatalf("err = %v, want ErrVerifyMismatch", err)
	}
}

func TestVerifyCanceled(t *testing.T) {
	dst := &fileDev{File: makeFile(t, "dst", randomData(t, 1<<20))}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Verify(ctx, dst, 1<<20, nil, nil); !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestAlignedBuf(t *testing.T) {
	for i := 0; i < 10; i++ {
		b := AlignedBuf(5000 + i)
		if len(b) != 5000+i {
			t.Fatal("wrong length")
		}
		if addr := uintptr(unsafePointer(b)); addr%alignment != 0 {
			t.Fatalf("buffer not aligned: %x", addr)
		}
	}
}
