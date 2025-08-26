package engine

import "math/rand"

func NewDeck(r *rand.Rand) []Card {
	deck := make([]Card, 0, 52)
	for s := SuitClubs; s <= SuitSpades; s++ {
		for rnk := RankTwo; rnk <= RankAce; rnk++ {
			deck = append(deck, Card{Rank: rnk, Suit: s})
		}
	}
	// Fisher-Yates
	for i := len(deck) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	return deck
}
