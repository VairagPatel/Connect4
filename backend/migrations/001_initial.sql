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