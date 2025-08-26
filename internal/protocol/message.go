package protocol

type MsgType string

const (
	MsgPropose    MsgType = "PROPOSE"
	MsgCommit     MsgType = "COMMIT"
	MsgSnapshot   MsgType = "SNAPSHOT"
	MsgStateQuery MsgType = "STATE_QUERY"
	MsgHeartbeat  MsgType = "HEARTBEAT"
)

type NetMessage struct {
	Table   TableID        `json:"table"`
	From    NodeID         `json:"from"`
	Type    MsgType        `json:"type"`
	Epoch   Epoch          `json:"epoch"`
	Lamport uint64         `json:"lamport"`
	Seq     uint64         `json:"seq"`
	Action  *Action        `json:"action,omitempty"`
	State   *TableSnapshot `json:"state,omitempty"`
}
