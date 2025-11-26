package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/nats-io/nats.go"
)

// NATSQueue implements the Queue interface using NATS JetStream.
type NATSQueue struct {
	nc *nats.Conn
	js nats.JetStreamContext
	url string
}

// NewNATSQueue creates a new NATS JetStream queue client.
// It connects to the NATS server and initializes JetStream context.
//
// Parameters:
//   - url: NATS server URL (e.g., "nats://nats.nats.svc.cluster.local:4222")
//
// Returns error if connection fails or JetStream is not enabled.
func NewNATSQueue(url string) (*NATSQueue, error) {
	if url == "" {
		return nil, fmt.Errorf("NATS URL cannot be empty")
	}

	// Connect to NATS with reconnect logic
	nc, err := nats.Connect(
		url,
		nats.Name("sentientd"),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logging.Error.Printf("NATS disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logging.Info.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logging.Info.Printf("NATS connection closed")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", url, err)
	}

	logging.Info.Printf("Connected to NATS server at %s", url)

	// Initialize JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to initialize JetStream: %w", err)
	}

	// Ensure the alerts stream exists
	if err := ensureStream(js); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to ensure stream exists: %w", err)
	}

	logging.Info.Printf("NATS JetStream initialized")

	return &NATSQueue{
		nc:  nc,
		js:  js,
		url: url,
	}, nil
}

// ensureStream creates the alerts stream if it doesn't exist.
func ensureStream(js nats.JetStreamContext) error {
	streamName := "alerts"

	// Check if stream exists
	_, err := js.StreamInfo(streamName)
	if err == nil {
		// Stream already exists
		logging.Info.Printf("JetStream stream '%s' already exists", streamName)
		return nil
	}

	// Create stream
	streamConfig := &nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{"alerts.>"},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour, // 7 days retention
		MaxBytes:  5 * 1024 * 1024 * 1024, // 5 GB
		Replicas:  1, // Single replica for dev
	}

	_, err = js.AddStream(streamConfig)
	if err != nil {
		return fmt.Errorf("failed to create stream '%s': %w", streamName, err)
	}

	logging.Info.Printf("Created JetStream stream '%s' with subjects: %v", streamName, streamConfig.Subjects)
	return nil
}

// Publish sends a message to the specified subject.
func (q *NATSQueue) Publish(ctx context.Context, subject string, data []byte) error {
	if q.js == nil {
		return fmt.Errorf("JetStream context not initialized")
	}

	// Publish with timeout from context
	pubAck, err := q.js.Publish(subject, data, nats.Context(ctx))
	if err != nil {
		logging.Error.Printf("Failed to publish message to subject '%s': %v", subject, err)
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	logging.Info.Printf("Published message to subject '%s' (stream: %s, seq: %d)",
		subject, pubAck.Stream, pubAck.Sequence)
	return nil
}

// Subscribe creates a durable pull-based subscription to the specified subject.
// Messages are processed by the handler and auto-acknowledged on success.
func (q *NATSQueue) Subscribe(subject string, handler HandlerFunc) error {
	if q.js == nil {
		return fmt.Errorf("JetStream context not initialized")
	}

	// Durable consumer name based on subject
	consumerName := "sentientd-ingest"
	streamName := "alerts"

	// Create or get durable consumer
	consumerConfig := &nats.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subject,
		AckPolicy:     nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverAllPolicy,
		MaxDeliver:    10, // Retry up to 10 times
		AckWait:       30 * time.Second,
	}

	// Subscribe with durable consumer
	sub, err := q.js.PullSubscribe(
		subject,
		consumerName,
		nats.BindStream(streamName),
		nats.ManualAck(),
	)
	if err != nil {
		// Try to create the consumer explicitly
		_, err = q.js.AddConsumer(streamName, consumerConfig)
		if err != nil {
			return fmt.Errorf("failed to create consumer '%s': %w", consumerName, err)
		}

		// Retry subscription
		sub, err = q.js.PullSubscribe(
			subject,
			consumerName,
			nats.BindStream(streamName),
			nats.ManualAck(),
		)
		if err != nil {
			return fmt.Errorf("failed to subscribe to subject '%s': %w", subject, err)
		}
	}

	logging.Info.Printf("Subscribed to subject '%s' with consumer '%s'", subject, consumerName)

	// Start message processing loop
	go q.processMessages(sub, handler)

	return nil
}

// processMessages is the main message processing loop for a subscription.
func (q *NATSQueue) processMessages(sub *nats.Subscription, handler HandlerFunc) {
	ctx := context.Background()
	batchSize := 10

	logging.Info.Printf("Starting message processing loop")

	for {
		// Check if subscription is still valid
		if !sub.IsValid() {
			logging.Error.Printf("Subscription is no longer valid, exiting processing loop")
			return
		}

		// Fetch messages in batches
		msgs, err := sub.Fetch(batchSize, nats.MaxWait(5*time.Second))
		if err != nil {
			// Timeout is expected when no messages available
			if err == nats.ErrTimeout {
				continue
			}
			logging.Error.Printf("Error fetching messages: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Process each message
		for _, msg := range msgs {
			// Create context with timeout for handler
			handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			// Invoke handler
			if err := handler(handlerCtx, msg.Data); err != nil {
				logging.Error.Printf("Handler error for message (subject: %s): %v",
					msg.Subject, err)

				// Negative acknowledge (will be redelivered)
				if nakErr := msg.Nak(); nakErr != nil {
					logging.Error.Printf("Failed to NAK message: %v", nakErr)
				}
			} else {
				// Acknowledge successful processing
				if ackErr := msg.Ack(); ackErr != nil {
					logging.Error.Printf("Failed to ACK message: %v", ackErr)
				}
			}

			cancel()
		}
	}
}

// Close closes the NATS connection and releases resources.
func (q *NATSQueue) Close() error {
	if q.nc != nil {
		q.nc.Close()
		logging.Info.Printf("NATS connection closed")
	}
	return nil
}
