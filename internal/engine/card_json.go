package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalJSON encodes a Card as "As", "Th", "2c", etc.
func (c Card) MarshalJSON() ([]byte, error) {
	r, ok := rankToChar(c.Rank)
	if !ok {
		return nil, fmt.Errorf("invalid rank: %d", c.Rank)
	}
	s, ok := suitToChar(c.Suit)
	if !ok {
		return nil, fmt.Errorf("invalid suit: %d", c.Suit)
	}
	str := string([]byte{r, s})
	return json.Marshal(str)
}

// UnmarshalJSON decodes "As", "th", "2C", etc. into a Card.
// Accepts uppercase/lowercase for both rank and suit.
// Ten must be 'T'/'t' (not '10').
func (c *Card) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if len(s) != 2 {
		return fmt.Errorf("invalid card literal %q (want 2 chars like As, Td)", s)
	}
	rCh := s[0]
	sCh := s[1]

	r, ok := charToRank(rCh)
	if !ok {
		return fmt.Errorf("invalid rank char %q", rCh)
	}
	u := byte(sCh)
	if u >= 'A' && u <= 'Z' {
		u += 'a' - 'A'
	} // lowercase
	var suit Suit
	switch u {
	case 'c':
		suit = SuitClubs
	case 'd':
		suit = SuitDiamonds
	case 'h':
		suit = SuitHearts
	case 's':
		suit = SuitSpades
	default:
		return fmt.Errorf("invalid suit char %q (use c/d/h/s)", sCh)
	}
	c.Rank = r
	c.Suit = suit
	return nil
}

// Helpers

func rankToChar(r Rank) (byte, bool) {
	switch r {
	case RankTwo:
		return '2', true
	case RankThree:
		return '3', true
	case RankFour:
		return '4', true
	case RankFive:
		return '5', true
	case RankSix:
		return '6', true
	case RankSeven:
		return '7', true
	case RankEight:
		return '8', true
	case RankNine:
		return '9', true
	case RankTen:
		return 'T', true
	case RankJack:
		return 'J', true
	case RankQueen:
		return 'Q', true
	case RankKing:
		return 'K', true
	case RankAce:
		return 'A', true
	default:
		return 0, false
	}
}

func charToRank(ch byte) (Rank, bool) {
	// normalize upper
	u := ch
	if u >= 'a' && u <= 'z' {
		u -= 'a' - 'A'
	}
	switch u {
	case '2':
		return RankTwo, true
	case '3':
		return RankThree, true
	case '4':
		return RankFour, true
	case '5':
		return RankFive, true
	case '6':
		return RankSix, true
	case '7':
		return RankSeven, true
	case '8':
		return RankEight, true
	case '9':
		return RankNine, true
	case 'T':
		return RankTen, true
	case 'J':
		return RankJack, true
	case 'Q':
		return RankQueen, true
	case 'K':
		return RankKing, true
	case 'A':
		return RankAce, true
	default:
		return 0, false
	}
}

func suitToChar(s Suit) (byte, bool) {
	switch s {
	case SuitClubs:
		return 'c', true
	case SuitDiamonds:
		return 'd', true
	case SuitHearts:
		return 'h', true
	case SuitSpades:
		return 's', true
	default:
		return 0, false
	}
}
