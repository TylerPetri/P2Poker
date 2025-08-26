package types

import "time"

// TableConfig holds per-table runtime configuration that can be serialized
// and shared via snapshots. Keep this struct stable and backward-compatible.
type TableConfig struct {
	Name          string
	MinBuyin      int64
	SmallBlind    int64
	BigBlind      int64
	AuthorityTick time.Duration
	FollowerTO    time.Duration
}
