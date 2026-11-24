// Package gpt moves the backup GPT of a disk cloned onto a larger one to the
// end of the disk, as `sgdisk -e` does. After a bit-by-bit copy the backup
// header and partition array stay where the source disk ended: firmware and
// tools then report the table as damaged.
package gpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"exactclone/internal/clone"
)

// Header field offsets (UEFI spec, "GPT Header").
const (
	offHeaderSize = 12
	offHeaderCRC  = 16
	offMyLBA      = 24
	offAltLBA     = 32
	offLastUsable = 48
	offEntriesLBA = 72
	offNumEntries = 80
	offEntrySize  = 84
	offEntriesCRC = 88
	minHeaderSize = 92
	maxEntryBytes = 1 << 20 // sanity limit; the usual array is 16 KiB
)

var signature = []byte("EFI PART")

// ErrNotGPT: no valid primary GPT (signature, CRCs) was found.
var ErrNotGPT = errors.New("no valid GPT")

// Device is a disk opened for reading and writing.
type Device interface {
	io.ReaderAt
	io.WriterAt
}

var le = binary.LittleEndian

// table is a validated primary GPT.
type table struct {
	ss           int64  // logical sector size
	hdr          []byte // sector holding the primary header
	hsize        uint32
	entries      []byte // partition array, whole sectors
	entrySectors int64
}

// readSectors reads n sectors at lba into a 4096-aligned buffer: raw disks
// opened unbuffered on Windows accept only aligned, sector-sized I/O.
func readSectors(dev io.ReaderAt, ss, lba, n int64) ([]byte, error) {
	buf := clone.AlignedBuf(int(ss * n))
	if _, err := dev.ReadAt(buf, lba*ss); err != nil {
		return nil, fmt.Errorf("read LBA %d: %w", lba, err)
	}
	return buf, nil
}

func writeSectors(dev io.WriterAt, ss, lba int64, data []byte) error {
	buf := clone.AlignedBuf(len(data))
	copy(buf, data)
	if _, err := dev.WriteAt(buf, lba*ss); err != nil {
		return fmt.Errorf("write LBA %d: %w", lba, err)
	}
	return nil
}

func headerCRC(hdr []byte, size uint32) uint32 {
	h := bytes.Clone(hdr[:size])
	le.PutUint32(h[offHeaderCRC:], 0)
	return crc32.ChecksumIEEE(h)
}

func setHeaderCRC(hdr []byte, size uint32) {
	le.PutUint32(hdr[offHeaderCRC:], headerCRC(hdr, size))
}

// readPrimary finds the primary header at LBA 1 with 512 or 4096-byte sectors.
func readPrimary(dev io.ReaderAt) (*table, error) {
	for _, ss := range []int64{512, 4096} {
		if t, err := readPrimaryAt(dev, ss); err == nil {
			return t, nil
		}
	}
	return nil, ErrNotGPT
}

func readPrimaryAt(dev io.ReaderAt, ss int64) (*table, error) {
	hdr, err := readSectors(dev, ss, 1, 1)
	if err != nil {
		return nil, err
	}
	hsize := le.Uint32(hdr[offHeaderSize:])
	switch {
	case !bytes.Equal(hdr[:8], signature),
		hsize < minHeaderSize || int64(hsize) > ss,
		headerCRC(hdr, hsize) != le.Uint32(hdr[offHeaderCRC:]),
		le.Uint64(hdr[offMyLBA:]) != 1:
		return nil, ErrNotGPT
	}
	num, esize := int64(le.Uint32(hdr[offNumEntries:])), int64(le.Uint32(hdr[offEntrySize:]))
	entryBytes := num * esize
	if num == 0 || esize < 128 || entryBytes > maxEntryBytes {
		return nil, ErrNotGPT
	}
	entrySectors := (entryBytes + ss - 1) / ss
	entries, err := readSectors(dev, ss, int64(le.Uint64(hdr[offEntriesLBA:])), entrySectors)
	if err != nil {
		return nil, err
	}
	if crc32.ChecksumIEEE(entries[:entryBytes]) != le.Uint32(hdr[offEntriesCRC:]) {
		return nil, ErrNotGPT
	}
	return &table{ss: ss, hdr: hdr, hsize: hsize, entries: entries, entrySectors: entrySectors}, nil
}

// MoveBackup writes the backup partition array and header at the end of a
// disk of diskSize bytes, extends the last usable LBA and updates the primary
// header and the protective MBR. It reports false when the backup is already
// at the end. Partitions are not touched.
func MoveBackup(dev Device, diskSize int64) (bool, error) {
	t, err := readPrimary(dev)
	if err != nil {
		return false, err
	}
	ss := t.ss
	last := diskSize/ss - 1
	oldAlt := int64(le.Uint64(t.hdr[offAltLBA:]))
	switch {
	case oldAlt == last:
		return false, nil
	case oldAlt > last:
		return false, fmt.Errorf("backup GPT at LBA %d is beyond the end of the disk (%d)", oldAlt, last)
	}
	backupEntries := last - t.entrySectors
	lastUsable := backupEntries - 1
	if lastUsable < int64(le.Uint64(t.hdr[offLastUsable:])) {
		return false, fmt.Errorf("no room for the backup GPT")
	}

	// Backup first: until the primary is rewritten it still describes a
	// consistent table with the old backup.
	if err := writeSectors(dev, ss, backupEntries, t.entries); err != nil {
		return false, err
	}
	backup := bytes.Clone(t.hdr)
	le.PutUint64(backup[offMyLBA:], uint64(last))
	le.PutUint64(backup[offAltLBA:], 1)
	le.PutUint64(backup[offLastUsable:], uint64(lastUsable))
	le.PutUint64(backup[offEntriesLBA:], uint64(backupEntries))
	setHeaderCRC(backup, t.hsize)
	if err := writeSectors(dev, ss, last, backup); err != nil {
		return false, err
	}

	primary := t.hdr
	le.PutUint64(primary[offAltLBA:], uint64(last))
	le.PutUint64(primary[offLastUsable:], uint64(lastUsable))
	setHeaderCRC(primary, t.hsize)
	if err := writeSectors(dev, ss, 1, primary); err != nil {
		return false, err
	}

	if err := fixProtectiveMBR(dev, ss, last); err != nil {
		return true, err
	}
	// The old backup header now lies in free space: wipe it so recovery
	// tools do not pick up a stale table. Skip it if the new backup array
	// already overwrote it (destination only slightly larger).
	if oldAlt < backupEntries {
		old, err := readSectors(dev, ss, oldAlt, 1)
		if err != nil {
			return true, err
		}
		if bytes.Equal(old[:8], signature) {
			return true, writeSectors(dev, ss, oldAlt, make([]byte, ss))
		}
	}
	return true, nil
}

// fixProtectiveMBR extends the 0xEE partition of a protective MBR to the new
// disk size. Hybrid MBRs (other partitions present) are left alone.
func fixProtectiveMBR(dev Device, ss, last int64) error {
	mbr, err := readSectors(dev, ss, 0, 1)
	if err != nil {
		return err
	}
	if mbr[510] != 0x55 || mbr[511] != 0xAA {
		return nil
	}
	const table, entry = 446, 16
	if mbr[table+4] != 0xEE || le.Uint32(mbr[table+8:]) != 1 {
		return nil
	}
	for i := 1; i < 4; i++ {
		if mbr[table+i*entry+4] != 0 {
			return nil
		}
	}
	want := uint32(min(last, 0xFFFFFFFF))
	chs := endCHS(last)
	if le.Uint32(mbr[table+12:]) == want && bytes.Equal(mbr[table+5:table+8], chs[:]) {
		return nil
	}
	copy(mbr[table+5:], chs[:])
	le.PutUint32(mbr[table+12:], want)
	return writeSectors(dev, ss, 0, mbr)
}

// endCHS encodes lba as an MBR CHS address (255 heads, 63 sectors per
// track), or FE FF FF when it does not fit, as sgdisk writes it.
func endCHS(lba int64) [3]byte {
	const heads, sectors = 255, 63
	c := lba / (heads * sectors)
	if c > 1023 {
		return [3]byte{0xFE, 0xFF, 0xFF}
	}
	h := lba / sectors % heads
	s := lba%sectors + 1
	return [3]byte{byte(h), byte(s) | byte(c>>2&0xC0), byte(c)}
}
