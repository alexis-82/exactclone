package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.0.1", "v1.0.0", true},
		{"v1.10.0", "v1.9.9", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.0.1", false},
		{"v1.2.0-beta", "v1.1.0", true},
	}
	for _, c := range cases {
		got, err := IsNewer(c.latest, c.current)
		if err != nil || got != c.want {
			t.Errorf("IsNewer(%s, %s) = %v, %v; want %v", c.latest, c.current, got, err, c.want)
		}
	}
	for _, bad := range []string{"dev", "1.0.0", "v1.0", "v1.x.0", ""} {
		if _, err := IsNewer("v1.0.0", bad); err == nil {
			t.Errorf("expected error for current %q", bad)
		}
	}
}

func TestPickAsset(t *testing.T) {
	assets := []Asset{
		{Name: "ExactClone-1.2.0-amd64-installer.exe"},
		{Name: "exactclone-linux-amd64.tar.gz"},
		{Name: "exactclone-linux-arm64.tar.gz"},
		{Name: "notes.txt"},
	}
	if a, err := PickAsset(assets, "windows", "amd64"); err != nil || a.Name != "ExactClone-1.2.0-amd64-installer.exe" {
		t.Fatalf("windows: %v %v", a, err)
	}
	if a, err := PickAsset(assets, "linux", "arm64"); err != nil || a.Name != "exactclone-linux-arm64.tar.gz" {
		t.Fatalf("linux: %v %v", a, err)
	}
	if _, err := PickAsset(assets, "darwin", "arm64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("darwin: expected ErrNoAsset, got %v", err)
	}
	if _, err := PickAsset(assets[1:], "windows", "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("windows without setup: expected ErrNoAsset, got %v", err)
	}
}

func checker(srv *httptest.Server) *Checker {
	c := NewChecker("v1.0.0")
	c.Client = srv.Client()
	c.APIURL = srv.URL + "/api/latest"
	c.DownloadPrefix = srv.URL + "/dl/"
	c.TempDir = func() (string, error) { return os.MkdirTemp("", "exactclone-test-") }
	return c
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "ExactClone/v1.0.0" {
			t.Errorf("missing user agent: %q", r.Header.Get("User-Agent"))
		}
		fmt.Fprint(w, `{"tag_name":"v1.2.0","body":"## Novità\n- x","html_url":"https://github.com/r/1",
			"assets":[{"name":"ExactClone-1.2.0-amd64-installer.exe","size":10,"browser_download_url":"https://h/a.exe","digest":"sha256:ABCD"}]}`)
	}))
	defer srv.Close()

	r, err := checker(srv).Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "v1.2.0" || !strings.Contains(r.Notes, "Novità") || len(r.Assets) != 1 || r.Assets[0].SHA256 != "abcd" {
		t.Fatalf("unexpected release %+v", r)
	}
}

func TestLatestErrors(t *testing.T) {
	status := http.StatusNotFound
	body := `{"message":"Not Found"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	c := checker(srv)

	if _, err := c.Latest(context.Background()); !errors.Is(err, ErrNoRelease) {
		t.Fatalf("404: expected ErrNoRelease, got %v", err)
	}
	status = http.StatusForbidden // e.g. rate limit
	if _, err := c.Latest(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("403: expected generic error, got %v", err)
	}
	status, body = http.StatusOK, `{"tag_name":"release-1"}`
	if _, err := c.Latest(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("bad tag: expected ErrUnreachable, got %v", err)
	}

	srv.Close() // offline
	if _, err := c.Latest(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("offline: expected ErrUnreachable, got %v", err)
	}
}

func TestDownload(t *testing.T) {
	payload := []byte("setup-binary-content")
	sum := sha256.Sum256(payload)
	served := payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(served)
	}))
	defer srv.Close()
	c := checker(srv)
	asset := Asset{Name: "ExactClone-1.2.0-amd64-installer.exe", URL: srv.URL + "/dl/v1.2.0/ExactClone-1.2.0-amd64-installer.exe", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}

	var last int64
	path, err := c.Download(context.Background(), asset, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	got, _ := os.ReadFile(path)
	if string(got) != string(payload) || filepath.Base(path) != asset.Name || last != asset.Size {
		t.Fatalf("download mismatch: %q %s progress=%d", got, path, last)
	}

	// wrong checksum
	bad := asset
	bad.SHA256 = strings.Repeat("0", 64)
	if _, err := c.Download(context.Background(), bad, nil); !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected checksum error, got %v", err)
	}

	// size mismatch (truncated or longer body)
	for _, b := range [][]byte{payload[:5], append(append([]byte{}, payload...), 'x')} {
		served = b
		if _, err := c.Download(context.Background(), asset, nil); !errors.Is(err, ErrChecksum) {
			t.Fatalf("expected size error for %d bytes, got %v", len(b), err)
		}
	}
	served = payload

	// URL outside the allowed prefix
	evil := asset
	evil.URL = srv.URL + "/other/ExactClone-1.2.0-amd64-installer.exe"
	if _, err := c.Download(context.Background(), evil, nil); !errors.Is(err, ErrDownload) {
		t.Fatalf("expected prefix error, got %v", err)
	}

	// path traversal in the asset name
	trav := asset
	trav.Name = "../evil.exe"
	if _, err := c.Download(context.Background(), trav, nil); !errors.Is(err, ErrDownload) {
		t.Fatalf("expected invalid name error, got %v", err)
	}

	// no digest from GitHub: refused before downloading
	nodigest := asset
	nodigest.SHA256 = ""
	if _, err := c.Download(context.Background(), nodigest, nil); !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected missing digest error, got %v", err)
	}
}

func TestDefaultPrefixMatchesRepo(t *testing.T) {
	if !strings.HasPrefix("https://github.com/alexis-82/exactclone/releases/download/v1.0.0/ExactClone-1.0.0-amd64-installer.exe", DefaultDownloadPrefix) {
		t.Fatal("default prefix does not match the repo download URLs")
	}
}

// Opt-in check against the real GitHub API: EXACTCLONE_LIVE_GITHUB=1 go test ./internal/update -run Live
func TestLiveGitHub(t *testing.T) {
	if os.Getenv("EXACTCLONE_LIVE_GITHUB") != "1" {
		t.Skip("set EXACTCLONE_LIVE_GITHUB=1 to query api.github.com")
	}
	r, err := NewChecker("v0.0.0").Latest(context.Background())
	if errors.Is(err, ErrNoRelease) {
		t.Log("no release published yet")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("latest %s, assets: %d", r.Version, len(r.Assets))
	for _, a := range r.Assets {
		if !strings.HasPrefix(a.URL, DefaultDownloadPrefix) {
			t.Errorf("asset URL outside the allowed prefix: %s", a.URL)
		}
	}
}

func TestPickDeb(t *testing.T) {
	assets := []Asset{
		{Name: "exactclone-linux-amd64.tar.gz"},
		{Name: "exactclone_1.2.0_arm64.deb"},
		{Name: "exactclone_1.2.0_amd64.deb"},
	}
	if a, err := PickDeb(assets, "amd64"); err != nil || a.Name != "exactclone_1.2.0_amd64.deb" {
		t.Fatalf("amd64: %v %v", a, err)
	}
	if a, err := PickDeb(assets, "arm64"); err != nil || a.Name != "exactclone_1.2.0_arm64.deb" {
		t.Fatalf("arm64: %v %v", a, err)
	}
	if _, err := PickDeb(assets[:1], "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("no deb: expected ErrNoAsset, got %v", err)
	}
	if _, err := PickDeb([]Asset{{Name: "other_1.0.0_amd64.deb"}}, "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("other package: expected ErrNoAsset, got %v", err)
	}
}

func TestPickRpm(t *testing.T) {
	assets := []Asset{
		{Name: "exactclone-linux-amd64.tar.gz"},
		{Name: "exactclone_1.2.0_amd64.deb"},
		{Name: "exactclone-1.2.0-1.aarch64.rpm"},
		{Name: "exactclone-1.2.0-1.x86_64.rpm"},
	}
	if a, err := PickRpm(assets, "amd64"); err != nil || a.Name != "exactclone-1.2.0-1.x86_64.rpm" {
		t.Fatalf("amd64: %v %v", a, err)
	}
	if a, err := PickRpm(assets, "arm64"); err != nil || a.Name != "exactclone-1.2.0-1.aarch64.rpm" {
		t.Fatalf("arm64: %v %v", a, err)
	}
	if _, err := PickRpm(assets[:2], "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("no rpm (only tar.gz and deb): expected ErrNoAsset, got %v", err)
	}
	if _, err := PickRpm(assets, "riscv64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("unknown arch: expected ErrNoAsset, got %v", err)
	}
	if _, err := PickRpm([]Asset{{Name: "other-1.0.0-1.x86_64.rpm"}}, "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("other package: expected ErrNoAsset, got %v", err)
	}
}

// A refused download leaves nothing behind: the directory created for it is
// removed whatever the failure (checksum, size, interrupted transfer).
func TestDownloadFailureRemovesDir(t *testing.T) {
	payload := []byte("setup-binary-content")
	sum := sha256.Sum256(payload)
	var serve func(w http.ResponseWriter)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serve(w) }))
	defer srv.Close()

	parent := t.TempDir()
	c := checker(srv)
	c.TempDir = func() (string, error) { return os.MkdirTemp(parent, "dl-") }
	asset := Asset{Name: "ExactClone-1.2.0-amd64-installer.exe", URL: srv.URL + "/dl/x.exe", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}

	cases := map[string]func(w http.ResponseWriter){
		"checksum": func(w http.ResponseWriter) { w.Write([]byte("setup-binary-CONTENT")) },
		"short":    func(w http.ResponseWriter) { w.Write(payload[:3]) },
		"long":     func(w http.ResponseWriter) { w.Write(append(append([]byte{}, payload...), '!')) },
		"interrupted": func(w http.ResponseWriter) {
			// announce the full length, send half, drop the connection
			conn, buf, _ := w.(http.Hijacker).Hijack()
			fmt.Fprintf(buf, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", len(payload))
			buf.Write(payload[:len(payload)/2])
			buf.Flush()
			conn.Close()
		},
	}
	for name, fn := range cases {
		serve = fn
		if _, err := c.Download(context.Background(), asset, nil); err == nil {
			t.Fatalf("%s: download accepted", name)
		}
		if left, _ := os.ReadDir(parent); len(left) != 0 {
			t.Fatalf("%s: %d entries left in the download directory", name, len(left))
		}
	}

	serve = func(w http.ResponseWriter) { w.Write(payload) }
	path, err := c.Download(context.Background(), asset, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(path)) != parent {
		t.Fatalf("download not in the directory from TempDir: %s", path)
	}
}

// A connection that stays open but sends nothing more must not keep the
// download (and the update lock) waiting forever.
func TestDownloadStalledConnection(t *testing.T) {
	payload := []byte("setup-binary-content")
	sum := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Write(payload[:len(payload)/2])
		w.(http.Flusher).Flush()
		<-r.Context().Done() // silent until the client gives up
	}))
	defer srv.Close()

	parent := t.TempDir()
	c := checker(srv)
	c.TempDir = func() (string, error) { return os.MkdirTemp(parent, "dl-") }
	c.IdleTimeout = 200 * time.Millisecond
	asset := Asset{Name: "ExactClone-1.2.0-amd64-installer.exe", URL: srv.URL + "/dl/x.exe", Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}

	start := time.Now()
	_, err := c.Download(context.Background(), asset, nil)
	if !errors.Is(err, ErrDownload) || !strings.Contains(err.Error(), "no data") {
		t.Fatalf("expected a stalled-download error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("gave up after %s", elapsed)
	}
	if left, _ := os.ReadDir(parent); len(left) != 0 {
		t.Fatalf("%d entries left in the download directory", len(left))
	}

	// a slow but steady transfer is not cut: each read restarts the timer
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		for _, b := range payload {
			w.Write([]byte{b})
			w.(http.Flusher).Flush()
			time.Sleep(20 * time.Millisecond) // 20 x 20 ms > IdleTimeout in total
		}
	}))
	defer slow.Close()
	c.DownloadPrefix = slow.URL + "/dl/"
	c.Client = slow.Client()
	asset.URL = slow.URL + "/dl/x.exe"
	if _, err := c.Download(context.Background(), asset, nil); err != nil {
		t.Fatalf("slow but steady download refused: %v", err)
	}
}
