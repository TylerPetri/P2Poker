package table

import (
	"log"
	"time"

	"p2poker/internal/protocol"
)

func (t *Table) tryAuthorityTakeover() {
	if t.authority {
		return
	}
	if time.Since(t.lastHeartbeat) < maxDur(t.cfg.FollowerTO, 3*time.Second) {
		return
	}
	if !t.isSmallestNodeID() {
		return
	}
	// Takeover
	t.authority = true
	t.epoch++
	t.authorityID = t.self
	log.Printf("table %s: %s assumes authority, epoch=%d", t.id, t.self, t.epoch)
	t.sendHeartbeat()
	t.sendSnapshotTo("") // broadcast in real network layer
}

func (t *Table) sendHeartbeat() {
	if !t.authority {
		return
	}
	t.netOut <- protocol.NetMessage{Table: t.id, From: t.self, Type: protocol.MsgHeartbeat, Epoch: t.epoch, Lamport: t.clock.TickLocal(), Seq: t.seq}
}

func (t *Table) isSmallestNodeID() bool {
	if t.authorityID == "" {
		return true
	}
	return string(t.self) < string(t.authorityID)
}
