// Package main provides a basic example of using Heimdall
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ciaranRoche/heimdall-go"
	_ "github.com/ciaranRoche/heimdall-go/provider/kafka"
	_ "github.com/ciaranRoche/heimdall-go/provider/rabbitmq"
)

func main() {
	// Determine provider from environment or use Kafka as default
	provider := os.Getenv("PROVIDER")
	if provider == "" {
		provider = "kafka"
	}

	log.Printf("Starting Heimdall example with provider: %s", provider)

	// Create Heimdall instance based on provider
	var h *heimdall.Heimdall
	var err error

	if provider == "kafka" {
		h, err = createKafkaInstance()
	} else if provider == "rabbitmq" {
		h, err = createRabbitMQInstance()
	} else {
		log.Fatalf("Unknown provider: %s", provider)
	}

	if err != nil {
		log.Fatalf("Failed to create Heimdall instance: %v", err)
	}
	defer h.Close()

	ctx := context.Background()

	// Subscribe to messages
	topic := "example.topic"
	cancel, err := h.Subscribe(ctx, topic, func(ctx context.Context, msg *heimdall.Message) error {
		log.Printf("Received message from %s: %s", msg.Topic, string(msg.Data))
		log.Printf("  Correlation ID: %s", msg.CorrelationID)
		log.Printf("  Headers: %v", msg.Headers)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer cancel()

	log.Printf("Subscribed to topic: %s", topic)

	// Publish some test messages
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-ticker.C:
				count++
				message := []byte(`{"message": "Hello, Heimdall!", "count": ` + string(rune(count+'0')) + `}`)

				err := h.Publish(ctx, topic, message,
					heimdall.WithCorrelationID("example-"+string(rune(count+'0'))),
					heimdall.WithHeaders(map[string]interface{}{
						"content-type": "application/json",
						"example":      "basic",
					}),
				)

				if err != nil {
					log.Printf("Failed to publish message: %v", err)
				} else {
					log.Printf("Published message #%d to %s", count, topic)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

func createKafkaInstance() (*heimdall.Heimdall, error) {
	// Get Kafka configuration from environment or use defaults
	bootstrapServers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	if bootstrapServers == "" {
		bootstrapServers = "localhost:9092"
	}

	consumerGroup := os.Getenv("KAFKA_CONSUMER_GROUP")
	if consumerGroup == "" {
		consumerGroup = "heimdall-example"
	}

	return heimdall.New(
		heimdall.WithProvider("kafka"),
		heimdall.WithConfig(map[string]interface{}{
			"bootstrap_servers": []string{bootstrapServers},
			"consumer_group":    consumerGroup,
		}),
	)
}

func createRabbitMQInstance() (*heimdall.Heimdall, error) {
	// Get RabbitMQ configuration from environment or use defaults
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	exchange := os.Getenv("RABBITMQ_EXCHANGE")
	if exchange == "" {
		exchange = "heimdall.events"
	}

	return heimdall.New(
		heimdall.WithProvider("rabbitmq"),
		heimdall.WithConfig(map[string]interface{}{
			"url":           url,
			"exchange":      exchange,
			"exchange_type": "topic",
		}),
	)
}
