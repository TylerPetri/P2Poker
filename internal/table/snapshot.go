package table

import (
	"encoding/json"

	"p2poker/internal/protocol"
)

func (t *Table) Snapshot() protocol.TableSnapshot { return t.snapshot() }

func (t *Table) snapshot() protocol.TableSnapshot {
	return protocol.TableSnapshot{Cfg: t.cfg, Seq: t.seq, Epoch: t.epoch, Authority: t.authorityID}
}

func (t *Table) installSnapshot(ss protocol.TableSnapshot) {
	// NOTE: engine state mirror to be added later when protocol snapshot embeds it
	t.cfg = ss.Cfg
	t.seq = ss.Seq
	t.epoch = ss.Epoch
	t.authorityID = ss.Authority
}

func (t *Table) sendSnapshotTo(target protocol.NodeID) {
	if !t.authority {
		return
	}
	ss := t.snapshot()
	_, _ = json.Marshal(ss) // keep in mind message size; not used here
	t.netOut <- protocol.NetMessage{Table: t.id, From: t.self, Type: protocol.MsgSnapshot, Epoch: t.epoch, Lamport: t.clock.TickLocal(), State: &ss}
}
