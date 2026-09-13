package config

import "time"

type Config struct {
	FlushInterval      time.Duration
	SnapshotInterval   time.Duration
	DefaultTTL         time.Duration
	ExpiryScanInterval time.Duration
	BTreeOrder         int
	AOFPath            string
	SnapshotPath       string
}
