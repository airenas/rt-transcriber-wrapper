package service

import (
	"testing"
	"time"
)

func TestWorkerTrackerSnapshot(t *testing.T) {
	tracker := NewWorkerTracker(3)
	snapshot := tracker.Snapshot()

	if snapshot.WorkersAvailable != 3 {
		t.Fatalf("WorkersAvailable = %d, want 3", snapshot.WorkersAvailable)
	}
	if snapshot.RequestsProcessed != 0 {
		t.Fatalf("RequestsProcessed = %d, want 0", snapshot.RequestsProcessed)
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

	snapshot := tracker.Snapshot()
	if snapshot.RequestsProcessed != 2 {
		t.Fatalf("RequestsProcessed after release = %d, want 2", snapshot.RequestsProcessed)
	}
	if snapshot.WorkersAvailable != 2 {
		t.Fatalf("WorkersAvailable after release = %d, want 2", snapshot.WorkersAvailable)
	}
}

func TestWorkerTrackerSubscribeGetsUpdates(t *testing.T) {
	tracker := NewWorkerTracker(2)
	updates, cancel := tracker.Subscribe(t.Context())
	defer cancel()

	first := readSnapshot(t, updates)
	if first.WorkersAvailable != 2 || first.RequestsProcessed != 0 {
		t.Fatalf("initial snapshot = %+v", first)
	}

	if err := tracker.Reserve(t.Context()); err != nil {
		t.Fatalf("Reserve() failed: %v", err)
	}
	second := readSnapshot(t, updates)
	if second.WorkersAvailable != 1 || second.RequestsProcessed != 1 {
		t.Fatalf("reserve snapshot = %+v", second)
	}

	tracker.Release(t.Context())
	third := readSnapshot(t, updates)
	if third.WorkersAvailable != 2 || third.RequestsProcessed != 1 {
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