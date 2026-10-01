package archive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ErrUnsafePath is returned when an archive entry would be written outside
// the destination folder.
var ErrUnsafePath = errors.New("unsafe path in archive")

type countingReader struct {
	r          io.Reader
	onProgress func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 && c.onProgress != nil {
		c.onProgress(int64(n))
	}
	return n, err
}

// Extract restores archivePath into destDir. onProgress receives the number of
// compressed bytes read, so the total is the archive file size. Entries that
// would escape destDir abort the extraction with ErrUnsafePath; symlinks
// pointing outside destDir are skipped with a warning. On Linux the
// extracted files are owned by the owner of destDir.
func Extract(ctx context.Context, archivePath, destDir string, onProgress func(int64)) ([]Warning, error) {
	dest, err := filepath.Abs(destDir)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := zstd.NewReader(&countingReader{r: f, onProgress: onProgress})
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	own := ownerOf(dest)
	tr := tar.NewReader(zr)
	buf := make([]byte, copyBufSize)
	var warnings []Warning
	first := true
	for {
		if ctx.Err() != nil {
			return warnings, ErrCanceled
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			return warnings, nil
		}
		if err != nil {
			return warnings, err
		}
		if first {
			first = false
			if hdr.Name != ManifestName {
				return nil, ErrNotDiskcloneArchive
			}
			continue
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return warnings, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return warnings, err
			}
			own.apply(target)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return warnings, err
			}
			if err := writeFile(ctx, target, tr, hdr, buf); err != nil {
				return warnings, err
			}
			own.apply(target)
		case tar.TypeSymlink:
			// Absolute or escaping links are common on system partitions
			// (junctions, /etc/alternatives): skip them, never create them.
			if err := checkLinkTarget(dest, target, hdr.Linkname); err != nil {
				warnings = append(warnings, Warning{Path: hdr.Name, Reason: "symlink not restored: " + err.Error()})
				continue
			}
			os.MkdirAll(filepath.Dir(target), 0o755)
			if err := os.Symlink(filepath.FromSlash(hdr.Linkname), target); err != nil {
				warnings = append(warnings, Warning{Path: hdr.Name, Reason: "symlink not restored: " + err.Error()})
				continue
			}
			own.apply(target)
		case tar.TypeLink:
			src, err := safeJoin(dest, hdr.Linkname)
			if err != nil {
				return warnings, err
			}
			os.MkdirAll(filepath.Dir(target), 0o755)
			if err := os.Link(src, target); err != nil {
				// File systems without hard links (FAT, exFAT): store a copy.
				if err := copyFile(src, target); err != nil {
					warnings = append(warnings, Warning{Path: hdr.Name, Reason: "hard link not restored: " + err.Error()})
				}
			}
		default:
			warnings = append(warnings, Warning{Path: hdr.Name, Reason: fmt.Sprintf("skipped entry type %q", hdr.Typeflag)})
		}
	}
}

// safeJoin joins an archive entry name to dest, rejecting absolute paths,
// drive letters and ".." components.
func safeJoin(dest, name string) (string, error) {
	clean := filepath.FromSlash(strings.TrimSuffix(name, "/"))
	if clean == "" || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || isRooted(name) {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
		}
	}
	target := filepath.Join(dest, clean)
	if !insideDir(dest, target) {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}
	return target, nil
}

// isRooted reports Unix roots, Windows roots and drive letters regardless of
// the running OS, so an archive is judged the same way everywhere.
func isRooted(s string) bool {
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\`) {
		return true
	}
	return len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

// checkLinkTarget rejects symlinks that are absolute or resolve outside dest.
func checkLinkTarget(dest, linkPath, linkname string) error {
	ln := filepath.FromSlash(linkname)
	if filepath.IsAbs(ln) || filepath.VolumeName(ln) != "" || isRooted(linkname) {
		return fmt.Errorf("%w: symlink %q -> %q", ErrUnsafePath, linkPath, linkname)
	}
	resolved := filepath.Join(filepath.Dir(linkPath), ln)
	if !insideDir(dest, resolved) {
		return fmt.Errorf("%w: symlink %q -> %q", ErrUnsafePath, linkPath, linkname)
	}
	return nil
}

func writeFile(ctx context.Context, target string, r io.Reader, hdr *tar.Header, buf []byte) error {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode).Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := copyCtx(ctx, f, r, buf, nil); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chtimes(target, hdr.ModTime, hdr.ModTime)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
