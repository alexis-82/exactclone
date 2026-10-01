package archive

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// ManifestName is the first entry of every archive.
const ManifestName = "manifest.json"

const (
	manifestVersion = 1
	copyBufSize     = 1 << 20
)

// ErrCanceled is returned when the context is canceled during an operation.
var ErrCanceled = errors.New("operation canceled")

// ErrNotDiskcloneArchive is returned when an archive does not start with a manifest.
var ErrNotDiskcloneArchive = errors.New("not a diskclone archive (manifest missing)")

// Root is a mounted partition to back up; Name becomes its top-level folder.
type Root struct {
	Name string
	Path string
}

// Manifest describes an archive; it is stored as its first entry.
type Manifest struct {
	Version    int                 `json:"version"`
	CreatedAt  time.Time           `json:"createdAt"`
	SourceDisk string              `json:"sourceDisk"`
	Partitions []ManifestPartition `json:"partitions"`
}

type ManifestPartition struct {
	Name      string `json:"name"`
	FSType    string `json:"fsType"`
	Label     string `json:"label"`
	UsedBytes uint64 `json:"usedBytes"`
}

// Warning is a non-fatal problem with a single file (skipped, unreadable, ...).
type Warning struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Estimate returns the space needed by an archive of partitions with the
// given used bytes. Compression is ignored on purpose (safety factor 1.0).
func Estimate(usages []uint64) uint64 {
	var sum uint64
	for _, u := range usages {
		sum += u
	}
	return sum
}

// CreateFile writes an archive to outPath. On any error or cancellation the
// partial file is removed.
func CreateFile(ctx context.Context, roots []Root, m Manifest, outPath string, onProgress func(int64)) (warnings []Warning, err error) {
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
		if err != nil {
			os.Remove(outPath)
		}
	}()
	return Create(ctx, roots, m, f, onProgress)
}

// Create writes a zstd-compressed tar of roots to w: the manifest first, then
// each root under its Name folder. Symlinks and junctions are stored as links
// and never followed; FIFOs, sockets and devices are skipped without being
// opened. onProgress receives the number of file-content bytes read.
func Create(ctx context.Context, roots []Root, m Manifest, w io.Writer, onProgress func(int64)) ([]Warning, error) {
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(zw)
	c := &creator{ctx: ctx, tw: tw, onProgress: onProgress, buf: make([]byte, copyBufSize), links: map[fileKey]string{}}

	if err := c.writeManifest(m); err != nil {
		zw.Close()
		return nil, err
	}
	for _, r := range roots {
		if err := c.addRoot(r); err != nil {
			zw.Close()
			return c.warnings, err
		}
	}
	if err := tw.Close(); err != nil {
		zw.Close()
		return c.warnings, err
	}
	return c.warnings, zw.Close()
}

type creator struct {
	ctx        context.Context
	tw         *tar.Writer
	onProgress func(int64)
	buf        []byte
	links      map[fileKey]string // hard links already stored: file id -> archive name
	warnings   []Warning
}

func (c *creator) warn(p, reason string) {
	c.warnings = append(c.warnings, Warning{Path: p, Reason: reason})
}

func (c *creator) writeManifest(m Manifest) error {
	m.Version = manifestVersion
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: ManifestName, Mode: 0o644, Size: int64(len(data)), ModTime: m.CreatedAt, Typeflag: tar.TypeReg}
	if err := c.tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = c.tw.Write(data)
	return err
}

func (c *creator) addRoot(r Root) error {
	return filepath.WalkDir(r.Path, func(p string, d fs.DirEntry, walkErr error) error {
		if err := c.ctx.Err(); err != nil {
			return ErrCanceled
		}
		if walkErr != nil {
			c.warn(p, walkErr.Error())
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(r.Path, p)
		if err != nil {
			return err
		}
		name := r.Name
		if rel != "." {
			name = path.Join(r.Name, filepath.ToSlash(rel))
		}
		fi, err := d.Info()
		if err != nil {
			c.warn(p, err.Error())
			return nil
		}

		// Windows junctions/mount points: store as links, never descend.
		if rel != "." && isReparsePoint(fi) {
			c.addLink(p, name, fi)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case d.IsDir():
			return c.writeHeader(fi, name+"/", "")
		case d.Type()&fs.ModeSymlink != 0:
			c.addLink(p, name, fi)
			return nil
		case d.Type().IsRegular():
			return c.addFile(p, name, fi)
		default:
			c.warn(p, "skipped: not a regular file ("+d.Type().String()+")")
			return nil
		}
	})
}

func (c *creator) addLink(p, name string, fi fs.FileInfo) {
	target, err := os.Readlink(p)
	if err != nil {
		c.warn(p, "skipped link: "+err.Error())
		return
	}
	hdr := &tar.Header{Typeflag: tar.TypeSymlink, Name: name, Linkname: filepath.ToSlash(target), Mode: 0o777, ModTime: fi.ModTime()}
	if err := c.tw.WriteHeader(hdr); err != nil {
		c.warn(p, err.Error())
	}
}

func (c *creator) writeHeader(fi fs.FileInfo, name, link string) error {
	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Uname, hdr.Gname = "", ""
	return c.tw.WriteHeader(hdr)
}

func (c *creator) addFile(p, name string, fi fs.FileInfo) error {
	if key, ok := hardLinkKey(fi); ok {
		if first, seen := c.links[key]; seen {
			hdr, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			hdr.Typeflag, hdr.Name, hdr.Linkname, hdr.Size = tar.TypeLink, name, first, 0
			hdr.Uname, hdr.Gname = "", ""
			return c.tw.WriteHeader(hdr)
		}
		c.links[key] = name
	}
	f, err := os.Open(p)
	if err != nil {
		c.warn(p, err.Error())
		return nil
	}
	defer f.Close()
	if err := c.writeHeader(fi, name, ""); err != nil {
		return err
	}
	n, err := copyCtx(c.ctx, c.tw, io.LimitReader(f, fi.Size()), c.buf, c.onProgress)
	if err != nil {
		return err
	}
	if n < fi.Size() {
		return fmt.Errorf("%s: file shrank while reading (%d of %d bytes)", p, n, fi.Size())
	}
	return nil
}

// copyCtx copies src to dst checking ctx between chunks.
func copyCtx(ctx context.Context, dst io.Writer, src io.Reader, buf []byte, onProgress func(int64)) (int64, error) {
	var total int64
	for {
		if ctx.Err() != nil {
			return total, ErrCanceled
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return total, err
			}
			total += int64(n)
			if onProgress != nil {
				onProgress(int64(n))
			}
		}
		if rerr == io.EOF {
			return total, nil
		}
		if rerr != nil {
			return total, rerr
		}
	}
}

// ReadManifest reads only the manifest of an archive.
func ReadManifest(archivePath string) (Manifest, error) {
	var m Manifest
	f, err := os.Open(archivePath)
	if err != nil {
		return m, err
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		return m, err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	hdr, err := tr.Next()
	if err != nil || hdr.Name != ManifestName {
		return m, ErrNotDiskcloneArchive
	}
	if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&m); err != nil {
		return m, fmt.Errorf("%w: %v", ErrNotDiskcloneArchive, err)
	}
	return m, nil
}

// insideDir reports whether target (cleaned, absolute) is dir or below it.
func insideDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
