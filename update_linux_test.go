//go:build linux

package main

import (
	"errors"
	"testing"
)

func TestOwnedByDeb(t *testing.T) {
	answer := func(out string, err error) func(string) (string, error) {
		return func(string) (string, error) { return out, err }
	}
	cases := []struct {
		name  string
		exe   string
		query func(string) (string, error)
		want  bool
	}{
		{"owned", "/usr/bin/exactclone", answer("exactclone: /usr/bin/exactclone\n", nil), true},
		{"other package", "/usr/bin/exactclone", answer("other: /usr/bin/exactclone\n", nil), false},
		{"not owned", "/usr/bin/exactclone", answer("", errors.New("exit status 1")), false},
		{"no dpkg", "/usr/bin/exactclone", answer("", errors.New("executable file not found")), false},
		{"other path", "/home/u/exactclone", answer("exactclone: /home/u/exactclone\n", nil), false},
	}
	for _, c := range cases {
		if got := ownedByDeb(c.exe, c.query); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOwnedByRpm(t *testing.T) {
	answer := func(out string, err error) func(string) (string, error) {
		return func(string) (string, error) { return out, err }
	}
	if !ownedByRpm("/usr/bin/exactclone", answer("exactclone", nil)) {
		t.Error("owned: expected true")
	}
	if ownedByRpm("/usr/bin/exactclone", answer("file /usr/bin/exactclone is not owned by any package\n", errors.New("exit status 1"))) {
		t.Error("not owned: expected false")
	}
	if ownedByRpm("/usr/bin/exactclone", answer("", errors.New("executable file not found"))) {
		t.Error("no rpm: expected false")
	}
	if ownedByRpm("/opt/exactclone", answer("exactclone", nil)) {
		t.Error("other path: expected false")
	}
}
