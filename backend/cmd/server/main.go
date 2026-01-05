package main

import (
	"connect-four/internal/analytics"
	"connect-four/internal/database"
	"connect-four/internal/websocket"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Get configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081" // Default port
	}
	
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:Vairag@310@localhost/connect_four?sslmode=disable"
	}
	
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3001" // Default for development
	}

	// Initialize database
	db, err := database.NewPostgresDB(dbURL)
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		db = nil
	}
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	// Initialize Kafka producer (optional)
	var kafkaProducer *analytics.KafkaProducer
	if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); kafkaBrokers != "" {
		kafkaProducer = analytics.NewKafkaProducer([]string{kafkaBrokers}, "game-events")
		defer kafkaProducer.Close()
	}

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	hub.SetDatabase(db) // Pass database to hub
	
	// Set Kafka producer if available (optional)
	if kafkaProducer != nil {
		hub.SetKafkaProducer(kafkaProducer)
	}
	
	go hub.Run()

	// Setup HTTP routes with better error handling
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("WebSocket connection request from %s, Origin: %s", r.RemoteAddr, r.Header.Get("Origin"))
		websocket.ServeWS(hub, w, r)
	})

	http.HandleFunc("/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		handleLeaderboard(w, r, db)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", corsOrigins)
		
		status := map[string]interface{}{
			"status": "healthy",
			"timestamp": time.Now().Unix(),
			"database": db != nil,
			"kafka": kafkaProducer != nil,
		}
		
		json.NewEncoder(w).Encode(status)
	})

	// Enable CORS for preflight requests
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", corsOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// Only return 404 for unknown paths
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		
		// Root path - return basic info
		w.Header().Set("Content-Type", "application/json")
		info := map[string]string{
			"service": "4-in-a-row-backend",
			"status": "running",
		}
		json.NewEncoder(w).Encode(info)
	})

	// Start server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Start Kafka consumer (optional)
	if kafkaBrokers := os.Getenv("KAFKA_BROKERS"); kafkaBrokers != "" {
		consumer := analytics.NewKafkaConsumer([]string{kafkaBrokers}, "game-events", "analytics-group")
		ctx, cancel := context.WithCancel(context.Background())
		go consumer.StartConsuming(ctx)
		defer func() {
			cancel()
			consumer.Close()
		}()
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func handleLeaderboard(w http.ResponseWriter, r *http.Request, db *database.PostgresDB) {
	w.Header().Set("Content-Type", "application/json")
	
	// Get CORS origins from environment
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3001" // Default for development
	}
	w.Header().Set("Access-Control-Allow-Origin", corsOrigins)

	if db == nil {
		// Return mock data if database is not available
		mockLeaderboard := []database.LeaderboardEntry{
			{Username: "Player1", Wins: 10, TotalGames: 15},
			{Username: "Player2", Wins: 8, TotalGames: 12},
			{Username: "Player3", Wins: 5, TotalGames: 8},
		}
		json.NewEncoder(w).Encode(mockLeaderboard)
		return
	}

	leaderboard, err := db.GetLeaderboard(10)
	if err != nil {
		http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(leaderboard)
}