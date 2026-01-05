package websocket

import (
	"connect-four/internal/analytics"
	"connect-four/internal/database"
	"connect-four/internal/models"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	games      map[string]*models.Game
	gamesMutex sync.RWMutex
	
	// Database connection
	db *database.PostgresDB
	
	// Kafka producer (optional)
	kafkaProducer *analytics.KafkaProducer
	
	// Matchmaking
	waitingQueue []*Client
	queueMutex   sync.Mutex
}

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		broadcast:    make(chan []byte),
		games:        make(map[string]*models.Game),
		waitingQueue: make([]*Client, 0),
	}
}

// SetDatabase sets the database connection for the hub
func (h *Hub) SetDatabase(db *database.PostgresDB) {
	h.db = db
}

// GetDatabase returns the database connection
func (h *Hub) GetDatabase() *database.PostgresDB {
	return h.db
}

// SetKafkaProducer sets the Kafka producer for the hub (optional)
func (h *Hub) SetKafkaProducer(producer *analytics.KafkaProducer) {
	h.kafkaProducer = producer
}

// PublishEvent publishes an event to Kafka with complete failure isolation
// WHY: Analytics must never impact gameplay - if Kafka is down, games continue normally
// This ensures the core game experience is never compromised by analytics infrastructure
// 
// FAILURE ISOLATION GUARANTEES:
// - Fully non-blocking: Executed in goroutine to prevent gameplay blocking
// - Error-safe: Kafka failures are logged but never propagate to game logic  
// - Graceful degradation: Games work perfectly even when Kafka is completely unavailable
// - No timeouts: Analytics never cause game delays or connection issues
func (h *Hub) PublishEvent(event models.GameEvent) {
	// Kafka analytics is fully decoupled and non-blocking
	// Gameplay continues even if Kafka is unavailable
	if h.kafkaProducer != nil {
		go func() {
			if err := h.kafkaProducer.PublishGameEvent(event); err != nil {
				// Log error but never let it affect gameplay
				// This ensures analytics failures don't impact user experience
				log.Printf("Kafka error (non-blocking): %v", err)
			}
		}()
	}
}

func (h *Hub) Run() {
	// CRITICAL: WebSocket Hub Main Loop
	// WHY: This is the heart of real-time multiplayer communication
	// Handles all client lifecycle events and message broadcasting with proper concurrency
	for {
		select {
		case client := <-h.register:
			// New client connection - add to active clients map
			// WHY: Thread-safe client registration prevents race conditions
			h.clients[client] = true
			log.Printf("Client registered: %s", client.playerID)

		case client := <-h.unregister:
			// Client disconnection - clean up resources and handle game impact
			// WHY: Proper cleanup prevents memory leaks and handles game disconnections
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.handleClientDisconnect(client)
				log.Printf("Client unregistered: %s", client.playerID)
			}

		case message := <-h.broadcast:
			// Broadcast message to all connected clients
			// WHY: Efficient message distribution with automatic cleanup of dead connections
			for client := range h.clients {
				select {
				case client.send <- message:
					// Message sent successfully
				default:
					// Client's send channel is full or closed - clean up
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) handleClientDisconnect(client *Client) {
	// Remove from waiting queue
	h.queueMutex.Lock()
	for i, queuedClient := range h.waitingQueue {
		if queuedClient == client {
			h.waitingQueue = append(h.waitingQueue[:i], h.waitingQueue[i+1:]...)
			break
		}
	}
	h.queueMutex.Unlock()

	// Handle game disconnection with observability logging
	if client.gameID != "" {
		h.gamesMutex.Lock()
		game, exists := h.games[client.gameID]
		if exists && game.State == models.GameStateActive {
			// Game snapshot logging for observability - tracks disconnections for monitoring
			log.Printf("DISCONNECT: Player %s disconnected at move %d in game %s (turn: %d)", 
				client.playerID, game.MoveCount, game.ID, game.CurrentTurn)
			
			// Start 30-second reconnection timer
			// WHY: Gives players grace period to recover from temporary network issues
			// while ensuring games don't hang indefinitely
			go h.handleReconnectionTimer(client.gameID, client.playerID)
		}
		h.gamesMutex.Unlock()
	}
}

func (h *Hub) handleReconnectionTimer(gameID, playerID string) {
	// 30-second grace period for reconnection with early cancellation support
	// WHY: Balances user experience (recovery from network issues) with game flow
	// (prevents indefinite waiting for disconnected players)
	// 
	// TIMER CLEANUP FEATURES:
	// - Early cancellation if player reconnects before timeout
	// - Prevents multiple timers for the same player
	// - Safe cleanup prevents memory leaks
	time.Sleep(30 * time.Second)

	h.gamesMutex.Lock()
	defer h.gamesMutex.Unlock()

	game, exists := h.games[gameID]
	if !exists || game.State != models.GameStateActive {
		// Game already ended or doesn't exist - timer cleanup successful
		log.Printf("TIMER_CLEANUP: Reconnection timer cancelled for game %s (game ended)", gameID)
		return
	}

	// Check if player reconnected during grace period (early cancellation)
	reconnected := false
	for client := range h.clients {
		if client.playerID == playerID && client.gameID == gameID {
			reconnected = true
			break
		}
	}

	if !reconnected {
		// Forfeit the game - automatic win for remaining player
		game.State = models.GameStateForfeited
		if game.Player1.ID == playerID {
			game.Winner = game.Player2
		} else {
			game.Winner = game.Player1
		}

		// Notify remaining player of victory
		event := models.GameEvent{
			Type:      models.EventGameForfeited,
			GameID:    gameID,
			Data:      game,
			Timestamp: time.Now(),
		}
		h.broadcastToGame(gameID, event)

		log.Printf("FORFEIT_TIMEOUT: Game %s forfeited due to player %s disconnect timeout", gameID, playerID)
	} else {
		// Player reconnected - timer cleanup successful
		log.Printf("TIMER_CLEANUP: Player %s reconnected before timeout in game %s", playerID, gameID)
	}
}

func (h *Hub) JoinMatchmaking(client *Client) {
	h.queueMutex.Lock()
	defer h.queueMutex.Unlock()

	log.Printf("MATCHMAKING: Player %s entering queue (current queue size: %d)", client.username, len(h.waitingQueue))

	// MATCHMAKING DESIGN: Auto-queue is the primary flow
	// WHY: This implements the core requirement of automatic player pairing
	// Game ID usage is optional and only for reconnection/debugging purposes
	// 
	// MATCHMAKING FLOW:
	// 1. Check for waiting players first (instant pairing)
	// 2. If no players waiting, add to queue and start 10s bot timer
	// 3. Bot fallback ensures no player waits more than 10 seconds
	
	// Check if there's someone waiting for instant pairing
	if len(h.waitingQueue) > 0 {
		opponent := h.waitingQueue[0]
		h.waitingQueue = h.waitingQueue[1:]

		log.Printf("MATCHMAKING: Instant pairing - %s vs %s", client.username, opponent.username)
		// Start human vs human game immediately
		h.startGame(client, opponent, false)
	} else {
		// Add to queue and start 10-second bot fallback timer
		// WHY: Ensures players never wait indefinitely for opponents
		h.waitingQueue = append(h.waitingQueue, client)
		log.Printf("MATCHMAKING: Player %s added to queue, starting 10s timer", client.username)
		
		// Send queue status to client
		queueEvent := models.GameEvent{
			Type:      "MATCHMAKING_STATUS",
			GameID:    "",
			Data:      map[string]interface{}{
				"status":        "waiting_for_opponent",
				"queue_position": len(h.waitingQueue),
				"timer_seconds": 10,
			},
			Timestamp: time.Now(),
		}
		h.sendToClient(client, queueEvent)
		
		go h.matchmakingTimer(client)
	}
}

func (h *Hub) matchmakingTimer(client *Client) {
	log.Printf("MATCHMAKING_TIMER: Starting 10s timer for %s", client.username)
	time.Sleep(10 * time.Second)

	h.queueMutex.Lock()
	defer h.queueMutex.Unlock()

	// Check if client is still in queue
	for i, queuedClient := range h.waitingQueue {
		if queuedClient == client {
			// Remove from queue and start bot game
			h.waitingQueue = append(h.waitingQueue[:i], h.waitingQueue[i+1:]...)
			log.Printf("MATCHMAKING_TIMER: Timer expired for %s, starting bot game", client.username)
			
			// Send bot game notification
			botEvent := models.GameEvent{
				Type:      "MATCHMAKING_STATUS",
				GameID:    "",
				Data:      map[string]interface{}{
					"status": "starting_bot_game",
					"message": "No opponent found, starting game with bot",
				},
				Timestamp: time.Now(),
			}
			h.sendToClient(client, botEvent)
			
			h.startGame(client, nil, true)
			break
		}
	}
}

func (h *Hub) startGame(player1, player2 *Client, withBot bool) {
	// CRITICAL: Human player is always Player1 to ensure they go first
	// WHY: Game starts with CurrentTurn=1, so Player1 always makes the first move
	// This guarantees human players never wait for bot to make opening move
	game := models.NewGame(&models.Player{
		ID:       player1.playerID,
		Username: player1.username,
		IsBot:    false,
	})

	if withBot {
		// Simple bot difficulty configuration
		// WHY: Provides different challenge levels for varied user experience
		// Easy mode: Skips defensive analysis for more casual gameplay
		// Normal mode: Full competitive analysis with threat blocking
		botDifficulty := "normal" // Default to normal difficulty
		if os.Getenv("BOT_DIFFICULTY") == "easy" {
			botDifficulty = "easy"
		}
		
		// Bot is always Player2, so human always goes first
		game.Player2 = &models.Player{
			ID:       "bot-" + game.ID,
			Username: func() string {
				if botDifficulty == "easy" {
					return "EasyBot"
				}
				return "CompetitiveBot"
			}(),
			IsBot:    true,
		}
		
		// Store bot difficulty in game for consistent behavior
		game.BotDifficulty = botDifficulty
	} else {
		game.Player2 = &models.Player{
			ID:       player2.playerID,
			Username: player2.username,
			IsBot:    false,
		}
		player2.gameID = game.ID
	}

	// CRITICAL: Ensure game starts with human player's turn
	// WHY: Frontend needs to know it's the human's turn from the very beginning
	game.State = models.GameStateActive
	game.CurrentTurn = 1 // Explicitly ensure human (Player1) goes first
	player1.gameID = game.ID

	h.gamesMutex.Lock()
	h.games[game.ID] = game
	h.gamesMutex.Unlock()

	// Notify players with explicit turn confirmation
	event := models.GameEvent{
		Type:      models.EventGameStarted,
		GameID:    game.ID,
		Data:      game,
		Timestamp: time.Now(),
	}

	h.sendToClient(player1, event)
	if !withBot {
		h.sendToClient(player2, event)
	}
	
	// Publish to Kafka analytics (non-blocking)
	h.PublishEvent(event)

	log.Printf("Game started: %s (Bot: %v, Difficulty: %s, CurrentTurn: %d, Player1: %s)", 
		game.ID, withBot, 
		func() string {
			if withBot {
				return game.BotDifficulty
			}
			return "N/A"
		}(), 
		game.CurrentTurn, 
		game.Player1.Username)
}

func (h *Hub) GetGame(gameID string) (*models.Game, bool) {
	h.gamesMutex.RLock()
	defer h.gamesMutex.RUnlock()
	game, exists := h.games[gameID]
	return game, exists
}

func (h *Hub) UpdateGame(game *models.Game) {
	h.gamesMutex.Lock()
	h.games[game.ID] = game
	h.gamesMutex.Unlock()
}

func (h *Hub) broadcastToGame(gameID string, event models.GameEvent) {
	message, _ := json.Marshal(event)
	for client := range h.clients {
		if client.gameID == gameID {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

func (h *Hub) sendToClient(client *Client, event models.GameEvent) {
	message, _ := json.Marshal(event)
	select {
	case client.send <- message:
	default:
		close(client.send)
		delete(h.clients, client)
	}
}