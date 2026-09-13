package engine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
	"wasmredis/internal/command"
	"wasmredis/internal/index"
	"wasmredis/internal/storage"
)

func (e *Engine) Flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.flush()
}

func (e *Engine) flush() error {
	if e.store == nil || len(e.buffer) == 0 {
		return nil
	}
	if err := e.store.AppendAOF(e.buffer); err != nil {
		return err
	}
	e.buffer = nil
	return nil
}

func (e *Engine) Snapshot() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.store == nil {
		return nil
	}
	if err := e.flush(); err != nil {
		return err
	}
	snapshot := storage.Snapshot{Sequence: e.sequence, Entries: map[string]storage.Op{}}
	for key, entry := range e.state {
		if expired(entry, e.now()) {
			continue
		}
		op := storage.Op{Kind: command.KindSet, Key: key, Value: entry.Value, IsNumber: entry.IsNumber}
		if !entry.ExpiresAt.IsZero() {
			op.ExpiresAt = entry.ExpiresAt.UnixNano()
		}
		snapshot.Entries[key] = op
	}
	if err := e.store.WriteSnapshot(snapshot); err != nil {
		return err
	}
	return e.store.ClearAOF()
}

func (e *Engine) Restore() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.store == nil {
		return nil
	}
	if e.running || e.closed || len(e.buffer) > 0 {
		return errors.New("restauration réservée au démarrage")
	}
	snapshot, err := e.store.ReadSnapshot()
	if err != nil {
		return fmt.Errorf("snapshot : %w", err)
	}
	ops, err := e.store.ReadAOF()
	if err != nil {
		return fmt.Errorf("journal : %w", err)
	}
	state := map[string]Entry{}
	for key, op := range snapshot.Entries {
		if op.Kind != command.KindSet || op.Key != key {
			return errors.New("entrée de snapshot invalide")
		}
		if err := applyOp(state, op); err != nil {
			return err
		}
	}
	sequence := snapshot.Sequence
	for _, op := range ops {
		if op.Sequence == 0 {
			return errors.New("séquence du journal invalide")
		}
		if op.Sequence <= snapshot.Sequence {
			continue
		}
		if op.Sequence <= sequence {
			continue
		}
		if op.Sequence != sequence+1 {
			return errors.New("séquence du journal interrompue")
		}
		if err := applyOp(state, op); err != nil {
			return err
		}
		sequence = op.Sequence
	}
	for key, entry := range state {
		if expired(entry, e.now()) {
			delete(state, key)
		}
	}
	idx := index.New(e.cfg.BTreeOrder)
	for key, entry := range state {
		if err := idx.Set(key, entry.Value, entry.IsNumber); err != nil {
			return err
		}
	}
	e.state, e.sequence, e.index = state, sequence, idx
	return nil
}

func applyOp(state map[string]Entry, op storage.Op) error {
	switch op.Kind {
	case command.KindSet:
		if op.IsNumber {
			n, err := strconv.ParseFloat(op.Value, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				return errors.New("nombre invalide dans le stockage")
			}
		}
		entry := Entry{Key: op.Key, Value: op.Value, IsNumber: op.IsNumber}
		if op.ExpiresAt != 0 {
			entry.ExpiresAt = time.Unix(0, op.ExpiresAt)
		}
		state[op.Key] = entry
	case command.KindDelete:
		delete(state, op.Key)
	default:
		return fmt.Errorf("opération stockée inconnue : %s", op.Kind)
	}
	return nil
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	if err := e.flush(); err != nil {
		return err
	}
	e.closed = true
	return nil
}
