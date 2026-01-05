package game

import (
	"connect-four/internal/models"
	"errors"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

// MakeMove attempts to place a disc in the specified column
func (e *Engine) MakeMove(game *models.Game, playerID string, column int) (*models.Move, error) {
	if game.State != models.GameStateActive {
		return nil, errors.New("game is not active")
	}

	if column < 0 || column >= models.BoardCols {
		return nil, errors.New("invalid column")
	}

	// Check if it's the player's turn
	var playerNum int
	if game.Player1.ID == playerID {
		playerNum = 1
	} else if game.Player2.ID == playerID {
		playerNum = 2
	} else {
		return nil, errors.New("player not in this game")
	}

	if game.CurrentTurn != playerNum {
		return nil, errors.New("not your turn")
	}

	// Find the lowest available row in the column
	row := -1
	for r := models.BoardRows - 1; r >= 0; r-- {
		if game.Board[r][column] == 0 {
			row = r
			break
		}
	}

	if row == -1 {
		return nil, errors.New("column is full")
	}

	// Place the disc
	game.Board[row][column] = playerNum
	game.MoveCount++

	move := &models.Move{
		GameID:   game.ID,
		PlayerID: playerID,
		Column:   column,
		Row:      row,
	}

	// Check for win condition
	if e.CheckWin(game.Board, row, column, playerNum) {
		game.State = models.GameStateFinished
		if playerNum == 1 {
			game.Winner = game.Player1
		} else {
			game.Winner = game.Player2
		}
	} else if e.IsBoardFull(game.Board) {
		game.State = models.GameStateFinished
		// Draw - no winner
	} else {
		// Switch turns
		if game.CurrentTurn == 1 {
			game.CurrentTurn = 2
		} else {
			game.CurrentTurn = 1
		}
	}

	return move, nil
}

// CheckWin checks if the last move resulted in a win
func (e *Engine) CheckWin(board [][]int, row, col, player int) bool {
	directions := [][]int{
		{0, 1},  // horizontal
		{1, 0},  // vertical
		{1, 1},  // diagonal /
		{1, -1}, // diagonal \
	}

	for _, dir := range directions {
		count := 1 // Count the current piece
		
		// Check in positive direction
		r, c := row+dir[0], col+dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols && board[r][c] == player {
			count++
			r += dir[0]
			c += dir[1]
		}
		
		// Check in negative direction
		r, c = row-dir[0], col-dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols && board[r][c] == player {
			count++
			r -= dir[0]
			c -= dir[1]
		}
		
		if count >= models.WinCondition {
			return true
		}
	}
	
	return false
}

// IsBoardFull checks if the board is completely filled
func (e *Engine) IsBoardFull(board [][]int) bool {
	for c := 0; c < models.BoardCols; c++ {
		if board[0][c] == 0 {
			return false
		}
	}
	return true
}

// GetValidMoves returns all valid column indices where a move can be made
func (e *Engine) GetValidMoves(board [][]int) []int {
	var validMoves []int
	for c := 0; c < models.BoardCols; c++ {
		if board[0][c] == 0 {
			validMoves = append(validMoves, c)
		}
	}
	return validMoves
}

// SimulateMove simulates placing a disc without modifying the original board
func (e *Engine) SimulateMove(board [][]int, column, player int) ([][]int, int, error) {
	// Create a copy of the board
	newBoard := make([][]int, models.BoardRows)
	for i := range board {
		newBoard[i] = make([]int, models.BoardCols)
		copy(newBoard[i], board[i])
	}

	// Find the lowest available row
	row := -1
	for r := models.BoardRows - 1; r >= 0; r-- {
		if newBoard[r][column] == 0 {
			row = r
			break
		}
	}

	if row == -1 {
		return nil, -1, errors.New("column is full")
	}

	newBoard[row][column] = player
	return newBoard, row, nil
}