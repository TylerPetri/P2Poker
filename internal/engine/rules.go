package engine

// Rule helpers and round-closure logic live here, so state.go remains uncluttered.

// eligible returns true if a player can act this street.
func (s *State) eligible(pid PlayerID) bool {
	st, ok := s.Seats[pid]
	return ok && st.InHand && !st.Folded && !st.AllIn
}

// countNeedToAct recomputes how many eligible players still need to act
// before the current betting round is closed.
func (s *State) countNeedToAct() int {
	if len(s.Order) == 0 {
		return 0
	}
	need := 0
	for _, pid := range s.Order {
		if !s.eligible(pid) {
			continue
		}
		st := s.Seats[pid]
		if s.CurrentBet == 0 {
			// no bet: every eligible player must act once (check or bet)
			need++
		} else if st.Committed < s.CurrentBet {
			// must call/raise/fold to meet the current bet
			need++
		}
	}
	return need
}

// RoundClosed returns true when betting is closed this street.
// (Authority may auto-advance when this becomes true.)
func (s *State) RoundClosed() bool {
	if !s.HandActive {
		return false
	}
	elig := 0
	for _, pid := range s.Order {
		if s.eligible(pid) {
			elig++
		}
	}
	return s.ActorsToAct <= 0 || elig <= 1
}

// Utility used by raise/call logic.
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
