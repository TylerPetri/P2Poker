package cluster

import (
	"context"
	"errors"
	"sync"
	"time"

	"p2poker/internal/netx"
	"p2poker/internal/protocol"
	"p2poker/pkg/types"
)

type Node struct {
	ID     protocol.NodeID
	Addr   string
	net    netx.Network
	router *Router
	mgr    *TableManager
	clock  *protocol.Lamport

	// discovery: waiters for snapshots of tables not yet attached locally
	pendMu    sync.Mutex
	pendingSS map[protocol.TableID]chan protocol.TableSnapshot
}

func NewNode(addr string, network netx.Network) *Node {
	id := protocol.NewNodeID()
	r := NewRouter()
	clk := &protocol.Lamport{}
	mgr := NewTableManager(id, clk, r, network.Outbox())
	return &Node{ID: id, Addr: addr, net: network, router: r, mgr: mgr, clock: clk, pendingSS: make(map[protocol.TableID]chan protocol.TableSnapshot)}
}

func (n *Node) Start(ctx context.Context) error {
	if err := n.net.Start(ctx); err != nil {
		return err
	}
	go n.dispatcher(ctx)
	return nil
}

func (n *Node) dispatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-n.net.Inbox():
			// Route to table if present; otherwise, see if someone is waiting on discovery
			if !n.router.Route(msg) {
				n.maybeDeliverDiscovery(msg)
			}
		}
	}
}

func (n *Node) maybeDeliverDiscovery(msg protocol.NetMessage) {
	if msg.Type != protocol.MsgSnapshot {
		return
	}
	n.pendMu.Lock()
	ch, ok := n.pendingSS[msg.Table]
	n.pendMu.Unlock()
	if ok {
		select {
		case ch <- *msg.State:
		default:
		}
	}
}

// CreateTable creates and immediately broadcasts a CREATE_TABLE.
func (n *Node) CreateTable(name string, sb, bb, minBuy int64) (protocol.TableID, error) {
	id := protocol.NewTableID()
	cfg := types.TableConfig{Name: name, SmallBlind: sb, BigBlind: bb, MinBuyin: minBuy}
	t, err := n.mgr.CreateLocalAuthorityTable(id, cfg)
	if err != nil {
		return "", err
	}
	t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActCreateTable, PlayerID: string(n.ID)})
	return id, nil
}

// DiscoverAndAttach asks the network for a snapshot of tableID, then attaches as follower using that snapshot.
func (n *Node) DiscoverAndAttach(tableID protocol.TableID) (protocol.TableID, error) {
	// create waiter
	n.pendMu.Lock()
	if _, exists := n.pendingSS[tableID]; exists {
		n.pendMu.Unlock()
		return "", errors.New("discovery already in progress")
	}
	ch := make(chan protocol.TableSnapshot, 1)
	n.pendingSS[tableID] = ch
	n.pendMu.Unlock()

	// ask for state
	n.net.Outbox() <- protocol.NetMessage{Table: tableID, From: n.ID, Type: protocol.MsgStateQuery, Lamport: n.clock.TickLocal()}

	// wait with timeout
	select {
	case ss := <-ch:
		// clean up
		n.pendMu.Lock()
		delete(n.pendingSS, tableID)
		n.pendMu.Unlock()
		// attach follower using snapshot's cfg/epoch
		if _, err := n.mgr.AttachFollowerTable(tableID, ss.Cfg, ss.Epoch); err != nil {
			return "", err
		}
		// propose join
		if t, ok := n.mgr.Get(tableID); ok {
			t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActJoin, PlayerID: string(n.ID)})
		}
		return tableID, nil
	case <-time.After(3 * time.Second):
		// timeout
		n.pendMu.Lock()
		delete(n.pendingSS, tableID)
		n.pendMu.Unlock()
		return "", errors.New("discover timeout (no snapshot received)")
	}
}

func (n *Node) JoinTableRemote(tableID protocol.TableID, epoch protocol.Epoch, cfg types.TableConfig) error {
	t, err := n.mgr.AttachFollowerTable(tableID, cfg, epoch)
	if err != nil {
		return err
	}
	n.net.Outbox() <- protocol.NetMessage{Table: tableID, From: n.ID, Type: protocol.MsgStateQuery, Epoch: epoch, Lamport: n.clock.TickLocal()}
	t.ProposeLocal(protocol.Action{ID: protocol.RandActionID(), Type: protocol.ActJoin, PlayerID: string(n.ID)})
	return nil
}

func (n *Node) Network() netx.Network  { return n.net }
func (n *Node) Manager() *TableManager { return n.mgr }
