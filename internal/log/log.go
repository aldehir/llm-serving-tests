package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger handles request/response logging to files.
type Logger struct {
	dir     string
	mu      sync.Mutex
	current *logFile
}

type logFile struct {
	name string
	buf  bytes.Buffer
}

// New creates a new Logger, creating the log directory.
// Logs are grouped by model name: logs/<model>/<timestamp>/
func New(model string) (*Logger, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	dir := filepath.Join("logs", model, timestamp)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	return &Logger{dir: dir}, nil
}

// Dir returns the log directory path.
func (l *Logger) Dir() string {
	return l.dir
}

// StartEval starts logging for a new eval.
func (l *Logger) StartEval(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.current = &logFile{name: name}
	l.current.buf.WriteString(fmt.Sprintf("=== Eval: %s ===\n", name))
	l.current.buf.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))
}

// LogRequest logs an HTTP request.
func (l *Logger) LogRequest(method, url string, body []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	l.current.buf.WriteString(">>> REQUEST\n")
	l.current.buf.WriteString(fmt.Sprintf("%s %s\n", method, url))
	l.current.buf.WriteString("\n")
	l.current.buf.Write(formatJSON(body))
	l.current.buf.WriteString("\n\n")
}

// LogResponse logs an HTTP response.
func (l *Logger) LogResponse(status int, body []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	l.current.buf.WriteString("<<< RESPONSE\n")
	l.current.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	l.current.buf.WriteString("\n")
	l.current.buf.Write(formatJSON(body))
	l.current.buf.WriteString("\n\n")
}

// LogStreamResponse logs a streaming response.
func (l *Logger) LogStreamResponse(status int, rawChunks []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	l.current.buf.WriteString("<<< STREAM RESPONSE\n")
	l.current.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	l.current.buf.WriteString("\n")
	l.current.buf.Write(rawChunks)
	l.current.buf.WriteString("\n")
}

// LogError logs an error.
func (l *Logger) LogError(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	l.current.buf.WriteString(fmt.Sprintf("!!! ERROR: %v\n\n", err))
}

// LogValidation logs validation details.
func (l *Logger) LogValidation(description string, expected, actual any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	l.current.buf.WriteString(fmt.Sprintf("--- VALIDATION: %s\n", description))
	l.current.buf.WriteString(fmt.Sprintf("Expected: %v\n", expected))
	l.current.buf.WriteString(fmt.Sprintf("Actual:   %v\n\n", actual))
}

// LogResult logs the eval result.
func (l *Logger) LogResult(passed bool, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return
	}

	status := "PASSED"
	if !passed {
		status = "FAILED"
	}

	l.current.buf.WriteString(fmt.Sprintf("=== Result: %s ===\n", status))
	if message != "" {
		l.current.buf.WriteString(message)
		l.current.buf.WriteString("\n")
	}
}

// EndEval finishes logging for the current eval and writes to file.
func (l *Logger) EndEval() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.current == nil {
		return nil
	}

	filename := filepath.Join(l.dir, l.current.name+".log")
	if err := os.WriteFile(filename, l.current.buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}

	l.current = nil
	return nil
}

// Close closes the logger.
func (l *Logger) Close() error {
	return l.EndEval()
}

// formatJSON formats JSON for readability.
func formatJSON(data []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return data // Return original if not valid JSON
	}
	return buf.Bytes()
}
