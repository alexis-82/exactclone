package job

import (
	"context"
	"errors"
	"sync"
	"testing"

	"diskclone/internal/clone"
)

type recorder struct {
	mu     sync.Mutex
	events []string
	done   []DoneEvent
	prog   []ProgressEvent
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
	switch d := data.(type) {
	case DoneEvent:
		r.done = append(r.done, d)
	case ProgressEvent:
		r.prog = append(r.prog, d)
	}
}

func TestJobLifecycle(t *testing.T) {
	rec := &recorder{}
	m := NewManager(rec.emit)
	if m.Status() != StatusIdle {
		t.Fatal("new manager must be idle")
	}
	release := make(chan struct{})
	err := m.Start("clone", func(ctx context.Context, r *Reporter) (Result, error) {
		r.Phase("copy", 100)
		r.Add(100)
		<-release
		return Result{Notices: []string{"clone_offline"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status() != StatusRunning {
		t.Fatal("status must be running")
	}
	if err := m.Start("archive", nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("second start: %v, want ErrBusy", err)
	}
	close(release)
	m.Wait()
	if m.Status() != StatusDone {
		t.Fatalf("status = %s", m.Status())
	}
	if len(rec.done) != 1 || rec.done[0].Status != StatusDone || rec.done[0].Notices[0] != "clone_offline" {
		t.Fatalf("done events = %+v", rec.done)
	}
	last := rec.prog[len(rec.prog)-1]
	if last.Phase != "copy" || last.Done != 100 || last.Kind != "clone" {
		t.Fatalf("last progress = %+v", last)
	}
	// A new job can start once the previous one has finished.
	if err := m.Start("archive", func(context.Context, *Reporter) (Result, error) { return Result{}, nil }); err != nil {
		t.Fatal(err)
	}
	m.Wait()
}

func TestJobCancel(t *testing.T) {
	rec := &recorder{}
	m := NewManager(rec.emit)
	started := make(chan struct{})
	m.Start("clone", func(ctx context.Context, r *Reporter) (Result, error) {
		close(started)
		<-ctx.Done()
		return Result{}, clone.ErrCanceled
	})
	<-started
	m.Cancel()
	m.Wait()
	if m.Status() != StatusCanceled || rec.done[0].Status != StatusCanceled {
		t.Fatalf("status = %s, event = %+v", m.Status(), rec.done)
	}
}

func TestJobFailure(t *testing.T) {
	rec := &recorder{}
	m := NewManager(rec.emit)
	m.Start("restore", func(context.Context, *Reporter) (Result, error) {
		return Result{}, errors.New("insufficient_space")
	})
	m.Wait()
	if m.Status() != StatusFailed || rec.done[0].Error != "insufficient_space" {
		t.Fatalf("status = %s, event = %+v", m.Status(), rec.done)
	}
}

func TestCancelWhenIdleIsNoop(t *testing.T) {
	m := NewManager(func(string, any) {})
	m.Cancel()
	if m.Status() != StatusIdle {
		t.Fatal("cancel must not change an idle manager")
	}
}
