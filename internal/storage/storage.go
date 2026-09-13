package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func NewFileStorage(aofPath, snapshotPath string) *FileStorage {
	return &FileStorage{aofPath: aofPath, snapshotPath: snapshotPath}
}

func (s *FileStorage) AppendAOF(ops []Op) error {
	if len(ops) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	for _, op := range ops {
		if err := encoder.Encode(op); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.aofPath), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(s.aofPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if _, err = file.Write(data.Bytes()); err == nil {
		err = file.Sync()
	}
	if err != nil {
		return errors.Join(err, file.Truncate(info.Size()), file.Sync())
	}
	return syncDir(s.aofPath)
}

func (s *FileStorage) ReadAOF() ([]Op, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.aofPath)
	if errors.Is(err, os.ErrNotExist) {
		return []Op{}, nil
	}
	if err != nil {
		return nil, err
	}
	end := bytes.LastIndexByte(data, '\n') + 1
	ops := []Op{}
	for i, line := range bytes.Split(data[:end], []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var op Op
		if err := json.Unmarshal(line, &op); err != nil {
			return nil, fmt.Errorf("journal ligne %d : %w", i+1, err)
		}
		ops = append(ops, op)
	}
	if end < len(data) {
		file, err := os.OpenFile(s.aofPath, os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		err = file.Truncate(int64(end))
		if err == nil {
			err = file.Sync()
		}
		err = errors.Join(err, file.Close())
		if err != nil {
			return nil, err
		}
	}
	return ops, nil
}

func (s *FileStorage) ClearAOF() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeAtomic(s.aofPath, nil)
}

func (s *FileStorage) WriteSnapshot(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return writeAtomic(s.snapshotPath, data)
}

func (s *FileStorage) ReadSnapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{Entries: map[string]Op{}}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Entries == nil {
		return Snapshot{}, errors.New("snapshot invalide : entries absent")
	}
	return snapshot, nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".wasmredis-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDir(path)
}

func syncDir(path string) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
