import React from 'react';

function GameBoard({ game, username, onMove, gameStatus, onNewGame, onGoHome, gameEnded, showGameId, onToggleGameId }) {
  const [processingMove, setProcessingMove] = React.useState(false);
  const [gameIdCopied, setGameIdCopied] = React.useState(false);

  const isMyTurn = () => {
    if (!game || game.state !== 'active') return false;
    
    const currentPlayer = game.current_turn === 1 ? game.player1 : game.player2;
    return currentPlayer.username === username && !processingMove;
  };

  const handleColumnClick = (column) => {
    if (isMyTurn()) {
      setProcessingMove(true);
      onMove(column);
      // Reset processing state after a short delay
      setTimeout(() => setProcessingMove(false), 1000);
    }
  };

  // Reset processing state when it's no longer our turn
  React.useEffect(() => {
    if (game && game.state === 'active') {
      const currentPlayer = game.current_turn === 1 ? game.player1 : game.player2;
      if (currentPlayer.username !== username) {
        setProcessingMove(false);
      }
    }
  }, [game, username]);

  const getCellClass = (cellValue) => {
    if (cellValue === 0) return 'board-cell empty';
    if (cellValue === 1) return 'board-cell player1';
    if (cellValue === 2) return 'board-cell player2';
    return 'board-cell';
  };

  const getCellSymbol = (cellValue) => {
    if (cellValue === 1) return '●';
    if (cellValue === 2) return '●';
    return '';
  };

  const getStatusClass = () => {
    if (game.state === 'waiting') return 'game-status waiting';
    if (game.state === 'active') return 'game-status active';
    if (gameEnded) return 'game-status finished';
    return 'game-status finished';
  };

  const copyGameId = () => {
    navigator.clipboard.writeText(game.id).then(() => {
      setGameIdCopied(true);
      setTimeout(() => setGameIdCopied(false), 2000);
    });
  };

  const isMultiplayerGame = () => {
    return game && !game.player2.is_bot;
  };

  return (
    <div className="game-container">
      <div className="game-info">
        <div className="player-info">
          <div className={`player-card ${game.current_turn === 1 ? 'active' : ''}`}>
            <h3>{game.player1.username}</h3>
            <div style={{ color: '#ff4444' }}>● Red</div>
            {game.player1.username === username && <small>(You)</small>}
          </div>
          <div className={`player-card ${game.current_turn === 2 ? 'active' : ''}`}>
            <h3>{game.player2.username}</h3>
            <div style={{ color: '#ffdd44' }}>● Yellow</div>
            {game.player2.username === username && <small>(You)</small>}
            {game.player2.is_bot && <small>(Bot)</small>}
          </div>
        </div>
      </div>

      <div className={getStatusClass()}>
        {processingMove ? 'Processing move...' : gameStatus}
      </div>

      <div className="game-board">
        {game.board.map((row, rowIndex) => (
          <div key={rowIndex} className="board-row">
            {row.map((cell, colIndex) => (
              <div
                key={`${rowIndex}-${colIndex}`}
                className={getCellClass(cell)}
                onClick={() => handleColumnClick(colIndex)}
                style={{
                  cursor: isMyTurn() && cell === 0 ? 'pointer' : 'default',
                  opacity: processingMove ? 0.7 : 1
                }}
              >
                {getCellSymbol(cell)}
              </div>
            ))}
          </div>
        ))}
      </div>

      <div style={{ marginTop: '20px' }}>
        <div className="game-info-section">
          <div className="game-details">
            <p><strong>Game ID:</strong> {game.id}</p>
            <p><strong>Moves:</strong> {game.move_count}</p>
            {isMultiplayerGame() && (
              <div className="multiplayer-info">
                <p>🌐 <strong>Multiplayer Game</strong></p>
                <button 
                  onClick={copyGameId}
                  className="copy-button"
                  title="Copy Game ID to share with friends"
                >
                  {gameIdCopied ? '✅ Copied!' : '📋 Copy Game ID'}
                </button>
                <p className="share-info">
                  Share this Game ID with friends to let them join from other devices!
                </p>
              </div>
            )}
          </div>
        </div>
        
        {gameEnded ? (
          <div className="game-end-actions">
            <h3>Game Over!</h3>
            <div className="action-buttons">
              <button onClick={onNewGame} className="primary-button">
                🎮 Play Again
              </button>
              <button onClick={onGoHome} className="secondary-button">
                🏠 Go Home
              </button>
            </div>
          </div>
        ) : (
          <div className="game-active-info">
            {game.state === 'active' && (
              <div style={{ marginTop: '10px', fontSize: '12px', color: '#666' }}>
                💡 If disconnected, you have 30 seconds to reconnect automatically
              </div>
            )}
            
            {isMultiplayerGame() && game.state === 'active' && (
              <div style={{ marginTop: '10px', fontSize: '12px', color: '#007bff' }}>
                🌐 Playing with {game.player1.username === username ? game.player2.username : game.player1.username} on another device
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default GameBoard;