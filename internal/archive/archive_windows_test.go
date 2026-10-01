package archive

import (
	"archive/tar"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Junctions can be created without admin rights and Go reports them as
// directories: they must be stored as links, never traversed.
func TestJunctionNotFollowed(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]int{"Users/me/f": 1})
	junction := filepath.Join(src, "Users", "me", "Application Data")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, filepath.Join(src, "Users", "me")).CombinedOutput(); err != nil {
		t.Skipf("mklink /J failed: %v %s", err, out)
	}
	out := filepath.Join(t.TempDir(), "a.tar.zst")
	if _, err := CreateFile(context.Background(), []Root{{Name: "p", Path: src}}, Manifest{}, out, nil); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range listEntries(t, out) {
		if strings.HasPrefix(h.Name, "p/Users/me/Application Data/") {
			t.Fatalf("junction was traversed: %s", h.Name)
		}
		if h.Name == "p/Users/me/Application Data" {
			found = h.Typeflag == tar.TypeSymlink
		}
	}
	if !found {
		t.Fatal("junction not stored as a symlink")
	}
}
