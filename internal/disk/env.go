package disk

import "os"

// DevSafe reports whether safe development mode is on: EXACTCLONE_DEV_SAFE=1
// (DISKCLONE_DEV_SAFE=1, the name used before the rename, is still accepted).
func DevSafe() bool {
	return os.Getenv("EXACTCLONE_DEV_SAFE") == "1" || os.Getenv("DISKCLONE_DEV_SAFE") == "1"
}
