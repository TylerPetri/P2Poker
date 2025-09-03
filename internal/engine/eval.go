package engine

import (
	"fmt"
	"sort"
)

// Category is the standard poker hand class.
type Category int

const (
	CatHighCard Category = iota
	CatOnePair
	CatTwoPair
	CatTrips
	CatStraight
	CatFlush
	CatFullHouse
	CatQuads
	CatStraightFlush
)

func (c Category) String() string {
	switch c {
	case CatHighCard:
		return "High Card"
	case CatOnePair:
		return "One Pair"
	case CatTwoPair:
		return "Two Pair"
	case CatTrips:
		return "Three of a Kind"
	case CatStraight:
		return "Straight"
	case CatFlush:
		return "Flush"
	case CatFullHouse:
		return "Full House"
	case CatQuads:
		return "Four of a Kind"
	case CatStraightFlush:
		return "Straight Flush"
	default:
		return fmt.Sprintf("cat(%d)", int(c))
	}
}

// HandValue encodes a 5-card hand for comparison.
// Ranks are kicker/tiebreakers in descending order for the category.
type HandValue struct {
	Cat   Category
	Ranks [5]Rank // e.g., for Pair: [pairRank, k1, k2, k3]; for Straight: [top, 0,0,0,0]
}

// Less reports whether hv < other (i.e., worse hand).
func (hv HandValue) Less(other HandValue) bool {
	if hv.Cat != other.Cat {
		return hv.Cat < other.Cat
	}
	for i := 0; i < 5; i++ {
		if hv.Ranks[i] != other.Ranks[i] {
			return hv.Ranks[i] < other.Ranks[i]
		}
	}
	return false
}

// Equal reports hv == other.
func (hv HandValue) Equal(other HandValue) bool {
	if hv.Cat != other.Cat {
		return false
	}
	for i := 0; i < 5; i++ {
		if hv.Ranks[i] != other.Ranks[i] {
			return false
		}
	}
	return true
}

// BestHand7 evaluates the best 5-card hand from 7 cards (board 5 + hole 2).
// Returns a comparable HandValue and the 5 cards that make it (useful later for UI/showdown).
func BestHand7(board []Card, holes []Card) (HandValue, [5]Card) {
	// Collect the 7 cards.
	all := make([]Card, 0, 7)
	all = append(all, board...)
	all = append(all, holes...)

	// Counts per rank and suit
	var rankCount [15]int // 0..14; ranks used = 2..14
	var suitCount [4]int
	var bySuit [4][]Card
	present := uint16(0) // bitset for ranks 2..14 => bits 2..14

	for _, c := range all {
		r := int(c.Rank)
		s := int(c.Suit)
		rankCount[r]++
		suitCount[s]++
		bySuit[s] = append(bySuit[s], c)
		present |= 1 << r
	}

	// Helper to build HandValue with sorted kickers desc
	fill := func(cat Category, ks ...Rank) HandValue {
		var r [5]Rank
		for i := range ks {
			r[i] = ks[i]
		}
		return HandValue{Cat: cat, Ranks: r}
	}

	// Straight-high (top rank) in a rank bitset; includes wheel A-5 straight (returns 5 as top)
	straightTop := func(bits uint16) Rank {
		// Wheel: A(14) + 5..2 present -> treat as 5-high straight
		wheelMask := uint16((1 << 14) | (1 << 5) | (1 << 4) | (1 << 3) | (1 << 2))
		if bits&wheelMask == wheelMask {
			return Rank(5)
		}
		// Regular: look for 5 consecutive ranks
		run := 0
		for r := 14; r >= 2; r-- {
			if (bits>>r)&1 == 1 {
				run++
				if run == 5 {
					return Rank(r + 4) // top rank of the run
				}
			} else {
				run = 0
			}
		}
		return 0
	}

	// Build a descending list of ranks by multiplicity (quads, trips, pairs) + kickers
	type group struct {
		rank Rank
		cnt  int
	}
	var groups []group
	for r := 14; r >= 2; r-- {
		if rankCount[r] > 0 {
			groups = append(groups, group{rank: Rank(r), cnt: rankCount[r]})
		}
	}
	// Sort: higher count first (4,3,2,1), then by rank desc
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].cnt != groups[j].cnt {
			return groups[i].cnt > groups[j].cnt
		}
		return groups[i].rank > groups[j].rank
	})

	// Check for flush / straight flush
	var flushSuit = -1
	for s := 0; s < 4; s++ {
		if suitCount[s] >= 5 {
			flushSuit = s
			break
		}
	}
	if flushSuit >= 0 {
		// Straight flush?
		// Build rank bits for cards of that suit
		var suitBits uint16
		for _, c := range bySuit[flushSuit] {
			suitBits |= 1 << int(c.Rank)
		}
		if top := straightTop(suitBits); top != 0 {
			// Return straight flush; we don't need exact 5 cards yet, but let's also pick them
			best, five := pickStraight(ofSuit(bySuit[flushSuit]), top)
			_ = best
			return fill(CatStraightFlush, top), five
		}
		// Regular flush: take top 5 ranks of that suit
		sort.Slice(bySuit[flushSuit], func(i, j int) bool { return bySuit[flushSuit][i].Rank > bySuit[flushSuit][j].Rank })
		var five [5]Card
		copy(five[:], bySuit[flushSuit][:5])
		return fill(CatFlush, five[0].Rank, five[1].Rank, five[2].Rank, five[3].Rank, five[4].Rank), five
	}

	// Four of a Kind
	if len(groups) > 0 && groups[0].cnt == 4 {
		quad := groups[0].rank
		kicker := highestExcept(rankCount, quad)
		five := collectOfRank(all, quad, 4)
		five[4] = kickerCard(all, kicker)
		return fill(CatQuads, quad, kicker), five
	}

	// Full House (3+2; handle multiple trips/pairs)
	if len(groups) > 1 && groups[0].cnt == 3 {
		trip1 := groups[0].rank
		// Find next trip or a pair
		var pairOrTrip Rank
		for i := 1; i < len(groups); i++ {
			if groups[i].cnt >= 2 {
				pairOrTrip = groups[i].rank
				break
			}
		}
		if pairOrTrip != 0 {
			five := collectOfRank(all, trip1, 3)
			two := collectOfRank(all, pairOrTrip, 2)
			five[3], five[4] = two[0], two[1]
			return fill(CatFullHouse, trip1, pairOrTrip), five
		}
	}

	// Straight
	if top := straightTop(present); top != 0 {
		best, five := pickStraight(all, top)
		_ = best
		return fill(CatStraight, top), five
	}

	// Trips
	if len(groups) > 0 && groups[0].cnt == 3 {
		trip := groups[0].rank
		// pick two highest kickers excluding trip
		k1, k2 := topKickers(rankCount, trip, 2)
		five := collectOfRank(all, trip, 3)
		five[3] = kickerCard(all, k1)
		five[4] = kickerCard(all, k2)
		return fill(CatTrips, trip, k1, k2), five
	}

	// Two Pair
	if len(groups) > 1 && groups[0].cnt == 2 && groups[1].cnt == 2 {
		high := groups[0].rank
		low := groups[1].rank
		k := topKicker(rankCount, high, low)
		five := collectOfRank(all, high, 2)
		p2 := collectOfRank(all, low, 2)
		five[2], five[3] = p2[0], p2[1]
		five[4] = kickerCard(all, k)
		return fill(CatTwoPair, high, low, k), five
	}

	// One Pair
	if len(groups) > 0 && groups[0].cnt == 2 {
		pair := groups[0].rank
		k1, k2, k3 := topKickers3(rankCount, pair)
		five := collectOfRank(all, pair, 2)
		five[2] = kickerCard(all, k1)
		five[3] = kickerCard(all, k2)
		five[4] = kickerCard(all, k3)
		return fill(CatOnePair, pair, k1, k2, k3), five
	}

	// High Card
	hi := topNRanks(rankCount, 5, nil)
	var five [5]Card
	i := 0
	for _, r := range hi {
		five[i] = kickerCard(all, r)
		i++
	}
	return fill(CatHighCard, hi[0], hi[1], hi[2], hi[3], hi[4]), five
}

// ===== helpers (kept local to eval.go) =====

func highestExcept(rankCount [15]int, except Rank) Rank {
	for r := 14; r >= 2; r-- {
		if Rank(r) == except {
			continue
		}
		if rankCount[r] > 0 {
			return Rank(r)
		}
	}
	return 0
}

func topKicker(rankCount [15]int, ex1, ex2 Rank) Rank {
	for r := 14; r >= 2; r-- {
		if Rank(r) == ex1 || Rank(r) == ex2 {
			continue
		}
		if rankCount[r] > 0 {
			return Rank(r)
		}
	}
	return 0
}

func topKickers(rankCount [15]int, ex Rank, n int) (Rank, Rank) {
	var out []Rank
	for r := 14; r >= 2 && len(out) < n; r-- {
		if Rank(r) == ex {
			continue
		}
		if rankCount[r] > 0 {
			out = append(out, Rank(r))
		}
	}
	if len(out) == 1 {
		return out[0], 0
	}
	if len(out) == 0 {
		return 0, 0
	}
	return out[0], out[1]
}

func topKickers3(rankCount [15]int, ex Rank) (Rank, Rank, Rank) {
	var out []Rank
	for r := 14; r >= 2 && len(out) < 3; r-- {
		if Rank(r) == ex {
			continue
		}
		if rankCount[r] > 0 {
			out = append(out, Rank(r))
		}
	}
	for len(out) < 3 {
		out = append(out, 0)
	}
	return out[0], out[1], out[2]
}

func topNRanks(rankCount [15]int, n int, exclude []Rank) []Rank {
	ex := make(map[Rank]struct{}, len(exclude))
	for _, e := range exclude {
		ex[e] = struct{}{}
	}
	out := make([]Rank, 0, n)
	for r := 14; r >= 2 && len(out) < n; r-- {
		rr := Rank(r)
		if _, bad := ex[rr]; bad {
			continue
		}
		if rankCount[r] > 0 {
			out = append(out, rr)
		}
	}
	for len(out) < n {
		out = append(out, 0)
	}
	return out
}

func collectOfRank(all []Card, r Rank, want int) [5]Card {
	var out [5]Card
	i := 0
	for _, c := range all {
		if c.Rank == r {
			out[i] = c
			i++
			if i == want {
				break
			}
		}
	}
	return out
}

func kickerCard(all []Card, r Rank) Card {
	// return the highest card with rank r
	var best Card
	found := false
	for _, c := range all {
		if c.Rank == r {
			if !found || c.Suit > best.Suit { // break ties deterministically by suit
				best = c
				found = true
			}
		}
	}
	return best
}

func ofSuit(cards []Card) []Card { return cards }

// pickStraight returns the exact 5 cards forming a straight with given top rank.
// Works for wheel (top==5 → A-5).
func pickStraight(all []Card, top Rank) (HandValue, [5]Card) {
	var need [5]Rank
	if top == 5 {
		need = [5]Rank{5, 4, 3, 2, 14} // 5-4-3-2-A
	} else {
		need = [5]Rank{top, top - 1, top - 2, top - 3, top - 4}
	}
	var five [5]Card
	used := make(map[int]bool) // avoid reusing exact same card if duplicates (shouldn't happen)
	idx := 0
	for _, want := range need {
		// pick highest suit for determinism
		bestIdx := -1
		for i, c := range all {
			if used[i] {
				continue
			}
			if c.Rank == want {
				if bestIdx == -1 || c.Suit > all[bestIdx].Suit {
					bestIdx = i
				}
			}
		}
		if bestIdx >= 0 {
			five[idx] = all[bestIdx]
			used[bestIdx] = true
			idx++
		}
	}
	return HandValue{Cat: CatStraight, Ranks: [5]Rank{top}}, five
}

type ShowdownWinner struct {
	Player PlayerID
	Value  HandValue
	Cards  [5]Card
}

type ShowdownSummary struct {
	Winners     []ShowdownWinner
	PayoutPer   int64
	Remainder   int64
	TotalPayout int64 // = PayoutPer*len(Winners) + Remainder
}

// ResolveShowdown evaluates in-hand players, splits the pot evenly among winners,
// distributes any remainder deterministically (seat order from dealer+1), and ends the hand.
// It mutates stacks, clears Pot, sets HandActive=false, and leaves Phase as-is (typically PhaseShowdown).
func (s *State) ResolveShowdown() ShowdownSummary {
	// Collect eligible players (still in hand)
	type eval struct {
		pid   PlayerID
		val   HandValue
		cards [5]Card
	}
	var evals []eval
	for _, pid := range s.Order {
		st, ok := s.Seats[pid]
		_ = ok
		if !ok {
			continue
		}
		if !st.InHand || st.Folded {
			continue
		}
		hc := s.Holes[pid]
		if len(hc) != 2 {
			// If a player somehow lacks holes (mid-hand discover), treat as high-card only.
			// (Alternatively, skip; but this keeps the hand progressing.)
			hv, five := BestHand7(s.Board, hc)
			evals = append(evals, eval{pid: pid, val: hv, cards: five})
			continue
		}
		hv, five := BestHand7(s.Board, hc)
		evals = append(evals, eval{pid: pid, val: hv, cards: five})
	}
	if len(evals) == 0 {
		// No one to award: just end the hand.
		per, rem := int64(0), s.Pot
		s.Pot = 0
		s.HandActive = false
		return ShowdownSummary{Winners: nil, PayoutPer: per, Remainder: rem, TotalPayout: per + rem}
	}

	// Find best value
	best := evals[0].val
	for _, e := range evals[1:] {
		if best.Less(e.val) {
			best = e.val
		}
	}
	// Collect all winners (ties)
	var winners []ShowdownWinner
	for _, e := range evals {
		if !best.Less(e.val) && !e.val.Less(best) {
			winners = append(winners, ShowdownWinner{Player: e.pid, Value: e.val, Cards: e.cards})
		}
	}

	// Payout split
	nw := int64(len(winners))
	per := int64(0)
	rem := int64(0)
	if s.Pot > 0 && nw > 0 {
		per = s.Pot / nw
		rem = s.Pot % nw
	}

	// Pay each winner 'per'
	for _, w := range winners {
		if st, ok := s.Seats[w.Player]; ok {
			st.Stack += per
		}
	}
	// Deterministic remainder: distribute +1 to winners in seat order starting left of dealer
	if rem > 0 {
		// Build winner index set for quick membership
		winSet := map[PlayerID]int{}
		for i, w := range winners {
			winSet[w.Player] = i
		}
		start := (s.DealerIdx + 1) % len(s.Order)
		for i := 0; i < len(s.Order) && rem > 0; i++ {
			pid := s.Order[(start+i)%len(s.Order)]
			if idx, ok := winSet[pid]; ok {
				if st, ok2 := s.Seats[winners[idx].Player]; ok2 {
					st.Stack += 1
					rem--
				}
			}
		}
	}

	// End hand
	total := per*int64(len(winners)) + (s.Pot - per*int64(len(winners))) // we’ll zero pot next
	s.Pot = 0
	s.HandActive = false

	// Sort winners by seat order for stable logs
	sort.SliceStable(winners, func(i, j int) bool {
		pi, pj := posInOrder(s.Order, winners[i].Player), posInOrder(s.Order, winners[j].Player)
		return pi < pj
	})

	return ShowdownSummary{
		Winners:     winners,
		PayoutPer:   per,
		Remainder:   0, // already distributed
		TotalPayout: total,
	}
}

func posInOrder(order []PlayerID, pid PlayerID) int {
	for i, id := range order {
		if id == pid {
			return i
		}
	}
	return 1 << 30
}
