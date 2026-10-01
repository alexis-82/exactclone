//go:build linux

package archive

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestFIFOSkippedWithoutBlocking(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]int{"f": 1})
	if err := syscall.Mkfifo(filepath.Join(src, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "a.tar.zst")
	done := make(chan []Warning, 1)
	go func() {
		w, err := CreateFile(context.Background(), []Root{{Name: "p", Path: src}}, Manifest{}, out, nil)
		if err != nil {
			t.Error(err)
		}
		done <- w
	}()
	select {
	case w := <-done:
		if len(w) != 1 || filepath.Base(w[0].Path) != "pipe" {
			t.Fatalf("warnings = %v, want one for the FIFO", w)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Create blocked on a FIFO")
	}
}
