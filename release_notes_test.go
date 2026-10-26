package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// build/release-notes.sh gives the release (and the update dialog) the
// CHANGELOG section of the tag, and stops the release when it is missing.
func TestReleaseNotesScript(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	changelog := filepath.Join(t.TempDir(), "CHANGELOG.md")
	os.WriteFile(changelog, []byte(`# Changelog

## [1.10.0] - 2026-11-01

### Aggiunto

- Nuove funzioni della 1.10.0.

## [1.1.0] - 2026-10-26

- Aggiornamenti automatici.

## [1.0.0] - 2026-10-03

## [0.9.0] - 2026-09-01

- Vecchia.

[1.1.0]: https://github.com/alexis-82/exactclone/releases/tag/v1.1.0
[1.0.0]: https://github.com/alexis-82/exactclone/releases/tag/v1.0.0
`), 0o644)

	run := func(version string) (string, error) {
		cmd := exec.Command(sh, "build/release-notes.sh", version)
		cmd.Env = append(os.Environ(), "CHANGELOG="+changelog)
		out, err := cmd.Output()
		return string(out), err
	}

	if out, err := run("1.1.0"); err != nil || out != "- Aggiornamenti automatici.\n" {
		t.Fatalf("1.1.0: %q %v", out, err)
	}
	// 1.10.0 must not be taken for 1.1.0 (and vice versa)
	if out, err := run("1.10.0"); err != nil || !strings.HasPrefix(out, "### Aggiunto\n") || strings.Contains(out, "1.1.0") {
		t.Fatalf("1.10.0: %q %v", out, err)
	}
	// last section: the link list at the end is not part of the notes
	if out, err := run("0.9.0"); err != nil || out != "- Vecchia.\n" {
		t.Fatalf("0.9.0: %q %v", out, err)
	}
	for _, v := range []string{"1.0.0", "2.0.0", "1.1"} {
		if out, err := run(v); err == nil {
			t.Fatalf("%s: expected failure (missing or empty section), got %q", v, out)
		}
	}
}
