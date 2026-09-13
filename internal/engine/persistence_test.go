package engine

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"wasmredis/internal/config"
	"wasmredis/internal/storage"
)

type memoryStorage struct {
	ops          []storage.Op
	snapshot     storage.Snapshot
	failAppend   bool
	failSnapshot bool
	failClear    bool
}

func (s *memoryStorage) AppendAOF(ops []storage.Op) error {
	if s.failAppend {
		return errors.New("disque indisponible")
	}
	s.ops = append(s.ops, ops...)
	return nil
}
func (s *memoryStorage) ReadAOF() ([]storage.Op, error) { return s.ops, nil }
func (s *memoryStorage) ClearAOF() error {
	if s.failClear {
		return errors.New("compaction impossible")
	}
	s.ops = nil
	return nil
}
func (s *memoryStorage) WriteSnapshot(snapshot storage.Snapshot) error {
	if s.failSnapshot {
		return errors.New("snapshot impossible")
	}
	s.snapshot = snapshot
	return nil
}
func (s *memoryStorage) ReadSnapshot() (storage.Snapshot, error) { return s.snapshot, nil }

func TestRestoreAfterRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.AOFPath, cfg.SnapshotPath = filepath.Join(dir, "aof"), filepath.Join(dir, "snapshot")
	disk := storage.NewFileStorage(cfg.AOFPath, cfg.SnapshotPath)
	e, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"SET number 42", `SET text "42"`, "SET deleted old", "SET expires value EX 1"} {
		if result := e.ExecuteString(input); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	if err := e.Snapshot(); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"SET number 43", "DEL deleted", "SET added yes"} {
		if result := e.ExecuteString(input); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.state, restored.state) {
		for key, want := range e.state {
			got := restored.state[key]
			if got.Value != want.Value || got.IsNumber != want.IsNumber || !got.ExpiresAt.Equal(want.ExpiresAt) {
				t.Fatalf("%s : %+v != %+v", key, got, want)
			}
		}
		if len(e.state) != len(restored.state) {
			t.Fatal("nombre de clés différent")
		}
	}
	restored.now = func() time.Time { return time.Now().Add(2 * time.Second) }
	if removed := restored.SweepExpired(); removed != 1 {
		t.Fatal("expiration attendue", removed)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	final, err := NewWithStorage(disk, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := final.Get("expires"); !errors.Is(err, ErrNotFound) {
		t.Fatal("clé expirée ressuscitée")
	}
	if final.ExecuteString("GET number").IsNumber != true || final.ExecuteString("GET text").IsNumber != false {
		t.Fatal("types perdus")
	}
}

func TestPersistenceFailuresKeepPendingWrites(t *testing.T) {
	store := &memoryStorage{}
	e, err := NewWithStorage(store, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Set("key", "first", 0); err != nil {
		t.Fatal(err)
	}
	store.failAppend = true
	if err := e.Flush(); err == nil {
		t.Fatal("erreur attendue")
	}
	if len(e.buffer) != 1 {
		t.Fatal("buffer perdu")
	}
	store.failAppend = false
	if err := e.Flush(); err != nil {
		t.Fatal(err)
	}
	store.failSnapshot = true
	if err := e.Snapshot(); err == nil {
		t.Fatal("erreur attendue")
	}
	if len(store.ops) != 1 {
		t.Fatal("journal effacé trop tôt")
	}
	store.failSnapshot = false
	store.failClear = true
	if err := e.Delete("key"); err != nil {
		t.Fatal(err)
	}
	if err := e.Snapshot(); err == nil {
		t.Fatal("erreur attendue")
	}
	restored, err := NewWithStorage(store, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Get("key"); !errors.Is(err, ErrNotFound) {
		t.Fatal("ancien journal rejoué sur le snapshot")
	}
	if err := restored.Set("next", "ok", 0); err != nil {
		t.Fatal(err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := NewWithStorage(store, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := again.Get("next"); got != "ok" || err != nil {
		t.Fatal(got, err)
	}
}
