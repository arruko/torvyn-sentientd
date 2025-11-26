package queue

import "context"

// HandlerFunc is a function that processes a message from the queue.
// It receives the message payload as raw bytes and returns an error if processing fails.
// Errors trigger retry logic according to the queue implementation's configuration.
type HandlerFunc func(ctx context.Context, msg []byte) error

// Queue provides an abstraction over message queue implementations (NATS, Kafka, etc).
// It supports publish/subscribe patterns with at-least-once delivery semantics.
type Queue interface {
	// Publish sends a message to the specified subject/topic.
	// The message is persisted according to the queue's durability settings.
	//
	// Returns an error if the publish fails (network error, queue full, etc).
	Publish(ctx context.Context, subject string, data []byte) error

	// Subscribe creates a durable subscription to the specified subject/topic.
	// The handler is invoked for each message received.
	// Messages are auto-acknowledged on successful handler return, or retried on error.
	//
	// This is a blocking call that runs until the context is cancelled or an
	// unrecoverable error occurs.
	//
	// Returns an error if subscription setup fails.
	Subscribe(subject string, handler HandlerFunc) error

	// Close releases resources held by the Queue implementation.
	// Should be called when the queue is no longer needed.
	Close() error
}
