package log

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RequestLogger is the interface used by the client for logging requests/responses.
type RequestLogger interface {
	LogRequest(method, url string, body []byte)
	LogResponse(status int, body []byte)
	LogStreamResponse(status int, rawChunks []byte)
	LogStreamChunks(jsonl []byte)
}

// TurnData captures a single request/response pair for report generation.
type TurnData struct {
	URL          string
	RequestBody  json.RawMessage
	ResponseBody json.RawMessage // synthesized from stream chunks for streaming
}

// EvalResult holds the structured result of an eval for report generation.
type EvalResult struct {
	Name    string
	Passed  bool
	Message string
	Turns   []TurnData
}

// Logger handles request/response logging to files.
type Logger struct {
	dir   string
	model string

	mu    sync.Mutex
	evals []EvalResult
}

// New creates a new Logger, creating the log directory.
// Logs are grouped by model name: logs/<model>/<timestamp>/
func New(model string) (*Logger, error) {
	timestamp := time.Now().Format("2006-01-02_150405")
	dir := filepath.Join("logs", model, timestamp)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	return &Logger{dir: dir, model: model}, nil
}

// Dir returns the log directory path.
func (l *Logger) Dir() string {
	return l.dir
}

// Model returns the model name.
func (l *Logger) Model() string {
	return l.model
}

// Evals returns the collected eval results.
func (l *Logger) Evals() []EvalResult {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]EvalResult(nil), l.evals...)
}

// registerEval adds a completed eval result.
func (l *Logger) registerEval(result EvalResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evals = append(l.evals, result)
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

	// Structured data for report generation
	pendingURL     string
	pendingRequest json.RawMessage
	turns          []TurnData
	passed         bool
	message        string
}

// LogRequest logs an HTTP request.
func (el *EvalLog) LogRequest(method, url string, body []byte) {
	el.buf.WriteString(">>> REQUEST\n")
	el.buf.WriteString(fmt.Sprintf("%s %s\n", method, url))
	el.buf.WriteString("\n")
	el.buf.Write(formatJSON(body))
	el.buf.WriteString("\n\n")

	// Capture for report
	el.pendingURL = url
	el.pendingRequest = append(json.RawMessage(nil), body...)
}

// LogResponse logs an HTTP response.
func (el *EvalLog) LogResponse(status int, body []byte) {
	el.buf.WriteString("<<< RESPONSE\n")
	el.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	el.buf.WriteString("\n")
	el.buf.Write(formatJSON(body))
	el.buf.WriteString("\n\n")

	// Capture turn for report
	el.turns = append(el.turns, TurnData{
		URL:          el.pendingURL,
		RequestBody:  el.pendingRequest,
		ResponseBody: append(json.RawMessage(nil), body...),
	})
	el.pendingRequest = nil
	el.pendingURL = ""
}

// LogStreamResponse logs a streaming response.
func (el *EvalLog) LogStreamResponse(status int, rawChunks []byte) {
	el.buf.WriteString("<<< STREAM RESPONSE\n")
	el.buf.WriteString(fmt.Sprintf("Status: %d\n", status))
	el.buf.WriteString("\n")
	el.buf.Write(rawChunks)
	el.buf.WriteString("\n")
}

// LogStreamChunks stores JSONL-formatted stream chunks for replay,
// and reconstructs a synthetic response for report generation.
func (el *EvalLog) LogStreamChunks(jsonl []byte) {
	el.streamChunks = jsonl

	// Reconstruct a synthetic response from stream chunks for the report
	synthetic := reconstructFromChunks(jsonl)
	if synthetic != nil {
		el.turns = append(el.turns, TurnData{
			URL:          el.pendingURL,
			RequestBody:  el.pendingRequest,
			ResponseBody: synthetic,
		})
		el.pendingRequest = nil
		el.pendingURL = ""
	}
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

	el.passed = passed
	el.message = message
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

	// Register structured data with parent logger
	el.logger.registerEval(EvalResult{
		Name:    el.name,
		Passed:  el.passed,
		Message: el.message,
		Turns:   el.turns,
	})

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

// reconstructFromChunks builds a synthetic ChatCompletion-shaped response
// from JSONL stream chunks. Uses generic maps to avoid importing client types.
func reconstructFromChunks(jsonl []byte) json.RawMessage {
	var content strings.Builder
	var reasoningContent strings.Builder
	toolCalls := make(map[int]map[string]any) // index -> tool call object

	scanner := bufio.NewScanner(bytes.NewReader(jsonl))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk map[string]any
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		choices, ok := chunk["choices"].([]any)
		if !ok || len(choices) == 0 {
			continue
		}

		choice, ok := choices[0].(map[string]any)
		if !ok {
			continue
		}

		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}

		if c, ok := delta["content"].(string); ok {
			content.WriteString(c)
		}
		if rc, ok := delta["reasoning_content"].(string); ok {
			reasoningContent.WriteString(rc)
		}

		if tcs, ok := delta["tool_calls"].([]any); ok {
			for _, tcRaw := range tcs {
				tc, ok := tcRaw.(map[string]any)
				if !ok {
					continue
				}

				idx := 0
				if idxF, ok := tc["index"].(float64); ok {
					idx = int(idxF)
				}

				existing, ok := toolCalls[idx]
				if !ok {
					existing = map[string]any{
						"type": "function",
						"function": map[string]any{
							"name":      "",
							"arguments": "",
						},
					}
					toolCalls[idx] = existing
				}

				if id, ok := tc["id"].(string); ok && id != "" {
					existing["id"] = id
				}
				if typ, ok := tc["type"].(string); ok && typ != "" {
					existing["type"] = typ
				}

				if fn, ok := tc["function"].(map[string]any); ok {
					efn := existing["function"].(map[string]any)
					if name, ok := fn["name"].(string); ok && name != "" {
						efn["name"] = name
					}
					if args, ok := fn["arguments"].(string); ok {
						efn["arguments"] = efn["arguments"].(string) + args
					}
				}
			}
		}
	}

	// Build the synthetic response message
	msg := map[string]any{
		"role": "assistant",
	}
	if content.Len() > 0 {
		msg["content"] = content.String()
	}
	if reasoningContent.Len() > 0 {
		msg["reasoning_content"] = reasoningContent.String()
	}
	if len(toolCalls) > 0 {
		var tcs []any
		for i := 0; i < len(toolCalls); i++ {
			if tc, ok := toolCalls[i]; ok {
				tcs = append(tcs, tc)
			}
		}
		msg["tool_calls"] = tcs
	}

	resp := map[string]any{
		"choices": []any{
			map[string]any{
				"message":       msg,
				"finish_reason": "stop",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return nil
	}
	return data
}
