package image

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"exactclone/internal/clone"
)

// diskData simulates a disk: some random data followed by empty space.
func diskData(t *testing.T, random, zeros int) []byte {
	t.Helper()
	b := make([]byte, random+zeros)
	rand.Read(b[:random])
	return b
}

func fileOf(t *testing.T, data []byte) *os.File {
	t.Helper()
	p := filepath.Join(t.TempDir(), "disk.raw")
	os.WriteFile(p, data, 0o600)
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func makeImage(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "disk"+Extension)
	sum, err := CreateFile(context.Background(), fileOf(t, data), int64(len(data)), Info{SourceDisk: "Test USB (sdz)"}, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(data)
	if !bytes.Equal(sum, want[:]) {
		t.Fatal("Create returned a wrong checksum")
	}
	return path
}

// restore writes an image to dst the way the restore job does: copy, then Check.
func restore(t *testing.T, path string, dst *os.File) ([]byte, error) {
	t.Helper()
	r, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	sum, err := clone.Copy(context.Background(), r, dst, r.Size(), clone.Options{HeadLast: true}, nil)
	if err != nil {
		return nil, err
	}
	return sum, r.Check(sum)
}

func TestRoundTrip(t *testing.T) {
	for _, c := range []struct{ random, zeros int }{{0, 0}, {1, 0}, {3 << 20, 40 << 20}, {17 << 20, 0}} {
		data := diskData(t, c.random, c.zeros)
		path := makeImage(t, data)

		info, err := ReadInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(data)
		if info.SizeBytes != int64(len(data)) || info.SHA256 != hex.EncodeToString(want[:]) ||
			info.SourceDisk != "Test USB (sdz)" || info.CreatedAt.IsZero() || info.Version != 1 {
			t.Fatalf("info = %+v", info)
		}
		if err := VerifyFile(context.Background(), path, nil); err != nil {
			t.Fatalf("verify: %v", err)
		}

		dst := fileOf(t, make([]byte, len(data)+4096)) // destination larger than the image
		sum, err := restore(t, path, dst)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sum, want[:]) {
			t.Fatal("restored data has a different checksum")
		}
		got, _ := os.ReadFile(dst.Name())
		if !bytes.Equal(got[:len(data)], data) {
			t.Fatal("restored data differs")
		}
	}
}

func TestEmptySpaceIsCompressed(t *testing.T) {
	path := makeImage(t, diskData(t, 1<<20, 63<<20)) // 64 MiB "disk", 1 MiB used
	fi, _ := os.Stat(path)
	if fi.Size() > 2<<20 {
		t.Fatalf("image of a mostly empty 64 MiB disk is %d bytes", fi.Size())
	}
}

func TestProgressCountsDiskBytes(t *testing.T) {
	data := diskData(t, 5<<20, 20<<20)
	var n int64
	path := filepath.Join(t.TempDir(), "p"+Extension)
	if _, err := CreateFile(context.Background(), fileOf(t, data), int64(len(data)), Info{}, path, func(k int64) { n += k }); err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) {
		t.Fatalf("progress = %d, want %d", n, len(data))
	}
}

func TestNotAnImage(t *testing.T) {
	for name, content := range map[string][]byte{
		"text":     []byte("hello"),
		"empty":    nil,
		"zstdOnly": {0x28, 0xB5, 0x2F, 0xFD, 0, 0, 0, 0},
	} {
		p := filepath.Join(t.TempDir(), name+Extension)
		os.WriteFile(p, content, 0o644)
		if _, err := ReadInfo(p); !errors.Is(err, ErrNotImage) {
			t.Errorf("%s: err = %v, want ErrNotImage", name, err)
		}
		if _, err := Open(p); !errors.Is(err, ErrNotImage) {
			t.Errorf("%s: Open err = %v", name, err)
		}
	}
}

func TestIncompleteImage(t *testing.T) {
	path := makeImage(t, diskData(t, 2<<20, 0))
	fi, _ := os.Stat(path)
	os.Truncate(path, fi.Size()-100) // trailer cut
	if _, err := ReadInfo(path); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("err = %v, want ErrIncomplete", err)
	}
}

func TestCorruptDataDetected(t *testing.T) {
	data := diskData(t, 4<<20, 0)
	path := makeImage(t, data)
	f, _ := os.OpenFile(path, os.O_RDWR, 0)
	fi, _ := f.Stat()
	f.WriteAt([]byte{0xFF, 0x00, 0xFF, 0x00}, fi.Size()/2) // inside the compressed data
	f.Close()

	if err := VerifyFile(context.Background(), path, nil); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("VerifyFile err = %v, want ErrCorrupt", err)
	}
	_, err := restore(t, path, fileOf(t, make([]byte, len(data))))
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("restore err = %v, want ErrCorrupt", err)
	}
}

func TestChecksumMismatchDetected(t *testing.T) {
	path := makeImage(t, diskData(t, 1<<20, 0))
	info, _ := ReadInfo(path)
	b, _ := os.ReadFile(path)
	b = bytes.Replace(b, []byte(info.SHA256), []byte(strings.Repeat("0", 64)), 1)
	os.WriteFile(path, b, 0o644)
	if err := VerifyFile(context.Background(), path, nil); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("err = %v, want ErrCorrupt", err)
	}
}

func TestCreateCancelRemovesPartial(t *testing.T) {
	data := diskData(t, 64<<20, 0)
	path := filepath.Join(t.TempDir(), "c"+Extension)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := CreateFile(ctx, fileOf(t, data), int64(len(data)), Info{}, path, func(int64) { cancel() })
	if !errors.Is(err, clone.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial image not removed")
	}
}

func TestCreateDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x"+Extension)
	os.WriteFile(path, []byte("keep"), 0o644)
	if _, err := CreateFile(context.Background(), fileOf(t, []byte{1}), 1, Info{}, path, nil); err == nil {
		t.Fatal("expected an error for an existing file")
	}
	if b, _ := os.ReadFile(path); string(b) != "keep" {
		t.Fatal("existing file modified")
	}
}

// Images created before the rename to ExactClone carry "diskclone-image":
// the identifier must never change, or they could not be restored anymore.
func TestFormatIdentifierUnchanged(t *testing.T) {
	path := makeImage(t, diskData(t, 1024, 0))
	head := make([]byte, 200)
	f, _ := os.Open(path)
	defer f.Close()
	f.Read(head)
	if !bytes.Contains(head, []byte(`"format":"diskclone-image"`)) {
		t.Fatalf("header = %q", head)
	}
}
