package game

import (
	"connect-four/internal/models"
)

type BotDifficulty string

const (
	Easy   BotDifficulty = "easy"
	Normal BotDifficulty = "normal"
)

type Bot struct {
	engine     *Engine
	player     *models.Player
	difficulty BotDifficulty
}

func NewBot(playerID string) *Bot {
	return &Bot{
		engine: NewEngine(),
		player: &models.Player{
			ID:       playerID,
			Username: "CompetitiveBot",
			IsBot:    true,
		},
		difficulty: Normal, // Default difficulty
	}
}

func NewBotWithDifficulty(playerID string, difficulty BotDifficulty) *Bot {
	return &Bot{
		engine: NewEngine(),
		player: &models.Player{
			ID:       playerID,
			Username: "CompetitiveBot",
			IsBot:    true,
		},
		difficulty: difficulty,
	}
}

// GetBestMove implements competitive bot strategy with difficulty-based decision making
// WHY: Provides strategic gameplay while allowing different skill levels for user experience
//
// BOT DECISION HIERARCHY (by priority):
// 1. WINNING MOVE: Always take immediate wins (all difficulties) - prevents missed opportunities
// 2. DEFENSIVE BLOCKING: Block opponent wins (Normal difficulty only) - prevents easy losses  
// 3. THREAT ANALYSIS: Look for opponent's potential threats (Normal difficulty) - prevents setups
// 4. STRATEGIC POSITIONING: Build chains and control center (difficulty-dependent) - creates wins
//
// DIFFICULTY DIFFERENCES:
// - Easy: Skips defensive analysis, simpler positioning - more casual gameplay
// - Normal: Full competitive analysis with threat blocking and multi-move planning
//
// RACE CONDITION PREVENTION:
// - All bot decisions are deterministic based on board state
// - No random elements that could cause inconsistent behavior during reconnections
// - Move selection uses board state as pseudo-random seed for variation while maintaining determinism
func (b *Bot) GetBestMove(game *models.Game) int {
	board := game.Board
	botPlayer := 2 // Bot is always player 2
	humanPlayer := 1

	// PRIORITY 1: Check if bot can win immediately (always highest priority)
	// WHY: Winning moves must never be missed regardless of difficulty
	if winMove := b.findWinningMove(board, botPlayer); winMove != -1 {
		return winMove
	}

	// PRIORITY 2: Block opponent's winning move (difficulty-dependent)
	// WHY: Easy mode skips defensive analysis for more casual gameplay
	// Normal mode includes full defensive analysis for competitive play
	if b.difficulty != Easy {
		if blockMove := b.findWinningMove(board, humanPlayer); blockMove != -1 {
			return blockMove
		}
		
		// PRIORITY 3: Advanced threat analysis - look for opponent's setup moves
		// This prevents the bot from being exploited by the same patterns
		if threatMove := b.findThreatMove(board, humanPlayer); threatMove != -1 {
			return threatMove
		}
	}

	// PRIORITY 4: Strategic positioning based on difficulty level
	// WHY: Different difficulties provide appropriate challenge levels
	return b.getStrategicMove(board, botPlayer)
}

// findThreatMove looks for opponent moves that would create multiple winning threats
// WHY: Prevents opponent from setting up winning combinations that are hard to defend
func (b *Bot) findThreatMove(board [][]int, opponent int) int {
	validMoves := b.engine.GetValidMoves(board)
	
	for _, col := range validMoves {
		// Simulate opponent's move
		newBoard, row, err := b.engine.SimulateMove(board, col, opponent)
		if err != nil {
			continue
		}
		
		// Check if this move creates multiple threats for opponent
		threatCount := b.countThreats(newBoard, row, col, opponent)
		if threatCount >= 2 {
			// Opponent would have multiple ways to win - we should block this setup
			return col
		}
		
		// Also check if opponent gets a strong position (3 in a row with open ends)
		if b.hasStrongPosition(newBoard, row, col, opponent) {
			return col
		}
	}
	
	return -1
}

// countThreats counts how many different ways a player can win from a position
func (b *Bot) countThreats(board [][]int, row, col, player int) int {
	threats := 0
	directions := [][]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	
	for _, dir := range directions {
		count := 1
		openEnds := 0
		
		// Check positive direction
		r, c := row+dir[0], col+dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols {
			if board[r][c] == player {
				count++
				r += dir[0]
				c += dir[1]
			} else if board[r][c] == 0 {
				openEnds++
				break
			} else {
				break
			}
		}
		
		// Check negative direction
		r, c = row-dir[0], col-dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols {
			if board[r][c] == player {
				count++
				r -= dir[0]
				c -= dir[1]
			} else if board[r][c] == 0 {
				openEnds++
				break
			} else {
				break
			}
		}
		
		// Count as threat if we have 3+ in a row with open ends
		if count >= 3 && openEnds > 0 {
			threats++
		}
	}
	
	return threats
}

// hasStrongPosition checks if a position gives strong strategic advantage
func (b *Bot) hasStrongPosition(board [][]int, row, col, player int) bool {
	directions := [][]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	
	for _, dir := range directions {
		count := 1
		spaces := 0
		
		// Check both directions for potential 4-in-a-row
		for _, mult := range []int{1, -1} {
			r, c := row+dir[0]*mult, col+dir[1]*mult
			for i := 0; i < 3 && r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols; i++ {
				if board[r][c] == player {
					count++
				} else if board[r][c] == 0 {
					spaces++
				} else {
					break
				}
				r += dir[0] * mult
				c += dir[1] * mult
			}
		}
		
		// Strong position: 2+ pieces with room to grow to 4
		if count >= 2 && count+spaces >= 4 {
			return true
		}
	}
	
	return false
}
func (b *Bot) findWinningMove(board [][]int, player int) int {
	validMoves := b.engine.GetValidMoves(board)
	
	for _, col := range validMoves {
		// Simulate the move
		newBoard, row, err := b.engine.SimulateMove(board, col, player)
		if err != nil {
			continue
		}
		
		// Check if this move wins
		if b.engine.CheckWin(newBoard, row, col, player) {
			return col
		}
	}
	
	return -1
}

// getQuickStrategicMove implements enhanced strategic positioning with randomization
func (b *Bot) getQuickStrategicMove(board [][]int, player int) int {
	validMoves := b.engine.GetValidMoves(board)
	if len(validMoves) == 0 {
		return -1
	}

	type moveScore struct {
		column int
		score  int
	}
	
	var moves []moveScore
	bestScore := -1000

	// Evaluate all moves and collect the best ones
	for _, col := range validMoves {
		score := b.enhancedEvaluateMove(board, col, player)
		moves = append(moves, moveScore{col, score})
		if score > bestScore {
			bestScore = score
		}
	}

	// Collect all moves within 20% of the best score for randomization
	// WHY: This prevents predictable play while maintaining good strategy
	var goodMoves []int
	threshold := bestScore - (bestScore / 5) // 20% tolerance
	
	for _, move := range moves {
		if move.score >= threshold {
			goodMoves = append(goodMoves, move.column)
		}
	}

	// Add some randomization to prevent predictable patterns
	if len(goodMoves) > 1 {
		// Use game state as pseudo-random seed to ensure deterministic but varied play
		seed := 0
		for i := 0; i < models.BoardRows; i++ {
			for j := 0; j < models.BoardCols; j++ {
				seed += board[i][j] * (i*7 + j + 1)
			}
		}
		return goodMoves[seed%len(goodMoves)]
	}

	return goodMoves[0]
}

// enhancedEvaluateMove provides comprehensive move evaluation
func (b *Bot) enhancedEvaluateMove(board [][]int, col, player int) int {
	score := 0

	// Simulate the move
	newBoard, row, err := b.engine.SimulateMove(board, col, player)
	if err != nil {
		return -1000
	}

	// 1. Center control (but less predictable)
	centerDistance := abs(col - 3)
	score += (4 - centerDistance) * 8 // Reduced weight

	// 2. Enhanced position evaluation
	score += b.enhancedPositionScore(newBoard, row, col, player)

	// 3. Avoid giving opponent immediate win (critical)
	opponentPlayer := 3 - player
	if b.findWinningMove(newBoard, opponentPlayer) != -1 {
		score -= 500 // Heavy penalty
	}

	// 4. Look for opponent threats after our move
	threatCount := 0
	for _, testCol := range b.engine.GetValidMoves(newBoard) {
		testBoard, testRow, err := b.engine.SimulateMove(newBoard, testCol, opponentPlayer)
		if err != nil {
			continue
		}
		if b.engine.CheckWin(testBoard, testRow, testCol, opponentPlayer) {
			threatCount++
		}
	}
	score -= threatCount * 100 // Penalty for giving opponent multiple threats

	// 5. Prefer moves that create our own threats
	ourThreats := b.countThreats(newBoard, row, col, player)
	score += ourThreats * 150

	// 6. Avoid edges early in game (first few moves)
	moveCount := 0
	for i := 0; i < models.BoardRows; i++ {
		for j := 0; j < models.BoardCols; j++ {
			if board[i][j] != 0 {
				moveCount++
			}
		}
	}
	if moveCount < 8 && (col == 0 || col == 6) {
		score -= 30 // Slight penalty for edge moves early
	}

	// 7. Height consideration - prefer lower positions for stability
	score += (models.BoardRows - row) * 5

	return score
}

// enhancedPositionScore provides better position evaluation
func (b *Bot) enhancedPositionScore(board [][]int, row, col, player int) int {
	score := 0
	directions := [][]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

	for _, dir := range directions {
		// Count consecutive pieces and potential
		consecutive := 1
		potential := 1 // Space for growth
		
		// Check positive direction
		r, c := row+dir[0], col+dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols {
			if board[r][c] == player {
				consecutive++
				potential++
			} else if board[r][c] == 0 {
				potential++
				if potential >= 4 {
					break // Don't count beyond 4-in-a-row potential
				}
			} else {
				break // Blocked by opponent
			}
			r += dir[0]
			c += dir[1]
		}
		
		// Check negative direction
		r, c = row-dir[0], col-dir[1]
		for r >= 0 && r < models.BoardRows && c >= 0 && c < models.BoardCols {
			if board[r][c] == player {
				consecutive++
				potential++
			} else if board[r][c] == 0 {
				potential++
				if potential >= 4 {
					break
				}
			} else {
				break
			}
			r -= dir[0]
			c -= dir[1]
		}
		
		// Score based on consecutive pieces and potential
		if potential >= 4 { // Can potentially make 4 in a row
			if consecutive >= 3 {
				score += 100 // Very strong position
			} else if consecutive >= 2 {
				score += 30  // Good position
			} else {
				score += 10  // Some potential
			}
		}
	}

	return score
}

// getStrategicMove implements strategic positioning based on difficulty
func (b *Bot) getStrategicMove(board [][]int, player int) int {
	if b.difficulty == Easy {
		return b.getEasyMove(board, player)
	}
	return b.getQuickStrategicMove(board, player)
}

// getEasyMove implements simplified strategy for easy difficulty
func (b *Bot) getEasyMove(board [][]int, player int) int {
	validMoves := b.engine.GetValidMoves(board)
	if len(validMoves) == 0 {
		return -1
	}

	// Easy mode: prefer center columns but with some randomization
	centerPreference := []int{3, 2, 4, 1, 5, 0, 6}
	
	// Add some variation based on board state
	seed := 0
	for i := 0; i < models.BoardRows; i++ {
		for j := 0; j < models.BoardCols; j++ {
			seed += board[i][j] * (i + j + 1)
		}
	}
	
	// Rotate the preference order based on game state
	rotation := seed % len(centerPreference)
	rotatedPreference := make([]int, len(centerPreference))
	for i := range centerPreference {
		rotatedPreference[i] = centerPreference[(i+rotation)%len(centerPreference)]
	}
	
	for _, col := range rotatedPreference {
		for _, validCol := range validMoves {
			if col == validCol {
				return col
			}
		}
	}
	
	return validMoves[0]
}

// findWinningMove checks if player can win in the next move

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}