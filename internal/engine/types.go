package engine

import (
	"sync"
	"time"
	"wasmredis/internal/config"
	"wasmredis/internal/index"
	"wasmredis/internal/storage"
)

type Engine struct {
	state    map[string]Entry
	mu       sync.Mutex
	now      func() time.Time
	store    storage.Storage
	cfg      config.Config
	buffer   []storage.Op
	sequence uint64
	closed   bool
	running  bool
	index    *index.Index
}

type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	IsNumber  bool      `json:"isNumber"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Result struct {
	Value    string  `json:"value"`
	IsNumber bool    `json:"isNumber"`
	Entries  []Entry `json:"entries,omitempty"`
	Err      error   `json:"-"`
}

type Task struct {
	ID        int       `json:"id"`
	Payload   string    `json:"payload"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Queues struct {
	mu     sync.Mutex
	queues map[string]*queue
	nextID int
}

type queue struct {
	tasks  []Task
	counts map[string]int
}

type QueueSummary struct {
	Name   string         `json:"name"`
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
}

type TaskPage struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
}
