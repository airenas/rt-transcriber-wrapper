package service

import (
	"context"
	"errors"
	"sync"

	"github.com/rs/zerolog/log"
)

var ErrNoWorkersAvailable = errors.New("no workers available")

type WorkerSnapshot struct {
	Available int `json:"num_workers_available"`
}

// WorkerTracker keeps worker usage state and broadcasts changes to subscribers.
type WorkerTracker struct {
	mu          sync.RWMutex
	maxWorkers  int
	usedWorkers int
	nextSubID   uint64
	subscribers map[uint64]chan WorkerSnapshot
}

func NewWorkerTracker(maxWorkers int) *WorkerTracker {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	return &WorkerTracker{
		maxWorkers:  maxWorkers,
		subscribers: map[uint64]chan WorkerSnapshot{},
	}
}

func (wt *WorkerTracker) Reserve(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("reserving worker")
	wt.mu.Lock()
	if wt.usedWorkers >= wt.maxWorkers {
		wt.mu.Unlock()
		return ErrNoWorkersAvailable
	}
	wt.usedWorkers++
	snapshot := wt.snapshotLocked()
	subs := wt.copySubscribersLocked()
	wt.mu.Unlock()
	wt.broadcast(snapshot, subs)
	return nil
}

func (wt *WorkerTracker) Release(ctx context.Context) {
	log.Ctx(ctx).Debug().Msg("releasing worker")
	wt.mu.Lock()
	if wt.usedWorkers == 0 {
		wt.mu.Unlock()
		return
	}
	wt.usedWorkers--
	snapshot := wt.snapshotLocked()
	subs := wt.copySubscribersLocked()
	wt.mu.Unlock()
	wt.broadcast(snapshot, subs)
}

func (wt *WorkerTracker) Snapshot() WorkerSnapshot {
	wt.mu.RLock()
	defer wt.mu.RUnlock()
	return wt.snapshotLocked()
}

func (wt *WorkerTracker) Subscribe(ctx context.Context) (<-chan WorkerSnapshot, func()) {
	log.Ctx(ctx).Debug().Msg("subscribing to worker tracker")
	wt.mu.Lock()
	defer wt.mu.Unlock()

	id := wt.nextSubID
	wt.nextSubID++

	ch := make(chan WorkerSnapshot, 4)
	wt.subscribers[id] = ch

	ch <- wt.snapshotLocked()

	cancel := func() {
		wt.mu.Lock()
		defer wt.mu.Unlock()
		sub, ok := wt.subscribers[id]
		if !ok {
			return
		}
		delete(wt.subscribers, id)
		close(sub)
	}

	return ch, cancel
}

func (wt *WorkerTracker) snapshotLocked() WorkerSnapshot {
	return WorkerSnapshot{
		Available: wt.maxWorkers - wt.usedWorkers,
	}
}

func (wt *WorkerTracker) copySubscribersLocked() []chan WorkerSnapshot {
	res := make([]chan WorkerSnapshot, 0, len(wt.subscribers))
	for _, ch := range wt.subscribers {
		res = append(res, ch)
	}
	return res
}

func (wt *WorkerTracker) broadcast(snapshot WorkerSnapshot, subs []chan WorkerSnapshot) {
	for _, sub := range subs {
		select {
		case sub <- snapshot:
			continue
		default:
		}
		select {
		case <-sub:
		default:
		}
		select {
		case sub <- snapshot:
		default:
		}
	}
}
