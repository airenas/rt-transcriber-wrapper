package service

import (
	"testing"
	"time"
)

func TestWorkerTrackerSnapshot(t *testing.T) {
	tracker := NewWorkerTracker(3)
	snapshot := tracker.Snapshot()

	if snapshot.Available != 3 {
		t.Fatalf("Available = %d, want 3", snapshot.Available)
	}
}

func TestWorkerTrackerReserveAndRelease(t *testing.T) {
	tracker := NewWorkerTracker(2)

	if err := tracker.Reserve(t.Context()); err != nil {
		t.Fatalf("Reserve() first failed: %v", err)
	}
	if err := tracker.Reserve(t.Context()); err != nil {
		t.Fatalf("Reserve() second failed: %v", err)
	}
	if err := tracker.Reserve(t.Context()); err != ErrNoWorkersAvailable {
		t.Fatalf("Reserve() third error = %v, want %v", err, ErrNoWorkersAvailable)
	}

	tracker.Release(t.Context())
	tracker.Release(t.Context())
	tracker.Release(t.Context())

	snapshot := tracker.Snapshot()
	if snapshot.Available != 2 {
		t.Fatalf("Available after release = %d, want 2", snapshot.Available)
	}
}

func TestWorkerTrackerSubscribeGetsUpdates(t *testing.T) {
	tracker := NewWorkerTracker(2)
	updates, cancel := tracker.Subscribe(t.Context())
	defer cancel()

	first := readSnapshot(t, updates)
	if first.Available != 2 {
		t.Fatalf("initial snapshot = %+v", first)
	}

	if err := tracker.Reserve(t.Context()); err != nil {
		t.Fatalf("Reserve() failed: %v", err)
	}
	second := readSnapshot(t, updates)
	if second.Available != 1 {
		t.Fatalf("reserve snapshot = %+v", second)
	}

	tracker.Release(t.Context())
	third := readSnapshot(t, updates)
	if third.Available != 2 {
		t.Fatalf("release snapshot = %+v", third)
	}
}

func readSnapshot(t *testing.T, ch <-chan WorkerSnapshot) WorkerSnapshot {
	t.Helper()
	select {
	case v, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		return v
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for snapshot")
		return WorkerSnapshot{}
	}
}
