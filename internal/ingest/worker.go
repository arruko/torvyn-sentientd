package ingest

import (
	"context"
	"encoding/json"

	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/arruko/torvyn-sentientd/internal/queue"
)

// Worker processes alert messages from the queue.
// It subscribes to the alerts.ingest subject and handles incoming alert batches.
type Worker struct {
	queue queue.Queue
}

// NewWorker creates a new ingest worker.
func NewWorker(q queue.Queue) *Worker {
	return &Worker{
		queue: q,
	}
}

// Start begins processing messages from the queue.
// This is a blocking call that runs until the context is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	logging.Info.Printf("Starting ingest worker")

	// Subscribe to alerts.ingest subject
	err := w.queue.Subscribe("alerts.ingest", w.handleMessage)
	if err != nil {
		return err
	}

	// Block until context is cancelled
	<-ctx.Done()
	logging.Info.Printf("Ingest worker shutting down")
	return nil
}

// handleMessage processes a single message from the queue.
// For now, this is a skeleton that logs the message.
// Future: invoke correlator, store incidents, etc.
func (w *Worker) handleMessage(ctx context.Context, msg []byte) error {
	logging.Info.Printf("Received message from queue (%d bytes)", len(msg))

	// Parse the message to validate it's proper JSON
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		logging.Error.Printf("Failed to parse message as JSON: %v", err)
		return err
	}

	logging.Info.Printf("Successfully processed queue message: %v", data)

	// TODO (future PRs):
	// 1. Deserialize into domain.AlertBatch
	// 2. Invoke correlator.Process()
	// 3. Store incidents
	// 4. Trigger investigation if needed

	return nil
}
