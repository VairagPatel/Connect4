import React from 'react';

function Leaderboard({ leaderboard }) {
  if (!leaderboard || leaderboard.length === 0) {
    return (
      <div className="leaderboard">
        <h2>🏆 Leaderboard</h2>
        <p>No games played yet. Be the first to play!</p>
      </div>
    );
  }

  return (
    <div className="leaderboard">
      <h2>🏆 Leaderboard</h2>
      <ul className="leaderboard-list">
        {leaderboard.map((player, index) => (
          <li key={player.username} className="leaderboard-item">
            <span>
              {index + 1}. {player.username}
            </span>
            <span>
              {player.wins} wins ({player.total_games} games)
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export default Leaderboard;