package protocol

import (
	"fmt"
	"math/rand"
)

type NodeID string

type TableID string

func NewNodeID() NodeID   { return NodeID(fmt.Sprintf("n-%d", rand.Int63())) }
func NewTableID() TableID { return TableID(fmt.Sprintf("t-%d", rand.Int63())) }

// RandActionID generates a unique-ish action id for deduplication.
func RandActionID() string { return fmt.Sprintf("a-%d", rand.Int63()) }
