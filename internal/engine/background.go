package engine

import (
	"context"
	"errors"
	"time"
)

func (e *Engine) SweepExpired() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return 0
	}
	removed := 0
	now := e.now()
	for key, entry := range e.state {
		if expired(entry, now) {
			e.remove(key)
			removed++
		}
	}
	return removed
}

func (e *Engine) StartBackground(ctx context.Context) error {
	e.mu.Lock()
	if e.running || e.closed {
		e.mu.Unlock()
		return errors.New("moteur fermé ou tâches de fond déjà lancées")
	}
	if err := e.cfg.Validate(); err != nil {
		e.mu.Unlock()
		return err
	}
	e.running = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.running = false; e.mu.Unlock() }()
	flush := time.NewTicker(e.cfg.FlushInterval)
	snapshot := time.NewTicker(e.cfg.SnapshotInterval)
	expiry := time.NewTicker(e.cfg.ExpiryScanInterval)
	defer flush.Stop()
	defer snapshot.Stop()
	defer expiry.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-flush.C:
			if err := e.Flush(); err != nil {
				return err
			}
		case <-snapshot.C:
			if err := e.Snapshot(); err != nil {
				return err
			}
		case <-expiry.C:
			e.SweepExpired()
		}
	}
}
