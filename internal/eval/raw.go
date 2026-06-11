package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const rawCategory = "Raw Completion"

// rawEvals returns all raw completion evals (apply-template + /completions).
func rawEvals() []Eval {
	return []Eval{
		&rawCompletionEval{},
		&rawToolCallEval{},
		&rawParallelToolCallEval{},
		&rawJSONSchemaEval{},
		&rawComplexJSONSchemaEval{},
	}
}

// rawCompletionEval verifies basic apply-template + /completions flow.
type rawCompletionEval struct{}

func (e *rawCompletionEval) Name() string             { return "raw_completion" }
func (e *rawCompletionEval) Category() string          { return rawCategory }
func (e *rawCompletionEval) Class() string             { return ClassStandard }
func (e *rawCompletionEval) IsDefaultDisabled() bool   { return true }

func (e *rawCompletionEval) Run(ctx context.Context, c *client.Client) Result {
	messages := []client.Message{
		{Role: "user", Content: "Say hello."},
	}

	prompt, err := c.ApplyTemplate(ctx, client.ApplyTemplateRequest{Messages: messages})
	if err != nil {
		return Result{Passed: false, Message: "/apply-template failed: " + err.Error()}
	}

	if strings.TrimSpace(prompt) == "" {
		return Result{Passed: false, Message: "rendered prompt is empty"}
	}

	result, err := c.CompletionStream(ctx, client.CompletionRequest{Prompt: prompt})
	if err != nil {
		return Result{Passed: false, Message: "/completions stream failed: " + err.Error()}
	}

	if strings.TrimSpace(result.Text) == "" {
		return Result{Passed: false, Message: "completion text is empty"}
	}

	if len(result.Tokens) == 0 {
		return Result{Passed: false, Message: "no tokens received in stream"}
	}

	return Result{Passed: true, Message: fmt.Sprintf("%d tokens", len(result.Tokens))}
}

// rawToolCallEval verifies tool call output in raw completions.
type rawToolCallEval struct{}

func (e *rawToolCallEval) Name() string             { return "raw_tool_call" }
func (e *rawToolCallEval) Category() string          { return rawCategory }
func (e *rawToolCallEval) Class() string             { return ClassStandard }
func (e *rawToolCallEval) IsDefaultDisabled() bool   { return true }

func (e *rawToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in San Francisco?"},
	}

	tools := []client.Tool{
		{
			Type: "function",
			Function: client.ToolFunction{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"location": {
							"type": "string",
							"description": "The city and state, e.g. San Francisco, CA"
						}
					},
					"required": ["location"]
				}`),
			},
		},
	}

	prompt, err := c.ApplyTemplate(ctx, client.ApplyTemplateRequest{
		Messages: messages,
		Tools:    tools,
	})
	if err != nil {
		return Result{Passed: false, Message: "/apply-template failed: " + err.Error()}
	}

	if strings.TrimSpace(prompt) == "" {
		return Result{Passed: false, Message: "rendered prompt is empty"}
	}

	result, err := c.CompletionStream(ctx, client.CompletionRequest{Prompt: prompt})
	if err != nil {
		return Result{Passed: false, Message: "/completions stream failed: " + err.Error()}
	}

	text := result.Text

	if strings.TrimSpace(text) == "" {
		return Result{Passed: false, Message: "completion text is empty"}
	}

	if !strings.Contains(text, "get_weather") {
		return Result{Passed: false, Message: "raw output does not contain tool name 'get_weather'"}
	}

	if !strings.Contains(text, "location") {
		return Result{Passed: false, Message: "raw output does not contain 'location' parameter"}
	}

	return Result{Passed: true, Message: fmt.Sprintf("%d tokens", len(result.Tokens))}
}

// rawParallelToolCallEval verifies parallel tool call output in raw completions.
type rawParallelToolCallEval struct{}

func (e *rawParallelToolCallEval) Name() string           { return "raw_parallel_tool_calls" }
func (e *rawParallelToolCallEval) Category() string        { return rawCategory }
func (e *rawParallelToolCallEval) Class() string           { return ClassStandard }
func (e *rawParallelToolCallEval) IsDefaultDisabled() bool { return true }

func (e *rawParallelToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in Tokyo, London, and New York? Call the tool for each city."},
	}

	tools := []client.Tool{
		{
			Type: "function",
			Function: client.ToolFunction{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"location": {
							"type": "string",
							"description": "The city and state/country, e.g. Tokyo, Japan"
						}
					},
					"required": ["location"]
				}`),
			},
		},
	}

	prompt, err := c.ApplyTemplate(ctx, client.ApplyTemplateRequest{
		Messages: messages,
		Tools:    tools,
	})
	if err != nil {
		return Result{Passed: false, Message: "/apply-template failed: " + err.Error()}
	}

	if strings.TrimSpace(prompt) == "" {
		return Result{Passed: false, Message: "rendered prompt is empty"}
	}

	result, err := c.CompletionStream(ctx, client.CompletionRequest{Prompt: prompt})
	if err != nil {
		return Result{Passed: false, Message: "/completions stream failed: " + err.Error()}
	}

	text := result.Text

	if strings.TrimSpace(text) == "" {
		return Result{Passed: false, Message: "completion text is empty"}
	}

	if !strings.Contains(text, "get_weather") {
		return Result{Passed: false, Message: "raw output does not contain tool name 'get_weather'"}
	}

	// Count occurrences of the tool name to verify multiple calls
	count := strings.Count(text, "get_weather")
	if count < 3 {
		return Result{
			Passed:  false,
			Message: fmt.Sprintf("expected at least 3 get_weather invocations, found %d", count),
		}
	}

	// Verify each city appears in the output
	for _, city := range []string{"Tokyo", "London", "New York"} {
		if !strings.Contains(text, city) {
			return Result{
				Passed:  false,
				Message: fmt.Sprintf("raw output does not contain city %q", city),
			}
		}
	}

	return Result{Passed: true, Message: fmt.Sprintf("%d tokens, %d tool invocations", len(result.Tokens), count)}
}

// rawJSONSchemaEval verifies structured JSON output from raw completions.
type rawJSONSchemaEval struct{}

func (e *rawJSONSchemaEval) Name() string             { return "raw_json_schema" }
func (e *rawJSONSchemaEval) Category() string          { return rawCategory }
func (e *rawJSONSchemaEval) Class() string             { return ClassStandard }
func (e *rawJSONSchemaEval) IsDefaultDisabled() bool   { return true }

func (e *rawJSONSchemaEval) Run(ctx context.Context, c *client.Client) Result {
	messages := []client.Message{
		{Role: "user", Content: "Generate a fictional person with a name, age, and occupation. Respond with only valid JSON, no other text."},
	}

	prompt, err := c.ApplyTemplate(ctx, client.ApplyTemplateRequest{Messages: messages})
	if err != nil {
		return Result{Passed: false, Message: "/apply-template failed: " + err.Error()}
	}

	if strings.TrimSpace(prompt) == "" {
		return Result{Passed: false, Message: "rendered prompt is empty"}
	}

	result, err := c.CompletionStream(ctx, client.CompletionRequest{Prompt: prompt})
	if err != nil {
		return Result{Passed: false, Message: "/completions stream failed: " + err.Error()}
	}

	text := strings.TrimSpace(result.Text)

	if text == "" {
		return Result{Passed: false, Message: "completion text is empty"}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return Result{Passed: false, Message: "raw output is not valid JSON: " + err.Error()}
	}

	for _, field := range []string{"name", "age", "occupation"} {
		if _, ok := parsed[field]; !ok {
			return Result{Passed: false, Message: "JSON missing expected field: " + field}
		}
	}

	return Result{Passed: true, Message: fmt.Sprintf("%d tokens", len(result.Tokens))}
}

// rawComplexJSONSchemaEval verifies complex nested JSON output from raw completions.
type rawComplexJSONSchemaEval struct{}

func (e *rawComplexJSONSchemaEval) Name() string           { return "raw_complex_json_schema" }
func (e *rawComplexJSONSchemaEval) Category() string        { return rawCategory }
func (e *rawComplexJSONSchemaEval) Class() string           { return ClassStandard }
func (e *rawComplexJSONSchemaEval) IsDefaultDisabled() bool { return true }

func (e *rawComplexJSONSchemaEval) Run(ctx context.Context, c *client.Client) Result {
	messages := []client.Message{
		{
			Role: "user",
			Content: `Generate a JSON object for a catering request with the following structure. Respond with ONLY valid JSON, no other text.

{
  "event": {
    "name": "<string>",
    "event_type": "<one of: breakfast, lunch, dinner, reception>",
    "date": "<YYYY-MM-DD>"
  },
  "venue": {
    "name": "<string>",
    "address": {
      "street": "<string>",
      "city": "<string>",
      "state": "<string>"
    }
  },
  "guests": [
    {
      "name": "<string>",
      "dietary_restrictions": ["<string>"],
      "meal_preference": "<one of: standard, light, hearty>"
    }
  ],
  "budget": {
    "total_amount": <number>,
    "currency": "<string>"
  }
}

Include exactly 3 guests with different dietary restrictions. Use realistic data.`,
		},
	}

	prompt, err := c.ApplyTemplate(ctx, client.ApplyTemplateRequest{Messages: messages})
	if err != nil {
		return Result{Passed: false, Message: "/apply-template failed: " + err.Error()}
	}

	if strings.TrimSpace(prompt) == "" {
		return Result{Passed: false, Message: "rendered prompt is empty"}
	}

	result, err := c.CompletionStream(ctx, client.CompletionRequest{Prompt: prompt})
	if err != nil {
		return Result{Passed: false, Message: "/completions stream failed: " + err.Error()}
	}

	text := strings.TrimSpace(result.Text)

	if text == "" {
		return Result{Passed: false, Message: "completion text is empty"}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return Result{Passed: false, Message: "raw output is not valid JSON: " + err.Error()}
	}

	// Validate top-level required fields
	for _, field := range []string{"event", "venue", "guests", "budget"} {
		if _, ok := parsed[field]; !ok {
			return Result{Passed: false, Message: "missing required top-level field: " + field}
		}
	}

	// Validate nested event object
	event, ok := parsed["event"].(map[string]any)
	if !ok {
		return Result{Passed: false, Message: "event field is not an object"}
	}
	for _, field := range []string{"name", "event_type", "date"} {
		if _, ok := event[field]; !ok {
			return Result{Passed: false, Message: "event missing required field: " + field}
		}
	}

	// Validate nested venue with address
	venue, ok := parsed["venue"].(map[string]any)
	if !ok {
		return Result{Passed: false, Message: "venue field is not an object"}
	}
	address, ok := venue["address"].(map[string]any)
	if !ok {
		return Result{Passed: false, Message: "venue.address field is not an object"}
	}
	for _, field := range []string{"street", "city", "state"} {
		if _, ok := address[field]; !ok {
			return Result{Passed: false, Message: "venue.address missing required field: " + field}
		}
	}

	// Validate guests array with 3 entries
	guests, ok := parsed["guests"].([]any)
	if !ok {
		return Result{Passed: false, Message: "guests field is not an array"}
	}
	if len(guests) != 3 {
		return Result{Passed: false, Message: fmt.Sprintf("expected 3 guests, got %d", len(guests))}
	}

	for i, g := range guests {
		guest, ok := g.(map[string]any)
		if !ok {
			return Result{Passed: false, Message: fmt.Sprintf("guest %d is not an object", i)}
		}
		if _, ok := guest["name"]; !ok {
			return Result{Passed: false, Message: fmt.Sprintf("guest %d missing field: name", i)}
		}
		if _, ok := guest["dietary_restrictions"]; !ok {
			return Result{Passed: false, Message: fmt.Sprintf("guest %d missing field: dietary_restrictions", i)}
		}
	}

	// Validate budget
	budget, ok := parsed["budget"].(map[string]any)
	if !ok {
		return Result{Passed: false, Message: "budget field is not an object"}
	}
	if _, ok := budget["total_amount"]; !ok {
		return Result{Passed: false, Message: "budget missing required field: total_amount"}
	}

	return Result{Passed: true, Message: fmt.Sprintf("%d tokens", len(result.Tokens))}
}
