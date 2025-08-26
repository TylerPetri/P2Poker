package engine

import (
	"errors"
	"math/rand"
	"sort"
)

var (
	ErrAlreadySeated  = errors.New("already seated")
	ErrUnknownPlayer  = errors.New("unknown player")
	ErrInsufficient   = errors.New("insufficient chips")
	ErrNotPlayersTurn = errors.New("not player's turn")
)

func NewState(sb, bb int64) State {
	return State{
		SmallBlind: sb,
		BigBlind:   bb,
		DealerIdx:  0,
		Order:      []PlayerID{},
		TurnIdx:    0,
		Phase:      PhasePreflop,
		Seats:      make(map[PlayerID]*Seat),
		Deck:       nil,
		Board:      nil,
		HandActive: false,
	}
}

func (s *State) Sit(p PlayerID, buyin int64) error {
	if _, ok := s.Seats[p]; ok {
		return ErrAlreadySeated
	}
	s.Seats[p] = &Seat{Player: p, Stack: buyin, InHand: false}
	s.Order = append(s.Order, p)
	s.sortOrder()
	return nil
}

func (s *State) Leave(p PlayerID) {
	delete(s.Seats, p)
	// remove from order
	out := s.Order[:0]
	for _, id := range s.Order {
		if id != p {
			out = append(out, id)
		}
	}
	s.Order = out
	if s.TurnIdx >= len(s.Order) {
		s.TurnIdx = 0
	}
}

func (s *State) sortOrder() {
	sort.Slice(s.Order, func(i, j int) bool { return s.Order[i] < s.Order[j] })
}

// StartHand deals new hand, posts blinds, sets turn to UTG (after BB)
func (s *State) StartHand(r *rand.Rand) error {
	if len(s.Order) < 2 {
		return errors.New("need at least 2 players")
	}
	// reset board/pot/committed
	s.Pot = 0
	for _, seat := range s.Seats {
		seat.Committed = 0
		seat.InHand = true
		seat.Folded = false
		seat.AllIn = false
	}
	s.HandActive = true
	// rotate dealer
	s.DealerIdx = (s.DealerIdx + 1) % len(s.Order)
	// post blinds (SB = next, BB = next)
	sbIdx := (s.DealerIdx + 1) % len(s.Order)
	bbIdx := (s.DealerIdx + 2) % len(s.Order)
	s.postBlind(s.Order[sbIdx], s.SmallBlind)
	s.postBlind(s.Order[bbIdx], s.BigBlind)
	// set turn to UTG (after BB)
	s.TurnIdx = (bbIdx + 1) % len(s.Order)
	s.Phase = PhasePreflop
	// set round state
	s.CurrentBet = s.BigBlind
	s.ActorsToAct = s.countNeedToAct()
	// shuffle new deck (placeholder; no hole cards dealt here yet)
	s.Deck = NewDeck(r)
	s.Board = s.Board[:0]
	return nil
}

func (s *State) postBlind(p PlayerID, amt int64) {
	seat := s.Seats[p]
	if seat.Stack <= 0 {
		seat.AllIn = true
		return
	}
	pay := amt
	if seat.Stack < amt {
		pay = seat.Stack
		seat.AllIn = true
	}
	seat.Stack -= pay
	seat.Committed += pay
	s.Pot += pay
}

func (s *State) eligible(pid PlayerID) bool {
	st, ok := s.Seats[pid]
	return ok && st.InHand && !st.Folded && !st.AllIn
}

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
			// no bet yet: everyone eligible must act once
			need++
		} else if st.Committed < s.CurrentBet {
			// must call/raise/fold to meet the current bet
			need++
		}
	}
	return need
}

// RoundClosed returns true when betting is closed for this street.
func (s *State) RoundClosed() bool {
	// guard
	if !s.HandActive {
		return false
	}
	// Closed if no one left to act, or only one eligible player remains.
	elig := 0
	for _, pid := range s.Order {
		if s.eligible(pid) {
			elig++
		}
	}
	return s.ActorsToAct <= 0 || elig <= 1
}

func (s *State) AdvancePhase() {
	switch s.Phase {
	case PhasePreflop:
		// deal 3 board cards
		if len(s.Deck) >= 3 {
			s.Board = append(s.Board, s.Deck[:3]...)
			s.Deck = s.Deck[3:]
		}
		s.resetCommittedAndSetTurnFromDealer()
		s.Phase = PhaseFlop
	case PhaseFlop:
		if len(s.Deck) >= 1 {
			s.Board = append(s.Board, s.Deck[0])
			s.Deck = s.Deck[1:]
		}
		s.resetCommittedAndSetTurnFromDealer()
		s.Phase = PhaseTurn
	case PhaseTurn:
		if len(s.Deck) >= 1 {
			s.Board = append(s.Board, s.Deck[0])
			s.Deck = s.Deck[1:]
		}
		s.resetCommittedAndSetTurnFromDealer()
		s.Phase = PhaseRiver
	case PhaseRiver:
		s.Phase = PhaseShowdown
		s.HandActive = false
	}
}

func (s *State) resetCommittedAndSetTurnFromDealer() {
	for _, seat := range s.Seats {
		seat.Committed = 0
	}
	// if nobody is seated, avoid modulo-by-zero and consider the round closed
	if len(s.Order) == 0 {
		s.TurnIdx = 0
		s.CurrentBet = 0
		s.ActorsToAct = 0
		return
	}
	// after each street, first to act is left of dealer
	first := (s.DealerIdx + 1) % len(s.Order)
	s.TurnIdx = first
	s.CurrentBet = 0
	s.ActorsToAct = s.countNeedToAct()
}

// CurrentPlayer returns the PlayerID whose turn it is, or "" if none.
func (s *State) CurrentPlayer() PlayerID {
	if len(s.Order) == 0 {
		return ""
	}
	return s.Order[s.TurnIdx]
}

// Dealer returns the dealer's PlayerID, or "" if none.
func (s *State) Dealer() PlayerID {
	if len(s.Order) == 0 {
		return ""
	}
	return s.Order[s.DealerIdx]
}

// SeatView is a read-only view for UIs/CLIs.
type SeatView struct {
	Player    PlayerID
	Stack     int64
	Committed int64
	InHand    bool
	AllIn     bool
	Folded    bool
}

// Summary is a compact snapshot of user-facing state.
type Summary struct {
	Phase  string
	Pot    int64
	Dealer PlayerID
	Turn   PlayerID
	Order  []PlayerID
	Seats  []SeatView // ordered by s.Order
}

// Summary returns a UI-friendly summary of the current state.
func (s *State) Summary() Summary {
	views := make([]SeatView, 0, len(s.Order))
	for _, pid := range s.Order {
		if seat, ok := s.Seats[pid]; ok {
			views = append(views, SeatView{
				Player:    pid,
				Stack:     seat.Stack,
				Committed: seat.Committed,
				InHand:    seat.InHand,
				AllIn:     seat.AllIn,
				Folded:    seat.Folded,
			})
		} else {
			// Seat was removed but still in Order (shouldn't happen, but be safe)
			views = append(views, SeatView{Player: pid})
		}
	}
	return Summary{
		Phase:  s.Phase.String(),
		Pot:    s.Pot,
		Dealer: s.Dealer(),
		Turn:   s.CurrentPlayer(),
		Order:  append([]PlayerID{}, s.Order...),
		Seats:  views,
	}
}

func (s *State) ensureTurn(p PlayerID) error {
	if s.CurrentPlayer() != p {
		return ErrNotPlayersTurn
	}
	return nil
}

// Bet is a simple add-to-pot action for now (no min-raise logic yet).
func (s *State) Bet(p PlayerID, amt int64) error {
	st, ok := s.Seats[p]
	if !ok {
		return ErrUnknownPlayer
	}
	if err := s.ensureTurn(p); err != nil {
		return err
	}
	if amt <= 0 {
		return errors.New("bet must be > 0")
	}

	// If there is already a bet, interpret as Raise
	if s.CurrentBet > 0 {
		return s.Raise(p, amt)
	}

	if st.Stack < amt {
		return ErrInsufficient
	}
	st.Stack -= amt
	st.Committed += amt
	s.Pot += amt

	// New current bet is the bettor's committed amount
	s.CurrentBet = st.Committed
	s.ActorsToAct = s.countNeedToAct() // everyone else must respond
	s.advanceTurn()
	return nil
}

func (s *State) Check(p PlayerID) error {
	st, ok := s.Seats[p]
	if !ok {
		return ErrUnknownPlayer
	}
	if err := s.ensureTurn(p); err != nil {
		return err
	}
	// Can only check if you're already matched to CurrentBet
	if st.Committed != s.CurrentBet {
		return errors.New("cannot check; unmatched to current bet")
	}
	s.ActorsToAct-- // this actor has acted
	s.advanceTurn()
	return nil
}

func (s *State) Fold(p PlayerID) error {
	st, ok := s.Seats[p]
	if !ok {
		return ErrUnknownPlayer
	}
	if err := s.ensureTurn(p); err != nil {
		return err
	}
	if s.CurrentBet > 0 && st.Committed < s.CurrentBet {
		s.ActorsToAct-- // one fewer to call
	}
	st.Folded = true
	st.InHand = false
	s.advanceTurn()
	return nil
}

func (s *State) Call(p PlayerID) error {
	st, ok := s.Seats[p]
	if !ok {
		return ErrUnknownPlayer
	}
	if err := s.ensureTurn(p); err != nil {
		return err
	}
	if s.CurrentBet == 0 {
		return errors.New("nothing to call")
	}
	need := s.CurrentBet - st.Committed
	if need <= 0 {
		return errors.New("already matched")
	}
	if need > st.Stack {
		return ErrInsufficient
	}
	st.Stack -= need
	st.Committed += need
	s.Pot += need
	s.ActorsToAct--
	s.advanceTurn()
	return nil
}

func (s *State) Raise(p PlayerID, add int64) error {
	st, ok := s.Seats[p]
	if !ok {
		return ErrUnknownPlayer
	}
	if err := s.ensureTurn(p); err != nil {
		return err
	}
	if add <= 0 {
		return errors.New("raise must be > 0")
	}

	// First, call up to CurrentBet if needed
	need := int64(0)
	if st.Committed < s.CurrentBet {
		need = s.CurrentBet - st.Committed
	}
	total := need + add
	if st.Stack < total {
		return ErrInsufficient
	}

	// Pay call portion
	if need > 0 {
		st.Stack -= need
		st.Committed += need
		s.Pot += need
	}
	// Pay raise portion
	st.Stack -= add
	st.Committed += add
	s.Pot += add

	// New bar and reset responders
	s.CurrentBet = st.Committed
	s.ActorsToAct = s.countNeedToAct() // everyone else behind must respond
	s.advanceTurn()
	return nil
}

func (s *State) advanceTurn() {
	if len(s.Order) == 0 {
		return
	}
	for i := 0; i < len(s.Order); i++ {
		s.TurnIdx = (s.TurnIdx + 1) % len(s.Order)
		pid := s.Order[s.TurnIdx]
		st := s.Seats[pid]
		if st.InHand && !st.Folded && !st.AllIn {
			return // found next actor
		}
	}
	// if no eligible player found, do nothing (round will be advanced by outer logic)
}
