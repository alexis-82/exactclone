package main

// Behavior tests for the acceptance criteria of .claude/spec.md that can run
// without touching real disks: disks are simulated by regular files.

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"diskclone/internal/disk"
	"diskclone/internal/job"
	"diskclone/internal/privilege"
	"diskclone/internal/rawdev"
)

type events struct {
	mu       sync.Mutex
	done     []job.DoneEvent
	progress []job.ProgressEvent
	onProg   func()
}

func (e *events) emit(name string, data any) {
	e.mu.Lock()
	switch d := data.(type) {
	case job.DoneEvent:
		e.done = append(e.done, d)
	case job.ProgressEvent:
		e.progress = append(e.progress, d)
	}
	cb := e.onProg
	e.mu.Unlock()
	if name == job.EventProgress && cb != nil {
		cb()
	}
}

func (e *events) last() job.DoneEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.done[len(e.done)-1]
}

// fileDevice is a regular file used as a disk.
type fileDevice struct{ *os.File }

func (f fileDevice) Size() int64 {
	fi, _ := f.Stat()
	return fi.Size()
}
func (f fileDevice) DropCache() error { return nil }

// useFileDevices makes the jobs open disks as regular files and skips the
// OS-specific preparation (locking, offline, partition rescan).
func useFileDevices(t *testing.T) {
	t.Helper()
	o, pr, pw, fw := openDevice, prepareForRead, prepareForWrite, finishWrite
	openDevice = func(path string, write bool) (rawdev.Device, error) {
		flag := os.O_RDONLY
		if write {
			flag = os.O_RDWR
		}
		f, err := os.OpenFile(path, flag, 0)
		if err != nil {
			return nil, err
		}
		return fileDevice{f}, nil
	}
	prepareForRead = func(disk.Disk) (func(), error) { return func() {}, nil }
	prepareForWrite = func(disk.Disk, disk.Disk) (func(), error) { return func() {}, nil }
	finishWrite = func(disk.Disk) error { return nil }
	t.Cleanup(func() { openDevice, prepareForRead, prepareForWrite, finishWrite = o, pr, pw, fw })
}

// fakeDisk creates a "disk" file: random data followed by empty space.
func fakeDisk(t *testing.T, id string, random, zeros int) (disk.Disk, []byte) {
	t.Helper()
	data := make([]byte, random+zeros)
	rand.Read(data[:random])
	p := filepath.Join(t.TempDir(), id+".raw")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return disk.Disk{ID: id, Path: p, Model: "Fake " + id, SizeBytes: int64(len(data))}, data
}

func runJob(t *testing.T, ev *events, kind string, fn job.Func) job.DoneEvent {
	t.Helper()
	m := job.NewManager(ev.emit)
	if err := m.Start(kind, fn); err != nil {
		t.Fatal(err)
	}
	m.Wait()
	return ev.last()
}

func phases(ev *events) []string {
	var out []string
	for _, p := range ev.progress {
		if len(out) == 0 || out[len(out)-1] != p.Phase {
			out = append(out, p.Phase)
		}
	}
	return out
}

func wantNotices(t *testing.T, got []string, onWindows ...string) {
	t.Helper()
	want := []string(nil)
	if goruntime.GOOS == "windows" {
		want = onWindows
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("notices = %v, want %v", got, want)
	}
}

func readHead(t *testing.T, path string, n int) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b[:n]
}

// cancelOnFirstBytes cancels the job as soon as some bytes were processed.
func cancelOnFirstBytes(ev *events, m *job.Manager) {
	var once sync.Once
	ev.onProg = func() {
		ev.mu.Lock()
		last := ev.progress[len(ev.progress)-1]
		ev.mu.Unlock()
		if last.Done > 0 {
			once.Do(func() { go m.Cancel() })
		}
	}
}

// CA1: disk-to-disk clone with verification; the destination is larger.
func TestCA1CloneJob(t *testing.T) {
	useFileDevices(t)
	src, data := fakeDisk(t, "src", 6<<20, 18<<20)
	dst, _ := fakeDisk(t, "dst", 0, 32<<20)
	ev := &events{}
	done := runJob(t, ev, "clone", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runClone(ctx, r, src, dst, true)
	})
	if done.Status != job.StatusDone {
		t.Fatalf("job = %+v", done)
	}
	if fmt.Sprint(phases(ev)) != "[copy verify]" {
		t.Fatalf("phases = %v", phases(ev))
	}
	if !bytes.Equal(readHead(t, dst.Path, len(data)), data) {
		t.Fatal("destination differs from source")
	}
	wantNotices(t, done.Notices, NoticeCloneOffline)
}

// CA3: disk -> image -> another disk gives the same bytes; the image keeps
// size and source disk.
func TestCA3ImageAndRestore(t *testing.T) {
	useFileDevices(t)
	src, data := fakeDisk(t, "src", 5<<20, 27<<20)
	img := filepath.Join(t.TempDir(), "backup.img.zst")

	ev := &events{}
	done := runJob(t, ev, "image", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runImage(ctx, r, src, img, true)
	})
	if done.Status != job.StatusDone || len(done.Notices) != 0 {
		t.Fatalf("image job = %+v", done)
	}
	if fmt.Sprint(phases(ev)) != "[copy verify]" {
		t.Fatalf("phases = %v", phases(ev))
	}
	fi, _ := os.Stat(img)
	if fi.Size() >= int64(len(data)) {
		t.Fatalf("image (%d bytes) not compressed (disk %d bytes)", fi.Size(), len(data))
	}
	info, err := (&App{}).ReadImageInfo(img)
	if err != nil || info.SizeBytes != int64(len(data)) || !strings.Contains(info.SourceDisk, "Fake src") {
		t.Fatalf("info = %+v, %v", info, err)
	}

	dst, _ := fakeDisk(t, "dst", 0, 40<<20)
	ev2 := &events{}
	done = runJob(t, ev2, "restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestoreImage(ctx, r, img, dst, true)
	})
	if done.Status != job.StatusDone {
		t.Fatalf("restore job = %+v", done)
	}
	if fmt.Sprint(phases(ev2)) != "[copy verify]" {
		t.Fatalf("restore phases = %v", phases(ev2))
	}
	if !bytes.Equal(readHead(t, dst.Path, len(data)), data) {
		t.Fatal("restored disk differs from the source disk")
	}
	wantNotices(t, done.Notices, NoticeCloneOffline)
}

// CA3: a damaged image is refused with a clear code.
func TestCA3CorruptImageDetected(t *testing.T) {
	useFileDevices(t)
	src, data := fakeDisk(t, "src", 4<<20, 0)
	img := filepath.Join(t.TempDir(), "bad.img.zst")
	runJob(t, &events{}, "image", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runImage(ctx, r, src, img, false)
	})
	f, _ := os.OpenFile(img, os.O_RDWR, 0)
	fi, _ := f.Stat()
	f.WriteAt([]byte{0xAA, 0x55, 0xAA, 0x55}, fi.Size()/2)
	f.Close()

	dst, _ := fakeDisk(t, "dst", 0, len(data))
	done := runJob(t, &events{}, "restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestoreImage(ctx, r, img, dst, false)
	})
	if done.Status != job.StatusFailed || !strings.HasPrefix(done.Error, "image_corrupt") {
		t.Fatalf("job = %+v, want image_corrupt", done)
	}
	wantNotices(t, done.Notices, NoticeDestOffline)
}

// CA4: the image decompresses with standard tools into a raw copy of the disk.
func TestCA4StandardTools(t *testing.T) {
	useFileDevices(t)
	src, data := fakeDisk(t, "src", 3<<20, 5<<20)
	dir := t.TempDir()
	img := filepath.Join(dir, "std.img.zst")
	runJob(t, &events{}, "image", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runImage(ctx, r, src, img, false)
	})
	raw := filepath.Join(dir, "std.img")
	if sevenZip := find7zWithZstd(); sevenZip != "" {
		if out, err := exec.Command(sevenZip, "x", "-y", "-o"+dir, img).CombinedOutput(); err != nil {
			t.Fatalf("7z: %v\n%s", err, out)
		}
		t.Logf("decompressed with %s", sevenZip)
	} else if _, err := exec.LookPath("zstd"); err == nil {
		if out, err := exec.Command("zstd", "-d", "-q", "-f", img, "-o", raw).CombinedOutput(); err != nil {
			t.Fatalf("zstd -d: %v\n%s", err, out)
		}
		t.Log("decompressed with zstd")
	} else {
		t.Skip("no standard tool with zstd support installed (7-Zip-zstd or zstd)")
	}
	got, err := os.ReadFile(raw)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("decompressed image differs from the disk (%d vs %d bytes, %v)", len(got), len(data), err)
	}
}

// find7zWithZstd returns the first installed 7-Zip that can decode zstd
// (7-Zip-zstd: https://github.com/mcmilk/7-Zip-zstd/releases).
func find7zWithZstd() string {
	for _, c := range []string{"7z", "7zz", `C:\Program Files\7-Zip-Zstandard\7z.exe`, `C:\Program Files\7-Zip\7z.exe`} {
		if path, err := exec.LookPath(c); err == nil {
			out, err := exec.Command(path, "i").CombinedOutput()
			if err == nil && bytes.Contains(bytes.ToLower(out), []byte("zstd")) {
				return path
			}
		}
	}
	return ""
}

// CA7: canceling the image creation stops it quickly and deletes the partial file.
func TestCA7CancelImage(t *testing.T) {
	useFileDevices(t)
	src, _ := fakeDisk(t, "src", 160<<20, 0)
	img := filepath.Join(t.TempDir(), "partial.img.zst")
	ev := &events{}
	m := job.NewManager(ev.emit)
	cancelOnFirstBytes(ev, m)
	start := time.Now()
	m.Start("image", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runImage(ctx, r, src, img, false)
	})
	m.Wait()
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("cancel took %v", d)
	}
	if got := ev.last().Status; got != job.StatusCanceled {
		t.Fatalf("status = %s, want canceled", got)
	}
	if _, err := os.Stat(img); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial image not deleted")
	}
}

// CA7: canceling a restore is reported as canceled (destination incomplete,
// offline on Windows).
func TestCA7CancelRestore(t *testing.T) {
	useFileDevices(t)
	src, _ := fakeDisk(t, "src", 120<<20, 0)
	img := filepath.Join(t.TempDir(), "r.img.zst")
	runJob(t, &events{}, "image", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runImage(ctx, r, src, img, false)
	})
	dst, _ := fakeDisk(t, "dst", 0, 120<<20)
	ev := &events{}
	m := job.NewManager(ev.emit)
	cancelOnFirstBytes(ev, m)
	m.Start("restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestoreImage(ctx, r, img, dst, false)
	})
	m.Wait()
	done := ev.last()
	if done.Status != job.StatusCanceled {
		t.Fatalf("status = %s, want canceled", done.Status)
	}
	wantNotices(t, done.Notices, NoticeDestOffline)
}

func TestReadImageInfoRejectsForeignFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.img.zst")
	os.WriteFile(p, []byte("not an image"), 0o644)
	if _, err := (&App{}).ReadImageInfo(p); err == nil || !strings.HasPrefix(err.Error(), "not_diskclone_image") {
		t.Fatalf("err = %v", err)
	}
}

// CA8: without privileges the operations are refused up front with a clear
// code instead of failing during the copy.
func TestCA8RefusedWithoutPrivileges(t *testing.T) {
	if privilege.IsElevated() {
		t.Skip("running elevated")
	}
	a := &App{}
	if err := a.StartClone("x", "y", true); err == nil || err.Error() != "not_elevated" {
		t.Fatalf("StartClone err = %v, want not_elevated", err)
	}
	if err := a.StartImage("x", "out.img.zst", true); err == nil || err.Error() != "not_elevated" {
		t.Fatalf("StartImage err = %v, want not_elevated", err)
	}
	if err := a.StartRestoreImage("in.img.zst", "y", true); err == nil || err.Error() != "not_elevated" {
		t.Fatalf("StartRestoreImage err = %v, want not_elevated", err)
	}
}
