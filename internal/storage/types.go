package storage

import "sync"

type Op struct {
	Sequence  uint64 `json:"sequence"`
	Kind      string `json:"kind"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	IsNumber  bool   `json:"isNumber"`
	ExpiresAt int64  `json:"expiresAt"`
}

type Snapshot struct {
	Sequence uint64        `json:"sequence"`
	Entries  map[string]Op `json:"entries"`
}

type Storage interface {
	AppendAOF([]Op) error
	ReadAOF() ([]Op, error)
	ClearAOF() error
	WriteSnapshot(Snapshot) error
	ReadSnapshot() (Snapshot, error)
}

type FileStorage struct {
	mu           sync.Mutex
	aofPath      string
	snapshotPath string
}
