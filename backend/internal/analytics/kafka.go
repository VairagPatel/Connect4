package analytics

import (
	"connect-four/internal/models"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

type KafkaConsumer struct {
	reader *kafka.Reader
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        true,
	}

	return &KafkaProducer{writer: writer}
}

func NewKafkaConsumer(brokers []string, topic, groupID string) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	return &KafkaConsumer{reader: reader}
}

func (p *KafkaProducer) PublishGameEvent(event models.GameEvent) error {
	message, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte(event.GameID),
			Value: message,
			Time:  event.Timestamp,
		},
	)
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

func (c *KafkaConsumer) StartConsuming(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			message, err := c.reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading Kafka message: %v", err)
				continue
			}

			var event models.GameEvent
			if err := json.Unmarshal(message.Value, &event); err != nil {
				log.Printf("Error unmarshaling event: %v", err)
				continue
			}

			c.processEvent(event)
		}
	}
}

func (c *KafkaConsumer) processEvent(event models.GameEvent) {
	switch event.Type {
	case models.EventGameStarted:
		log.Printf("Analytics: Game started - %s", event.GameID)
		// Track game start metrics
		
	case models.EventMovePlayed:
		log.Printf("Analytics: Move played in game %s", event.GameID)
		// Track move metrics
		
	case models.EventGameWon:
		log.Printf("Analytics: Game won - %s", event.GameID)
		// Track win metrics
		
	case models.EventGameDraw:
		log.Printf("Analytics: Game draw - %s", event.GameID)
		// Track draw metrics
		
	case models.EventGameForfeited:
		log.Printf("Analytics: Game forfeited - %s", event.GameID)
		// Track forfeit metrics
	}
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}

// Analytics metrics that could be tracked
type GameMetrics struct {
	TotalGames       int64         `json:"total_games"`
	AverageDuration  time.Duration `json:"average_duration"`
	GamesPerDay      int64         `json:"games_per_day"`
	MostActivePlayer string        `json:"most_active_player"`
	BotWinRate       float64       `json:"bot_win_rate"`
}