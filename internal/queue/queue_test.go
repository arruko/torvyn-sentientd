package queue

import (
	"context"
	"errors"
	"testing"
)

// TestHandlerFunc_TypeSignature verifies the HandlerFunc type signature
func TestHandlerFunc_TypeSignature(t *testing.T) {
	t.Parallel()

	// Create a valid HandlerFunc
	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		return nil
	}

	if handler == nil {
		t.Error("HandlerFunc should not be nil")
	}

	// Test calling the handler
	ctx := context.Background()
	msg := []byte("test message")

	err := handler(ctx, msg)
	if err != nil {
		t.Errorf("Handler() error = %v, want nil", err)
	}
}

// TestHandlerFunc_ErrorHandling tests that HandlerFunc can return errors
func TestHandlerFunc_ErrorHandling(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("handler failed")

	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		return expectedErr
	}

	ctx := context.Background()
	msg := []byte("test message")

	err := handler(ctx, msg)
	if err == nil {
		t.Error("Handler() expected error, got nil")
	}

	if err != expectedErr {
		t.Errorf("Handler() error = %v, want %v", err, expectedErr)
	}
}

// TestHandlerFunc_ContextUsage tests that HandlerFunc receives context
func TestHandlerFunc_ContextUsage(t *testing.T) {
	t.Parallel()

	contextReceived := false

	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		if ctx != nil {
			contextReceived = true
		}
		return nil
	}

	ctx := context.Background()
	msg := []byte("test message")

	_ = handler(ctx, msg)

	if !contextReceived {
		t.Error("HandlerFunc should receive non-nil context")
	}
}

// TestHandlerFunc_MessageData tests that HandlerFunc receives message data
func TestHandlerFunc_MessageData(t *testing.T) {
	t.Parallel()

	var receivedMsg []byte

	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		receivedMsg = msg
		return nil
	}

	ctx := context.Background()
	expectedMsg := []byte(`{"id":"inc-999","cluster":"prod","service":"api","severity":"critical"}`)

	_ = handler(ctx, expectedMsg)

	if string(receivedMsg) != string(expectedMsg) {
		t.Errorf("Received message = %s, want %s", string(receivedMsg), string(expectedMsg))
	}
}

// MockQueue implements the Queue interface for testing
type MockQueue struct {
	subscribed bool
	closed     bool
	handler    HandlerFunc
	subject    string
	published  [][]byte
}

func (m *MockQueue) Publish(ctx context.Context, subject string, data []byte) error {
	m.published = append(m.published, data)
	return nil
}

func (m *MockQueue) Subscribe(subject string, handler HandlerFunc) error {
	m.subscribed = true
	m.handler = handler
	m.subject = subject
	return nil
}

func (m *MockQueue) Close() error {
	m.closed = true
	return nil
}

// TestQueue_Interface verifies the Queue interface contract
func TestQueue_Interface(t *testing.T) {
	t.Parallel()

	var queue Queue = &MockQueue{}

	// Verify interface implementation
	if err := queue.Publish(context.Background(), "test", []byte("data")); err != nil {
		t.Errorf("Publish should succeed: %v", err)
	}
}

// TestMockQueue_Publish tests the mock implementation
func TestMockQueue_Publish(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}

	ctx := context.Background()
	subject := "incidents"
	data := []byte(`{"id":"inc-123"}`)

	err := mock.Publish(ctx, subject, data)

	if err != nil {
		t.Errorf("Publish() error = %v, want nil", err)
	}

	if len(mock.published) != 1 {
		t.Errorf("Published count = %d, want 1", len(mock.published))
	}

	if string(mock.published[0]) != string(data) {
		t.Errorf("Published data = %s, want %s", string(mock.published[0]), string(data))
	}
}

// TestMockQueue_Subscribe tests the mock implementation
func TestMockQueue_Subscribe(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}

	handler := func(ctx context.Context, msg []byte) error {
		return nil
	}

	subject := "incidents"
	err := mock.Subscribe(subject, handler)

	if err != nil {
		t.Errorf("Subscribe() error = %v, want nil", err)
	}

	if !mock.subscribed {
		t.Error("Subscribe() should set subscribed flag")
	}

	if mock.handler == nil {
		t.Error("Subscribe() should store handler")
	}

	if mock.subject != subject {
		t.Errorf("Subscribe() subject = %v, want %v", mock.subject, subject)
	}
}

// TestMockQueue_Close tests the mock implementation
func TestMockQueue_Close(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}

	err := mock.Close()

	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if !mock.closed {
		t.Error("Close() should set closed flag")
	}
}

// TestMockQueue_FullLifecycle tests publish, subscribe and close together
func TestMockQueue_FullLifecycle(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}

	// Publish
	ctx := context.Background()
	if err := mock.Publish(ctx, "test", []byte("data")); err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	if len(mock.published) != 1 {
		t.Error("Should have published 1 message")
	}

	// Subscribe
	handler := func(ctx context.Context, msg []byte) error {
		return nil
	}

	if err := mock.Subscribe("test", handler); err != nil {
		t.Errorf("Subscribe() error = %v", err)
	}

	if !mock.subscribed {
		t.Error("Should be subscribed")
	}

	// Close
	if err := mock.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !mock.closed {
		t.Error("Should be closed")
	}
}

// MockQueueWithError implements Queue with error scenarios
type MockQueueWithError struct {
	publishErr   error
	subscribeErr error
	closeErr     error
}

func (m *MockQueueWithError) Publish(ctx context.Context, subject string, data []byte) error {
	return m.publishErr
}

func (m *MockQueueWithError) Subscribe(subject string, handler HandlerFunc) error {
	return m.subscribeErr
}

func (m *MockQueueWithError) Close() error {
	return m.closeErr
}

// TestQueue_ErrorHandling tests error scenarios
func TestQueue_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		publishErr   error
		subscribeErr error
		closeErr     error
	}{
		{
			name:         "publish error",
			publishErr:   errors.New("publish failed"),
			subscribeErr: nil,
			closeErr:     nil,
		},
		{
			name:         "subscribe error",
			publishErr:   nil,
			subscribeErr: errors.New("subscribe failed"),
			closeErr:     nil,
		},
		{
			name:         "close error",
			publishErr:   nil,
			subscribeErr: nil,
			closeErr:     errors.New("close failed"),
		},
		{
			name:         "all errors",
			publishErr:   errors.New("publish failed"),
			subscribeErr: errors.New("subscribe failed"),
			closeErr:     errors.New("close failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &MockQueueWithError{
				publishErr:   tt.publishErr,
				subscribeErr: tt.subscribeErr,
				closeErr:     tt.closeErr,
			}

			handler := func(ctx context.Context, msg []byte) error {
				return nil
			}

			// Test Publish
			ctx := context.Background()
			err := mock.Publish(ctx, "test", []byte("data"))
			if tt.publishErr != nil && err == nil {
				t.Error("Publish() expected error, got nil")
			}
			if tt.publishErr == nil && err != nil {
				t.Errorf("Publish() unexpected error: %v", err)
			}

			// Test Subscribe
			err = mock.Subscribe("test", handler)
			if tt.subscribeErr != nil && err == nil {
				t.Error("Subscribe() expected error, got nil")
			}
			if tt.subscribeErr == nil && err != nil {
				t.Errorf("Subscribe() unexpected error: %v", err)
			}

			// Test Close
			err = mock.Close()
			if tt.closeErr != nil && err == nil {
				t.Error("Close() expected error, got nil")
			}
			if tt.closeErr == nil && err != nil {
				t.Errorf("Close() unexpected error: %v", err)
			}
		})
	}
}

// TestQueue_ContextCancellation tests handler behavior with cancelled context
func TestQueue_ContextCancellation(t *testing.T) {
	t.Parallel()

	handlerCalled := false

	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		handlerCalled = true
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := []byte("test")
	err := handler(ctx, msg)

	if !handlerCalled {
		t.Error("Handler should be called even with cancelled context")
	}

	if err == nil {
		t.Error("Handler should return error for cancelled context")
	}
}

// TestQueue_NilHandler tests behavior with nil handler
func TestQueue_NilHandler(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}

	err := mock.Subscribe("test", nil)

	if err != nil {
		t.Errorf("Subscribe() with nil handler error = %v", err)
	}

	if mock.handler != nil {
		t.Error("Nil handler should be stored as nil")
	}
}

// TestQueue_EmptyMessage tests handler with empty message
func TestQueue_EmptyMessage(t *testing.T) {
	t.Parallel()

	var receivedMsg []byte
	var handler HandlerFunc = func(ctx context.Context, msg []byte) error {
		receivedMsg = msg
		return nil
	}

	ctx := context.Background()
	emptyMsg := []byte{}

	err := handler(ctx, emptyMsg)

	if err != nil {
		t.Errorf("Handler() with empty message error = %v", err)
	}

	if len(receivedMsg) != 0 {
		t.Errorf("Received message length = %d, want 0", len(receivedMsg))
	}
}

// TestQueue_MultiplePublish tests publishing multiple messages
func TestQueue_MultiplePublish(t *testing.T) {
	t.Parallel()

	mock := &MockQueue{}
	ctx := context.Background()

	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("message 3"),
	}

	for _, msg := range messages {
		if err := mock.Publish(ctx, "test", msg); err != nil {
			t.Errorf("Publish() error = %v", err)
		}
	}

	if len(mock.published) != len(messages) {
		t.Errorf("Published count = %d, want %d", len(mock.published), len(messages))
	}

	for i, msg := range messages {
		if string(mock.published[i]) != string(msg) {
			t.Errorf("Published[%d] = %s, want %s", i, string(mock.published[i]), string(msg))
		}
	}
}

// TestQueue_SubjectMatching tests that subscribe uses correct subject
func TestQueue_SubjectMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
	}{
		{"incidents subject", "incidents"},
		{"alerts subject", "alerts"},
		{"metrics subject", "metrics"},
		{"empty subject", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &MockQueue{}
			handler := func(ctx context.Context, msg []byte) error {
				return nil
			}

			err := mock.Subscribe(tt.subject, handler)

			if err != nil {
				t.Errorf("Subscribe() error = %v", err)
			}

			if mock.subject != tt.subject {
				t.Errorf("Subject = %v, want %v", mock.subject, tt.subject)
			}
		})
	}
}
