package eval

import (
	"context"
	"encoding/json"

	"github.com/aldehir/llm-evals/internal/client"
)

const toolCategory = "Tool Calling"

// toolEvals returns all tool-calling-related evals.
func toolEvals() []Eval {
	return []Eval{
		&singleToolCallEval{streaming: false},
		&singleToolCallEval{streaming: true},
		&parallelToolCallEval{streaming: false},
		&parallelToolCallEval{streaming: true},
	}
}

// singleToolCallEval verifies that a single tool call is correctly returned.
type singleToolCallEval struct {
	streaming bool
}

func (e *singleToolCallEval) Name() string {
	if e.streaming {
		return "single_tool_call_streaming"
	}
	return "single_tool_call"
}

func (e *singleToolCallEval) Category() string {
	return toolCategory
}

func (e *singleToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools: []client.Tool{
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
		},
		ToolChoice: "auto",
	}

	var toolCalls []client.ToolCall

	if e.streaming {
		result, err := c.ChatCompletionStream(ctx, req)
		if err != nil {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "request failed: " + err.Error(),
			}
		}
		toolCalls = result.ToolCalls
	} else {
		resp, err := c.ChatCompletion(ctx, req)
		if err != nil {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "request failed: " + err.Error(),
			}
		}
		if len(resp.Choices) == 0 {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "no choices in response",
			}
		}
		toolCalls = resp.Choices[0].Message.ToolCalls
	}

	// Verify we got exactly one tool call
	if len(toolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool call, got none",
		}
	}

	if len(toolCalls) > 1 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected 1 tool call, got multiple",
		}
	}

	tc := toolCalls[0]

	// Verify tool name
	if tc.Function.Name != "get_weather" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool name 'get_weather', got '" + tc.Function.Name + "'",
		}
	}

	// Verify arguments are valid JSON
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "tool arguments are not valid JSON: " + err.Error(),
		}
	}

	// Verify location parameter exists
	if _, ok := args["location"]; !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "tool arguments missing 'location' parameter",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// parallelToolCallEval verifies that multiple tool calls are correctly returned.
type parallelToolCallEval struct {
	streaming bool
}

func (e *parallelToolCallEval) Name() string {
	if e.streaming {
		return "parallel_tool_calls_streaming"
	}
	return "parallel_tool_calls"
}

func (e *parallelToolCallEval) Category() string {
	return toolCategory
}

func (e *parallelToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in both San Francisco and New York?"},
		},
		Tools: []client.Tool{
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
		},
		ToolChoice:        "auto",
		ParallelToolCalls: true,
	}

	var toolCalls []client.ToolCall

	if e.streaming {
		result, err := c.ChatCompletionStream(ctx, req)
		if err != nil {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "request failed: " + err.Error(),
			}
		}
		toolCalls = result.ToolCalls
	} else {
		resp, err := c.ChatCompletion(ctx, req)
		if err != nil {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "request failed: " + err.Error(),
			}
		}
		if len(resp.Choices) == 0 {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "no choices in response",
			}
		}
		toolCalls = resp.Choices[0].Message.ToolCalls
	}

	// Verify we got at least 2 tool calls
	if len(toolCalls) < 2 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected at least 2 tool calls for parallel execution, got " + string(rune('0'+len(toolCalls))),
		}
	}

	// Verify all tool calls are valid
	for i, tc := range toolCalls {
		if tc.Function.Name != "get_weather" {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "tool call " + string(rune('0'+i)) + " has wrong name: " + tc.Function.Name,
			}
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "tool call " + string(rune('0'+i)) + " has invalid JSON arguments",
			}
		}

		if _, ok := args["location"]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "tool call " + string(rune('0'+i)) + " missing 'location' parameter",
			}
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
