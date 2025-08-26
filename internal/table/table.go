package table

import (
	"time"

	"p2poker/internal/engine"
	"p2poker/internal/protocol"
	"p2poker/pkg/types"
)

// Table is the per-table event-loop that applies poker actions and
// synchronizes state using an authority-driven commit stream.
//
// Dependencies:
//   - protocol: ids, messages, actions, snapshots, lamport, epoch
//   - types: TableConfig
// No direct network I/O here; we communicate via in/out channels of NetMessage.

type Table struct {
	id        protocol.TableID
	self      protocol.NodeID
	cfg       types.TableConfig
	authority bool
	epoch     protocol.Epoch
	clock     *protocol.Lamport

	in     <-chan protocol.NetMessage
	netOut chan<- protocol.NetMessage

	// consensus-ish bits
	seq         uint64
	log         []protocol.Action
	dedup       map[string]struct{}
	followers   map[protocol.NodeID]struct{}
	authorityID protocol.NodeID

	eng engine.State

	// timers
	lastHeartbeat time.Time
}

type gameState struct {
	Players   []string
	DealerIdx int
	Pot       int64
	Phase     string // preflop/flop/turn/river/showdown
}

func New(
	id protocol.TableID,
	self protocol.NodeID,
	cfg types.TableConfig,
	authority bool,
	epoch protocol.Epoch,
	clock *protocol.Lamport,
	in <-chan protocol.NetMessage,
	out chan<- protocol.NetMessage,
) *Table {
	return &Table{
		id: id, self: self, cfg: cfg, authority: authority, epoch: epoch, clock: clock,
		in: in, netOut: out,
		seq: 0, log: make([]protocol.Action, 0, 1024), dedup: make(map[string]struct{}), followers: make(map[protocol.NodeID]struct{}),
		authorityID: func() protocol.NodeID {
			if authority {
				return self
			}
			return ""
		}(),
		eng:           engine.NewState(cfg.SmallBlind, cfg.BigBlind),
		lastHeartbeat: time.Now(),
	}
}

func (t *Table) ID() protocol.TableID         { return t.id }
func (t *Table) IsAuthority() bool            { return t.authority }
func (t *Table) Epoch() protocol.Epoch        { return t.epoch }
func (t *Table) AuthorityID() protocol.NodeID { return t.authorityID }
func (t *Table) Eng() *engine.State           { return &t.eng }

// Run drives the event loop. When authority, it emits heartbeats.
func (t *Table) Run() {
	heartbeat := time.NewTicker(maxDur(t.cfg.AuthorityTick, 500*time.Millisecond))
	defer heartbeat.Stop()

	for {
		if t.authority {
			select {
			case msg := <-t.in:
				t.onNet(msg)
			case <-heartbeat.C:
				t.sendHeartbeat()
			}
		} else {
			select {
			case msg := <-t.in:
				t.onNet(msg)
			case <-time.After(maxDur(t.cfg.FollowerTO, 3*time.Second)):
				t.tryAuthorityTakeover()
			}
		}
	}
}

func (t *Table) onNet(msg protocol.NetMessage) {
	// integrate lamport clock
	t.clock.TickRemote(msg.Lamport)

	switch msg.Type {
	case protocol.MsgPropose:
		if !t.authority {
			return
		}
		if msg.Action == nil {
			return
		}
		// AUTH GUARD: only allow KICK if proposer is the current authority
		if msg.Action.Type == protocol.ActKick && msg.From != t.authorityID {
			// ignore unauthorized kick proposal
			return
		}

		t.commitAndBroadcast(*msg.Action)
	case protocol.MsgCommit:
		if msg.Action == nil {
			return
		}
		if msg.Epoch < t.epoch {
			return
		}
		// AUTH GUARD: only accept KICK commits if they came from the authority
		if msg.Action.Type == protocol.ActKick && msg.From != t.authorityID {
			return
		}

		t.applyCommit(*msg.Action, msg.Seq)
		if msg.Epoch > t.epoch || t.authorityID == "" {
			t.epoch = msg.Epoch
			t.authorityID = msg.From
		}
		t.lastHeartbeat = time.Now()
	case protocol.MsgSnapshot:
		if msg.State == nil {
			return
		}
		if msg.Epoch < t.epoch {
			return
		}
		t.installSnapshot(*msg.State)
		t.lastHeartbeat = time.Now()
	case protocol.MsgHeartbeat:
		if msg.Epoch < t.epoch {
			return
		}
		t.epoch = msg.Epoch
		t.authorityID = msg.From
		t.lastHeartbeat = time.Now()
	case protocol.MsgStateQuery:
		if t.authority {
			t.sendSnapshotTo(msg.From)
		}
	}
}

// ProposeLocal submits an action originating from this node.
func (t *Table) ProposeLocal(a protocol.Action) {
	if t.authority {
		t.commitAndBroadcast(a)
		return
	}
	t.netOut <- protocol.NetMessage{
		Table: t.id, From: t.self, Type: protocol.MsgPropose, Epoch: t.epoch,
		Lamport: t.clock.TickLocal(), Action: &a,
	}
}

func (t *Table) commitAndBroadcast(a protocol.Action) {
	if _, seen := t.dedup[a.ID]; seen {
		return
	}
	t.seq++
	t.apply(a)
	t.log = append(t.log, a)
	t.dedup[a.ID] = struct{}{}

	t.netOut <- protocol.NetMessage{
		Table: t.id, From: t.self, Type: protocol.MsgCommit, Epoch: t.epoch, Lamport: t.clock.TickLocal(), Seq: t.seq, Action: &a,
	}
}

func (t *Table) applyCommit(a protocol.Action, seq uint64) {
	if _, seen := t.dedup[a.ID]; seen {
		return
	}
	if seq != t.seq+1 {
		// gap: request snapshot
		t.netOut <- protocol.NetMessage{Table: t.id, From: t.self, Type: protocol.MsgStateQuery, Epoch: t.epoch, Lamport: t.clock.TickLocal()}
		return
	}
	t.seq = seq
	t.apply(a)
	t.log = append(t.log, a)
	t.dedup[a.ID] = struct{}{}
}
