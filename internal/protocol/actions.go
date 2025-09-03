package protocol

type ActionType string

const (
	ActCreateTable ActionType = "CREATE_TABLE"
	ActJoin        ActionType = "JOIN"
	ActLeave       ActionType = "LEAVE"
	ActStartHand   ActionType = "START_HAND"
	ActBet         ActionType = "BET"
	ActCall        ActionType = "CALL"
	ActRaise       ActionType = "RAISE"
	ActCheck       ActionType = "CHECK"
	ActFold        ActionType = "FOLD"
	ActKick        ActionType = "KICK"
	ActAdvance     ActionType = "ADVANCE_PHASE"
	ActShowdown    ActionType = "SHOWDOWN"
)

type Action struct {
	ID       string         `json:"id"`
	Type     ActionType     `json:"type"`
	PlayerID string         `json:"player_id"`
	Amount   int64          `json:"amount,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}
