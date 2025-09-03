package table

import (
	"errors"
	"fmt"
	"log"
	"math/rand"

	"p2poker/internal/engine"
	"p2poker/internal/protocol"
)

func allInTag(s *engine.State, pid string) string {
	if pid == "" {
		return ""
	}
	if st, ok := s.Seats[pid]; ok && (st.AllIn || st.Stack == 0) {
		return " (all-in)"
	}
	return ""
}

func dealerTag(s *engine.State, pid string) string {
	if pid == "" || len(s.Order) == 0 {
		return ""
	}
	if pid == s.Order[s.DealerIdx] {
		return " (dealer)"
	}
	return ""
}

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

		if err == nil {
			// Local-only: show my hole cards (not broadcast; every node prints its own)
			if hc, ok := t.eng.Holes[string(t.self)]; ok && len(hc) == 2 {
				log.Printf("table %s: your hole cards: %s %s", t.id, hc[0].String(), hc[1].String())
			}
		}

	case protocol.ActCheck:
		err = t.eng.Check(a.PlayerID)
		announceTurn = err == nil

	case protocol.ActFold:
		err = t.eng.Fold(a.PlayerID)
		announceTurn = err == nil

	case protocol.ActCall:
		err = t.eng.Call(a.PlayerID)

	case protocol.ActRaise:
		st, ok := t.eng.Seats[a.PlayerID]
		if !ok {
			err = errors.New("unknown player")
			break
		}
		current := t.eng.CurrentBet
		committed := st.Committed
		to := a.Amount

		if to <= current {
			err = t.eng.Call(a.PlayerID)
			break
		}

		additional := to - committed
		if additional <= 0 {
			break
		}

		needCall := int64(0)
		if committed < current {
			needCall = current - committed
		}

		raiseBy := additional - needCall
		if raiseBy <= 0 {
			err = t.eng.Call(a.PlayerID)
		} else {
			err = t.eng.Raise(a.PlayerID, raiseBy)
		}

	case protocol.ActBet:
		if t.eng.CurrentBet == 0 {
			err = t.eng.Bet(a.PlayerID, a.Amount)
		} else {
			ra := protocol.Action{
				ID:       a.ID,
				Type:     protocol.ActRaise,
				PlayerID: a.PlayerID,
				Amount:   a.Amount,
				Meta:     a.Meta,
			}
			t.apply(ra)
			return
		}

	case protocol.ActAdvance:
		t.eng.AdvancePhase()
		announcePhase = true
		announceTurn = true

		// If we just moved into showdown, resolve immediately (authority only)
		if t.authority && (&t.eng).Phase == engine.PhaseShowdown {
			sh := protocol.Action{
				ID:       protocol.RandActionID(),
				Type:     protocol.ActShowdown,
				PlayerID: string(t.self),
			}
			t.commitAndBroadcast(sh)
			// No need to announceTurn after showdown.
			announceTurn = false
		}

	case protocol.ActShowdown:
		// Resolve payouts & end hand
		sum := (&t.eng).ResolveShowdown()
		if len(sum.Winners) == 0 {
			log.Printf("table %s: showdown: no eligible winners; pot carried was 0", t.id)
		} else {
			// Log winners (could be multiple on a tie)
			for _, w := range sum.Winners {
				// Pretty print 5-card hand
				cards := fmt.Sprintf("%s %s %s %s %s", w.Cards[0].String(), w.Cards[1].String(), w.Cards[2].String(), w.Cards[3].String(), w.Cards[4].String())
				log.Printf("table %s: winner %s — %s [%v] +%d",
					t.id, w.Player, w.Value.Cat.String(), cards, sum.PayoutPer)
			}
		}
	}

	if err != nil {
		log.Printf("engine apply error: action=%s player=%s err=%v", a.Type, a.PlayerID, err)
		return
	}

	if announceStart {
		cur := t.eng.CurrentPlayer()
		dealer := dealerOf(&t.eng)
		log.Printf("table %s: hand started (SB=%d, BB=%d), dealer=%s%s, turn=%s%s%s",
			t.id, t.cfg.SmallBlind, t.cfg.BigBlind,
			dealer, dealerTag(&t.eng, dealer),
			cur, allInTag(&t.eng, cur), dealerTag(&t.eng, cur),
		)
	}

	if announcePhase {
		cur := t.eng.CurrentPlayer()
		log.Printf("table %s: phase advanced to %s, turn=%s%s%s",
			t.id, (&t.eng).Phase.String(),
			cur, allInTag(&t.eng, cur), dealerTag(&t.eng, cur),
		)
	}

	if announceTurn {
		cur := t.eng.CurrentPlayer()
		log.Printf("table %s: phase=%s pot=%d turn=%s%s%s",
			t.id, (&t.eng).Phase.String(), (&t.eng).Pot,
			cur, allInTag(&t.eng, cur), dealerTag(&t.eng, cur),
		)
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
