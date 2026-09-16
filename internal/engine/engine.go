package engine

import (
	"errors"
	"math"
	"strconv"
	"time"
	"wasmredis/internal/command"
	"wasmredis/internal/config"
	"wasmredis/internal/index"
	"wasmredis/internal/storage"
)

var ErrNotFound = errors.New("clé absente")
var ErrClosed = errors.New("moteur fermé")

func New() *Engine {
	cfg := config.Default()
	return &Engine{state: make(map[string]Entry), now: time.Now, cfg: cfg, index: index.New(cfg.BTreeOrder)}
}

func NewWithStorage(store storage.Storage, cfg config.Config) (*Engine, error) {
	if store == nil {
		return nil, errors.New("stockage requis")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	e := New()
	e.store, e.cfg = store, cfg
	if err := e.Restore(); err != nil {
		return nil, err
	}
	return e, nil
}

func NewMemory(cfg config.Config) (*Engine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	e := New()
	e.cfg = cfg
	e.index = index.New(cfg.BTreeOrder)
	return e, nil
}

func (e *Engine) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.state)
}

func (e *Engine) Buffered() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.buffer)
}

func (e *Engine) Set(key, value string, ttl time.Duration) error {
	return e.set(key, value, false, ttl)
}

func (e *Engine) set(key, value string, number bool, ttl time.Duration) error {
	if ttl < 0 {
		return errors.New("le TTL ne peut pas être négatif")
	}
	if number {
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return errors.New("nombre fini requis")
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	if ttl == 0 {
		ttl = e.cfg.DefaultTTL
	}
	entry := Entry{Key: key, Value: value, IsNumber: number}
	if ttl > 0 {
		entry.ExpiresAt = e.now().Add(ttl)
		if !time.Unix(0, entry.ExpiresAt.UnixNano()).Equal(entry.ExpiresAt) {
			return errors.New("date d'expiration hors des limites du stockage")
		}
	}
	if err := e.index.Set(key, value, number); err != nil {
		return err
	}
	e.state[key] = entry
	e.record(command.KindSet, entry)
	return nil
}

func (e *Engine) Get(key string) (string, error) {
	entry, err := e.getEntry(key)
	return entry.Value, err
}

func (e *Engine) getEntry(key string) (Entry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	entry, exists := e.state[key]
	if !exists {
		return Entry{}, ErrNotFound
	}
	if expired(entry, e.now()) {
		if !e.closed {
			e.remove(key)
		}
		return Entry{}, ErrNotFound
	}
	return entry, nil
}

func expired(entry Entry, now time.Time) bool {
	return !entry.ExpiresAt.IsZero() && !now.Before(entry.ExpiresAt)
}

func (e *Engine) Delete(key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	e.remove(key)
	return nil
}

func (e *Engine) remove(key string) {
	e.index.Delete(key)
	delete(e.state, key)
	e.record(command.KindDelete, Entry{Key: key})
}

func (e *Engine) record(kind string, entry Entry) {
	if e.store == nil {
		return
	}
	e.sequence++
	op := storage.Op{Sequence: e.sequence, Kind: kind, Key: entry.Key, Value: entry.Value, IsNumber: entry.IsNumber}
	if !entry.ExpiresAt.IsZero() {
		op.ExpiresAt = entry.ExpiresAt.UnixNano()
	}
	e.buffer = append(e.buffer, op)
}

func (e *Engine) Execute(cmd command.Command) Result {
	switch cmd.Kind {
	case command.KindSet:
		if err := e.set(cmd.Key, cmd.Value, cmd.IsNumber, cmd.TTL); err != nil {
			return Result{Err: err}
		}
		return Result{Value: "OK"}
	case command.KindGet:
		entry, err := e.getEntry(cmd.Key)
		return Result{Value: entry.Value, IsNumber: entry.IsNumber, Err: err}
	case command.KindDelete:
		if err := e.Delete(cmd.Key); err != nil {
			return Result{Err: err}
		}
		return Result{Value: "OK"}
	case command.KindPing:
		return Result{Value: "PONG"}
	case command.KindGetWhere:
		entries, err := e.GetWhere(cmd.Field, cmd.Operator, cmd.Operand, cmd.OperandIsNumber)
		return Result{Entries: entries, Err: err}
	default:
		return Result{Err: command.ErrUnknown}
	}
}

func (e *Engine) ExecuteString(input string) Result {
	cmd, err := command.Parse(input)
	if err != nil {
		return Result{Err: err}
	}
	return e.Execute(cmd)
}
