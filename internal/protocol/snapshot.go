package protocol

import (
	"encoding/json"
	"p2poker/pkg/types"
)

// TableSnapshot is the network-serializable state used for catch-up and
// authority handoffs. Keep minimal and stable.
type TableSnapshot struct {
	Cfg       types.TableConfig `json:"cfg"`
	Seq       uint64            `json:"seq"`
	Epoch     Epoch             `json:"epoch"`
	Authority NodeID            `json:"authority"`

	// engine snapshot payload as JSON to avoid protocol↔engine import cycles.
	EngineJSON json.RawMessage `json:"engine,omitempty"`
}
