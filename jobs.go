package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	goruntime "runtime"

	"diskclone/internal/archive"
	"diskclone/internal/clone"
	"diskclone/internal/disk"
	"diskclone/internal/job"
	"diskclone/internal/mount"
	"diskclone/internal/rawdev"
)

// Notice codes shown at the end of a job.
const (
	NoticeCloneOffline    = "clone_offline"
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
	size := s.Size()
	if d.Size() < size {
		return res, coded("dest_too_small", nil)
	}
	gpt := isGPT(s)

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
	if goruntime.GOOS == "windows" {
		res.Notices = append(res.Notices, NoticeCloneOffline)
	}
	if gpt && dst.SizeBytes > size {
		res.Notices = append(res.Notices, NoticeGPTBackupHeader)
	}
	return res, nil
}

func runArchive(ctx context.Context, r *job.Reporter, d disk.Disk, parts []disk.Partition, est Estimate, outPath string) (res job.Result, err error) {
	m := archive.Manifest{SourceDisk: fmt.Sprintf("%s (%s)", d.Model, d.ID)}
	var roots []archive.Root
	for i, p := range parts {
		root, cleanup, err := mount.ReadOnly(p)
		if err != nil {
			return res, coded("mount_failed", err)
		}
		defer cleanup()
		name := rootName(i, p)
		roots = append(roots, archive.Root{Name: name, Path: root})
		var used uint64
		for _, e := range est.Partitions {
			if e.ID == p.ID {
				used = e.UsedBytes
			}
		}
		m.Partitions = append(m.Partitions, archive.ManifestPartition{Name: name, FSType: p.FSType, Label: p.Label, UsedBytes: used})
	}
	r.Phase("scan", 0)
	total, err := archive.Scan(ctx, roots)
	if err != nil {
		return res, coded("archive_failed", err)
	}
	m.ContentBytes = total
	r.Phase("archive", total)
	warnings, err := archive.CreateFile(ctx, roots, m, outPath, r.Add)
	res.Warnings = warnings
	if err != nil {
		return res, coded("archive_failed", err)
	}
	return res, nil
}

func runRestore(ctx context.Context, r *job.Reporter, archivePath, destDir string) (res job.Result, err error) {
	fi, err := os.Stat(archivePath)
	if err != nil {
		return res, coded("restore_failed", err)
	}
	r.Phase("restore", fi.Size())
	warnings, err := archive.Extract(ctx, archivePath, destDir, r.Add)
	res.Warnings = warnings
	switch {
	case err == nil:
		return res, nil
	case errors.Is(err, archive.ErrUnsafePath):
		return res, coded("unsafe_archive", err)
	case errors.Is(err, archive.ErrNotDiskcloneArchive):
		return res, coded("not_diskclone_archive", err)
	default:
		return res, coded("restore_failed", err)
	}
}
