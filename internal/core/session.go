package core

import (
	"context"
	"sync"
)

// LocalSession is owned by the connector, never exposed through agent MCP.
// Its generation invalidates commands fetched before Pause/Resume or reconnect.
type LocalSession struct {
	mu              sync.Mutex
	epoch           string
	paused, stopped bool
	active          context.CancelFunc
	wake            chan struct{}
}

func NewLocalSession(paused bool) *LocalSession {
	return &LocalSession{epoch: Secret()[:24], paused: paused, wake: make(chan struct{}, 1)}
}
func (s *LocalSession) Snapshot() (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch, s.paused, s.stopped
}
func (s *LocalSession) Change(action string) {
	s.mu.Lock()
	switch action {
	case "pause":
		s.paused = true
	case "resume":
		if !s.stopped {
			s.paused = false
		}
	case "stop":
		s.stopped = true
		s.paused = true
	default:
		s.mu.Unlock()
		return
	}
	s.epoch = Secret()[:24]
	if s.active != nil {
		s.active()
		s.active = nil
	}
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *LocalSession) Begin(ctx context.Context, epoch string) (context.Context, context.CancelFunc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused || s.stopped || epoch != s.epoch {
		return nil, nil, false
	}
	work, cancel := context.WithCancel(ctx)
	s.active = cancel
	return work, cancel, true
}
func (s *LocalSession) End() { s.mu.Lock(); s.active = nil; s.mu.Unlock() }
