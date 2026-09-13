package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStorage(filepath.Join(dir, "aof"), filepath.Join(dir, "snapshot"))
	empty, err := s.ReadSnapshot()
	if err != nil || len(empty.Entries) != 0 {
		t.Fatal(empty, err)
	}
	ops := []Op{{Sequence: 1, Kind: "SET", Key: "a", Value: strings.Repeat("x", 70000)}, {Sequence: 2, Kind: "DELETE", Key: "a"}}
	if err := s.AppendAOF(ops); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadAOF()
	if err != nil || !reflect.DeepEqual(got, ops) {
		t.Fatal("journal différent", err)
	}
	snapshot := Snapshot{Sequence: 2, Entries: map[string]Op{}}
	if err := s.WriteSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	restored, err := s.ReadSnapshot()
	if err != nil || !reflect.DeepEqual(snapshot, restored) {
		t.Fatal(restored, err)
	}
	if err := s.ClearAOF(); err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadAOF()
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}

func TestTruncatedTailAndCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aof")
	s := NewFileStorage(path, filepath.Join(dir, "snapshot"))
	op := Op{Sequence: 1, Kind: "SET", Key: "a", Value: "1"}
	if err := s.AppendAOF([]Op{op}); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{incomplet"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadAOF()
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	op.Sequence = 2
	if err := s.AppendAOF([]Op{op}); err != nil {
		t.Fatal(err)
	}
	got, err = s.ReadAOF()
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if err := os.WriteFile(path, []byte("invalide\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadAOF(); err == nil {
		t.Fatal("corruption ignorée")
	}
	if err := os.WriteFile(s.snapshotPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadSnapshot(); err == nil {
		t.Fatal("snapshot invalide accepté")
	}
}
