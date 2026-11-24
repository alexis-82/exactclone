package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	goruntime "runtime"

	"exactclone/internal/clone"
	"exactclone/internal/disk"
	"exactclone/internal/gpt"
	"exactclone/internal/image"
	"exactclone/internal/job"
	"exactclone/internal/rawdev"
)

// Notice codes shown at the end of a job.
const (
	NoticeCloneOffline    = "clone_offline"
	NoticeDestOffline     = "dest_offline"
	NoticeGPTBackupHeader = "gpt_backup_header"
)

// Device operations, replaced in tests to use regular files as disks.
var (
	openDevice      = rawdev.Open
	prepareForRead  = disk.PrepareForRead
	prepareForWrite = disk.PrepareForWrite
	finishWrite     = disk.FinishWrite
)

// isGPT looks for the "EFI PART" signature at LBA 1 (512 or 4096-byte sectors).
func isGPT(dev io.ReaderAt) bool {
	buf := clone.AlignedBuf(8192)
	if _, err := dev.ReadAt(buf, 0); err != nil {
		return false
	}
	sig := []byte("EFI PART")
	return bytes.Equal(buf[512:520], sig) || bytes.Equal(buf[4096:4104], sig)
}

func runClone(ctx context.Context, r *job.Reporter, src, dst disk.Disk, verify bool) (res job.Result, err error) {
	release, err := prepareForWrite(src, dst)
	if err != nil {
		return res, coded("prepare_failed", err)
	}
	defer release()
	var gptMisplaced bool
	// From here on the destination may be offline (Windows), even if the
	// copy fails or is canceled: always tell the user.
	defer func() {
		res.Notices = cloneNotices(goruntime.GOOS, err, gptMisplaced)
	}()
	s, err := openDevice(src.Path, false)
	if err != nil {
		return res, coded("open_failed", err)
	}
	defer s.Close()
	gptMisplaced, err = writeToDisk(ctx, r, s, dst, s.Size(), verify, nil)
	return res, err
}

// runImage copies the whole disk src into a new image file at outPath.
func runImage(ctx context.Context, r *job.Reporter, src disk.Disk, outPath string, verify bool) (res job.Result, err error) {
	release, err := prepareForRead(src)
	if err != nil {
		return res, coded("prepare_failed", err)
	}
	defer release()
	s, err := openDevice(src.Path, false)
	if err != nil {
		return res, coded("open_failed", err)
	}
	defer s.Close()
	size := s.Size()

	r.Phase("copy", size)
	info := image.Info{SourceDisk: fmt.Sprintf("%s (%s)", src.Model, src.ID)}
	if _, err := image.CreateFile(ctx, s, size, info, outPath, r.Add); err != nil {
		return res, coded("image_failed", err)
	}
	if verify {
		r.Phase("verify", size)
		if err := image.VerifyFile(ctx, outPath, r.Add); err != nil {
			if errors.Is(err, clone.ErrCanceled) {
				return res, err // the image is complete: keep it
			}
			os.Remove(outPath) // never leave an image known to be bad
			if errors.Is(err, image.ErrCorrupt) {
				return res, coded("verify_mismatch", nil)
			}
			return res, coded("verify_failed", err)
		}
	}
	return res, nil
}

// runRestoreImage writes the image at imagePath onto dst.
func runRestoreImage(ctx context.Context, r *job.Reporter, imagePath string, dst disk.Disk, verify bool) (res job.Result, err error) {
	img, err := image.Open(imagePath)
	if err != nil {
		return res, imageError(err)
	}
	defer img.Close()
	release, err := prepareForWrite(disk.Disk{}, dst)
	if err != nil {
		return res, coded("prepare_failed", err)
	}
	defer release()
	var gptMisplaced bool
	defer func() {
		res.Notices = cloneNotices(goruntime.GOOS, err, gptMisplaced)
	}()
	gptMisplaced, err = writeToDisk(ctx, r, img, dst, img.Size(), verify, img.Check)
	return res, err
}

// writeToDisk copies size bytes of src onto the prepared disk dst, writing the
// partition table last. check (optional) validates the SHA-256 of the data
// read before the optional verification pass. On a larger destination the
// backup GPT is moved to the end of the disk; it reports whether that was
// needed and failed (the copy itself is still good).
func writeToDisk(ctx context.Context, r *job.Reporter, src io.ReaderAt, dst disk.Disk, size int64, verify bool, check func(sum []byte) error) (gptMisplaced bool, err error) {
	d, err := openDevice(dst.Path, true)
	if err != nil {
		return false, coded("open_failed", err)
	}
	defer func() {
		if d != nil {
			d.Close()
		}
	}()
	if d.Size() < size {
		return false, coded("dest_too_small", nil)
	}

	r.Phase("copy", size)
	sum, err := clone.Copy(ctx, src, d, size, clone.Options{HeadLast: true}, r.Add)
	if err != nil {
		if errors.Is(err, image.ErrCorrupt) {
			return false, coded("image_corrupt", err)
		}
		return false, coded("copy_failed", err)
	}
	if check != nil {
		if err := check(sum); err != nil {
			return false, coded("image_corrupt", err)
		}
	}
	if verify {
		r.Phase("verify", size)
		if err := clone.Verify(ctx, d, size, sum, r.Add); err != nil {
			if errors.Is(err, clone.ErrVerifyMismatch) {
				return false, coded("verify_mismatch", nil)
			}
			return false, coded("verify_failed", err)
		}
	}
	// After the verification, which compares the copied bytes: this rewrites
	// the primary GPT header. isGPT is looser than MoveBackup, so a damaged
	// GPT that cannot be fixed still gets the notice.
	if d.Size() > size && isGPT(d) {
		if _, err := gpt.MoveBackup(d, d.Size()); err != nil {
			gptMisplaced = true
		}
	}
	err = d.Close()
	d = nil
	if err != nil {
		return gptMisplaced, coded("copy_failed", err)
	}
	if err := finishWrite(dst); err != nil {
		return gptMisplaced, coded("finish_failed", err)
	}
	return gptMisplaced, nil
}

// imageError maps image format errors to frontend codes.
func imageError(err error) error {
	switch {
	case errors.Is(err, image.ErrNotImage):
		return coded("not_exactclone_image", err)
	case errors.Is(err, image.ErrIncomplete):
		return coded("image_incomplete", err)
	case errors.Is(err, image.ErrCorrupt):
		return coded("image_corrupt", err)
	default:
		return coded("image_open_failed", err)
	}
}

// cloneNotices returns the messages shown at the end of a write to a disk
// that got past PrepareForWrite.
func cloneNotices(goos string, err error, gptMisplaced bool) []string {
	var notices []string
	if goos == "windows" {
		if err == nil {
			notices = append(notices, NoticeCloneOffline)
		} else {
			notices = append(notices, NoticeDestOffline)
		}
	}
	if err == nil && gptMisplaced {
		notices = append(notices, NoticeGPTBackupHeader)
	}
	return notices
}
