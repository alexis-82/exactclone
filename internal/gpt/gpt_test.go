package gpt

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const mib = 1 << 20

// gptDisk creates a 64 MiB file with a GPT made by sgdisk (two partitions),
// then grows it to size bytes, as a bit-by-bit clone onto a larger disk.
func gptDisk(t *testing.T, size int64) string {
	t.Helper()
	if _, err := exec.LookPath("sgdisk"); err != nil {
		t.Skip("sgdisk not installed")
	}
	p := filepath.Join(t.TempDir(), "disk.raw")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(p, 64*mib); err != nil {
		t.Fatal(err)
	}
	sgdisk(t, "-n", "1:0:+16M", "-t", "1:ef00", "-n", "2:0:0", "-t", "2:0700", p)
	if err := os.Truncate(p, size); err != nil {
		t.Fatal(err)
	}
	return p
}

func sgdisk(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("sgdisk", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("sgdisk %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func moveBackup(t *testing.T, path string) bool {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	moved, err := MoveBackup(f, fi.Size())
	if err != nil {
		t.Fatal(err)
	}
	return moved
}

func readAt(t *testing.T, path string, off, n int64) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, off); err != nil {
		t.Fatal(err)
	}
	return buf
}

func copyFile(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := src + ".ref"
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

// The result must match `sgdisk -e` on the same disk: protective MBR,
// primary header and array, backup array and header.
func TestMoveBackupMatchesSgdisk(t *testing.T) {
	const size = 256 * mib
	p := gptDisk(t, size)
	ref := copyFile(t, p)
	partsBefore := sgdisk(t, "-p", p)

	if !moveBackup(t, p) {
		t.Fatal("moved = false, want true")
	}
	sgdisk(t, "-e", ref)

	for _, r := range []struct {
		name   string
		off, n int64
	}{
		{"MBR + primary GPT", 0, 34 * 512},
		{"backup GPT", size - 33*512, 33 * 512},
	} {
		if !bytes.Equal(readAt(t, p, r.off, r.n), readAt(t, ref, r.off, r.n)) {
			t.Errorf("%s differs from sgdisk -e", r.name)
		}
	}
	if out := sgdisk(t, "-v", p); !strings.Contains(out, "No problems found") {
		t.Errorf("sgdisk -v:\n%s", out)
	}
	// Partitions unchanged (the "Disk ... sectors" line reports the new size).
	if a, b := partitions(partsBefore), partitions(sgdisk(t, "-p", p)); a != b {
		t.Errorf("partitions changed:\n%s\nvs\n%s", a, b)
	}
	// The stale backup header at the old end of the disk is wiped.
	if old := readAt(t, p, 64*mib-512, 8); bytes.Equal(old, signature) {
		t.Error("old backup header still present")
	}
}

func partitions(out string) string {
	_, after, _ := strings.Cut(out, "Number")
	return after
}

func TestMoveBackupAlreadyAtEnd(t *testing.T) {
	p := gptDisk(t, 64*mib)
	before, _ := os.ReadFile(p)
	if moveBackup(t, p) {
		t.Fatal("moved = true on a disk of the same size")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("disk modified")
	}
}

// Destination larger by less than the backup array: the new array overlaps
// the old backup header, which must not be wiped afterwards.
func TestMoveBackupSlightlyLarger(t *testing.T) {
	p := gptDisk(t, 64*mib+4096)
	if !moveBackup(t, p) {
		t.Fatal("moved = false, want true")
	}
	if out := sgdisk(t, "-v", p); !strings.Contains(out, "No problems found") {
		t.Errorf("sgdisk -v:\n%s", out)
	}
}

func TestMoveBackupNotGPT(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zero.raw")
	if err := os.WriteFile(p, make([]byte, mib), 0o600); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(p, os.O_RDWR, 0)
	defer f.Close()
	if _, err := MoveBackup(f, mib); !errors.Is(err, ErrNotGPT) {
		t.Fatalf("err = %v, want ErrNotGPT", err)
	}
}
