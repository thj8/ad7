package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ad7/internal/config"
)

func TestInitStdoutOnly(t *testing.T) {
	err := Init(config.LogConfig{Path: "", Level: "info"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	Info("test message", "key", "value")
}

func TestInitWithFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "info"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("file test message", "key", "value")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("log file not created")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"", "INFO"},
		{"unknown", "INFO"},
	}
	for _, tt := range tests {
		got := parseLevel(tt.input)
		if got.String() != tt.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLogFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "debug"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("hello", "key", "value")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"msg":"hello"`) {
		t.Errorf("log file should contain msg=hello, got: %s", content)
	}
	if !strings.Contains(content, `"key":"value"`) {
		t.Errorf("log file should contain key=value, got: %s", content)
	}
}

func TestLevelFiltering(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	err := Init(config.LogConfig{Path: path, Level: "warn"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("should be filtered")
	Warn("should appear", "key", "value")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "should be filtered") {
		t.Error("Info should be filtered at warn level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("Warn should appear at warn level")
	}
}
