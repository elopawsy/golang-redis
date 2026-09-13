package config

import (
	"testing"
	"time"
)

func TestConfiguration(t *testing.T) {
	t.Setenv("WASMREDIS_FLUSH_INTERVAL", "15ms")
	t.Setenv("WASMREDIS_DEFAULT_TTL", "2s")
	t.Setenv("WASMREDIS_BTREE_ORDER", "4")
	cfg := Load()
	if cfg.FlushInterval != 15*time.Millisecond || cfg.DefaultTTL != 2*time.Second || cfg.BTreeOrder != 4 {
		t.Fatal(cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.FlushInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("intervalle nul accepté")
	}
	cfg = Default()
	cfg.BTreeOrder = 1
	if err := cfg.Validate(); err == nil {
		t.Fatal("degré invalide accepté")
	}
	cfg = Default()
	cfg.SnapshotPath = cfg.AOFPath
	if err := cfg.Validate(); err == nil {
		t.Fatal("même fichier pour journal et snapshot")
	}
}
