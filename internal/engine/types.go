package engine

import (
	"fmt"
)

type Suit byte

const (
	SuitClubs Suit = iota
	SuitDiamonds
	SuitHearts
	SuitSpades
)

type Rank byte

const (
	RankTwo Rank = iota + 2
	RankThree
	RankFour
	RankFive
	RankSix
	RankSeven
	RankEight
	RankNine
	RankTen
	RankJack
	RankQueen
	RankKing
	RankAce
)

type Card struct {
	Rank Rank
	Suit Suit
}

func (c Card) String() string {
	ranks := map[Rank]string{
		RankTwo: "2", RankThree: "3", RankFour: "4", RankFive: "5", RankSix: "6",
		RankSeven: "7", RankEight: "8", RankNine: "9", RankTen: "T",
		RankJack: "J", RankQueen: "Q", RankKing: "K", RankAce: "A",
	}
	suits := map[Suit]string{
		SuitClubs: "♣", SuitDiamonds: "♦", SuitHearts: "♥", SuitSpades: "♠",
	}
	r, ok1 := ranks[c.Rank]
	s, ok2 := suits[c.Suit]
	if !ok1 || !ok2 {
		return "??"
	}
	return r + s
}

type Phase int

const (
	PhasePreflop Phase = iota
	PhaseFlop
	PhaseTurn
	PhaseRiver
	PhaseShowdown
)

func (p Phase) String() string {
	switch p {
	case PhasePreflop:
		return "preflop"
	case PhaseFlop:
		return "flop"
	case PhaseTurn:
		return "turn"
	case PhaseRiver:
		return "river"
	case PhaseShowdown:
		return "showdown"
	default:
		return fmt.Sprintf("phase(%d)", int(p))
	}
}

// PlayerID is a stable identifier (e.g. NodeID string)
type PlayerID = string

type Seat struct {
	Player    PlayerID
	Stack     int64
	Committed int64 // chips committed this betting round
	InHand    bool
	AllIn     bool
	Folded    bool
}

// Live state with game logic
type State struct {
	SmallBlind    int64
	BigBlind      int64
	DealerIdx     int
	Order         []PlayerID
	TurnIdx       int
	Phase         Phase
	Pot           int64
	Seats         map[PlayerID]*Seat
	Deck          []Card
	Board         []Card
	Holes         map[PlayerID][]Card
	CurrentBet    int64 // highest committed in this round
	ActorsToAct   int   // # eligible players who still must act this street
	LastRaiseSize int64 // size of last raise increment (open counts as a raise from 0)
	HandActive    bool  // true between StartHand() and end of hand
}

// Serializable struct for network/discovery
type EngineSnapshot struct {
	SmallBlind int64
	BigBlind   int64
	DealerIdx  int
	Order      []PlayerID
	TurnIdx    int
	Phase      Phase
	Pot        int64
	Board      []Card
	Seats      map[PlayerID]Seat
}

// Snapshot produces a serializable copy of the current engine state.
func (s *State) Snapshot() EngineSnapshot {
	seatsCopy := make(map[PlayerID]Seat, len(s.Seats))
	for id, st := range s.Seats {
		seatsCopy[id] = *st
	}
	return EngineSnapshot{
		SmallBlind: s.SmallBlind,
		BigBlind:   s.BigBlind,
		DealerIdx:  s.DealerIdx,
		Order:      append([]PlayerID{}, s.Order...),
		TurnIdx:    s.TurnIdx,
		Phase:      s.Phase,
		Pot:        s.Pot,
		Board:      append([]Card{}, s.Board...),
		Seats:      seatsCopy,
	}
}

// RestoreFromSnapshot installs a previously captured snapshot into the engine.
func (s *State) RestoreFromSnapshot(ss EngineSnapshot) {
	s.SmallBlind = ss.SmallBlind
	s.BigBlind = ss.BigBlind
	s.DealerIdx = ss.DealerIdx
	s.Order = append([]PlayerID{}, ss.Order...)
	s.TurnIdx = ss.TurnIdx
	s.Phase = ss.Phase
	s.Pot = ss.Pot
	s.Board = append([]Card{}, ss.Board...)

	// Rebuild Seats as pointers from the value map in the snapshot
	if s.Seats == nil {
		s.Seats = make(map[PlayerID]*Seat, len(ss.Seats))
	} else {
		for k := range s.Seats {
			delete(s.Seats, k)
		}
	}
	for id, st := range ss.Seats {
		copy := st
		s.Seats[id] = &copy
	}
}
