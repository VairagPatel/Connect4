package database

import (
	"connect-four/internal/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection with retry logic
func NewPostgresDB(connectionString string) (*PostgresDB, error) {
	if connectionString == "" {
		return nil, fmt.Errorf("database connection string is empty")
	}

	var db *sql.DB
	var err error
	maxRetries := 5
	retryDelay := 2 * time.Second

	// Retry connection with exponential backoff
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connectionString)
		if err != nil {
			log.Printf("Database connection attempt %d/%d failed: %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff
				continue
			}
			return nil, fmt.Errorf("failed to open database connection after %d attempts: %w", maxRetries, err)
		}

		// Set connection pool settings
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Test connection with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err != nil {
			log.Printf("Database ping attempt %d/%d failed: %v", i+1, maxRetries, err)
			db.Close()
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				retryDelay *= 2
				continue
			}
			return nil, fmt.Errorf("failed to ping database after %d attempts: %w", maxRetries, err)
		}

		log.Printf("Successfully connected to PostgreSQL database")
		break
	}

	postgresDB := &PostgresDB{db: db}

	// Run migrations automatically
	if err := postgresDB.RunMigrations(); err != nil {
		log.Printf("Warning: Failed to run database migrations: %v", err)
		// Don't fail completely, but log the error
	}

	return postgresDB, nil
}

// RunMigrations creates necessary tables if they don't exist
func (p *PostgresDB) RunMigrations() error {
	migrationSQL := `
		-- Create games table
		CREATE TABLE IF NOT EXISTS games (
			id VARCHAR(50) PRIMARY KEY,
			player1_id VARCHAR(100) NOT NULL,
			player1_username VARCHAR(50) NOT NULL,
			player2_id VARCHAR(100) NOT NULL,
			player2_username VARCHAR(50) NOT NULL,
			winner_id VARCHAR(100),
			move_count INTEGER NOT NULL DEFAULT 0,
			duration_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			finished_at TIMESTAMP,
			is_bot_game BOOLEAN NOT NULL DEFAULT FALSE
		);

		-- Create indexes for better query performance
		CREATE INDEX IF NOT EXISTS idx_games_winner ON games(winner_id);
		CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at);
		CREATE INDEX IF NOT EXISTS idx_games_is_bot ON games(is_bot_game);
		CREATE INDEX IF NOT EXISTS idx_games_player1 ON games(player1_username);
		CREATE INDEX IF NOT EXISTS idx_games_player2 ON games(player2_username);
	`

	_, err := p.db.Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Database migrations completed successfully")
	return nil
}

func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// SaveCompletedGame stores a finished game in the database
func (p *PostgresDB) SaveCompletedGame(game *models.Game) error {
	query := `
		INSERT INTO games (id, player1_id, player1_username, player2_id, player2_username, 
						  winner_id, move_count, duration_seconds, created_at, finished_at, is_bot_game)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	var winnerID *string
	if game.Winner != nil {
		winnerID = &game.Winner.ID
	}

	var duration int64
	if game.FinishedAt != nil {
		duration = int64(game.FinishedAt.Sub(game.CreatedAt).Seconds())
	}

	_, err := p.db.Exec(query,
		game.ID,
		game.Player1.ID,
		game.Player1.Username,
		game.Player2.ID,
		game.Player2.Username,
		winnerID,
		game.MoveCount,
		duration,
		game.CreatedAt,
		game.FinishedAt,
		game.Player2.IsBot,
	)

	return err
}

// GetLeaderboard returns top players by wins
func (p *PostgresDB) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	query := `
		SELECT 
			COALESCE(p1_wins.username, p2_wins.username, p1_games.username, p2_games.username) as username,
			COALESCE(p1_wins.wins, 0) + COALESCE(p2_wins.wins, 0) as total_wins,
			COALESCE(p1_games.games, 0) + COALESCE(p2_games.games, 0) as total_games
		FROM (
			-- Count wins as player1 (including wins against bots)
			SELECT player1_username as username, COUNT(*) as wins
			FROM games 
			WHERE winner_id = player1_id
			GROUP BY player1_username
		) p1_wins
		FULL OUTER JOIN (
			-- Count wins as player2 (only against humans, since player2 is bot in bot games)
			SELECT player2_username as username, COUNT(*) as wins
			FROM games 
			WHERE winner_id = player2_id AND NOT is_bot_game
			GROUP BY player2_username
		) p2_wins ON p1_wins.username = p2_wins.username
		FULL OUTER JOIN (
			-- Count total games as player1 (including bot games)
			SELECT player1_username as username, COUNT(*) as games
			FROM games 
			GROUP BY player1_username
		) p1_games ON COALESCE(p1_wins.username, p2_wins.username) = p1_games.username
		FULL OUTER JOIN (
			-- Count total games as player2 (only human vs human games)
			SELECT player2_username as username, COUNT(*) as games
			FROM games 
			WHERE NOT is_bot_game
			GROUP BY player2_username
		) p2_games ON COALESCE(p1_wins.username, p2_wins.username, p1_games.username) = p2_games.username
		WHERE COALESCE(p1_wins.username, p2_wins.username, p1_games.username, p2_games.username) IS NOT NULL
		ORDER BY total_wins DESC, total_games ASC
		LIMIT $1
	`

	rows, err := p.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		err := rows.Scan(&entry.Username, &entry.Wins, &entry.TotalGames)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		leaderboard = append(leaderboard, entry)
	}

	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating leaderboard rows: %w", err)
	}

	// Return empty array if no results (instead of nil)
	if leaderboard == nil {
		leaderboard = []LeaderboardEntry{}
	}

	return leaderboard, nil
}

type LeaderboardEntry struct {
	Username   string `json:"username"`
	Wins       int    `json:"wins"`
	TotalGames int    `json:"total_games"`
}