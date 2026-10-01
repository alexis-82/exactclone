package job

import (
	"context"
	"errors"
	"sync"

	"diskclone/internal/archive"
	"diskclone/internal/clone"
	"diskclone/internal/progress"
)

// Event names emitted to the frontend.
const (
	EventProgress = "job:progress"
	EventDone     = "job:done"
)

// Status of the last job.
const (
	StatusIdle     = "idle"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// ErrBusy is returned when a job is started while another one is running.
var ErrBusy = errors.New("busy")

// Emitter publishes an event to the frontend.
type Emitter func(event string, data any)

// ProgressEvent is the payload of EventProgress.
type ProgressEvent struct {
	Kind  string `json:"kind"`
	Phase string `json:"phase"`
	progress.Snapshot
}

// Result is what a job returns on success.
type Result struct {
	Warnings []archive.Warning `json:"warnings"`
	Notices  []string          `json:"notices"` // codes of informational messages
}

// DoneEvent is the payload of EventDone.
type DoneEvent struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Result
}

// Reporter is handed to a running job to publish its progress.
type Reporter struct {
	m     *Manager
	kind  string
	phase string
	t     *progress.Tracker
}

// Phase starts a new phase (e.g. "copy", "verify") of total bytes.
func (r *Reporter) Phase(name string, total int64) {
	r.phase = name
	r.t = progress.New(total, nil)
	r.m.emit(EventProgress, ProgressEvent{Kind: r.kind, Phase: name, Snapshot: r.t.Snapshot()})
}

// Add records n processed bytes of the current phase.
func (r *Reporter) Add(n int64) {
	if snap, emit := r.t.Add(n); emit {
		r.m.emit(EventProgress, ProgressEvent{Kind: r.kind, Phase: r.phase, Snapshot: snap})
	}
}

// Func is the body of a job.
type Func func(ctx context.Context, r *Reporter) (Result, error)

// Manager runs one job at a time.
type Manager struct {
	mu     sync.Mutex
	emit   Emitter
	status string
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(emit Emitter) *Manager {
	return &Manager{emit: emit, status: StatusIdle}
}

// Start runs fn in the background. It fails with ErrBusy if a job is running.
func (m *Manager) Start(kind string, fn Func) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status == StatusRunning {
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.status, m.cancel, m.done = StatusRunning, cancel, make(chan struct{})
	r := &Reporter{m: m, kind: kind, t: progress.New(0, nil)}
	go func() {
		defer close(m.done)
		res, err := fn(ctx, r)
		ev := DoneEvent{Kind: kind, Result: res}
		switch {
		case err == nil:
			ev.Status = StatusDone
		case ctx.Err() != nil || errors.Is(err, clone.ErrCanceled) || errors.Is(err, archive.ErrCanceled):
			ev.Status = StatusCanceled
		default:
			ev.Status, ev.Error = StatusFailed, err.Error()
		}
		m.mu.Lock()
		m.status = ev.Status
		cancel()
		m.mu.Unlock()
		m.emit(EventDone, ev)
	}()
	return nil
}

// Cancel asks the running job to stop; it has no effect when idle.
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status == StatusRunning {
		m.cancel()
	}
}

// Status returns the status of the current or last job.
func (m *Manager) Status() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Wait blocks until the current job (if any) has finished.
func (m *Manager) Wait() {
	m.mu.Lock()
	done := m.done
	m.mu.Unlock()
	if done != nil {
		<-done
	}
}
