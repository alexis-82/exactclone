package main

import (
	"bytes"
	"context"
	"errors"
	goruntime "runtime"

	"diskclone/internal/clone"
	"diskclone/internal/disk"
	"diskclone/internal/job"
	"diskclone/internal/rawdev"
)

// Notice codes shown at the end of a job.
const (
	NoticeCloneOffline    = "clone_offline"
	NoticeDestOffline     = "dest_offline"
	NoticeGPTBackupHeader = "gpt_backup_header"
)

// isGPT looks for the "EFI PART" signature at LBA 1 (512 or 4096-byte sectors).
func isGPT(dev rawdev.Device) bool {
	buf := clone.AlignedBuf(8192)
	if _, err := dev.ReadAt(buf, 0); err != nil {
		return false
	}
	sig := []byte("EFI PART")
	return bytes.Equal(buf[512:520], sig) || bytes.Equal(buf[4096:4104], sig)
}

func runClone(ctx context.Context, r *job.Reporter, src, dst disk.Disk, verify bool) (res job.Result, err error) {
	release, err := disk.PrepareForWrite(src, dst)
	if err != nil {
		return res, coded("prepare_failed", err)
	}
	defer release()
	var gpt bool
	var size int64
	// From here on the destination may be offline (Windows), even if the
	// copy fails or is canceled: always tell the user.
	defer func() {
		res.Notices = cloneNotices(goruntime.GOOS, err, gpt, dst.SizeBytes > size)
	}()
	s, err := rawdev.Open(src.Path, false)
	if err != nil {
		return res, coded("open_failed", err)
	}
	defer s.Close()
	d, err := rawdev.Open(dst.Path, true)
	if err != nil {
		return res, coded("open_failed", err)
	}
	defer func() {
		if d != nil {
			d.Close()
		}
	}()
	size = s.Size()
	if d.Size() < size {
		return res, coded("dest_too_small", nil)
	}
	gpt = isGPT(s)

	r.Phase("copy", size)
	sum, err := clone.Copy(ctx, s, d, size, clone.Options{HeadLast: true}, r.Add)
	if err != nil {
		return res, coded("copy_failed", err)
	}
	if verify {
		r.Phase("verify", size)
		if err := clone.Verify(ctx, d, size, sum, r.Add); err != nil {
			if errors.Is(err, clone.ErrVerifyMismatch) {
				return res, coded("verify_mismatch", nil)
			}
			return res, coded("verify_failed", err)
		}
	}
	err = d.Close()
	d = nil
	if err != nil {
		return res, coded("copy_failed", err)
	}
	if err := disk.FinishWrite(dst); err != nil {
		return res, coded("finish_failed", err)
	}
	return res, nil
}

// cloneNotices returns the messages shown at the end of a clone that got
// past PrepareForWrite.
func cloneNotices(goos string, err error, gpt, destLarger bool) []string {
	var notices []string
	if goos == "windows" {
		if err == nil {
			notices = append(notices, NoticeCloneOffline)
		} else {
			notices = append(notices, NoticeDestOffline)
		}
	}
	if err == nil && gpt && destLarger {
		notices = append(notices, NoticeGPTBackupHeader)
	}
	return notices
}
