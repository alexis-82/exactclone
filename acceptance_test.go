package main

// Behavior tests for the acceptance criteria of .claude/spec.md that can run
// without touching real disks. Partitions are simulated by folders: a
// partition with a mount point is read in place by mount.ReadOnly.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"diskclone/internal/archive"
	"diskclone/internal/disk"
	"diskclone/internal/job"
	"diskclone/internal/privilege"
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

func fillTree(t *testing.T, root string, files map[string]int) {
	t.Helper()
	for name, size := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		data := make([]byte, size)
		rand.Read(data)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fakePartition is a "mounted" partition whose files live in a temp folder.
func fakePartition(t *testing.T, id, label, fs string, files map[string]int) disk.Partition {
	t.Helper()
	dir := t.TempDir()
	fillTree(t, dir, files)
	return disk.Partition{ID: id, Path: dir, Label: label, FSType: fs, MountPoints: []string{dir}, Supported: true}
}

type fileInfo struct {
	size int64
	hash [32]byte
}

func snapshot(t *testing.T, root string) map[string]fileInfo {
	t.Helper()
	out := map[string]fileInfo{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if d.Type().IsRegular() {
			data, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(root, p)
			out[filepath.ToSlash(rel)] = fileInfo{int64(len(data)), sha256.Sum256(data)}
		}
		return nil
	})
	return out
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

// CA3: archive of an NTFS and a FAT32 partition contains one folder per
// partition; after restore, files, sizes and hashes match.
func TestCA3ArchiveAndRestore(t *testing.T) {
	ntfs := fakePartition(t, "p1", "DATI", "ntfs", map[string]int{
		"documenti/lettera.txt": 1200, "foto/2024/img001.jpg": 3 << 20, "vuoto.bin": 0,
		"nomi con spazi/àccènti è ok.txt": 42,
	})
	fat := fakePartition(t, "p2", "USB KEY", "vfat", map[string]int{"README.TXT": 10, "DIR/SUB/FILE.DAT": 70000})
	src := disk.Disk{ID: "sdz", Model: "Test Disk", Partitions: []disk.Partition{ntfs, fat}}
	out := filepath.Join(t.TempDir(), "backup.tar.zst")
	contentBytes := int64(1200 + 3<<20 + 42 + 10 + 70000)
	// File-system used space differs from the file sizes (metadata, sparse
	// files): the progress total must come from the scan, not from here.
	est := Estimate{TotalBytes: 999, Partitions: []PartitionEstimate{{"p1", 100}, {"p2", 200}}}

	ev := &events{}
	done := runJob(t, ev, "archive", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runArchive(ctx, r, src, src.Partitions, est, out)
	})
	if done.Status != job.StatusDone || len(done.Warnings) != 0 {
		t.Fatalf("archive job = %+v", done)
	}
	lastProg := ev.progress[len(ev.progress)-1]
	if lastProg.Phase != "archive" || lastProg.Done != contentBytes || lastProg.Total != contentBytes || lastProg.Percent != 100 {
		t.Fatalf("final progress = %+v", lastProg)
	}
	if ev.progress[0].Phase != "scan" {
		t.Fatalf("first phase = %q, want scan", ev.progress[0].Phase)
	}

	m, err := archive.ReadManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Partitions) != 2 || m.Partitions[0].Name != "part1_DATI" || m.Partitions[1].Name != "part2_USB_KEY" ||
		m.Partitions[0].FSType != "ntfs" || m.Partitions[1].UsedBytes != 200 || !strings.Contains(m.SourceDisk, "Test Disk") ||
		m.ContentBytes != contentBytes {
		t.Fatalf("manifest = %+v", m)
	}

	dest := t.TempDir()
	ev2 := &events{}
	done = runJob(t, ev2, "restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestore(ctx, r, out, dest)
	})
	if done.Status != job.StatusDone {
		t.Fatalf("restore job = %+v", done)
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 2 {
		t.Fatalf("restored folders = %v, want one per partition", entries)
	}
	for name, p := range map[string]disk.Partition{"part1_DATI": ntfs, "part2_USB_KEY": fat} {
		want, got := snapshot(t, p.Path), snapshot(t, filepath.Join(dest, name))
		if len(want) != len(got) {
			t.Fatalf("%s: %d files restored, want %d", name, len(got), len(want))
		}
		for f, w := range want {
			if got[f] != w {
				t.Fatalf("%s/%s: size or hash differs after restore", name, f)
			}
		}
	}
}

// CA4: the archive opens with standard tools. 7-Zip on Windows (two steps:
// .zst -> .tar -> files), tar --zstd on Linux. Skipped when the tool is missing.
func TestCA4StandardTools(t *testing.T) {
	part := fakePartition(t, "p1", "DATA", "ntfs", map[string]int{"a/b.txt": 100, "c.bin": 5000})
	out := filepath.Join(t.TempDir(), "std.tar.zst")
	if _, err := archive.CreateFile(context.Background(), []archive.Root{{Name: "part1_DATA", Path: part.Path}}, archive.Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	want := snapshot(t, part.Path)
	dest := t.TempDir()

	if sevenZip := find7zWithZstd(); sevenZip != "" {
		if b, err := exec.Command(sevenZip, "x", "-y", "-o"+dest, out).CombinedOutput(); err != nil {
			t.Fatalf("7z cannot decompress .zst: %v\n%s", err, b)
		}
		tarFile := filepath.Join(dest, "std.tar")
		if b, err := exec.Command(sevenZip, "x", "-y", "-o"+dest, tarFile).CombinedOutput(); err != nil {
			t.Fatalf("7z cannot extract the tar: %v\n%s", err, b)
		}
		t.Logf("opened with %s", sevenZip)
	} else if tarSupportsZstd() {
		if b, err := exec.Command("tar", "--zstd", "-xf", out, "-C", dest).CombinedOutput(); err != nil {
			t.Fatalf("tar --zstd: %v\n%s", err, b)
		}
		t.Log("opened with tar --zstd")
	} else {
		t.Skip("no standard tool with zstd support installed (7-Zip without zstd codec, tar without --zstd)")
	}

	got := snapshot(t, filepath.Join(dest, "part1_DATA"))
	for f, w := range want {
		if got[f] != w {
			t.Fatalf("%s differs when extracted with the standard tool", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, archive.ManifestName)); err != nil {
		t.Fatal("manifest.json not visible to the standard tool")
	}
}

// find7zWithZstd returns the first installed 7-Zip that can decode zstd
// (7-Zip-zstd: https://github.com/mcmilk/7-Zip-zstd/releases).
func find7zWithZstd() string {
	candidates := []string{"7z", "7zz", `C:\Program Files\7-Zip-Zstandard\7z.exe`, `C:\Program Files\7-Zip\7z.exe`}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil && sevenZipSupportsZstd(path) {
			return path
		}
	}
	return ""
}

// sevenZipSupportsZstd: mainline 7-Zip up to at least 22.01 has no zstd codec.
func sevenZipSupportsZstd(exe string) bool {
	out, err := exec.Command(exe, "i").CombinedOutput()
	return err == nil && bytes.Contains(bytes.ToLower(out), []byte("zstd"))
}

// tarSupportsZstd: GNU tar needs the external zstd program, bsdtar must be
// built with libzstd (the Windows 10 tar.exe is not).
func tarSupportsZstd() bool {
	ver, err := exec.Command("tar", "--version").CombinedOutput()
	if err != nil {
		return false
	}
	if bytes.Contains(ver, []byte("bsdtar")) {
		return bytes.Contains(ver, []byte("libzstd"))
	}
	_, err = exec.LookPath("zstd")
	return err == nil
}

// CA7: cancel stops the archive within a few seconds, the job is reported as
// canceled (destination incomplete) and the partial archive is deleted.
func TestCA7CancelArchive(t *testing.T) {
	files := map[string]int{}
	for i := 0; i < 40; i++ {
		files["big/"+string(rune('a'+i%26))+string(rune('a'+i/26))+".bin"] = 4 << 20
	}
	part := fakePartition(t, "p1", "", "ntfs", files)
	src := disk.Disk{ID: "sdz", Partitions: []disk.Partition{part}}
	out := filepath.Join(t.TempDir(), "partial.tar.zst")

	ev := &events{}
	m := job.NewManager(ev.emit)
	var once sync.Once
	ev.onProg = func() { once.Do(func() { go m.Cancel() }) }
	start := time.Now()
	m.Start("archive", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runArchive(ctx, r, src, src.Partitions, Estimate{TotalBytes: 160 << 20}, out)
	})
	m.Wait()
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("cancel took %v", d)
	}
	if got := ev.last().Status; got != job.StatusCanceled {
		t.Fatalf("status = %s, want canceled", got)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial archive was not deleted")
	}
}

// CA7: same as above, but canceling once bytes are being written to the
// archive (after the scan phase): the partial file must be deleted.
func TestCA7CancelWhileWritingArchive(t *testing.T) {
	files := map[string]int{}
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("big/f%02d.bin", i)] = 4 << 20
	}
	part := fakePartition(t, "p1", "", "ntfs", files)
	src := disk.Disk{ID: "sdz", Partitions: []disk.Partition{part}}
	out := filepath.Join(t.TempDir(), "partial.tar.zst")

	ev := &events{}
	m := job.NewManager(ev.emit)
	var once sync.Once
	var sawFile bool
	ev.onProg = func() {
		ev.mu.Lock()
		last := ev.progress[len(ev.progress)-1]
		ev.mu.Unlock()
		if last.Phase == "archive" && last.Done > 0 {
			once.Do(func() {
				_, err := os.Stat(out)
				sawFile = err == nil
				go m.Cancel()
			})
		}
	}
	m.Start("archive", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runArchive(ctx, r, src, src.Partitions, Estimate{}, out)
	})
	m.Wait()
	if !sawFile {
		t.Fatal("cancel did not happen while the archive file was being written")
	}
	if got := ev.last().Status; got != job.StatusCanceled {
		t.Fatalf("status = %s, want canceled", got)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial archive was not deleted")
	}
}

// CA7 (restore): cancel during extraction is reported as canceled.
func TestCA7CancelRestore(t *testing.T) {
	part := fakePartition(t, "p1", "", "ntfs", map[string]int{"a.bin": 8 << 20, "b.bin": 8 << 20, "c.bin": 8 << 20})
	out := filepath.Join(t.TempDir(), "a.tar.zst")
	if _, err := archive.CreateFile(context.Background(), []archive.Root{{Name: "p", Path: part.Path}}, archive.Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	ev := &events{}
	m := job.NewManager(ev.emit)
	var once sync.Once
	ev.onProg = func() { once.Do(func() { go m.Cancel() }) }
	m.Start("restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestore(ctx, r, out, t.TempDir())
	})
	m.Wait()
	if got := ev.last().Status; got != job.StatusCanceled {
		t.Fatalf("status = %s, want canceled", got)
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
	if err := a.StartArchive("x", []string{"p"}, "out.tar.zst"); err == nil || err.Error() != "not_elevated" {
		t.Fatalf("StartArchive err = %v, want not_elevated", err)
	}
}

// Restore of a file that is not a Disk Clone archive fails with a clear code.
func TestRestoreRejectsForeignFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.tar.zst")
	os.WriteFile(p, []byte("not an archive"), 0o644)
	a := &App{}
	if err := a.StartRestore(p, t.TempDir()); err == nil || !strings.HasPrefix(err.Error(), "not_diskclone_archive") {
		t.Fatalf("err = %v", err)
	}
}
