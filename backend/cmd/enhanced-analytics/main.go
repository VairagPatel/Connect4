package main

import (
	"connect-four/internal/analytics"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("🚀 Starting Enhanced Analytics Service for 4 in a Row")
	
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}

	// Initialize enhanced analytics service
	analyticsService := analytics.NewAnalyticsService(
		[]string{kafkaBrokers}, 
		"game-events", 
		"enhanced-analytics-service",
	)
	defer analyticsService.Close()

	// Start analytics processing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go analyticsService.StartAnalytics(ctx)

	// Start periodic metrics reporting
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Report every 30 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				analyticsService.PrintMetrics()
			}
		}
	}()

	log.Println("📊 Enhanced Analytics Service Started")
	log.Println("📊 Consuming from Kafka topic: game-events")
	log.Println("📊 Tracking:")
	log.Println("📊   - Average game duration")
	log.Println("📊   - Most frequent winners")
	log.Println("📊   - Games per day/hour")
	log.Println("📊   - User-specific metrics")
	log.Println("📊   - Bot performance analytics")
	log.Println("📊 Metrics reported every 30 seconds")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("📊 Enhanced Analytics Service shutting down...")
	
	// Print final metrics report
	log.Println("📊 === FINAL ANALYTICS REPORT ===")
	analyticsService.PrintMetrics()
}