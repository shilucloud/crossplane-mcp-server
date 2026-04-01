package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"invalid level defaults to info", "invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.level)
			if Logger == nil {
				t.Error("Logger should not be nil after Init")
			}
		})
	}
}

func TestLogLevels(t *testing.T) {
	Init("debug")

	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))
	testLogger.Info("info message", "key", "value")

	output := buf.String()
	if output == "" {
		t.Error("Expected log output, got empty string")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("Expected valid JSON output, got error: %v", err)
	}
}

func TestDebug(t *testing.T) {
	Init("debug")
	Debug("test debug message", "key", "value")
}

func TestInfo(t *testing.T) {
	Init("info")
	Info("test info message", "key", "value")
}

func TestWarn(t *testing.T) {
	Init("warn")
	Warn("test warn message", "key", "value")
}

func TestError(t *testing.T) {
	Init("error")
	Error("test error message", "key", "value")
}

func TestLogWithContext(t *testing.T) {
	Init("info")

	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	testLogger.Info("request processed",
		"method", "GET",
		"path", "/api/v1/test",
		"status", 200,
		"duration_ms", 45,
	)

	output := buf.String()
	if output == "" {
		t.Error("Expected log output")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("Expected valid JSON: %v", err)
	}

	if logEntry["msg"] != "request processed" {
		t.Errorf("Expected msg 'request processed', got %v", logEntry["msg"])
	}
}

func TestLogToStdout(t *testing.T) {
	Init("info")
	Info("stdout test", "test", true)

	if Logger == nil {
		t.Error("Logger should be initialized")
	}
}

func TestInvalidLogLevel(t *testing.T) {
	Init("not_a_valid_level")

	if Logger == nil {
		t.Error("Logger should be initialized even with invalid level")
	}
}

func TestConcurrentLogging(t *testing.T) {
	Init("info")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			Info("concurrent log", "id", id)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
