package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func Default() Config {
	return Config{
		FlushInterval: time.Second, SnapshotInterval: 2 * time.Minute,
		ExpiryScanInterval: 10 * time.Second, BTreeOrder: 32,
		AOFPath: "data/aof.log", SnapshotPath: "data/snapshot.json",
	}
}

func Load() Config {
	c := Default()
	c.FlushInterval = getDuration("WASMREDIS_FLUSH_INTERVAL", c.FlushInterval)
	c.SnapshotInterval = getDuration("WASMREDIS_SNAPSHOT_INTERVAL", c.SnapshotInterval)
	c.DefaultTTL = getDuration("WASMREDIS_DEFAULT_TTL", c.DefaultTTL)
	c.ExpiryScanInterval = getDuration("WASMREDIS_EXPIRY_SCAN_INTERVAL", c.ExpiryScanInterval)
	c.BTreeOrder = getInt("WASMREDIS_BTREE_ORDER", c.BTreeOrder)
	c.AOFPath = getString("WASMREDIS_AOF_PATH", c.AOFPath)
	c.SnapshotPath = getString("WASMREDIS_SNAPSHOT_PATH", c.SnapshotPath)
	return c
}

func (c Config) Validate() error {
	if c.FlushInterval <= 0 || c.SnapshotInterval <= 0 || c.ExpiryScanInterval <= 0 || c.DefaultTTL < 0 || c.BTreeOrder < 2 {
		return errors.New("intervalles positifs, TTL >= 0 et degré B-Tree >= 2 requis")
	}
	aof, err := filepath.Abs(c.AOFPath)
	if err != nil {
		return err
	}
	snapshot, err := filepath.Abs(c.SnapshotPath)
	if err != nil {
		return err
	}
	if c.AOFPath == "" || c.SnapshotPath == "" || aof == snapshot {
		return errors.New("chemins AOF et snapshot distincts requis")
	}
	return nil
}

func getString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if number, err := strconv.Atoi(value); err == nil {
			return number
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}
