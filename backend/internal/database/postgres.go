package database

import (
	"connect-four/internal/models"
	"database/sql"

	_ "github.com/lib/pq"
)

type PostgresDB struct {
	db *sql.DB
}

func NewPostgresDB(connectionString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresDB{db: db}, nil
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
		return nil, err
	}
	defer rows.Close()

	var leaderboard []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		err := rows.Scan(&entry.Username, &entry.Wins, &entry.TotalGames)
		if err != nil {
			return nil, err
		}
		leaderboard = append(leaderboard, entry)
	}

	return leaderboard, nil
}

type LeaderboardEntry struct {
	Username   string `json:"username"`
	Wins       int    `json:"wins"`
	TotalGames int    `json:"total_games"`
}