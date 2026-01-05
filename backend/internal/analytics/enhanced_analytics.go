package analytics

import (
	"connect-four/internal/models"
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// AnalyticsService provides comprehensive game analytics
type AnalyticsService struct {
	consumer *KafkaConsumer
	metrics  *GameAnalytics
	mutex    sync.RWMutex
}

// GameAnalytics stores comprehensive game metrics
type GameAnalytics struct {
	// Game Duration Metrics
	TotalGames      int64         `json:"total_games"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	
	// Winner Frequency Tracking
	WinnerCounts    map[string]int64 `json:"winner_counts"`
	MostFrequentWinner string        `json:"most_frequent_winner"`
	
	// Time-based Metrics
	GamesPerHour    map[string]int64 `json:"games_per_hour"`    // Hour -> Count
	GamesPerDay     map[string]int64 `json:"games_per_day"`     // Date -> Count
	
	// User-specific Metrics
	PlayerStats     map[string]*PlayerMetrics `json:"player_stats"`
	
	// Bot Analytics
	BotGames        int64   `json:"bot_games"`
	BotWins         int64   `json:"bot_wins"`
	BotWinRate      float64 `json:"bot_win_rate"`
	
	// Real-time Tracking
	ActiveGames     map[string]*GameSession `json:"active_games"`
	LastUpdated     time.Time              `json:"last_updated"`
}

// PlayerMetrics tracks individual player statistics
type PlayerMetrics struct {
	Username        string        `json:"username"`
	TotalGames      int64         `json:"total_games"`
	Wins            int64         `json:"wins"`
	Losses          int64         `json:"losses"`
	Draws           int64         `json:"draws"`
	WinRate         float64       `json:"win_rate"`
	AverageGameTime time.Duration `json:"average_game_time"`
	TotalPlayTime   time.Duration `json:"total_play_time"`
	LastPlayed      time.Time     `json:"last_played"`
}

// GameSession tracks active game metrics
type GameSession struct {
	GameID      string    `json:"game_id"`
	StartTime   time.Time `json:"start_time"`
	Player1     string    `json:"player1"`
	Player2     string    `json:"player2"`
	IsBot       bool      `json:"is_bot"`
	MoveCount   int       `json:"move_count"`
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(brokers []string, topic, groupID string) *AnalyticsService {
	consumer := NewKafkaConsumer(brokers, topic, groupID)
	
	return &AnalyticsService{
		consumer: consumer,
		metrics: &GameAnalytics{
			WinnerCounts:  make(map[string]int64),
			GamesPerHour:  make(map[string]int64),
			GamesPerDay:   make(map[string]int64),
			PlayerStats:   make(map[string]*PlayerMetrics),
			ActiveGames:   make(map[string]*GameSession),
			LastUpdated:   time.Now(),
		},
	}
}

// StartAnalytics begins consuming and processing game events
func (a *AnalyticsService) StartAnalytics(ctx context.Context) {
	log.Println("🔥 Analytics Service Started - Processing Game Events")
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := a.consumer.reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading Kafka message: %v", err)
				continue
			}

			var event models.GameEvent
			if err := json.Unmarshal(message.Value, &event); err != nil {
				log.Printf("Error unmarshaling event: %v", err)
				continue
			}

			a.processGameEvent(event)
		}
	}
}

// processGameEvent processes individual game events for analytics
func (a *AnalyticsService) processGameEvent(event models.GameEvent) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	a.metrics.LastUpdated = time.Now()
	
	switch event.Type {
	case models.EventGameStarted:
		a.handleGameStarted(event)
	case models.EventMovePlayed:
		a.handleMovePlayed(event)
	case models.EventGameWon:
		a.handleGameWon(event)
	case models.EventGameDraw:
		a.handleGameDraw(event)
	case models.EventGameForfeited:
		a.handleGameForfeited(event)
	}
	
	// Update derived metrics
	a.updateDerivedMetrics()
}

// handleGameStarted processes game start events
func (a *AnalyticsService) handleGameStarted(event models.GameEvent) {
	gameData, ok := event.Data.(*models.Game)
	if !ok {
		log.Printf("Invalid game data in GameStarted event")
		return
	}
	
	log.Printf("📊 Analytics: Game Started - %s", event.GameID)
	
	// Track active game session
	session := &GameSession{
		GameID:    event.GameID,
		StartTime: event.Timestamp,
		Player1:   gameData.Player1.Username,
		Player2:   gameData.Player2.Username,
		IsBot:     gameData.Player2.IsBot,
		MoveCount: 0,
	}
	a.metrics.ActiveGames[event.GameID] = session
	
	// Update time-based metrics
	hour := event.Timestamp.Format("2006-01-02-15")
	day := event.Timestamp.Format("2006-01-02")
	a.metrics.GamesPerHour[hour]++
	a.metrics.GamesPerDay[day]++
	
	// Initialize player stats if needed
	a.ensurePlayerStats(gameData.Player1.Username)
	if !gameData.Player2.IsBot {
		a.ensurePlayerStats(gameData.Player2.Username)
	}
}

// handleMovePlayed processes move events
func (a *AnalyticsService) handleMovePlayed(event models.GameEvent) {
	log.Printf("📊 Analytics: Move Played - Game %s", event.GameID)
	
	if session, exists := a.metrics.ActiveGames[event.GameID]; exists {
		session.MoveCount++
	}
}

// handleGameWon processes game completion with winner
func (a *AnalyticsService) handleGameWon(event models.GameEvent) {
	gameData, ok := event.Data.(*models.Game)
	if !ok {
		log.Printf("Invalid game data in GameWon event")
		return
	}
	
	log.Printf("📊 Analytics: Game Won - %s, Winner: %s", event.GameID, gameData.Winner.Username)
	
	a.processGameCompletion(gameData, event.Timestamp)
	
	// Track winner frequency
	winner := gameData.Winner.Username
	a.metrics.WinnerCounts[winner]++
	
	// Update player stats
	a.updatePlayerWin(gameData.Winner.Username, gameData, event.Timestamp)
	a.updatePlayerLoss(a.getOpponent(gameData, gameData.Winner.Username), gameData, event.Timestamp)
	
	// Track bot performance
	if gameData.Player2.IsBot {
		a.metrics.BotGames++
		if gameData.Winner.IsBot {
			a.metrics.BotWins++
		}
	}
}

// handleGameDraw processes draw games
func (a *AnalyticsService) handleGameDraw(event models.GameEvent) {
	gameData, ok := event.Data.(*models.Game)
	if !ok {
		log.Printf("Invalid game data in GameDraw event")
		return
	}
	
	log.Printf("📊 Analytics: Game Draw - %s", event.GameID)
	
	a.processGameCompletion(gameData, event.Timestamp)
	
	// Update player stats for draw
	a.updatePlayerDraw(gameData.Player1.Username, gameData, event.Timestamp)
	if !gameData.Player2.IsBot {
		a.updatePlayerDraw(gameData.Player2.Username, gameData, event.Timestamp)
	}
}

// handleGameForfeited processes forfeited games
func (a *AnalyticsService) handleGameForfeited(event models.GameEvent) {
	gameData, ok := event.Data.(*models.Game)
	if !ok {
		log.Printf("Invalid game data in GameForfeited event")
		return
	}
	
	log.Printf("📊 Analytics: Game Forfeited - %s, Winner: %s", event.GameID, gameData.Winner.Username)
	
	a.processGameCompletion(gameData, event.Timestamp)
	
	// Track forfeit as win/loss
	winner := gameData.Winner.Username
	a.metrics.WinnerCounts[winner]++
	a.updatePlayerWin(winner, gameData, event.Timestamp)
	a.updatePlayerLoss(a.getOpponent(gameData, winner), gameData, event.Timestamp)
}

// processGameCompletion handles common game completion logic
func (a *AnalyticsService) processGameCompletion(game *models.Game, endTime time.Time) {
	session, exists := a.metrics.ActiveGames[game.ID]
	if !exists {
		log.Printf("No active session found for game %s", game.ID)
		return
	}
	
	// Calculate game duration
	duration := endTime.Sub(session.StartTime)
	a.metrics.TotalGames++
	a.metrics.TotalDuration += duration
	
	// Remove from active games
	delete(a.metrics.ActiveGames, game.ID)
	
	log.Printf("📊 Game %s completed in %v with %d moves", game.ID, duration, session.MoveCount)
}

// ensurePlayerStats initializes player stats if they don't exist
func (a *AnalyticsService) ensurePlayerStats(username string) {
	if _, exists := a.metrics.PlayerStats[username]; !exists {
		a.metrics.PlayerStats[username] = &PlayerMetrics{
			Username:    username,
			TotalGames:  0,
			Wins:        0,
			Losses:      0,
			Draws:       0,
			WinRate:     0.0,
			LastPlayed:  time.Now(),
		}
	}
}

// updatePlayerWin updates winner statistics
func (a *AnalyticsService) updatePlayerWin(username string, game *models.Game, timestamp time.Time) {
	if username == "" {
		return
	}
	
	a.ensurePlayerStats(username)
	stats := a.metrics.PlayerStats[username]
	
	stats.TotalGames++
	stats.Wins++
	stats.LastPlayed = timestamp
	
	// Calculate win rate
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames)
	}
}

// updatePlayerLoss updates loser statistics
func (a *AnalyticsService) updatePlayerLoss(username string, game *models.Game, timestamp time.Time) {
	if username == "" {
		return
	}
	
	a.ensurePlayerStats(username)
	stats := a.metrics.PlayerStats[username]
	
	stats.TotalGames++
	stats.Losses++
	stats.LastPlayed = timestamp
	
	// Calculate win rate
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames)
	}
}

// updatePlayerDraw updates draw statistics
func (a *AnalyticsService) updatePlayerDraw(username string, game *models.Game, timestamp time.Time) {
	if username == "" {
		return
	}
	
	a.ensurePlayerStats(username)
	stats := a.metrics.PlayerStats[username]
	
	stats.TotalGames++
	stats.Draws++
	stats.LastPlayed = timestamp
	
	// Calculate win rate
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames)
	}
}

// getOpponent returns the opponent's username
func (a *AnalyticsService) getOpponent(game *models.Game, winner string) string {
	if game.Player1.Username == winner {
		return game.Player2.Username
	}
	return game.Player1.Username
}

// updateDerivedMetrics calculates derived analytics metrics
func (a *AnalyticsService) updateDerivedMetrics() {
	// Calculate average game duration
	if a.metrics.TotalGames > 0 {
		a.metrics.AverageDuration = a.metrics.TotalDuration / time.Duration(a.metrics.TotalGames)
	}
	
	// Find most frequent winner
	maxWins := int64(0)
	mostFrequentWinner := ""
	for username, wins := range a.metrics.WinnerCounts {
		if wins > maxWins {
			maxWins = wins
			mostFrequentWinner = username
		}
	}
	a.metrics.MostFrequentWinner = mostFrequentWinner
	
	// Calculate bot win rate
	if a.metrics.BotGames > 0 {
		a.metrics.BotWinRate = float64(a.metrics.BotWins) / float64(a.metrics.BotGames)
	}
}

// GetMetrics returns current analytics metrics
func (a *AnalyticsService) GetMetrics() *GameAnalytics {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	// Create a copy to avoid race conditions
	metricsCopy := *a.metrics
	return &metricsCopy
}

// PrintMetrics logs current analytics metrics
func (a *AnalyticsService) PrintMetrics() {
	metrics := a.GetMetrics()
	
	log.Printf("📊 === GAME ANALYTICS REPORT ===")
	log.Printf("📊 Total Games: %d", metrics.TotalGames)
	log.Printf("📊 Average Duration: %v", metrics.AverageDuration)
	log.Printf("📊 Most Frequent Winner: %s (%d wins)", metrics.MostFrequentWinner, metrics.WinnerCounts[metrics.MostFrequentWinner])
	log.Printf("📊 Bot Win Rate: %.2f%% (%d/%d)", metrics.BotWinRate*100, metrics.BotWins, metrics.BotGames)
	log.Printf("📊 Active Games: %d", len(metrics.ActiveGames))
	log.Printf("📊 Tracked Players: %d", len(metrics.PlayerStats))
	
	// Print top players
	log.Printf("📊 Top Players by Win Rate:")
	for username, stats := range metrics.PlayerStats {
		if stats.TotalGames >= 3 { // Only show players with 3+ games
			log.Printf("📊   %s: %.1f%% (%d/%d games)", username, stats.WinRate*100, stats.Wins, stats.TotalGames)
		}
	}
}

// Close closes the analytics service
func (a *AnalyticsService) Close() error {
	return a.consumer.Close()
}