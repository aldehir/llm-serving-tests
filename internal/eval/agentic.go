package eval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aldehir/llm-evals/internal/client"
)

const agenticCategory = "Agentic"

// agenticEvals returns all agentic (multi-turn) evals.
func agenticEvals() []Eval {
	return []Eval{
		&agenticToolCallEval{},
		&agenticReasoningInTemplateEval{},
		&agenticReasoningNotInUserTemplateEval{},
	}
}

// weatherTool is the tool definition used in agentic evals.
var weatherTool = client.Tool{
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
}

// agenticToolCallEval tests a multi-turn tool call flow with interleaved reasoning.
type agenticToolCallEval struct{}

func (e *agenticToolCallEval) Name() string {
	return "agentic_tool_call"
}

func (e *agenticToolCallEval) Category() string {
	return agenticCategory
}

func (e *agenticToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	// Turn 1: User asks question requiring tool use
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	resp1, err := c.ChatCompletion(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1 request failed: " + err.Error(),
		}
	}

	if len(resp1.Choices) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: no choices in response",
		}
	}

	msg1 := resp1.Choices[0].Message

	// Verify we got a tool call
	if len(msg1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool call, got none",
		}
	}

	tc := msg1.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool 'get_weather', got '" + tc.Function.Name + "'",
		}
	}

	// Turn 2: Send assistant message with reasoning + tool result
	req2 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
			{
				Role:             "assistant",
				ReasoningContent: msg1.ReasoningContent,
				ToolCalls:        msg1.ToolCalls,
			},
			{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    `{"temperature": 72, "conditions": "sunny"}`,
			},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	resp2, err := c.ChatCompletion(ctx, req2)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2 request failed: " + err.Error(),
		}
	}

	if len(resp2.Choices) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2: no choices in response",
		}
	}

	msg2 := resp2.Choices[0].Message

	// Verify we got a final response with content
	if strings.TrimSpace(msg2.Content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2: expected content in response, got empty",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// agenticReasoningInTemplateEval verifies reasoning appears in the template
// when messages end with a tool result after an assistant message.
type agenticReasoningInTemplateEval struct{}

func (e *agenticReasoningInTemplateEval) Name() string {
	return "agentic_reasoning_in_template"
}

func (e *agenticReasoningInTemplateEval) Category() string {
	return agenticCategory
}

func (e *agenticReasoningInTemplateEval) Run(ctx context.Context, c *client.Client) Result {
	// First, get reasoning content from the model
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	resp1, err := c.ChatCompletion(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "initial request failed: " + err.Error(),
		}
	}

	if len(resp1.Choices) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "no choices in response",
		}
	}

	msg1 := resp1.Choices[0].Message

	// Need reasoning content for this test
	if strings.TrimSpace(msg1.ReasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return reasoning_content, cannot test template",
		}
	}

	if len(msg1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return tool calls, cannot test template",
		}
	}

	tc := msg1.ToolCalls[0]

	// Build messages ending with tool result (should include reasoning in template)
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in San Francisco?"},
		{
			Role:             "assistant",
			ReasoningContent: msg1.ReasoningContent,
			ToolCalls:        msg1.ToolCalls,
		},
		{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    `{"temperature": 72, "conditions": "sunny"}`,
		},
	}

	// Call /apply-template
	prompt, err := c.ApplyTemplate(ctx, messages)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "/apply-template failed: " + err.Error(),
		}
	}

	// Verify reasoning content appears in the prompt
	if !strings.Contains(prompt, msg1.ReasoningContent) {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content not found in rendered template",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// agenticReasoningNotInUserTemplateEval verifies reasoning does NOT appear
// when messages end with a user message.
type agenticReasoningNotInUserTemplateEval struct{}

func (e *agenticReasoningNotInUserTemplateEval) Name() string {
	return "agentic_reasoning_not_in_user_template"
}

func (e *agenticReasoningNotInUserTemplateEval) Category() string {
	return agenticCategory
}

func (e *agenticReasoningNotInUserTemplateEval) Run(ctx context.Context, c *client.Client) Result {
	// First, get reasoning content from the model
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	resp1, err := c.ChatCompletion(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "initial request failed: " + err.Error(),
		}
	}

	if len(resp1.Choices) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "no choices in response",
		}
	}

	msg1 := resp1.Choices[0].Message

	// Need reasoning content for this test
	if strings.TrimSpace(msg1.ReasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return reasoning_content, cannot test template",
		}
	}

	if len(msg1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return tool calls, cannot test template",
		}
	}

	tc := msg1.ToolCalls[0]

	// Build messages ending with USER message (reasoning should NOT appear)
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in San Francisco?"},
		{
			Role:             "assistant",
			ReasoningContent: msg1.ReasoningContent,
			ToolCalls:        msg1.ToolCalls,
		},
		{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    `{"temperature": 72, "conditions": "sunny"}`,
		},
		{Role: "user", Content: "Thanks, what about New York?"},
	}

	// Call /apply-template
	prompt, err := c.ApplyTemplate(ctx, messages)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "/apply-template failed: " + err.Error(),
		}
	}

	// Verify reasoning content does NOT appear in the prompt
	if strings.Contains(prompt, msg1.ReasoningContent) {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content found in template when it should not be (ends with user message)",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
