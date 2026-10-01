package privilege

import "golang.org/x/sys/windows"

// IsElevated reports whether the process runs with an elevated admin token.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
