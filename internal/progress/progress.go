package progress

import (
	"sync"
	"time"
)

const (
	speedWindow  = 3 * time.Second
	emitInterval = 250 * time.Millisecond // at most 4 events per second
)

// Snapshot is the state of a job at a given instant.
type Snapshot struct {
	Done        int64   `json:"done"`
	Total       int64   `json:"total"`
	Percent     float64 `json:"percent"`
	BytesPerSec float64 `json:"bytesPerSec"`
	ETASeconds  float64 `json:"etaSeconds"` // -1 when unknown
}

type sample struct {
	t    time.Time
	done int64
}

// Tracker accumulates processed bytes and computes speed (moving average over
// the last 3 seconds) and ETA. It is safe for concurrent use.
type Tracker struct {
	mu       sync.Mutex
	now      func() time.Time
	total    int64
	done     int64
	samples  []sample
	lastEmit time.Time
}

// New returns a Tracker for a job of total bytes. now may be nil (time.Now).
func New(total int64, now func() time.Time) *Tracker {
	if now == nil {
		now = time.Now
	}
	t := &Tracker{now: now, total: total}
	t.samples = []sample{{t: now(), done: 0}}
	return t
}

// Add records n more processed bytes. emit reports whether the caller should
// publish the snapshot now (throttled to 4/s; always true when the job completes).
func (t *Tracker) Add(n int64) (snap Snapshot, emit bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.done += n
	t.samples = append(t.samples, sample{t: now, done: t.done})
	// Keep one sample at or before the window start as the reference point.
	cut := 0
	for i := 1; i < len(t.samples) && now.Sub(t.samples[i].t) >= speedWindow; i++ {
		cut = i
	}
	t.samples = t.samples[cut:]

	completed := t.total > 0 && t.done >= t.total
	if completed || t.lastEmit.IsZero() || now.Sub(t.lastEmit) >= emitInterval {
		t.lastEmit = now
		emit = true
	}
	return t.snapshot(now), emit
}

// Snapshot returns the current state without recording progress.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snapshot(t.now())
}

func (t *Tracker) snapshot(now time.Time) Snapshot {
	s := Snapshot{Done: t.done, Total: t.total, ETASeconds: -1}
	if t.total > 0 {
		s.Percent = float64(t.done) * 100 / float64(t.total)
	}
	ref := t.samples[0]
	if dt := now.Sub(ref.t).Seconds(); dt > 0 {
		s.BytesPerSec = float64(t.done-ref.done) / dt
	}
	if s.BytesPerSec > 0 && t.total > 0 {
		s.ETASeconds = float64(t.total-t.done) / s.BytesPerSec
	}
	return s
}
