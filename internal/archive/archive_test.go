package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func writeTree(t *testing.T, root string, files map[string]int) {
	t.Helper()
	for name, size := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, size)
		rand.Read(data)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// listEntries reads an archive with a plain tar+zstd reader.
func listEntries(t *testing.T, archivePath string) []*tar.Header {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	var hdrs []*tar.Header
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return hdrs
		}
		if err != nil {
			t.Fatal(err)
		}
		hdrs = append(hdrs, h)
	}
}

// regularFiles maps relative path -> content for every regular file under root.
func regularFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			data, _ := os.ReadFile(p)
			out[filepath.ToSlash(rel)] = data
		}
		return nil
	})
	return out
}

func TestRoundTrip(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	writeTree(t, src1, map[string]int{"a.txt": 10, "dir/b.bin": 3 << 20, "dir/sub/empty": 0, "àccénti è spazi.txt": 5})
	writeTree(t, src2, map[string]int{"x/y/z.dat": 70000})
	os.Mkdir(filepath.Join(src2, "emptydir"), 0o755)

	out := filepath.Join(t.TempDir(), "backup.tar.zst")
	var read int64
	m := Manifest{SourceDisk: "USB stick", Partitions: []ManifestPartition{{Name: "p1", FSType: "ntfs"}, {Name: "p2", FSType: "vfat"}}}
	warns, err := CreateFile(context.Background(), []Root{{Name: "p1", Path: src1}, {Name: "p2", Path: src2}}, m, out, func(n int64) { read += n })
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if want := int64(10 + 3<<20 + 5 + 70000); read != want {
		t.Fatalf("progress = %d, want %d", read, want)
	}

	hdrs := listEntries(t, out)
	if hdrs[0].Name != ManifestName {
		t.Fatalf("first entry %q, want manifest", hdrs[0].Name)
	}
	got, err := ReadManifest(out)
	if err != nil || got.SourceDisk != "USB stick" || len(got.Partitions) != 2 || got.Version != 1 {
		t.Fatalf("manifest = %+v, %v", got, err)
	}

	dest := t.TempDir()
	if _, err := Extract(context.Background(), out, dest, nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, src string }{{"p1", src1}, {"p2", src2}} {
		want, have := regularFiles(t, c.src), regularFiles(t, filepath.Join(dest, c.name))
		if len(want) != len(have) {
			t.Fatalf("%s: %d files restored, want %d", c.name, len(have), len(want))
		}
		for k, v := range want {
			if !bytes.Equal(have[k], v) {
				t.Fatalf("%s/%s differs after restore", c.name, k)
			}
		}
	}
	if fi, err := os.Stat(filepath.Join(dest, "p2", "emptydir")); err != nil || !fi.IsDir() {
		t.Fatal("empty directory not restored")
	}
	if _, err := os.Stat(filepath.Join(dest, ManifestName)); !os.IsNotExist(err) {
		t.Fatal("manifest must not be extracted as a file")
	}
}

// Paths longer than 100 bytes (classic tar header limit) and long single
// names must survive the round trip.
func TestLongPaths(t *testing.T) {
	src := t.TempDir()
	long := strings.Repeat("cartella_lunga_", 6) + "/" + strings.Repeat("x", 120) + ".txt"
	writeTree(t, src, map[string]int{long: 33})
	out := filepath.Join(t.TempDir(), "long.tar.zst")
	if _, err := CreateFile(context.Background(), []Root{{Name: "p", Path: src}}, Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if _, err := Extract(context.Background(), out, dest, nil); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dest, "p", filepath.FromSlash(long))); err != nil || fi.Size() != 33 {
		t.Fatalf("long path not restored: %v", err)
	}
}

func TestSymlinkCycleNotFollowed(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]int{"dir/f": 1})
	if err := os.Symlink("..", filepath.Join(src, "dir", "loop")); err != nil {
		t.Skip("cannot create symlinks here:", err)
	}
	out := filepath.Join(t.TempDir(), "a.tar.zst")
	if _, err := CreateFile(context.Background(), []Root{{Name: "p", Path: src}}, Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, h := range listEntries(t, out) {
		names = append(names, h.Name)
		if h.Name == "p/dir/loop" && h.Typeflag != tar.TypeSymlink {
			t.Fatal("loop stored as non-symlink")
		}
	}
	sort.Strings(names)
	want := []string{"manifest.json", "p/", "p/dir/", "p/dir/f", "p/dir/loop"}
	if len(names) != len(want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
}

func TestCreateFileCancelRemovesPartial(t *testing.T) {
	src := t.TempDir()
	files := map[string]int{}
	for i := 0; i < 30; i++ {
		files[fmt.Sprintf("d/f%02d", i)] = 2 << 20
	}
	writeTree(t, src, files)
	out := filepath.Join(t.TempDir(), "partial.tar.zst")
	ctx, cancel := context.WithCancel(context.Background())
	_, err := CreateFile(ctx, []Root{{Name: "p", Path: src}}, Manifest{}, out, func(int64) { cancel() })
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("partial archive was not removed")
	}
}

func TestCreateFileDoesNotOverwrite(t *testing.T) {
	out := filepath.Join(t.TempDir(), "exists.tar.zst")
	os.WriteFile(out, []byte("keep"), 0o644)
	if _, err := CreateFile(context.Background(), nil, Manifest{}, out, nil); err == nil {
		t.Fatal("expected error for existing file")
	}
	if data, _ := os.ReadFile(out); string(data) != "keep" {
		t.Fatal("existing file was modified or removed")
	}
}

func TestEstimate(t *testing.T) {
	if got := Estimate([]uint64{100, 250, 0}); got != 350 {
		t.Fatalf("Estimate = %d", got)
	}
}

// craft builds an archive with a manifest followed by the given headers.
func craft(t *testing.T, hdrs ...*tar.Header) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "evil.tar.zst")
	f, _ := os.Create(p)
	zw, _ := zstd.NewWriter(f)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0o644, Size: 2, Typeflag: tar.TypeReg})
	tw.Write([]byte("{}"))
	for _, h := range hdrs {
		if h.Typeflag == tar.TypeReg {
			h.Size = 1
		}
		tw.WriteHeader(h)
		if h.Typeflag == tar.TypeReg {
			tw.Write([]byte("x"))
		}
	}
	tw.Close()
	zw.Close()
	f.Close()
	return p
}

func TestExtractRejectsUnsafePaths(t *testing.T) {
	cases := map[string]*tar.Header{
		"dotdot":       {Name: "p/../../escape.txt", Typeflag: tar.TypeReg, Mode: 0o644},
		"absolute":     {Name: "/tmp/escape.txt", Typeflag: tar.TypeReg, Mode: 0o644},
		"drive":        {Name: "C:/escape.txt", Typeflag: tar.TypeReg, Mode: 0o644},
		"hardlinkOut":  {Name: "p/hl", Typeflag: tar.TypeLink, Linkname: "../outside"},
		"backslashDot": {Name: `p\..\..\escape.txt`, Typeflag: tar.TypeReg, Mode: 0o644},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dest := filepath.Join(parent, "dest")
			os.Mkdir(dest, 0o755)
			_, err := Extract(context.Background(), craft(t, h), dest, nil)
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("err = %v, want ErrUnsafePath", err)
			}
			if _, err := os.Lstat(filepath.Join(parent, "escape.txt")); !os.IsNotExist(err) {
				t.Fatal("file written outside destination")
			}
		})
	}
}

func TestExtractSkipsEscapingSymlinks(t *testing.T) {
	for name, target := range map[string]string{"relative": "../../outside", "absolute": "/etc", "drive": `C:\Users`} {
		t.Run(name, func(t *testing.T) {
			dest := t.TempDir()
			ar := craft(t, &tar.Header{Name: "p/link", Typeflag: tar.TypeSymlink, Linkname: target},
				&tar.Header{Name: "p/after.txt", Typeflag: tar.TypeReg, Mode: 0o644})
			warns, err := Extract(context.Background(), ar, dest, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(warns) != 1 {
				t.Fatalf("warnings = %v, want 1", warns)
			}
			if _, err := os.Lstat(filepath.Join(dest, "p", "link")); !os.IsNotExist(err) {
				t.Fatal("escaping symlink was created")
			}
			if _, err := os.Stat(filepath.Join(dest, "p", "after.txt")); err != nil {
				t.Fatal("extraction did not continue after the skipped link")
			}
		})
	}
}

func canSymlink(t *testing.T) {
	t.Helper()
	d := t.TempDir()
	if err := os.Symlink("x", filepath.Join(d, "l")); err != nil {
		t.Skip("cannot create symlinks here:", err)
	}
}

// Review R1: y -> "." and x -> "y/.." are lexically inside dest, but on
// Linux x resolves to the parent of dest. Nothing may be written there.
func TestExtractSymlinkChainDoesNotEscape(t *testing.T) {
	canSymlink(t)
	parent := t.TempDir()
	dest := filepath.Join(parent, "dest")
	os.Mkdir(dest, 0o755)
	ar := craft(t,
		&tar.Header{Name: "y", Typeflag: tar.TypeSymlink, Linkname: "."},
		&tar.Header{Name: "x", Typeflag: tar.TypeSymlink, Linkname: "y/.."},
		&tar.Header{Name: "x/evil.txt", Typeflag: tar.TypeReg, Mode: 0o644},
	)
	warns, _ := Extract(context.Background(), ar, dest, nil)
	if _, err := os.Stat(filepath.Join(parent, "evil.txt")); err == nil {
		t.Fatal("file written outside the destination")
	}
	if _, err := os.Lstat(filepath.Join(dest, "x")); err == nil && isLink(filepath.Join(dest, "x")) {
		t.Fatal("link through another link was created")
	}
	if len(warns) == 0 {
		t.Fatal("the rejected link must be reported")
	}
}

func TestExtractRejectsWritingThroughSymlink(t *testing.T) {
	canSymlink(t)
	dest := t.TempDir()
	ar := craft(t,
		&tar.Header{Name: "p/sub/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "p/link", Typeflag: tar.TypeSymlink, Linkname: "sub"},
		&tar.Header{Name: "p/link/f.txt", Typeflag: tar.TypeReg, Mode: 0o644},
	)
	if _, err := Extract(context.Background(), ar, dest, nil); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("err = %v, want ErrUnsafePath", err)
	}
}

func TestExtractKeepsLegitimateRelativeLinks(t *testing.T) {
	canSymlink(t)
	dest := t.TempDir()
	ar := craft(t,
		&tar.Header{Name: "p/lib/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "p/lib/libfoo.so.1", Typeflag: tar.TypeReg, Mode: 0o644},
		&tar.Header{Name: "p/bin/", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "p/bin/foo", Typeflag: tar.TypeSymlink, Linkname: "../lib/libfoo.so.1"},
	)
	warns, err := Extract(context.Background(), ar, dest, nil)
	if err != nil || len(warns) != 0 {
		t.Fatalf("err = %v, warnings = %v", err, warns)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "p", "bin", "foo")); err != nil || string(data) != "x" {
		t.Fatalf("relative link not usable: %q %v", data, err)
	}
}

func TestExtractReplacesExistingSymlinkInsteadOfFollowing(t *testing.T) {
	canSymlink(t)
	parent := t.TempDir()
	dest := filepath.Join(parent, "dest")
	os.MkdirAll(filepath.Join(dest, "p"), 0o755)
	outside := filepath.Join(parent, "outside.txt")
	os.WriteFile(outside, []byte("keep"), 0o644)
	if err := os.Symlink(outside, filepath.Join(dest, "p", "f.txt")); err != nil {
		t.Skip(err)
	}
	ar := craft(t, &tar.Header{Name: "p/f.txt", Typeflag: tar.TypeReg, Mode: 0o644})
	if _, err := Extract(context.Background(), ar, dest, nil); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(outside); string(data) != "keep" {
		t.Fatal("file outside the destination was overwritten through a symlink")
	}
	if isLink(filepath.Join(dest, "p", "f.txt")) {
		t.Fatal("symlink not replaced by the regular file")
	}
}

func TestExtractRejectsForeignArchive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "foreign.tar.zst")
	f, _ := os.Create(p)
	zw, _ := zstd.NewWriter(f)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: "random.txt", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
	tw.Write([]byte("x"))
	tw.Close()
	zw.Close()
	f.Close()
	if _, err := Extract(context.Background(), p, t.TempDir(), nil); !errors.Is(err, ErrNotDiskcloneArchive) {
		t.Fatalf("err = %v", err)
	}
	if _, err := ReadManifest(p); !errors.Is(err, ErrNotDiskcloneArchive) {
		t.Fatalf("ReadManifest err = %v", err)
	}
}

func TestExtractProgressCountsCompressedBytes(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]int{"f": 1 << 20})
	out := filepath.Join(t.TempDir(), "a.tar.zst")
	if _, err := CreateFile(context.Background(), []Root{{Name: "p", Path: src}}, Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	var n int64
	if _, err := Extract(context.Background(), out, t.TempDir(), func(k int64) { n += k }); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(out)
	if n != fi.Size() {
		t.Fatalf("progress %d, archive size %d", n, fi.Size())
	}
}
