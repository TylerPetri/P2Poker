package netx

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"sync"

	"p2poker/internal/protocol"
)

// TCP implements Network with a simple peer fan‑out writer and per‑conn readers.
// All messages placed on Outbox() are broadcast to all connected peers.
// Use AddPeer to dial and connect to others.

type TCP struct {
	addr   string
	inbox  chan protocol.NetMessage
	outbox chan protocol.NetMessage

	ln    net.Listener
	mu    sync.RWMutex
	peers map[string]net.Conn // addr -> conn
}

func NewTCP(addr string) *TCP {
	return &TCP{
		addr:   addr,
		inbox:  make(chan protocol.NetMessage, 4096),
		outbox: make(chan protocol.NetMessage, 4096),
		peers:  make(map[string]net.Conn),
	}
}

func (t *TCP) Inbox() <-chan protocol.NetMessage  { return t.inbox }
func (t *TCP) Outbox() chan<- protocol.NetMessage { return t.outbox }

func (t *TCP) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", t.addr)
	if err != nil {
		return err
	}
	t.ln = ln
	log.Printf("tcp listening on %s", t.addr)

	// accept loop
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				log.Printf("accept error: %v", err)
				continue
			}
			addr := c.RemoteAddr().String()
			t.addConn(addr, c)
			go t.readLoop(ctx, c)
		}
	}()

	// broadcast write loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-t.outbox:
				t.broadcast(msg)
			}
		}
	}()
	return nil
}

func (t *TCP) Close() error {
	if t.ln != nil {
		_ = t.ln.Close()
	}
	t.mu.Lock()
	for _, c := range t.peers {
		_ = c.Close()
	}
	t.peers = map[string]net.Conn{}
	t.mu.Unlock()
	return nil
}

// AddPeer dials a remote and registers it as a peer.
func (t *TCP) AddPeer(addr string) error {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	t.addConn(addr, c)
	go t.readLoop(context.Background(), c)
	return nil
}

func (t *TCP) addConn(addr string, c net.Conn) {
	t.mu.Lock()
	if old, ok := t.peers[addr]; ok {
		_ = old.Close()
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	t.peers[addr] = c
	t.mu.Unlock()
	log.Printf("peer connected: %s", addr)
}

func (t *TCP) readLoop(ctx context.Context, c net.Conn) {
	defer func() {
		addr := c.RemoteAddr().String()
		_ = c.Close()
		t.mu.Lock()
		delete(t.peers, addr)
		t.mu.Unlock()
		log.Printf("peer disconnected: %s", addr)
	}()

	r := bufio.NewReader(c)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := Decode(r)
			if err != nil {
				if err == io.EOF {
					return
				}
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				log.Printf("read error: %v", err)
				return
			}
			// deliver inbound message
			t.inbox <- msg
		}
	}
}

func (t *TCP) broadcast(msg protocol.NetMessage) {
	frame, err := Encode(msg)
	if err != nil {
		log.Printf("encode error: %v", err)
		return
	}
	// snapshot of peers to avoid holding lock while writing
	t.mu.RLock()
	peers := make([]net.Conn, 0, len(t.peers))
	for _, c := range t.peers {
		peers = append(peers, c)
	}
	t.mu.RUnlock()
	for _, c := range peers {
		if _, err := c.Write(frame); err != nil {
			log.Printf("write error to %s: %v", c.RemoteAddr(), err)
		}
	}
}
