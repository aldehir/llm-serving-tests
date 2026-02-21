package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"html/template"
	"time"

	"github.com/aldehir/llm-serving-tests/internal/log"
)

var reportTemplate = template.Must(template.New("report").Parse(htmlTemplate))

// reportData is the top-level JSON structure injected into the HTML template.
type reportData struct {
	Model     string      `json:"model"`
	Timestamp string      `json:"timestamp"`
	Passed    int         `json:"passed"`
	Total     int         `json:"total"`
	Evals     []evalEntry `json:"evals"`
}

// evalEntry represents one eval in the report.
type evalEntry struct {
	Name     string           `json:"name"`
	Passed   bool             `json:"passed"`
	Message  string           `json:"message,omitempty"`
	Tools    []json.RawMessage `json:"tools,omitempty"`
	Messages []json.RawMessage `json:"messages"`
}

// WriteReport generates report.html in the given directory from eval results.
func WriteReport(dir, model string, evals []log.EvalResult) error {
	data := reportData{
		Model:     model,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Total:     len(evals),
	}

	for _, ev := range evals {
		if ev.Passed {
			data.Passed++
		}

		entry := evalEntry{
			Name:    ev.Name,
			Passed:  ev.Passed,
			Message: ev.Message,
		}

		// Filter out apply-template turns
		var turns []log.TurnData
		for _, t := range ev.Turns {
			if strings.Contains(t.URL, "/apply-template") {
				continue
			}
			turns = append(turns, t)
		}

		// Extract tools from first turn's request
		if len(turns) > 0 {
			entry.Tools = extractTools(turns[0].RequestBody)
		}

		// Build conversation: take last turn's request messages + last turn's response
		entry.Messages = buildConversation(turns)

		data.Evals = append(data.Evals, entry)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal report data: %w", err)
	}

	outPath := filepath.Join(dir, "report.html")
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create report file: %w", err)
	}
	defer f.Close()

	if err := reportTemplate.Execute(f, struct {
		DataJSON template.JS
	}{
		DataJSON: template.JS(jsonData),
	}); err != nil {
		return fmt.Errorf("execute report template: %w", err)
	}

	return nil
}

// extractTools pulls the tools array from a request body.
func extractTools(reqBody json.RawMessage) []json.RawMessage {
	if len(reqBody) == 0 {
		return nil
	}

	var req struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return nil
	}
	return req.Tools
}

// buildConversation reconstructs the full conversation from turn data.
// Takes the last turn's request messages (which contain the full history)
// and appends the last turn's response as the final assistant message.
func buildConversation(turns []log.TurnData) []json.RawMessage {
	if len(turns) == 0 {
		return nil
	}

	lastTurn := turns[len(turns)-1]

	// Extract messages from the last request
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(lastTurn.RequestBody, &req); err != nil {
		return nil
	}

	messages := req.Messages

	// Extract the assistant message from the last response
	assistantMsg := extractAssistantMessage(lastTurn.ResponseBody)
	if assistantMsg != nil {
		messages = append(messages, assistantMsg)
	}

	return messages
}

// extractAssistantMessage extracts choices[0].message from a response body.
func extractAssistantMessage(respBody json.RawMessage) json.RawMessage {
	if len(respBody) == 0 {
		return nil
	}

	var resp struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil
	}
	if len(resp.Choices) == 0 {
		return nil
	}
	return resp.Choices[0].Message
}
