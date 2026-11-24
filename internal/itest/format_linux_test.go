//go:build integration && linux

package itest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"exactclone/internal/disk"
	"exactclone/internal/format"
)

// Every installed file system, with both partition tables, on a 512 MiB loop
// disk (XFS needs 300 MiB). blkid must find the new table, file system and label.
func TestFormatAllFileSystems(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("integration tests need root")
	}
	blkType := map[string]string{"fat32": "vfat"}
	for _, fs := range format.List() {
		for _, scheme := range []string{format.SchemeMBR, format.SchemeGPT} {
			t.Run(fs.ID+"/"+scheme, func(t *testing.T) {
				if !fs.Available {
					t.Skip(fs.Tool + " not installed")
				}
				backing := filepath.Join(t.TempDir(), "disk.img")
				run(t, "truncate", "-s", "512M", backing)
				dev := run(t, "losetup", "--find", "--show", "--partscan", backing)
				t.Cleanup(func() { exec.Command("losetup", "-d", dev).Run() })

				d := disk.Disk{ID: filepath.Base(dev), Path: dev, SizeBytes: 512 << 20}
				if err := format.Run(context.Background(), d, fs.ID, scheme, "Test1"); err != nil {
					t.Fatal(err)
				}
				pt := run(t, "blkid", "-p", "-o", "value", "-s", "PTTYPE", dev)
				if want := map[string]string{"mbr": "dos", "gpt": "gpt"}[scheme]; pt != want {
					t.Errorf("PTTYPE = %q, want %q", pt, want)
				}
				part := dev + "p1"
				typ := run(t, "blkid", "-p", "-o", "value", "-s", "TYPE", part)
				want := fs.ID
				if w, ok := blkType[fs.ID]; ok {
					want = w
				}
				if typ != want {
					t.Errorf("TYPE = %q, want %q", typ, want)
				}
				label := run(t, "blkid", "-p", "-o", "value", "-s", "LABEL", part)
				if !strings.EqualFold(label, "Test1") {
					t.Errorf("LABEL = %q", label)
				}
			})
		}
	}
}
