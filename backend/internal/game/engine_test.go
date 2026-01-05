package game

import (
	"connect-four/internal/models"
	"testing"
)

func TestCheckWin(t *testing.T) {
	engine := NewEngine()
	
	// Create empty board
	board := make([][]int, models.BoardRows)
	for i := range board {
		board[i] = make([]int, models.BoardCols)
	}
	
	// Test horizontal win
	board[5][0] = 1
	board[5][1] = 1
	board[5][2] = 1
	board[5][3] = 1
	
	if !engine.CheckWin(board, 5, 3, 1) {
		t.Error("Should detect horizontal win")
	}
	
	// Test vertical win
	board = make([][]int, models.BoardRows)
	for i := range board {
		board[i] = make([]int, models.BoardCols)
	}
	
	board[2][0] = 2
	board[3][0] = 2
	board[4][0] = 2
	board[5][0] = 2
	
	if !engine.CheckWin(board, 2, 0, 2) {
		t.Error("Should detect vertical win")
	}
}

func TestBotStrategy(t *testing.T) {
	bot := NewBot("test-bot")
	
	// Create a game where bot can win
	game := models.NewGame(&models.Player{ID: "player1", Username: "Player1"})
	game.Player2 = bot.player
	game.State = models.GameStateActive
	
	// Set up board where bot (player 2) can win horizontally
	game.Board[5][0] = 2
	game.Board[5][1] = 2
	game.Board[5][2] = 2
	// Bot should play column 3 to win
	
	move := bot.GetBestMove(game)
	if move != 3 {
		t.Errorf("Bot should play winning move at column 3, but played %d", move)
	}
}