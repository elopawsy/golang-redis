package engine

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wasmredis/internal/config"
	"wasmredis/internal/storage"
)

func TestBackgroundFlushSnapshotAndExpiry(t *testing.T) {
	cfg := config.Default()
	dir := t.TempDir()
	cfg.AOFPath, cfg.SnapshotPath = filepath.Join(dir, "aof"), filepath.Join(dir, "snapshot")
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.SnapshotInterval = 10 * time.Millisecond
	cfg.ExpiryScanInterval = 5 * time.Millisecond
	disk := storage.NewFileStorage(cfg.AOFPath, cfg.SnapshotPath)
	e, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	e.now = func() time.Time { return time.Unix(0, clock.Load()) }
	if err := e.Set("expires", "value", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := e.Set("keep", "value", 0); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(2 * time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.StartBackground(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatal("snapshot ou expiration non exécuté")
		case <-poll.C:
			snapshot, err := disk.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			e.mu.Lock()
			_, exists := e.state["expires"]
			e.mu.Unlock()
			if !exists && snapshot.Entries["keep"].Value == "value" && len(snapshot.Entries) == 1 {
				return
			}
		}
	}
}

func TestDefaultTTLAndClose(t *testing.T) {
	cfg := config.Default()
	cfg.DefaultTTL = time.Second
	e, err := NewMemory(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	e.now = func() time.Time { return now }
	if err := e.Set("key", "value", 0); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := e.Get("key"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.Set("key", "value", 0); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if err := e.Delete("key"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestExpirationMustSurviveSerialization(t *testing.T) {
	e := New()
	e.now = func() time.Time { return time.Date(2262, 4, 11, 23, 47, 16, 0, time.UTC) }
	if err := e.Set("key", "value", time.Hour); err == nil {
		t.Fatal("date impossible à stocker acceptée")
	}
	if _, err := e.Get("key"); !errors.Is(err, ErrNotFound) {
		t.Fatal("écriture invalide appliquée")
	}
}

func TestBackgroundReturnsStorageFailure(t *testing.T) {
	cfg := config.Default()
	cfg.FlushInterval = time.Millisecond
	store := &memoryStorage{failAppend: true}
	e, err := NewWithStorage(store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Set("key", "value", 0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := e.StartBackground(ctx); err == nil {
		t.Fatal("erreur disque ignorée")
	}
	if len(e.buffer) != 1 {
		t.Fatal("buffer perdu")
	}
}

func TestConcurrentWritesWithPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.FlushInterval = time.Millisecond
	cfg.SnapshotInterval = 2 * time.Millisecond
	disk := storage.NewFileStorage(filepath.Join(dir, "aof"), filepath.Join(dir, "snapshot"))
	e, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.StartBackground(ctx) }()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if result := e.ExecuteString("SET key 42"); result.Err != nil {
					t.Error(result.Err)
				}
				if result := e.ExecuteString("GET WHERE value >= 40"); result.Err != nil {
					t.Error(result.Err)
				}
			}
		}()
	}
	wg.Wait()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := restored.Get("key"); got != "42" || err != nil {
		t.Fatal(got, err)
	}
}
