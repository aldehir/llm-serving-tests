package eval

import (
	"context"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const reasoningCategory = "Reasoning"

// reasoningEvals returns all reasoning-related evals.
func reasoningEvals() []Eval {
	return []Eval{
		&reasoningPresentEval{},
		&reasoningNotLeakedEval{},
	}
}

// reasoningPresentEval verifies that reasoning_content is populated.
type reasoningPresentEval struct {
	streaming bool
}

func (e *reasoningPresentEval) Name() string {
	return "reasoning_present"
}

func (e *reasoningPresentEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *reasoningPresentEval) Streaming() bool             { return e.streaming }

func (e *reasoningPresentEval) Category() string {
	return reasoningCategory
}

func (e *reasoningPresentEval) Class() string {
	return ClassReasoning
}

func (e *reasoningPresentEval) Run(ctx context.Context, c *client.Client) Result {
	// Use a prompt that should trigger reasoning
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What is 15 * 27? Think step by step."},
		},
	}

	var reasoningContent string
	var content string

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
		reasoningContent = result.ReasoningContent
		content = result.Content
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
		reasoningContent = resp.Choices[0].Message.ReasoningContent
		content = resp.Choices[0].Message.Content
	}

	// Check that reasoning_content is not empty
	if strings.TrimSpace(reasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content is empty",
		}
	}

	// Also verify we got some content
	if strings.TrimSpace(content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "content is empty (expected final answer)",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// reasoningNotLeakedEval verifies that reasoning is not leaked into content.
type reasoningNotLeakedEval struct {
	streaming bool
}

func (e *reasoningNotLeakedEval) Name() string {
	return "reasoning_not_leaked"
}

func (e *reasoningNotLeakedEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *reasoningNotLeakedEval) Streaming() bool             { return e.streaming }

func (e *reasoningNotLeakedEval) Category() string {
	return reasoningCategory
}

func (e *reasoningNotLeakedEval) Class() string {
	return ClassReasoning
}

func (e *reasoningNotLeakedEval) Run(ctx context.Context, c *client.Client) Result {
	// Use a prompt that should trigger reasoning
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What is 15 * 27? Think step by step."},
		},
	}

	var reasoningContent string
	var content string

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
		reasoningContent = result.ReasoningContent
		content = result.Content
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
		reasoningContent = resp.Choices[0].Message.ReasoningContent
		content = resp.Choices[0].Message.Content
	}

	// If there's no reasoning content, we can't verify it's not leaked
	if strings.TrimSpace(reasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content is empty, cannot verify leak prevention",
		}
	}

	// Check for common reasoning indicators in content
	// These are heuristics - we look for phrases that typically appear in CoT
	leakIndicators := []string{
		"<think>",
		"</think>",
		"Let me think",
		"Step 1:",
		"First, I",
		"First, let me",
		"I need to",
		"To solve this",
	}

	contentLower := strings.ToLower(content)
	for _, indicator := range leakIndicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "content appears to contain reasoning (found: " + indicator + ")",
			}
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
