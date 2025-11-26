package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLoggers_Initialization(t *testing.T) {
	t.Parallel()

	if Info == nil {
		t.Error("Info logger should not be nil")
	}

	if Error == nil {
		t.Error("Error logger should not be nil")
	}
}

func TestLoggers_Prefixes(t *testing.T) {
	t.Parallel()

	// Create test loggers with custom output buffers
	var infoBuf bytes.Buffer
	var errorBuf bytes.Buffer

	testInfo := log.New(&infoBuf, "INFO: ", log.LstdFlags)
	testError := log.New(&errorBuf, "ERROR: ", log.LstdFlags)

	// Test Info logger
	testInfo.Println("test info message")
	infoOutput := infoBuf.String()

	if !strings.Contains(infoOutput, "INFO:") {
		t.Error("Info logger output should contain 'INFO:' prefix")
	}

	if !strings.Contains(infoOutput, "test info message") {
		t.Error("Info logger output should contain the message")
	}

	// Test Error logger
	testError.Println("test error message")
	errorOutput := errorBuf.String()

	if !strings.Contains(errorOutput, "ERROR:") {
		t.Error("Error logger output should contain 'ERROR:' prefix")
	}

	if !strings.Contains(errorOutput, "test error message") {
		t.Error("Error logger output should contain the message")
	}
}

func TestLoggers_Methods(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := log.New(&buf, "TEST: ", log.LstdFlags)

	// Test Printf
	testLogger.Printf("formatted %s %d", "message", 123)
	if !strings.Contains(buf.String(), "formatted message 123") {
		t.Error("Printf should format the message correctly")
	}

	// Test Println
	buf.Reset()
	testLogger.Println("line message")
	if !strings.Contains(buf.String(), "line message") {
		t.Error("Println should write the message")
	}

	// Test Print
	buf.Reset()
	testLogger.Print("simple message")
	if !strings.Contains(buf.String(), "simple message") {
		t.Error("Print should write the message")
	}
}

func TestLoggers_Timestamps(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := log.New(&buf, "TEST: ", log.LstdFlags)

	testLogger.Println("message")
	output := buf.String()

	// LstdFlags includes date and time
	// Check if output has the expected format: "TEST: YYYY/MM/DD HH:MM:SS message"
	if !strings.HasPrefix(output, "TEST: ") {
		t.Error("Logger output should have prefix")
	}

	// Output should contain date separator "/"
	if !strings.Contains(output, "/") {
		t.Error("Logger output should contain date")
	}

	// Output should contain time separator ":"
	count := strings.Count(output, ":")
	if count < 2 { // At least HH:MM:SS = 2 colons
		t.Error("Logger output should contain time")
	}
}

func TestLoggers_ConcurrentUsage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := log.New(&buf, "CONCURRENT: ", log.LstdFlags)

	// log.Logger is safe for concurrent use
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			testLogger.Printf("message from goroutine %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that all messages were written
	output := buf.String()
	messageCount := strings.Count(output, "message from goroutine")

	if messageCount != 10 {
		t.Errorf("Expected 10 messages, got %d", messageCount)
	}
}

func TestLoggers_EmptyMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := log.New(&buf, "TEST: ", log.LstdFlags)

	testLogger.Println("")
	output := buf.String()

	// Should still have prefix and timestamp even with empty message
	if !strings.HasPrefix(output, "TEST: ") {
		t.Error("Logger should write prefix even with empty message")
	}
}

func TestLoggers_SpecialCharacters(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := log.New(&buf, "TEST: ", log.LstdFlags)

	specialMsg := "message with\nnewline\ttab\"quote"
	testLogger.Println(specialMsg)
	output := buf.String()

	if !strings.Contains(output, "newline") {
		t.Error("Logger should handle newlines in message")
	}

	if !strings.Contains(output, "tab") {
		t.Error("Logger should handle tabs in message")
	}

	if !strings.Contains(output, "quote") {
		t.Error("Logger should handle quotes in message")
	}
}
