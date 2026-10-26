//go:build !windows

package update

import "os"

// privateTempDir creates a 0700 directory. On Linux ExactClone runs as root
// through pkexec, which clears TMPDIR: the directory is in /tmp, owned by root.
func privateTempDir() (string, error) {
	return os.MkdirTemp("", "exactclone-update-")
}
