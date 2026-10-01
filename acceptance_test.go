package main

// Behavior tests for the acceptance criteria of .claude/spec.md that can run
// without touching real disks: disks are simulated by regular files.

import (
	"sync"
	"testing"

	"diskclone/internal/job"
	"diskclone/internal/privilege"
)

type events struct {
	mu       sync.Mutex
	done     []job.DoneEvent
	progress []job.ProgressEvent
	onProg   func()
}

func (e *events) emit(name string, data any) {
	e.mu.Lock()
	switch d := data.(type) {
	case job.DoneEvent:
		e.done = append(e.done, d)
	case job.ProgressEvent:
		e.progress = append(e.progress, d)
	}
	cb := e.onProg
	e.mu.Unlock()
	if name == job.EventProgress && cb != nil {
		cb()
	}
}

func (e *events) last() job.DoneEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.done[len(e.done)-1]
}

// CA8: without privileges the operations are refused up front with a clear
// code instead of failing during the copy.
func TestCA8RefusedWithoutPrivileges(t *testing.T) {
	if privilege.IsElevated() {
		t.Skip("running elevated")
	}
	a := &App{}
	if err := a.StartClone("x", "y", true); err == nil || err.Error() != "not_elevated" {
		t.Fatalf("StartClone err = %v, want not_elevated", err)
	}
}
