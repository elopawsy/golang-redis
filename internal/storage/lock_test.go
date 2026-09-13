package storage

import (
	"path/filepath"
	"testing"
)

func TestExclusiveFileLocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aof")
	unlock, err := LockFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if second, err := LockFiles(path); err == nil {
		second()
		t.Fatal("double ouverture autorisée")
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	second, err := LockFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := second(); err != nil {
		t.Fatal(err)
	}
}
