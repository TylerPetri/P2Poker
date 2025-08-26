package table

import (
	"log"
	"math/rand"

	"p2poker/internal/engine"
	"p2poker/internal/protocol"
)

func (t *Table) apply(a protocol.Action) {
	var err error
	announceTurn := false
	announceStart := false
	announcePhase := false

	switch a.Type {
	case protocol.ActCreateTable:
		// no-op

	case protocol.ActJoin:
		// idempotent join: ignore if already seated
		if _, ok := t.eng.Seats[a.PlayerID]; ok {
			return
		}
		err = t.eng.Sit(a.PlayerID, t.cfg.MinBuyin)

	case protocol.ActLeave:
		t.eng.Leave(a.PlayerID)
		announceTurn = true

	case protocol.ActKick:
		if a.Meta != nil {
			if tv, ok := a.Meta["target"]; ok {
				if target, ok := tv.(string); ok {
					t.eng.Leave(target)
					announceTurn = true
				}
			}
		}

	case protocol.ActStartHand:
		seed := seedFromActionID(a.ID)
		r := rand.New(rand.NewSource(seed))
		err = t.eng.StartHand(r)
		announceStart = err == nil
		announceTurn = err == nil

	case protocol.ActBet:
		err = t.eng.Bet(a.PlayerID, a.Amount)
		announceTurn = err == nil

	case protocol.ActCheck:
		err = t.eng.Check(a.PlayerID)
		announceTurn = err == nil

	case protocol.ActFold:
		err = t.eng.Fold(a.PlayerID)
		announceTurn = err == nil

	case protocol.ActCall:
		err = t.eng.Call(a.PlayerID)
		announceTurn = err == nil

	case protocol.ActRaise:
		err = t.eng.Raise(a.PlayerID, a.Amount)
		announceTurn = err == nil

	case protocol.ActAdvance:
		t.eng.AdvancePhase()
		announcePhase = true
		announceTurn = true
	}

	if err != nil {
		log.Printf("engine apply error: action=%s player=%s err=%v", a.Type, a.PlayerID, err)
		return
	}

	if announceStart {
		log.Printf("table %s: hand started (SB=%d, BB=%d), dealer=%s",
			t.id, t.cfg.SmallBlind, t.cfg.BigBlind, dealerOf(&t.eng))
	}

	if announcePhase {
		log.Printf("table %s: phase advanced to %s", t.id, t.eng.Phase.String())
	}

	if announceTurn {
		log.Printf("table %s: phase=%s pot=%d turn=%s",
			t.id, t.eng.Phase.String(), t.eng.Pot, t.eng.CurrentPlayer())
	}

	if t.authority && t.eng.HandActive && t.eng.RoundClosed() && a.Type != protocol.ActAdvance {
		adv := protocol.Action{
			ID:       protocol.RandActionID(),
			Type:     protocol.ActAdvance,
			PlayerID: string(t.self),
		}
		t.commitAndBroadcast(adv)
	}
}

func dealerOf(s *engine.State) string {
	if len(s.Order) == 0 {
		return ""
	}
	return s.Order[s.DealerIdx]
}
