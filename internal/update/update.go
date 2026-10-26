// Package update checks GitHub Releases for a newer ExactClone version and
// downloads the release asset for the current platform. It knows nothing
// about Wails or about how the asset is installed.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	Repo                  = "alexis-82/exactclone"
	DefaultAPIURL         = "https://api.github.com/repos/" + Repo + "/releases/latest"
	DefaultDownloadPrefix = "https://github.com/" + Repo + "/releases/download/"
)

var (
	// ErrNoRelease: the repository has no published release yet.
	ErrNoRelease = errors.New("no release published")
	// ErrNoAsset: the release has no file for this operating system.
	ErrNoAsset = errors.New("the release has no file for this system")
	// ErrUnreachable: GitHub could not be contacted or answered badly.
	ErrUnreachable = errors.New("github unreachable")
	// ErrDownload: the download failed or the URL/name is not allowed.
	ErrDownload = errors.New("download failed")
	// ErrChecksum: the downloaded file differs from the release (size, SHA-256).
	ErrChecksum = errors.New("checksum mismatch")
)

type Asset struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string // hex, empty if GitHub did not provide a digest (refused by Download)
}

type Release struct {
	Version string // tag, e.g. v1.2.3
	Notes   string // release body (Markdown)
	PageURL string
	Assets  []Asset
}

type Checker struct {
	Client *http.Client
	APIURL string
	// DownloadPrefix: assets are downloaded only from URLs starting with it.
	DownloadPrefix string
	UserAgent      string
	// TempDir creates the directory that receives a download (private to
	// root/Administrators by default; tests replace it).
	TempDir func() (string, error)
	// IdleTimeout cancels a download that receives no data for this long:
	// a stalled connection would otherwise keep the update (and the disk
	// jobs it excludes) waiting forever.
	IdleTimeout time.Duration
}

func NewChecker(currentVersion string) *Checker {
	return &Checker{
		// No overall timeout: it would cut large downloads; requests use ctx
		// and downloads IdleTimeout.
		Client:         &http.Client{},
		APIURL:         DefaultAPIURL,
		DownloadPrefix: DefaultDownloadPrefix,
		UserAgent:      "ExactClone/" + currentVersion,
		TempDir:        privateTempDir,
		IdleTimeout:    30 * time.Second,
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"` // "sha256:<hex>" on recent uploads
	} `json:"assets"`
}

// Latest returns the latest published (non draft, non prerelease) release.
func (c *Checker) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.Client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("%w: %s", ErrUnreachable, resp.Status)
	}
	var gr ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&gr); err != nil {
		return Release{}, fmt.Errorf("%w: invalid response: %v", ErrUnreachable, err)
	}
	if _, err := parseVersion(gr.TagName); err != nil {
		return Release{}, fmt.Errorf("%w: invalid latest tag %q (expected vX.Y.Z)", ErrUnreachable, gr.TagName)
	}
	r := Release{Version: gr.TagName, Notes: gr.Body, PageURL: gr.HTMLURL}
	for _, a := range gr.Assets {
		sum := ""
		if d, ok := strings.CutPrefix(a.Digest, "sha256:"); ok {
			sum = strings.ToLower(d)
		}
		r.Assets = append(r.Assets, Asset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size, SHA256: sum})
	}
	return r, nil
}

// parseVersion parses "vX.Y.Z" (a "-suffix" is ignored).
func parseVersion(v string) ([3]int, error) {
	var out [3]int
	s, ok := strings.CutPrefix(strings.TrimSpace(v), "v")
	if !ok {
		return out, fmt.Errorf("version without v prefix: %q", v)
	}
	s, _, _ = strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid version: %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("invalid version: %q", v)
		}
		out[i] = n
	}
	return out, nil
}

// IsNewer reports whether latest is a higher version than current.
func IsNewer(latest, current string) (bool, error) {
	l, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	c, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i], nil
		}
	}
	return false, nil
}

// PickAsset returns the release file to install on goos/goarch:
// Windows "*-installer.exe", Linux "exactclone-linux-<goarch>.tar.gz".
func PickAsset(assets []Asset, goos, goarch string) (Asset, error) {
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		switch goos {
		case "windows":
			if strings.HasSuffix(name, "-installer.exe") {
				return a, nil
			}
		case "linux":
			if name == "exactclone-linux-"+goarch+".tar.gz" {
				return a, nil
			}
		}
	}
	return Asset{}, ErrNoAsset
}

// PickDeb returns the Debian package for goarch, named by nfpm
// "exactclone_<version>_<goarch>.deb".
func PickDeb(assets []Asset, goarch string) (Asset, error) {
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "exactclone_") && strings.HasSuffix(name, "_"+goarch+".deb") {
			return a, nil
		}
	}
	return Asset{}, ErrNoAsset
}

// rpmArch maps Go architectures to the names rpm (and nfpm) use.
var rpmArch = map[string]string{"amd64": "x86_64", "arm64": "aarch64"}

// PickRpm returns the Fedora package for goarch, named by nfpm
// "exactclone-<version>-<release>.<rpmarch>.rpm".
func PickRpm(assets []Asset, goarch string) (Asset, error) {
	arch, ok := rpmArch[goarch]
	if !ok {
		return Asset{}, ErrNoAsset
	}
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "exactclone-") && strings.HasSuffix(name, "."+arch+".rpm") {
			return a, nil
		}
	}
	return Asset{}, ErrNoAsset
}

// Download saves the asset in a new private temporary directory and verifies
// its size and SHA-256. ExactClone runs elevated and starts what it
// downloads, so an asset without a digest is refused. progress may be nil.
func (c *Checker) Download(ctx context.Context, a Asset, progress func(done, total int64)) (string, error) {
	if !strings.HasPrefix(a.URL, c.DownloadPrefix) {
		return "", fmt.Errorf("%w: URL not allowed: %s", ErrDownload, a.URL)
	}
	if a.Name == "" || a.Name != filepath.Base(a.Name) {
		return "", fmt.Errorf("%w: invalid file name %q", ErrDownload, a.Name)
	}
	if len(a.SHA256) != sha256.Size*2 {
		return "", fmt.Errorf("%w: the release has no SHA-256 for %s", ErrChecksum, a.Name)
	}
	// The timer covers the wait for the response headers and every read.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := time.AfterFunc(c.IdleTimeout, cancel)
	defer idle.Stop()
	stalled := func(err error) error {
		if !idle.Stop() && ctx.Err() != nil {
			return fmt.Errorf("%w: no data for %s", ErrDownload, c.IdleTimeout)
		}
		return fmt.Errorf("%w: %v", ErrDownload, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", stalled(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %s", ErrDownload, resp.Status)
	}

	dir, err := c.TempDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, a.Name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	fail := func(err error) (string, error) {
		f.Close()
		os.RemoveAll(dir)
		return "", err
	}

	h := sha256.New()
	var done int64
	buf := make([]byte, 64*1024)
	// Never read more than the announced size (+1 to detect a longer body).
	body := io.LimitReader(resp.Body, a.Size+1)
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			idle.Reset(c.IdleTimeout)
			if _, err := f.Write(buf[:n]); err != nil {
				return fail(err)
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, a.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fail(stalled(rerr))
		}
	}
	if done != a.Size {
		return fail(fmt.Errorf("%w: %d bytes instead of %d", ErrChecksum, done, a.Size))
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return fail(fmt.Errorf("%w: SHA-256 differs from the release", ErrChecksum))
	}
	if err := f.Close(); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return path, nil
}
