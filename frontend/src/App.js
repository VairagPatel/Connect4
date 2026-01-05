import React, { useState, useEffect, useCallback } from 'react';
import GameBoard from './components/GameBoard';
import LoginScreen from './components/LoginScreen';
import Leaderboard from './components/Leaderboard';
import './App.css';

function App() {
  const [ws, setWs] = useState(null);
  const [connected, setConnected] = useState(false);
  const [username, setUsername] = useState('');
  const [gameState, setGameState] = useState(null);
  const [gameStatus, setGameStatus] = useState('');
  const [leaderboard, setLeaderboard] = useState([]);
  const [matchmakingTimer, setMatchmakingTimer] = useState(0);
  const [gameEnded, setGameEnded] = useState(false);
  const [showGameId, setShowGameId] = useState(false);

  const fetchLeaderboard = useCallback(async () => {
    try {
      const response = await fetch(`${process.env.REACT_APP_API_URL || 'http://localhost:8081'}/leaderboard`);
      const data = await response.json();
      setLeaderboard(data);
    } catch (error) {
      console.error('Error fetching leaderboard:', error);
    }
  }, []);

  const handleWebSocketMessage = useCallback((message) => {
    console.log('Received message:', message);
    
    switch (message.type) {
      case 'JOIN_ACKNOWLEDGED':
        console.log('Join acknowledged:', message.data);
        setGameStatus('🔍 Looking for opponent...');
        break;
        
      case 'MATCHMAKING_STATUS':
        if (message.data.status === 'waiting_for_opponent') {
          setGameStatus(`🔍 Looking for opponent... (${message.data.timer_seconds}s)`);
          startMatchmakingTimer();
        } else if (message.data.status === 'starting_bot_game') {
          setGameStatus('🤖 Starting game with bot...');
          setMatchmakingTimer(0);
        }
        break;
        
      case 'GAME_STARTED':
        setGameState(message.data);
        setGameEnded(false);
        setMatchmakingTimer(0);
        
        // CRITICAL: Ensure correct turn display - human always goes first
        // WHY: Game always starts with current_turn=1 (Player1), and human is always Player1
        const currentPlayerName = message.data.current_turn === 1 ? 
          message.data.player1.username : 
          message.data.player2.username;
        
        if (message.data.current_turn === 1) {
          // It's always the human's turn first (Player1)
          setGameStatus('Your turn! Make the first move.');
        } else {
          // This should never happen on game start, but handle it just in case
          setGameStatus(`${currentPlayerName}'s turn`);
        }
        
        console.log('Game started - Current turn:', message.data.current_turn, 
                   'Player1:', message.data.player1.username, 
                   'Player2:', message.data.player2.username,
                   'Your username:', username);
        break;
        
      case 'GAME_RECONNECTED':
        setGameState(message.data);
        setGameEnded(false);
        const currentPlayer = message.data.current_turn === 1 ? 
          message.data.player1.username : 
          message.data.player2.username;
        setGameStatus(`Reconnected! ${currentPlayer === username ? 'Your' : currentPlayer + "'s"} turn`);
        break;
        
      case 'RECONNECT_ERROR':
        console.error('Reconnection failed:', message.data);
        setGameStatus(`❌ Reconnection failed: ${message.data.error}`);
        break;
        
      case 'MOVE_PLAYED':
        setGameState(message.data.game);
        if (message.data.game.state === 'active') {
          const currentPlayerName = message.data.game.current_turn === 1 ? 
            message.data.game.player1.username : 
            message.data.game.player2.username;
          
          // CRITICAL FIX: Handle bot game turn display correctly
          // WHY: After human makes move, turn switches to bot, but we should show "Bot is thinking..."
          // instead of "Bot's turn" to avoid confusion
          
          if (message.data.game.player2.is_bot) {
            // This is a bot game
            if (message.data.game.current_turn === 1) {
              // It's the human's turn
              setGameStatus('Your turn!');
            } else {
              // It's the bot's turn - show thinking message instead of "Bot's turn"
              setGameStatus('🤖 Bot is thinking...');
            }
          } else {
            // This is a human vs human game
            const isHumanTurn = (message.data.game.current_turn === 1 && message.data.game.player1.username === username) ||
                               (message.data.game.current_turn === 2 && message.data.game.player2.username === username);
            
            setGameStatus(isHumanTurn ? 'Your turn!' : `${currentPlayerName}'s turn`);
          }
          
          console.log('Move played - Current turn:', message.data.game.current_turn,
                     'Current player:', currentPlayerName,
                     'Is bot game:', message.data.game.player2.is_bot);
        }
        break;
        
      case 'GAME_WON':
        setGameState(message.data);
        setGameEnded(true);
        const winner = message.data.winner;
        if (winner.username === username) {
          setGameStatus('🎉 You won! Congratulations!');
        } else {
          setGameStatus(`😔 ${winner.username} wins! Better luck next time.`);
        }
        // CRITICAL: Refresh leaderboard on game completion
        // WHY: Ensures users see updated win statistics immediately after games end
        fetchLeaderboard();
        break;
        
      case 'GAME_DRAW':
        setGameState(message.data);
        setGameEnded(true);
        setGameStatus('🤝 It\'s a draw! Good game!');
        // CRITICAL: Refresh leaderboard on draw
        // WHY: Draw games still update total game counts in leaderboard
        fetchLeaderboard();
        break;
        
      case 'GAME_FORFEITED':
        setGameState(message.data);
        setGameEnded(true);
        setGameStatus('🏃 Opponent forfeited. You win!');
        // CRITICAL: Refresh leaderboard on forfeit
        // WHY: Forfeit results in a win that should be reflected in leaderboard
        fetchLeaderboard();
        break;
        
      default:
        console.log('Unknown message type:', message.type);
    }
  }, [username, fetchLeaderboard]);

  const connectWebSocket = useCallback(() => {
    // Prevent multiple connection attempts
    if (ws && ws.readyState === WebSocket.CONNECTING) {
      console.log('WebSocket connection already in progress');
      return;
    }
    
    const websocket = new WebSocket(process.env.REACT_APP_WS_URL || 'ws://localhost:8081/ws');
    
    websocket.onopen = () => {
      console.log('Connected to WebSocket');
      setConnected(true);
      setWs(websocket);
      
      // CRITICAL: Automatic reconnection with RECONNECT payload
      // WHY: Ensures seamless reconnection for users with existing games
      // Automatically sends RECONNECT payload if username exists (user was playing)
      if (username) {
        console.log('Auto-reconnecting with username:', username);
        websocket.send(JSON.stringify({
          type: 'RECONNECT',
          data: { username: username }
        }));
      }
    };

    websocket.onclose = (event) => {
      console.log('Disconnected from WebSocket', event.code, event.reason);
      setConnected(false);
      setWs(null);
      
      // Only attempt reconnection if it wasn't a normal closure
      if (event.code !== 1000) {
        // Automatic reconnection attempt after 3 seconds
        // WHY: Provides seamless recovery from temporary network issues
        console.log('Attempting reconnection in 3 seconds...');
        setTimeout(connectWebSocket, 3000);
      }
    };

    websocket.onerror = (error) => {
      console.error('WebSocket error:', error);
      setGameStatus('❌ Connection error. Reconnecting...');
    };

    websocket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        handleWebSocketMessage(message);
      } catch (error) {
        console.error('Error parsing WebSocket message:', error);
      }
    };
    
    // Store websocket reference for cleanup
    setWs(websocket);
  }, [username, handleWebSocketMessage, ws]);

  useEffect(() => {
    connectWebSocket();
    fetchLeaderboard();
  }, [connectWebSocket, fetchLeaderboard]);

  const handleLogin = (playerUsername) => {
    if (!playerUsername || playerUsername.trim() === '') {
      alert('Please enter a valid username');
      return;
    }
    
    setUsername(playerUsername);
    setGameState(null);
    setGameEnded(false);
    setGameStatus('Connecting...');
    
    if (ws && connected) {
      console.log('Sending JOIN message for:', playerUsername);
      ws.send(JSON.stringify({
        type: 'JOIN',
        data: { username: playerUsername }
      }));
    } else {
      setGameStatus('❌ Not connected to server. Reconnecting...');
      // Trigger reconnection
      connectWebSocket();
    }
  };

  const startMatchmakingTimer = () => {
    setMatchmakingTimer(10);
    const timer = setInterval(() => {
      setMatchmakingTimer(prev => {
        if (prev <= 1) {
          clearInterval(timer);
          setGameStatus('🤖 Starting game with bot...');
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  };

  const handleMove = (column) => {
    if (ws && connected && gameState && gameState.state === 'active') {
      // Provide immediate feedback for user's move
      setGameStatus('Making your move...');
      
      ws.send(JSON.stringify({
        type: 'MAKE_MOVE',
        data: { column }
      }));
    }
  };

  const handleNewGame = () => {
    setGameState(null);
    setGameStatus('');
    setGameEnded(false);
    setMatchmakingTimer(0);
    if (ws && connected) {
      ws.send(JSON.stringify({
        type: 'JOIN',
        data: { username }
      }));
      setGameStatus('🔍 Looking for opponent...');
      startMatchmakingTimer();
    }
  };

  const handleGoHome = () => {
    setUsername('');
    setGameState(null);
    setGameStatus('');
    setGameEnded(false);
    setMatchmakingTimer(0);
    setShowGameId(false);
  };

  const handleJoinByGameId = (gameId) => {
    if (ws && connected && gameId.trim()) {
      ws.send(JSON.stringify({
        type: 'RECONNECT',
        data: { username: username, game_id: gameId.trim() }
      }));
      setGameStatus('🔗 Joining game...');
    }
  };

  if (!username) {
    return (
      <div className="app">
        <div className="connection-status">
          <span className={connected ? 'connected' : 'disconnected'}>
            {connected ? '● Connected' : '● Disconnected'}
          </span>
        </div>
        
        <div className="app-header">
          <img src="/logo.svg" alt="4 in a Row" className="app-logo" />
          <h1>4 in a Row</h1>
          <p className="app-subtitle">Real-time Multiplayer Connect Four</p>
        </div>
        
        <LoginScreen onLogin={handleLogin} connected={connected} />
        <Leaderboard leaderboard={leaderboard} />
      </div>
    );
  }

  return (
    <div className="app">
      <div className="connection-status">
        <span className={connected ? 'connected' : 'disconnected'}>
          {connected ? '● Connected' : '● Disconnected'}
        </span>
      </div>
      
      <div className="app-header">
        <img src="/logo.svg" alt="4 in a Row" className="app-logo" />
        <h1>4 in a Row</h1>
        <button 
          className="home-button" 
          onClick={handleGoHome}
          title="Go to Home"
        >
          🏠 Home
        </button>
      </div>
      
      {gameState ? (
        <GameBoard 
          game={gameState}
          username={username}
          onMove={handleMove}
          gameStatus={gameStatus}
          onNewGame={handleNewGame}
          onGoHome={handleGoHome}
          gameEnded={gameEnded}
          showGameId={showGameId}
          onToggleGameId={() => setShowGameId(!showGameId)}
        />
      ) : (
        <div className="game-container">
          <div className="game-status waiting">
            {gameStatus || 'Click "Find Game" to start playing!'}
            {matchmakingTimer > 0 && (
              <div className="matchmaking-timer">
                <div className="timer-bar">
                  <div 
                    className="timer-progress" 
                    style={{ width: `${(matchmakingTimer / 10) * 100}%` }}
                  ></div>
                </div>
                <p>Finding opponent... {matchmakingTimer}s (Bot game starts automatically)</p>
              </div>
            )}
          </div>
          
          <div className="game-actions">
            <button onClick={handleNewGame} disabled={!connected}>
              🎮 Find Game
            </button>
            
            <button 
              onClick={() => setShowGameId(!showGameId)} 
              disabled={!connected}
              className="secondary-button"
            >
              🔗 Join by Game ID
            </button>
          </div>
          
          {showGameId && (
            <JoinGameForm onJoin={handleJoinByGameId} />
          )}
        </div>
      )}
      
      <Leaderboard leaderboard={leaderboard} />
    </div>
  );
}

// JoinGameForm component for joining games by ID
function JoinGameForm({ onJoin }) {
  const [gameId, setGameId] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    if (gameId.trim()) {
      onJoin(gameId);
      setGameId('');
    }
  };

  return (
    <div className="join-game-form">
      <h3>Join Multiplayer Game</h3>
      <p>Enter a Game ID to join a friend's game on another device:</p>
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Enter Game ID (e.g., abc123de)"
          value={gameId}
          onChange={(e) => setGameId(e.target.value)}
          maxLength={8}
          style={{ textTransform: 'lowercase' }}
        />
        <button type="submit" disabled={!gameId.trim()}>
          Join Game
        </button>
      </form>
    </div>
  );
}

export default App;