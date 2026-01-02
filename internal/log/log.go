package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RequestLogger is the interface used by the client for logging requests/responses.
type RequestLogger interface {
	LogRequest(method, url string, body []byte)
	LogResponse(status int, body []byte)
	LogStreamResponse(status int, rawChunks []byte)
	LogStreamChunks(jsonl []byte)
}

// Logger handles request/response logging to files.
type Logger struct {
	dir string
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

// StartEval starts logging for a new eval and returns an EvalLog handle.
// The returned EvalLog is safe for concurrent use by a single eval.
// Each eval should have its own EvalLog to avoid race conditions.
func (l *Logger) StartEval(name string) *EvalLog {
	el := &EvalLog{
		logger: l,
		name:   name,
	}
	el.buf.WriteString(fmt.Sprintf("=== Eval: %s ===\n", name))
	el.buf.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))
	return el
}

// EvalLog represents logging context for a single eval.
// It is isolated from other evals and safe for concurrent use within one eval.
type EvalLog struct {
	logger       *Logger
	name         string
	buf          bytes.Buffer
	streamChunks []byte
}

// LogRequest logs an HTTP request.
func (el *EvalLog) LogRequest(method, url string, body []byte) {
	el.buf.WriteString(">>> REQUEST\n")
	el.buf.WriteString(fmt.Sprintf("%s %s\n", method, url))
	el.buf.WriteString("\n")
	el.buf.Write(formatJSON(body))
	el.buf.WriteString("\n\n")
}

// LogResponse logs an HTTP response.
func (el *EvalLog) LogResponse(status int, body []byte) {
	el.buf.WriteString("<<< RESPONSE\n")
	el.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	el.buf.WriteString("\n")
	el.buf.Write(formatJSON(body))
	el.buf.WriteString("\n\n")
}

// LogStreamResponse logs a streaming response.
func (el *EvalLog) LogStreamResponse(status int, rawChunks []byte) {
	el.buf.WriteString("<<< STREAM RESPONSE\n")
	el.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	el.buf.WriteString("\n")
	el.buf.Write(rawChunks)
	el.buf.WriteString("\n")
}

// LogStreamChunks stores JSONL-formatted stream chunks for replay.
func (el *EvalLog) LogStreamChunks(jsonl []byte) {
	el.streamChunks = jsonl
}

// LogError logs an error.
func (el *EvalLog) LogError(err error) {
	el.buf.WriteString(fmt.Sprintf("!!! ERROR: %v\n\n", err))
}

// LogValidation logs validation details.
func (el *EvalLog) LogValidation(description string, expected, actual any) {
	el.buf.WriteString(fmt.Sprintf("--- VALIDATION: %s\n", description))
	el.buf.WriteString(fmt.Sprintf("Expected: %v\n", expected))
	el.buf.WriteString(fmt.Sprintf("Actual:   %v\n\n", actual))
}

// LogResult logs the eval result.
func (el *EvalLog) LogResult(passed bool, message string) {
	status := "PASSED"
	if !passed {
		status = "FAILED"
	}

	el.buf.WriteString(fmt.Sprintf("=== Result: %s ===\n", status))
	if message != "" {
		el.buf.WriteString(message)
		el.buf.WriteString("\n")
	}
}

// End finishes logging for this eval and writes to file.
func (el *EvalLog) End() error {
	filename := filepath.Join(el.logger.dir, el.name+".log")
	if err := os.WriteFile(filename, el.buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write log file: %w", err)
	}

	// Write JSONL file for streaming responses
	if len(el.streamChunks) > 0 {
		jsonlFile := filepath.Join(el.logger.dir, el.name+".stream.jsonl")
		if err := os.WriteFile(jsonlFile, el.streamChunks, 0644); err != nil {
			return fmt.Errorf("write stream jsonl file: %w", err)
		}
	}

	return nil
}

// Close is a no-op for Logger. Individual EvalLogs handle their own cleanup.
func (l *Logger) Close() error {
	return nil
}

// formatJSON formats JSON for readability.
func formatJSON(data []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return data // Return original if not valid JSON
	}
	return buf.Bytes()
}
