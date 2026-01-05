package models

import (
	"time"
	"github.com/google/uuid"
)

const (
	BoardRows    = 6
	BoardCols    = 7
	WinCondition = 4
)

type Player struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

type GameState string

const (
	GameStateWaiting   GameState = "waiting"
	GameStateActive    GameState = "active"
	GameStateFinished  GameState = "finished"
	GameStateForfeited GameState = "forfeited"
)

type Game struct {
	ID             string      `json:"id"`
	Player1        *Player     `json:"player1"`
	Player2        *Player     `json:"player2"`
	Board          [][]int     `json:"board"` // 0=empty, 1=player1, 2=player2
	CurrentTurn    int         `json:"current_turn"` // 1 or 2
	State          GameState   `json:"state"`
	Winner         *Player     `json:"winner,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
	MoveCount      int         `json:"move_count"`
	LastMoveNumber int         `json:"last_move_number"` // For idempotency protection
	BotDifficulty  string      `json:"bot_difficulty,omitempty"` // "easy" or "normal"
}

type Move struct {
	GameID     string `json:"game_id"`
	PlayerID   string `json:"player_id"`
	Column     int    `json:"column"`
	Row        int    `json:"row"`
	MoveNumber int    `json:"move_number"` // For idempotency protection
}

type GameEvent struct {
	Type    string      `json:"type"`
	GameID  string      `json:"game_id"`
	Data    interface{} `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// Event types
const (
	EventGameStarted      = "GAME_STARTED"
	EventMovePlayed       = "MOVE_PLAYED"
	EventGameWon          = "GAME_WON"
	EventGameDraw         = "GAME_DRAW"
	EventGameForfeited    = "GAME_FORFEITED"
	EventPlayerJoined     = "PLAYER_JOINED"
	EventPlayerLeft       = "PLAYER_LEFT"
	EventJoinAcknowledged = "JOIN_ACKNOWLEDGED"
	EventMatchmakingStatus = "MATCHMAKING_STATUS"
	EventGameReconnected  = "GAME_RECONNECTED"
	EventReconnectError   = "RECONNECT_ERROR"
)

func NewGame(player1 *Player) *Game {
	board := make([][]int, BoardRows)
	for i := range board {
		board[i] = make([]int, BoardCols)
	}

	game := &Game{
		ID:             generateGameID(),
		Player1:        player1,
		Board:          board,
		CurrentTurn:    1, // Player1 always goes first - ensures human starts in bot games
		State:          GameStateWaiting,
		CreatedAt:      time.Now(),
		MoveCount:      0,
		LastMoveNumber: 0, // Initialize move sequence counter
	}
	
	// CRITICAL: Validate that human player always goes first
	// WHY: Ensures frontend receives correct turn information from the start
	if game.CurrentTurn != 1 {
		panic("Game initialization error: CurrentTurn must be 1 for human player")
	}
	
	return game
}

func generateGameID() string {
	return uuid.New().String()[:8]
}