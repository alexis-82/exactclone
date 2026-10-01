package disk

import (
	"strings"
	"testing"
)

// A volume that cannot be opened or locked must give an error, not a panic
// (the error path used to call a nil release function). Runs without admin.
func TestPrepareFailsCleanly(t *testing.T) {
	missing := Disk{ID: "fake", Partitions: []Partition{{
		ID:          "fake1",
		Path:        `\\?\Volume{00000000-0000-0000-0000-000000000000}\`,
		MountPoints: []string{`Z:\`},
	}}}
	release, err := PrepareForRead(missing)
	if err == nil || release != nil {
		t.Fatalf("PrepareForRead = %v, %v; want an error and no release", release != nil, err)
	}
	if !strings.Contains(err.Error(), `Z:\`) {
		t.Fatalf("error should name the volume: %v", err)
	}
	if _, err := PrepareForWrite(Disk{}, missing); err == nil {
		t.Fatal("PrepareForWrite must fail too")
	}
}
