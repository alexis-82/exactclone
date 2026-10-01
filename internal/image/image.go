// Package image reads and writes bit-by-bit disk images (.img.zst).
//
// An image is a standard zstd stream of the whole disk. Metadata travel in
// zstd skippable frames, which standard decompressors ignore:
//
//	[skippable frame: JSON header]  format, version, source disk, size, date
//	[zstd frames: disk data]
//	[skippable frame: JSON trailer] SHA-256 and size; fixed length, last in the file
//
// so `zstd -d` or 7-Zip-zstd turn the file into a raw .img of the disk.
package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"

	"exactclone/internal/clone"
)

// Extension of image files.
const Extension = ".img.zst"

const (
	// formatName identifies the format inside the file header. It keeps the
	// project's original name so that images created before the rename to
	// ExactClone stay readable: never change it.
	formatName         = "diskclone-image"
	formatVersion      = 1
	headerMagic        = 0x184D2A50 // zstd skippable frame, user nibble 0
	trailerMagic       = 0x184D2A51 // zstd skippable frame, user nibble 1
	trailerPayloadSize = 256
	maxHeaderSize      = 64 << 10
)

var (
	// ErrNotImage: the file is not an ExactClone image.
	ErrNotImage = errors.New("not an ExactClone image")
	// ErrIncomplete: the image has no trailer (creation interrupted).
	ErrIncomplete = errors.New("incomplete image")
	// ErrCorrupt: the image data is damaged or does not match its checksum.
	ErrCorrupt = errors.New("corrupt image")
)

// Info describes an image.
type Info struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	SourceDisk string    `json:"sourceDisk"`
	SizeBytes  int64     `json:"sizeBytes"`
	CreatedAt  time.Time `json:"createdAt"`
	SHA256     string    `json:"sha256,omitempty"` // from the trailer
}

type trailer struct {
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

func skippableFrame(magic uint32, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(b, magic)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(payload)))
	copy(b[8:], payload)
	return b
}

// seqWriterAt adapts a stream to the io.WriterAt used by clone.Copy, whose
// writer receives the chunks in order.
type seqWriterAt struct {
	w    io.Writer
	next int64
}

func (s *seqWriterAt) WriteAt(p []byte, off int64) (int, error) {
	if off != s.next {
		return 0, fmt.Errorf("non-sequential write at %d (expected %d)", off, s.next)
	}
	n, err := s.w.Write(p)
	s.next += int64(n)
	return n, err
}

// Create writes an image of src[0:size) to w and returns the SHA-256 of the
// disk data. onProgress receives the number of disk bytes read.
func Create(ctx context.Context, src io.ReaderAt, size int64, info Info, w io.Writer, onProgress func(int64)) ([]byte, error) {
	info.Format, info.Version, info.SizeBytes, info.SHA256 = formatName, formatVersion, size, ""
	if info.CreatedAt.IsZero() {
		info.CreatedAt = time.Now()
	}
	header, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(skippableFrame(headerMagic, header)); err != nil {
		return nil, err
	}
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	sum, err := clone.Copy(ctx, src, &seqWriterAt{w: zw}, size, clone.Options{}, onProgress)
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	t, _ := json.Marshal(trailer{SHA256: hex.EncodeToString(sum), SizeBytes: size})
	payload := bytes.Repeat([]byte(" "), trailerPayloadSize)
	copy(payload, t)
	if _, err := w.Write(skippableFrame(trailerMagic, payload)); err != nil {
		return nil, err
	}
	return sum, nil
}

// CreateFile writes an image to path, which must not exist. On error or
// cancellation the partial file is removed.
func CreateFile(ctx context.Context, src io.ReaderAt, size int64, info Info, path string, onProgress func(int64)) (sum []byte, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			err = f.Sync()
		}
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
		if err != nil {
			os.Remove(path)
			sum = nil
		}
	}()
	return Create(ctx, src, size, info, f, onProgress)
}

// ReadInfo reads header and trailer of an image file.
func ReadInfo(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	return readInfo(f)
}

func readInfo(f *os.File) (Info, error) {
	var info Info
	head := make([]byte, 8)
	if _, err := io.ReadFull(f, head); err != nil || binary.LittleEndian.Uint32(head) != headerMagic {
		return info, ErrNotImage
	}
	n := binary.LittleEndian.Uint32(head[4:])
	if n == 0 || n > maxHeaderSize {
		return info, ErrNotImage
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(f, payload); err != nil {
		return info, ErrNotImage
	}
	if err := json.Unmarshal(payload, &info); err != nil || info.Format != formatName {
		return info, ErrNotImage
	}
	if info.Version > formatVersion {
		return info, fmt.Errorf("%w: version %d is newer than supported", ErrNotImage, info.Version)
	}

	fi, err := f.Stat()
	if err != nil {
		return info, err
	}
	const tsize = 8 + trailerPayloadSize
	tail := make([]byte, tsize)
	if fi.Size() < int64(8+n)+tsize {
		return info, ErrIncomplete
	}
	if _, err := f.ReadAt(tail, fi.Size()-tsize); err != nil {
		return info, ErrIncomplete
	}
	if binary.LittleEndian.Uint32(tail) != trailerMagic || binary.LittleEndian.Uint32(tail[4:]) != trailerPayloadSize {
		return info, ErrIncomplete
	}
	var t trailer
	if err := json.Unmarshal(bytes.TrimRight(tail[8:], " "), &t); err != nil || t.SizeBytes != info.SizeBytes || len(t.SHA256) != 64 {
		return info, ErrCorrupt
	}
	info.SHA256 = t.SHA256
	return info, nil
}

// Reader decompresses an image sequentially; it implements the io.ReaderAt
// expected by clone.Copy (reads must come in order). Errors of the image data
// wrap ErrCorrupt.
type Reader struct {
	Info Info
	f    *os.File
	zr   *zstd.Decoder
	pos  int64
}

// Open opens an image for restoring.
func Open(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := readInfo(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	zr, err := zstd.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Reader{Info: info, f: f, zr: zr}, nil
}

func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	if off != r.pos {
		return 0, fmt.Errorf("non-sequential read at %d (expected %d)", off, r.pos)
	}
	n, err := io.ReadFull(r.zr, p)
	r.pos += int64(n)
	if err != nil {
		return n, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return n, nil
}

// DropCache is a no-op: an image file is verified by decompressing it.
func (r *Reader) DropCache() error { return nil }

// Size is the size of the disk the image was made from.
func (r *Reader) Size() int64 { return r.Info.SizeBytes }

func (r *Reader) Close() error {
	r.zr.Close()
	return r.f.Close()
}

// Checksum returns the SHA-256 stored in the image.
func (r *Reader) Checksum() []byte {
	sum, _ := hex.DecodeString(r.Info.SHA256)
	return sum
}

// Check must be called after all SizeBytes have been read: it reads to the
// end of the stream, so the decoder validates the zstd frame checksums
// (checked only at end of frame), and compares sum, the SHA-256 of the data
// read, with the one stored in the image.
func (r *Reader) Check(sum []byte) error {
	extra, err := io.Copy(io.Discard, r.zr)
	switch {
	case err != nil:
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	case extra != 0:
		return fmt.Errorf("%w: %d bytes beyond the declared size", ErrCorrupt, extra)
	case !bytes.Equal(sum, r.Checksum()):
		return fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
	}
	return nil
}

// VerifyFile decompresses the whole image and checks it against its stored
// SHA-256. onProgress receives the number of disk bytes decompressed.
func VerifyFile(ctx context.Context, path string, onProgress func(int64)) error {
	r, err := Open(path)
	if err != nil {
		return err
	}
	defer r.Close()
	err = clone.Verify(ctx, r, r.Info.SizeBytes, r.Checksum(), onProgress)
	if errors.Is(err, clone.ErrVerifyMismatch) {
		return fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
	}
	return err
}
