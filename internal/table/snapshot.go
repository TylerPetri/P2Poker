package table

import (
	"encoding/json"

	"p2poker/internal/engine"
	"p2poker/internal/protocol"
)

// Public wrapper
func (t *Table) Snapshot() protocol.TableSnapshot { return t.snapshot() }

// Build a protocol-level snapshot that embeds the engine state as JSON.
func (t *Table) snapshot() protocol.TableSnapshot {
	// 1) Capture engine snapshot (pure data struct)
	es := t.eng.Snapshot() // engine.EngineSnapshot

	// 2) Marshal to JSON so protocol stays leaf-only (no engine import)
	payload, _ := json.Marshal(es) // best-effort; err unlikely

	return protocol.TableSnapshot{
		Cfg:        t.cfg,
		Seq:        t.seq,
		Epoch:      t.epoch,
		Authority:  t.authorityID,
		EngineJSON: payload, // << include engine state
	}
}

// Install a received snapshot into the local table/engine.
func (t *Table) installSnapshot(ss protocol.TableSnapshot) {
	// Consensus/config bits
	t.cfg = ss.Cfg
	t.seq = ss.Seq
	t.epoch = ss.Epoch
	t.authorityID = ss.Authority

	// Engine state (if provided)
	if len(ss.EngineJSON) > 0 {
		var es engine.EngineSnapshot
		if err := json.Unmarshal(ss.EngineJSON, &es); err == nil {
			t.eng.RestoreFromSnapshot(es)
		} else {
			// Fallback: at least keep blinds aligned
			t.eng.SmallBlind = t.cfg.SmallBlind
			t.eng.BigBlind = t.cfg.BigBlind
		}
	} else {
		// Older peer without engine payload
		t.eng.SmallBlind = t.cfg.SmallBlind
		t.eng.BigBlind = t.cfg.BigBlind
	}
}

// Authority sends a snapshot (used by /discover and resync)
func (t *Table) sendSnapshotTo(target protocol.NodeID) {
	if !t.authority {
		return
	}
	ss := t.snapshot()
	t.netOut <- protocol.NetMessage{
		Table:   t.id,
		From:    t.self,
		Type:    protocol.MsgSnapshot,
		Epoch:   t.epoch,
		Lamport: t.clock.TickLocal(),
		State:   &ss,
	}
}
