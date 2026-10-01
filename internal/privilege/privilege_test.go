package privilege

import (
	"os"
	"testing"
)

// DISKCLONE_EXPECT_ELEVATED=1 when running the tests from an elevated shell.
func TestIsElevated(t *testing.T) {
	want := os.Getenv("DISKCLONE_EXPECT_ELEVATED") == "1"
	if got := IsElevated(); got != want {
		t.Fatalf("IsElevated() = %v, want %v", got, want)
	}
}
