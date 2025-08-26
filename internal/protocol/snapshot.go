package protocol

import "p2poker/pkg/types"

// TableSnapshot is the network-serializable state used for catch-up and
// authority handoffs. Keep minimal and stable.
type TableSnapshot struct {
	Cfg types.TableConfig `json:"cfg"`
	// Engine state fields will be added later (players, pot, phase, etc.).
	Seq       uint64 `json:"seq"`
	Epoch     Epoch  `json:"epoch"`
	Authority NodeID `json:"authority"`
}
