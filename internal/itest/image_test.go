//go:build integration

package itest

import (
	"context"
	"path/filepath"
	"testing"

	"diskclone/internal/clone"
	"diskclone/internal/disk"
	"diskclone/internal/image"
	"diskclone/internal/rawdev"
)

// imageRoundTrip copies src into an image file, restores it onto dst and
// returns the SHA-256 of the source disk. It uses the same steps as the app.
func imageRoundTrip(t *testing.T, src, dst disk.Disk) []byte {
	t.Helper()
	img := filepath.Join(t.TempDir(), "disk.img.zst")

	release, err := disk.PrepareForRead(src)
	if err != nil {
		t.Fatal(err)
	}
	s, err := rawdev.Open(src.Path, false)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := image.CreateFile(context.Background(), s, s.Size(), image.Info{SourceDisk: src.ID}, img, nil)
	s.Close()
	release()
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := image.VerifyFile(context.Background(), img, nil); err != nil {
		t.Fatalf("verify image: %v", err)
	}

	r, err := image.Open(img)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	release, err = disk.PrepareForWrite(disk.Disk{}, dst)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	d, err := rawdev.Open(dst.Path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	got, err := clone.Copy(context.Background(), r, d, r.Size(), clone.Options{HeadLast: true}, nil)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := r.Check(got); err != nil {
		t.Fatalf("image check: %v", err)
	}
	if err := clone.Verify(context.Background(), d, r.Size(), sum, nil); err != nil {
		t.Fatalf("verify destination: %v", err)
	}
	return sum
}
