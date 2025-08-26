package table

import (
	"sort"

	"p2poker/internal/protocol"
)

func (t *Table) apply(a protocol.Action) {
	switch a.Type {
	case protocol.ActCreateTable:
		// already created locally
	case protocol.ActJoin:
		if !contains(t.state.Players, a.PlayerID) {
			t.state.Players = append(t.state.Players, a.PlayerID)
			sort.Strings(t.state.Players)
		}
	case protocol.ActLeave:
		removeStr(&t.state.Players, a.PlayerID)
	case protocol.ActKick:
		if a.Meta != nil {
			if tv, ok := a.Meta["target"]; ok {
				if target, ok := tv.(string); ok {
					removeStr(&t.state.Players, target)
				}
			}
		}
	case protocol.ActStartHand:
		t.state.Phase = "preflop"
		// TODO(engine): blinds, deal, etc.
	case protocol.ActBet:
		t.state.Pot += a.Amount
	case protocol.ActCall:
		// TODO(engine)
	case protocol.ActRaise:
		// TODO(engine)
	case protocol.ActCheck:
		// TODO(engine)
	case protocol.ActFold:
		// TODO(engine)
	case protocol.ActAdvance:
		// TODO(engine): phase progression
	}
}
