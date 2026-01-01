package eval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const toolCategory = "Tool Calling"

// toolEvals returns all tool-calling-related evals.
func toolEvals() []Eval {
	return []Eval{
		&singleToolCallEval{},
		&parallelToolCallEval{},
		&requiredToolCallEval{},
		&requiredToolCallWithReasoningEval{},
		&complexSchemaToolCallEval{},
		&codeGenerationToolCallEval{},
	}
}

// singleToolCallEval verifies that a single tool call is correctly returned.
type singleToolCallEval struct {
	streaming bool
}

func (e *singleToolCallEval) Name() string {
	return "single_tool_call"
}

func (e *singleToolCallEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *singleToolCallEval) Streaming() bool             { return e.streaming }

func (e *singleToolCallEval) Category() string {
	return toolCategory
}

func (e *singleToolCallEval) Class() string {
	return ClassStandard
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
	return "parallel_tool_calls"
}

func (e *parallelToolCallEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *parallelToolCallEval) Streaming() bool             { return e.streaming }

func (e *parallelToolCallEval) Category() string {
	return toolCategory
}

func (e *parallelToolCallEval) Class() string {
	return ClassStandard
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

// requiredToolCallEval verifies tool_choice: "required" forces a tool call.
type requiredToolCallEval struct {
	streaming bool
}

func (e *requiredToolCallEval) Name() string {
	return "required_tool_call"
}

func (e *requiredToolCallEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *requiredToolCallEval) Streaming() bool             { return e.streaming }

func (e *requiredToolCallEval) Category() string {
	return toolCategory
}

func (e *requiredToolCallEval) Class() string {
	return ClassStandard
}

func (e *requiredToolCallEval) Run(ctx context.Context, c *client.Client) Result {
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
		ToolChoice: "required",
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

	// Verify we got a tool call
	if len(toolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool call with required tool_choice, got none",
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

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// requiredToolCallWithReasoningEval verifies that constrained decoding for
// required tool calls does not constrain reasoning output.
type requiredToolCallWithReasoningEval struct {
	streaming bool
}

func (e *requiredToolCallWithReasoningEval) Name() string {
	return "required_tool_call_with_reasoning"
}

func (e *requiredToolCallWithReasoningEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *requiredToolCallWithReasoningEval) Streaming() bool             { return e.streaming }

func (e *requiredToolCallWithReasoningEval) Category() string {
	return toolCategory
}

func (e *requiredToolCallWithReasoningEval) Class() string {
	return ClassReasoning
}

func (e *requiredToolCallWithReasoningEval) Run(ctx context.Context, c *client.Client) Result {
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
		ToolChoice: "required",
	}

	var toolCalls []client.ToolCall
	var reasoningContent string

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
		reasoningContent = result.ReasoningContent
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
		reasoningContent = resp.Choices[0].Message.ReasoningContent
	}

	// Verify we got a tool call
	if len(toolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool call with required tool_choice, got none",
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

	// Verify reasoning content is present (not constrained by tool decoding)
	if strings.TrimSpace(reasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content is empty - constrained decoding may be suppressing reasoning",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// complexSchemaToolCallEval tests tool calling with a deeply nested, real-world schema.
// Uses an event catering tool that requires nested objects, arrays of objects,
// enums, and optional fields - common patterns in production APIs.
type complexSchemaToolCallEval struct {
	streaming bool
}

func (e *complexSchemaToolCallEval) Name() string {
	return "complex_schema_tool_call"
}

func (e *complexSchemaToolCallEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *complexSchemaToolCallEval) Streaming() bool             { return e.streaming }

func (e *complexSchemaToolCallEval) Category() string {
	return toolCategory
}

func (e *complexSchemaToolCallEval) Class() string {
	return ClassStandard
}

// complexCateringSchema defines a realistic event catering tool schema with:
// - Nested objects (venue with address, contact info)
// - Arrays of complex objects (guests with dietary info)
// - Enums (event_type, dietary_restriction, meal_preference)
// - Optional fields (notes, accessibility_requirements)
// - Required field validation at multiple nesting levels
const complexCateringSchema = `{
	"type": "object",
	"properties": {
		"event": {
			"type": "object",
			"description": "Event details",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name or title of the event"
				},
				"event_type": {
					"type": "string",
					"enum": ["breakfast", "lunch", "dinner", "reception", "all_day"],
					"description": "Type of catering event"
				},
				"date": {
					"type": "string",
					"description": "Event date in ISO 8601 format (YYYY-MM-DD)"
				},
				"start_time": {
					"type": "string",
					"description": "Start time in 24-hour format (HH:MM)"
				},
				"duration_hours": {
					"type": "number",
					"description": "Expected duration in hours"
				}
			},
			"required": ["name", "event_type", "date"]
		},
		"venue": {
			"type": "object",
			"description": "Event venue information",
			"properties": {
				"name": {
					"type": "string",
					"description": "Venue name"
				},
				"address": {
					"type": "object",
					"properties": {
						"street": {
							"type": "string"
						},
						"city": {
							"type": "string"
						},
						"state": {
							"type": "string"
						},
						"postal_code": {
							"type": "string"
						}
					},
					"required": ["street", "city", "state"]
				},
				"contact": {
					"type": "object",
					"properties": {
						"name": {
							"type": "string"
						},
						"phone": {
							"type": "string"
						},
						"email": {
							"type": "string"
						}
					}
				},
				"accessibility_requirements": {
					"type": "array",
					"items": {
						"type": "string"
					},
					"description": "Special accessibility needs (wheelchair access, etc.)"
				}
			},
			"required": ["name", "address"]
		},
		"guests": {
			"type": "array",
			"description": "List of guests with their dietary requirements",
			"items": {
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Guest name"
					},
					"dietary_restrictions": {
						"type": "array",
						"items": {
							"type": "string",
							"enum": ["vegetarian", "vegan", "gluten_free", "dairy_free", "nut_allergy", "shellfish_allergy", "halal", "kosher", "none"]
						},
						"description": "List of dietary restrictions"
					},
					"meal_preference": {
						"type": "string",
						"enum": ["standard", "light", "hearty"],
						"description": "Portion size preference"
					}
				},
				"required": ["name", "dietary_restrictions"]
			}
		},
		"budget": {
			"type": "object",
			"properties": {
				"total_amount": {
					"type": "number",
					"description": "Total budget in dollars"
				},
				"currency": {
					"type": "string",
					"description": "Currency code (e.g., USD)"
				},
				"includes_gratuity": {
					"type": "boolean",
					"description": "Whether the budget includes gratuity"
				}
			},
			"required": ["total_amount"]
		},
		"notes": {
			"type": "string",
			"description": "Additional notes or special requests"
		}
	},
	"required": ["event", "venue", "guests", "budget"]
}`

func (e *complexSchemaToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{
				Role: "user",
				Content: `I need to plan a corporate lunch for our team next Friday (2025-01-24) at the Riverside Conference Center,
located at 123 Main Street, Portland, Oregon 97201. The event coordinator is Jamie Chen (jamie@riverside.com).

We have 3 attendees:
- Alex Kim is vegetarian and prefers lighter meals
- Jordan Patel has a severe nut allergy
- Sam Wilson has no dietary restrictions and likes hearty portions

Our budget is $450 USD including tip. Please note that we'll need the room set up boardroom style.`,
			},
		},
		Tools: []client.Tool{
			{
				Type: "function",
				Function: client.ToolFunction{
					Name:        "create_catering_request",
					Description: "Create a catering request for an event with guest dietary requirements",
					Parameters:  json.RawMessage(complexCateringSchema),
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

	// Verify we got a tool call
	if len(toolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool call, got none",
		}
	}

	tc := toolCalls[0]

	// Verify tool name
	if tc.Function.Name != "create_catering_request" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool name 'create_catering_request', got '" + tc.Function.Name + "'",
		}
	}

	// Parse and validate the complex nested structure
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "tool arguments are not valid JSON: " + err.Error(),
		}
	}

	// Validate top-level required fields exist
	for _, field := range []string{"event", "venue", "guests", "budget"} {
		if _, ok := args[field]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "missing required top-level field: " + field,
			}
		}
	}

	// Validate nested event object
	event, ok := args["event"].(map[string]any)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "event field is not an object",
		}
	}
	for _, field := range []string{"name", "event_type", "date"} {
		if _, ok := event[field]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "event missing required field: " + field,
			}
		}
	}

	// Validate nested venue object with address
	venue, ok := args["venue"].(map[string]any)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "venue field is not an object",
		}
	}
	if _, ok := venue["name"]; !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "venue missing required field: name",
		}
	}
	address, ok := venue["address"].(map[string]any)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "venue.address field is not an object",
		}
	}
	for _, field := range []string{"street", "city", "state"} {
		if _, ok := address[field]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "venue.address missing required field: " + field,
			}
		}
	}

	// Validate guests array
	guests, ok := args["guests"].([]any)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "guests field is not an array",
		}
	}
	if len(guests) != 3 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected 3 guests, got " + string(rune('0'+len(guests))),
		}
	}

	// Validate each guest has required fields
	for i, g := range guests {
		guest, ok := g.(map[string]any)
		if !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "guest " + string(rune('0'+i)) + " is not an object",
			}
		}
		if _, ok := guest["name"]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "guest " + string(rune('0'+i)) + " missing required field: name",
			}
		}
		if _, ok := guest["dietary_restrictions"]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "guest " + string(rune('0'+i)) + " missing required field: dietary_restrictions",
			}
		}
	}

	// Validate budget object
	budget, ok := args["budget"].(map[string]any)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "budget field is not an object",
		}
	}
	if _, ok := budget["total_amount"]; !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "budget missing required field: total_amount",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// codeGenerationToolCallEval tests tool calling with long-form text output.
// Asks the model to generate a non-trivial piece of code to verify correct
// handling of longer argument values.
type codeGenerationToolCallEval struct {
	streaming bool
}

func (e *codeGenerationToolCallEval) Name() string {
	return "code_generation_tool_call"
}

func (e *codeGenerationToolCallEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *codeGenerationToolCallEval) Streaming() bool             { return e.streaming }

func (e *codeGenerationToolCallEval) Category() string {
	return toolCategory
}

func (e *codeGenerationToolCallEval) Class() string {
	return ClassStandard
}

const codeGenerationSchema = `{
	"type": "object",
	"properties": {
		"language": {
			"type": "string",
			"description": "Programming language of the generated code"
		},
		"filename": {
			"type": "string",
			"description": "Suggested filename for the code"
		},
		"description": {
			"type": "string",
			"description": "Brief description of what the code does"
		},
		"code": {
			"type": "string",
			"description": "The complete, working code implementation"
		},
		"usage_example": {
			"type": "string",
			"description": "Example showing how to use the code"
		}
	},
	"required": ["language", "filename", "description", "code", "usage_example"]
}`

func (e *codeGenerationToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{
				Role: "user",
				Content: `Generate a Python implementation of a token bucket rate limiter.

Requirements:
- Class named TokenBucket with configurable capacity and refill_rate (tokens per second)
- Method acquire(tokens=1) that returns True if tokens are available, False otherwise
- Method wait_for_token(tokens=1) that blocks until tokens are available (use time.sleep)
- Thread-safe using a lock
- Include docstrings for the class and methods

The code should be complete and production-ready.`,
			},
		},
		Tools: []client.Tool{
			{
				Type: "function",
				Function: client.ToolFunction{
					Name:        "save_code",
					Description: "Save generated code to a file",
					Parameters:  json.RawMessage(codeGenerationSchema),
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

	// Verify we got a tool call
	if len(toolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool call, got none",
		}
	}

	tc := toolCalls[0]

	// Verify tool name
	if tc.Function.Name != "save_code" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "expected tool name 'save_code', got '" + tc.Function.Name + "'",
		}
	}

	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "tool arguments are not valid JSON: " + err.Error(),
		}
	}

	// Validate required fields
	for _, field := range []string{"language", "filename", "description", "code", "usage_example"} {
		if _, ok := args[field]; !ok {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "missing required field: " + field,
			}
		}
	}

	// Validate code is non-trivial (at least 500 chars for a proper implementation)
	code, ok := args["code"].(string)
	if !ok {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "code field is not a string",
		}
	}

	if len(code) < 500 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "code appears incomplete (less than 500 characters)",
		}
	}

	// Check for key implementation markers
	requiredPatterns := []string{
		"class TokenBucket",
		"def acquire",
		"def wait_for_token",
		"Lock",
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(code, pattern) {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "code missing expected pattern: " + pattern,
			}
		}
	}

	// Validate usage_example is non-empty
	example, ok := args["usage_example"].(string)
	if !ok || len(strings.TrimSpace(example)) < 20 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "usage_example is missing or too short",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
