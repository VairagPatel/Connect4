package main

import (
	"connect-four/internal/analytics"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	// Initialize Kafka consumer
	consumer := analytics.NewKafkaConsumer([]string{kafkaBrokers}, "game-events", "analytics-service")
	defer consumer.Close()

	// Start consuming
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go consumer.StartConsuming(ctx)

	log.Println("Analytics service started, consuming from Kafka...")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Analytics service shutting down...")
}