package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"exactclone/internal/job"
	ghupdate "exactclone/internal/update"
)

const fakeAsset = "ExactClone-1.1.0-amd64-installer.exe"

// fakeGitHub serves a "latest release" with one installer asset.
type fakeGitHub struct {
	srv       *httptest.Server
	tag       string
	payload   []byte // what /dl/ serves
	digest    []byte // what the release announces
	status    int
	downloads atomic.Int32
}

func newFakeGitHub(t *testing.T, tag string) *fakeGitHub {
	f := &fakeGitHub{tag: tag, payload: []byte("installer"), status: http.StatusOK}
	f.digest = f.payload
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/dl/") {
			f.downloads.Add(1)
			w.Write(f.payload)
			return
		}
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			return
		}
		sum := sha256.Sum256(f.digest)
		fmt.Fprintf(w, `{"tag_name":%q,"body":"Note","html_url":"https://github.com/alexis-82/exactclone/releases/tag/%s","assets":[{"name":%q,"size":%d,"browser_download_url":%q,"digest":"sha256:%s"}]}`,
			f.tag, f.tag, fakeAsset, len(f.digest), f.srv.URL+"/dl/"+fakeAsset, hex.EncodeToString(sum[:]))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func withVersion(t *testing.T, v string) {
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

// updatableApp is an App whose updater talks to the fake server and records
// installer launches and quits instead of performing them.
type updatableApp struct {
	*App
	installed []string
	quits     int
}

func newUpdatableApp(t *testing.T, gh *fakeGitHub) *updatableApp {
	noEmit := func(string, any) {}
	ua := &updatableApp{App: &App{ctx: context.Background(), emit: noEmit, jobs: job.NewManager(noEmit)}}
	c := ghupdate.NewChecker(version)
	c.Client = gh.srv.Client()
	c.APIURL = gh.srv.URL + "/api"
	c.DownloadPrefix = gh.srv.URL + "/dl/"
	c.TempDir = func() (string, error) { return os.MkdirTemp(t.TempDir(), "dl-") }
	ua.updates = updater{
		checker: c,
		pick:    func(a []ghupdate.Asset) (ghupdate.Asset, error) { return ghupdate.PickAsset(a, "windows", "amd64") },
		install: func(p string) error {
			data, _ := os.ReadFile(p)
			ua.installed = append(ua.installed, string(data))
			os.RemoveAll(filepath.Dir(p))
			return nil
		},
		quit: func(context.Context) { ua.quits++ },
	}
	return ua
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || strings.SplitN(err.Error(), "|", 2)[0] != code {
		t.Fatalf("expected error %s, got %v", code, err)
	}
}

func TestUpdate_CheckStates(t *testing.T) {
	withVersion(t, "v1.0.0")
	gh := newFakeGitHub(t, "v1.1.0")
	a := newUpdatableApp(t, gh)

	info, err := a.CheckUpdate()
	if err != nil || !info.Available || !info.CanInstall || info.Latest != "v1.1.0" || info.Current != "v1.0.0" || info.Notes != "Note" || info.PageURL == "" {
		t.Fatalf("newer release: %+v %v", info, err)
	}

	gh.tag = "v1.0.0"
	if info, _ := a.CheckUpdate(); info.Available || info.CanInstall {
		t.Fatalf("same version must not be offered: %+v", info)
	}

	withVersion(t, "dev")
	gh.tag = "v9.9.9"
	if info, _ := a.CheckUpdate(); info.Available {
		t.Fatalf("dev build must not be offered updates: %+v", info)
	}

	gh.status = http.StatusNotFound
	if info, err := a.CheckUpdate(); err != nil || !info.NoRelease {
		t.Fatalf("no release yet: %+v %v", info, err)
	}

	gh.srv.Close()
	_, err = a.CheckUpdate()
	wantCode(t, err, "github_unreachable")
}

// Portable exe (no installer to run): offered, but only as a link.
func TestUpdate_NoInstallableAsset(t *testing.T) {
	withVersion(t, "v1.0.0")
	a := newUpdatableApp(t, newFakeGitHub(t, "v1.1.0"))
	a.updates.pick = func([]ghupdate.Asset) (ghupdate.Asset, error) { return ghupdate.Asset{}, ghupdate.ErrNoAsset }
	info, err := a.CheckUpdate()
	if err != nil || !info.Available || info.CanInstall {
		t.Fatalf("portable: %+v %v", info, err)
	}
	wantCode(t, a.InstallUpdate(), "update_no_asset")
}

func TestUpdate_InstallLaunchesAndQuits(t *testing.T) {
	withVersion(t, "v1.0.0")
	a := newUpdatableApp(t, newFakeGitHub(t, "v1.1.0"))
	if _, err := a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := a.InstallUpdate(); err != nil {
		t.Fatal(err)
	}
	if len(a.installed) != 1 || a.installed[0] != "installer" || a.quits != 1 {
		t.Fatalf("installed=%v quits=%d", a.installed, a.quits)
	}
	// The app is quitting: no disk job may start before it exits.
	wantCode(t, a.start("clone", func(context.Context, *job.Reporter) (job.Result, error) { return job.Result{}, nil }), "busy")
	wantCode(t, a.InstallUpdate(), "update_busy")
}

func TestUpdate_CheckFirst(t *testing.T) {
	withVersion(t, "v1.0.0")
	a := newUpdatableApp(t, newFakeGitHub(t, "v1.1.0"))
	wantCode(t, a.InstallUpdate(), "update_check_first")
	// a failed install releases the lock
	if err := a.start("clone", func(context.Context, *job.Reporter) (job.Result, error) { return job.Result{}, nil }); err != nil {
		t.Fatalf("job refused after a failed update: %v", err)
	}
	a.jobs.Wait()
}

func TestUpdate_CorruptDownloadNotInstalled(t *testing.T) {
	withVersion(t, "v1.0.0")
	gh := newFakeGitHub(t, "v1.1.0")
	gh.payload = []byte("tampered")
	gh.digest = []byte("original")
	a := newUpdatableApp(t, gh)
	if _, err := a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	wantCode(t, a.InstallUpdate(), "update_checksum")
	if len(a.installed) != 0 || a.quits != 0 {
		t.Fatalf("corrupt file installed: %v quits=%d", a.installed, a.quits)
	}
}

func TestUpdate_ConcurrentInstallRejected(t *testing.T) {
	withVersion(t, "v1.0.0")
	a := newUpdatableApp(t, newFakeGitHub(t, "v1.1.0"))
	a.updates.busy = true
	wantCode(t, a.InstallUpdate(), "update_busy")
}

// No update while a disk job runs: nothing is downloaded.
func TestUpdate_RefusedWhileJobRuns(t *testing.T) {
	withVersion(t, "v1.0.0")
	gh := newFakeGitHub(t, "v1.1.0")
	a := newUpdatableApp(t, gh)
	if _, err := a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	if err := a.start("clone", func(context.Context, *job.Reporter) (job.Result, error) {
		<-release
		return job.Result{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	wantCode(t, a.InstallUpdate(), "update_job_running")
	close(release)
	a.jobs.Wait()
	if gh.downloads.Load() != 0 || len(a.installed) != 0 || a.quits != 0 {
		t.Fatalf("update went on during a job: downloads=%d installed=%v quits=%d", gh.downloads.Load(), a.installed, a.quits)
	}
	// once the job is over the update proceeds
	if err := a.InstallUpdate(); err != nil {
		t.Fatal(err)
	}
}
