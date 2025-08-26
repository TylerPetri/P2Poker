package netx

import (
	"context"
	"p2poker/internal/protocol"
)

// Inproc is a loopback transport that just echoes Outbox() → Inbox().
// Handy for single-process demos/tests without sockets.
type Inproc struct {
	inbox  chan protocol.NetMessage
	outbox chan protocol.NetMessage
}

func NewInproc() *Inproc {
	return &Inproc{
		inbox:  make(chan protocol.NetMessage, 1024),
		outbox: make(chan protocol.NetMessage, 1024),
	}
}

func (n *Inproc) Inbox() <-chan protocol.NetMessage  { return n.inbox }
func (n *Inproc) Outbox() chan<- protocol.NetMessage { return n.outbox }

func (n *Inproc) Start(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-n.outbox:
				// Echo to Inbox to simulate receipt
				n.inbox <- msg
			}
		}
	}()
	return nil
}

func (n *Inproc) Close() error { return nil }
