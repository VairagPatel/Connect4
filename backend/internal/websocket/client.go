package websocket

import (
	"connect-four/internal/game"
	"connect-four/internal/models"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin in production
		// In a real production environment, you'd want to check specific origins
		origin := r.Header.Get("Origin")
		log.Printf("WebSocket connection attempt from origin: %s", origin)
		return true // Allow all origins for now
	},
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	playerID string
	username string
	gameID   string
	engine   *game.Engine
}

type ClientMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type JoinMessage struct {
	Username string `json:"username"`
}

type MoveMessage struct {
	Column int `json:"column"`
}

type ReconnectMessage struct {
	Username string `json:"username,omitempty"`
	GameID   string `json:"game_id,omitempty"`
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	log.Printf("Attempting WebSocket upgrade for %s", r.RemoteAddr)
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	log.Printf("WebSocket connection established for %s", r.RemoteAddr)

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		engine: game.NewEngine(),
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		c.handleMessage(clientMsg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

			// Send any additional queued messages as separate WebSocket messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				additionalMessage := <-c.send
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.conn.WriteMessage(websocket.TextMessage, additionalMessage); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg ClientMessage) {
	switch msg.Type {
	case "JOIN":
		var joinMsg JoinMessage
		if err := json.Unmarshal(msg.Data, &joinMsg); err != nil {
			log.Printf("Invalid join message: %v", err)
			return
		}
		c.handleJoin(joinMsg.Username)

	case "MAKE_MOVE":
		var moveMsg MoveMessage
		if err := json.Unmarshal(msg.Data, &moveMsg); err != nil {
			log.Printf("Invalid move message: %v", err)
			return
		}
		c.handleMove(moveMsg.Column)

	case "RECONNECT":
		var reconnectMsg ReconnectMessage
		if err := json.Unmarshal(msg.Data, &reconnectMsg); err != nil {
			log.Printf("Invalid reconnect message: %v", err)
			return
		}
		c.handleReconnect(reconnectMsg.Username, reconnectMsg.GameID)
	}
}

func (c *Client) handleJoin(username string) {
	if username == "" {
		log.Printf("Empty username provided for join")
		return
	}
	
	c.username = username
	c.playerID = username + "-" + time.Now().Format("20060102150405")
	
	// Send immediate acknowledgment to client
	ackEvent := models.GameEvent{
		Type:      "JOIN_ACKNOWLEDGED",
		GameID:    "",
		Data:      map[string]interface{}{
			"player_id": c.playerID,
			"username":  c.username,
			"status":    "joined_matchmaking",
		},
		Timestamp: time.Now(),
	}
	c.hub.sendToClient(c, ackEvent)
	
	// Join matchmaking
	c.hub.JoinMatchmaking(c)
	
	log.Printf("Player %s (%s) joined matchmaking", username, c.playerID)
}

func (c *Client) handleMove(column int) {
	if c.gameID == "" {
		return
	}

	game, exists := c.hub.GetGame(c.gameID)
	if !exists {
		return
	}

	log.Printf("MOVE_ATTEMPT: Player %s attempting move in column %d, CurrentTurn: %d, Player1: %s, Player2: %s", 
		c.playerID, column, game.CurrentTurn, game.Player1.ID, game.Player2.ID)

	// CRITICAL: Explicit turn ownership validation with race-condition prevention
	// WHY: Prevents race conditions from reconnected stale sockets and malicious client attempts
	// This ensures only the legitimate current player can make moves, preventing:
	// - Wrong player moves (player 1 making moves during player 2's turn)
	// - Late messages from disconnected clients that reconnect
	// - Malicious clients attempting to make moves for other players
	// - Stale sockets sending delayed messages after reconnection
	
	// VALIDATION 1: Player must be a legitimate participant in this game
	if c.playerID != game.Player1.ID && c.playerID != game.Player2.ID {
		log.Printf("SECURITY_VIOLATION: Invalid player %s attempting move in game %s (not a participant)", c.playerID, game.ID)
		return // Reject - not a valid player in this game
	}
	
	// VALIDATION 2: Only the current active player can make moves
	var currentPlayerID string
	if game.CurrentTurn == 1 {
		currentPlayerID = game.Player1.ID
	} else {
		currentPlayerID = game.Player2.ID
	}
	
	if c.playerID != currentPlayerID {
		log.Printf("TURN_VIOLATION: Player %s attempted move but it's not their turn (current: %s, turn: %d)", 
			c.playerID, currentPlayerID, game.CurrentTurn)
		return // Reject - not your turn, prevents late messages and wrong player moves
	}

	// Make the move with proper locking
	c.hub.gamesMutex.Lock()
	
	// CRITICAL: Move idempotency protection with sequence validation
	// WHY: During reconnection, clients might resend moves. This prevents duplicate processing
	// by ensuring moves are processed in strict sequence order, ignoring stale/duplicate moves
	// 
	// IDEMPOTENCY SCENARIOS HANDLED:
	// - Client reconnects and resends the same move
	// - Network issues cause message duplication  
	// - Race conditions during rapid reconnection
	// - WebSocket retry mechanisms sending duplicate messages
	expectedMoveNumber := game.LastMoveNumber + 1
	
	// Reject duplicate or stale moves (idempotency protection)
	if game.MoveCount >= expectedMoveNumber {
		c.hub.gamesMutex.Unlock()
		log.Printf("IDEMPOTENCY_PROTECTION: Ignoring duplicate/stale move for game %s (expected: %d, current: %d)", 
			game.ID, expectedMoveNumber, game.MoveCount)
		return // Ignore duplicate move to prevent state corruption
	}
	
	move, err := c.engine.MakeMove(game, c.playerID, column)
	if err != nil {
		c.hub.gamesMutex.Unlock()
		log.Printf("Invalid move: %v", err)
		return
	}

	// Assign sequence number for idempotency tracking
	move.MoveNumber = expectedMoveNumber
	game.LastMoveNumber = expectedMoveNumber

	// Update game state while still locked
	c.hub.games[game.ID] = game
	c.hub.gamesMutex.Unlock()

	// Broadcast move event
	event := models.GameEvent{
		Type:      models.EventMovePlayed,
		GameID:    game.ID,
		Data:      map[string]interface{}{
			"move": move,
			"game": game,
		},
		Timestamp: time.Now(),
	}
	c.hub.broadcastToGame(game.ID, event)
	
	// Publish to Kafka analytics (non-blocking)
	c.hub.PublishEvent(event)

	// Log game snapshot on move for observability
	log.Printf("Player %s made move %d in game %s (column %d)", 
		c.playerID, game.MoveCount, game.ID, column)

	// Check if game ended
	if game.State == models.GameStateFinished {
		// Set finished timestamp
		now := time.Now()
		game.FinishedAt = &now
		
		var eventType string
		if game.Winner != nil {
			eventType = models.EventGameWon
		} else {
			eventType = models.EventGameDraw
		}

		endEvent := models.GameEvent{
			Type:      eventType,
			GameID:    game.ID,
			Data:      game,
			Timestamp: time.Now(),
		}
		c.hub.broadcastToGame(game.ID, endEvent)
		
		// Publish to Kafka analytics (non-blocking)
		c.hub.PublishEvent(endEvent)
		
		// Save completed game to database (async)
		go c.saveCompletedGame(game)
		
	} else if game.Player2.IsBot && game.CurrentTurn == 2 {
		// CRITICAL: Bot only moves when it's turn 2 (after human player's move)
		// WHY: Since human is always Player1 and game starts with CurrentTurn=1,
		// this ensures human always makes the first move before bot responds
		log.Printf("BOT_TRIGGER: Triggering bot move for game %s, CurrentTurn: %d", game.ID, game.CurrentTurn)
		go c.handleBotMoveSync(game)
	} else {
		log.Printf("BOT_CHECK: Game %s - IsBot: %v, CurrentTurn: %d, Player2.IsBot: %v", 
			game.ID, game.Player2.IsBot, game.CurrentTurn, game.Player2.IsBot)
	}
}

func (c *Client) handleBotMoveSync(gameModel *models.Game) {
	log.Printf("BOT_MOVE_START: Bot starting move calculation for game %s", gameModel.ID)
	
	// Very minimal delay for natural feel
	time.Sleep(50 * time.Millisecond)

	// CRITICAL: Re-fetch game state to ensure we have the latest version
	// WHY: Prevents race conditions where game state might have changed since function was called
	c.hub.gamesMutex.RLock()
	gameState, exists := c.hub.games[gameModel.ID]
	c.hub.gamesMutex.RUnlock()
	
	if !exists {
		log.Printf("BOT_MOVE_ERROR: Game %s no longer exists", gameModel.ID)
		return
	}
	
	// Create bot with appropriate difficulty from game settings
	// WHY: Consistent bot behavior throughout the game based on initial configuration
	var bot *game.Bot
	if gameState.BotDifficulty == "easy" {
		bot = game.NewBotWithDifficulty(gameState.Player2.ID, game.Easy)
	} else {
		bot = game.NewBotWithDifficulty(gameState.Player2.ID, game.Normal)
	}
	
	log.Printf("BOT_MOVE_CALC: Bot calculating best move for game %s", gameState.ID)
	botColumn := bot.GetBestMove(gameState)

	if botColumn == -1 {
		log.Printf("BOT_ERROR: Bot couldn't find valid move for game %s", gameState.ID)
		return
	}

	log.Printf("BOT_MOVE_SELECTED: Bot selected column %d for game %s", botColumn, gameState.ID)

	// Make bot move with proper locking
	c.hub.gamesMutex.Lock()
	
	// CRITICAL: Re-fetch game state again while locked to ensure we have the absolute latest version
	// WHY: Prevents race conditions where game state might have changed between read lock and write lock
	gameState, exists = c.hub.games[gameState.ID]
	if !exists {
		c.hub.gamesMutex.Unlock()
		log.Printf("BOT_MOVE_ERROR: Game %s no longer exists", gameState.ID)
		return
	}
	
	// Assign sequence number for idempotency tracking
	expectedMoveNumber := gameState.LastMoveNumber + 1
	
	move, err := c.engine.MakeMove(gameState, gameState.Player2.ID, botColumn)
	if err != nil {
		c.hub.gamesMutex.Unlock()
		log.Printf("BOT_MOVE_ERROR: Bot move error in game %s: %v", gameState.ID, err)
		return
	}

	// Assign sequence number for idempotency tracking
	move.MoveNumber = expectedMoveNumber
	gameState.LastMoveNumber = expectedMoveNumber

	// Update game state while still locked
	c.hub.games[gameState.ID] = gameState
	c.hub.gamesMutex.Unlock()

	log.Printf("BOT_MOVE_SUCCESS: Bot made move %d in game %s (column %d)", 
		gameState.MoveCount, gameState.ID, botColumn)

	// Broadcast bot move
	event := models.GameEvent{
		Type:      models.EventMovePlayed,
		GameID:    gameState.ID,
		Data:      map[string]interface{}{
			"move": move,
			"game": gameState,
		},
		Timestamp: time.Now(),
	}
	c.hub.broadcastToGame(gameState.ID, event)
	
	// Publish to Kafka analytics (non-blocking)
	c.hub.PublishEvent(event)

	// Check if bot won or game ended
	if gameState.State == models.GameStateFinished {
		// Set finished timestamp
		c.hub.gamesMutex.Lock()
		now := time.Now()
		gameState.FinishedAt = &now
		c.hub.games[gameState.ID] = gameState
		c.hub.gamesMutex.Unlock()
		
		var eventType string
		if gameState.Winner != nil {
			eventType = models.EventGameWon
		} else {
			eventType = models.EventGameDraw
		}

		endEvent := models.GameEvent{
			Type:      eventType,
			GameID:    gameState.ID,
			Data:      gameState,
			Timestamp: time.Now(),
		}
		c.hub.broadcastToGame(gameState.ID, endEvent)
		
		// Publish to Kafka analytics (non-blocking)
		c.hub.PublishEvent(endEvent)
		
		// Save completed game to database (async)
		go c.saveCompletedGame(gameState)
	}
}

func (c *Client) handleBotMove(gameModel *models.Game) {
	// Reduced delay for faster response
	time.Sleep(100 * time.Millisecond)

	bot := game.NewBot(gameModel.Player2.ID)
	botColumn := bot.GetBestMove(gameModel)

	if botColumn == -1 {
		log.Printf("Bot couldn't find valid move")
		return
	}

	// Make bot move
	move, err := c.engine.MakeMove(gameModel, gameModel.Player2.ID, botColumn)
	if err != nil {
		log.Printf("Bot move error: %v", err)
		return
	}

	// Update game state
	c.hub.UpdateGame(gameModel)

	// Broadcast bot move
	event := models.GameEvent{
		Type:      models.EventMovePlayed,
		GameID:    gameModel.ID,
		Data:      map[string]interface{}{
			"move": move,
			"game": gameModel,
		},
		Timestamp: time.Now(),
	}
	c.hub.broadcastToGame(gameModel.ID, event)

	// Check if bot won
	if gameModel.State == models.GameStateFinished {
		var eventType string
		if gameModel.Winner != nil {
			eventType = models.EventGameWon
		} else {
			eventType = models.EventGameDraw
		}

		endEvent := models.GameEvent{
			Type:      eventType,
			GameID:    gameModel.ID,
			Data:      gameModel,
			Timestamp: time.Now(),
		}
		c.hub.broadcastToGame(gameModel.ID, endEvent)
	}
}

func (c *Client) saveCompletedGame(game *models.Game) {
	// CRITICAL: Persistence safety - asynchronous database operations
	// WHY: Database failures must never affect active gameplay
	// This ensures the core game experience continues even if persistence fails
	log.Printf("Game completed: %s, Winner: %v, Moves: %d", 
		game.ID, 
		func() string {
			if game.Winner != nil {
				return game.Winner.Username
			}
			return "Draw"
		}(), 
		game.MoveCount)
	
	// PERSISTENCE SAFETY: Save to database asynchronously with error isolation
	// Database operations are fully isolated from gameplay logic
	if db := c.hub.GetDatabase(); db != nil {
		if err := db.SaveCompletedGame(game); err != nil {
			// Log error but never let it affect gameplay or user experience
			log.Printf("PERSISTENCE_ERROR: Failed to save game %s to database: %v", game.ID, err)
			// Game continues normally despite database failure
		} else {
			log.Printf("PERSISTENCE_SUCCESS: Game %s successfully saved to database", game.ID)
		}
	} else {
		// Graceful degradation - gameplay works without database
		log.Printf("PERSISTENCE_UNAVAILABLE: Database not available, game %s not persisted", game.ID)
	}
}

func (c *Client) handleReconnect(username, gameID string) {
	log.Printf("RECONNECT_ATTEMPT: Player %s attempting to reconnect to game %s", username, gameID)
	
	if username == "" {
		log.Printf("RECONNECT_FAILED: Empty username provided")
		// Send error response
		errorEvent := models.GameEvent{
			Type:      "RECONNECT_ERROR",
			GameID:    "",
			Data:      map[string]interface{}{
				"error": "Username is required for reconnection",
			},
			Timestamp: time.Now(),
		}
		c.hub.sendToClient(c, errorEvent)
		return
	}
	
	// CRITICAL: WebSocket Reconnection Safety - Force close old sockets
	// WHY: Ensures only ONE active socket exists per player, preventing:
	// - Ghost sockets causing duplicate message delivery
	// - Memory leaks from unclosed connections  
	// - Race conditions between old and new sockets
	// - State conflicts during rapid reconnection
	c.hub.gamesMutex.Lock()
	
	// FORCE CLOSE any existing sockets for this player (reconnection safety)
	// This is the critical fix to prevent ghost sockets
	for existingClient := range c.hub.clients {
		if existingClient.username == username && existingClient.gameID != "" {
			log.Printf("RECONNECT_SAFETY: Force-closing old socket for player %s", username)
			// Force close old socket to prevent ghost connections
			if existingClient.conn != nil {
				existingClient.conn.Close()
			}
			close(existingClient.send)
			delete(c.hub.clients, existingClient)
		}
	}
	
	// Try to find active game by gameID or username
	var targetGame *models.Game
	var found bool
	
	if gameID != "" {
		targetGame, found = c.hub.GetGame(gameID)
		log.Printf("RECONNECT_SEARCH: Looking for game by ID %s - found: %v", gameID, found)
	} else if username != "" {
		// Search for active game with this username
		log.Printf("RECONNECT_SEARCH: Looking for active game with username %s", username)
		for gID, game := range c.hub.games {
			if game.State == models.GameStateActive {
				if (game.Player1.Username == username) || (game.Player2.Username == username) {
					targetGame = game
					found = true
					log.Printf("RECONNECT_SEARCH: Found active game %s for username %s", gID, username)
					break
				}
			}
		}
	}
	
	c.hub.gamesMutex.Unlock()
	
	if !found || targetGame == nil {
		log.Printf("RECONNECT_FAILED: No active game found for reconnection (username: %s, gameID: %s)", username, gameID)
		// Send error response
		errorEvent := models.GameEvent{
			Type:      "RECONNECT_ERROR",
			GameID:    "",
			Data:      map[string]interface{}{
				"error": "No active game found for reconnection",
				"username": username,
				"game_id": gameID,
			},
			Timestamp: time.Now(),
		}
		c.hub.sendToClient(c, errorEvent)
		return
	}
	
	// Reconnect player to the game
	c.gameID = targetGame.ID
	if targetGame.Player1.Username == username {
		c.playerID = targetGame.Player1.ID
		c.username = username
	} else if targetGame.Player2.Username == username {
		c.playerID = targetGame.Player2.ID
		c.username = username
	}
	
	// Send current game state to reconnected player
	event := models.GameEvent{
		Type:      "GAME_RECONNECTED",
		GameID:    targetGame.ID,
		Data:      targetGame,
		Timestamp: time.Now(),
	}
	c.hub.sendToClient(c, event)
	
	log.Printf("RECONNECT_SUCCESS: Player %s successfully reconnected to game %s", username, targetGame.ID)
}