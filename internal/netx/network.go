package netx

import (
	"context"
	"p2poker/internal/protocol"
)

type Network interface {
	Inbox() <-chan protocol.NetMessage
	Outbox() chan<- protocol.NetMessage
	Start(ctx context.Context) error
	Close() error
}
