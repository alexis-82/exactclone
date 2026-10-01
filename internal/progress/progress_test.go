package progress

import (
	"math"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestSpeedAndETA(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	tr := New(1000, c.now)
	for i := 0; i < 4; i++ { // 100 bytes per second for 4 s
		c.advance(time.Second)
		tr.Add(100)
	}
	s := tr.Snapshot()
	if s.Done != 400 || !near(s.Percent, 40) {
		t.Fatalf("done/percent = %d/%v", s.Done, s.Percent)
	}
	if !near(s.BytesPerSec, 100) {
		t.Fatalf("speed = %v, want 100", s.BytesPerSec)
	}
	if !near(s.ETASeconds, 6) {
		t.Fatalf("eta = %v, want 6", s.ETASeconds)
	}
}

func TestSpeedUsesMovingWindow(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	tr := New(10000, c.now)
	for i := 0; i < 5; i++ { // slow phase: 10 B/s
		c.advance(time.Second)
		tr.Add(10)
	}
	for i := 0; i < 5; i++ { // fast phase: 1000 B/s
		c.advance(time.Second)
		tr.Add(1000)
	}
	if s := tr.Snapshot(); !near(s.BytesPerSec, 1000) {
		t.Fatalf("speed = %v, want 1000 (only last 3 s)", s.BytesPerSec)
	}
}

func TestETAUnknownWithoutProgress(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	tr := New(100, c.now)
	if s := tr.Snapshot(); s.ETASeconds != -1 {
		t.Fatalf("eta = %v, want -1", s.ETASeconds)
	}
}

func TestEmitThrottling(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	tr := New(1_000_000, c.now)
	emits := 0
	for i := 0; i < 100; i++ { // 100 updates over 1 s
		c.advance(10 * time.Millisecond)
		if _, e := tr.Add(1); e {
			emits++
		}
	}
	if emits < 4 || emits > 5 {
		t.Fatalf("emits = %d, want 4-5 per second", emits)
	}
}

func TestEmitOnCompletion(t *testing.T) {
	c := &fakeClock{t: time.Unix(0, 0)}
	tr := New(20, c.now)
	tr.Add(10) // first update always emits
	c.advance(time.Millisecond)
	if _, e := tr.Add(10); !e {
		t.Fatal("completion must always emit")
	}
}
