package eval

import (
	"context"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const basicCategory = "Basic"

// basicEvals returns all basic sanity-check evals.
func basicEvals() []Eval {
	return []Eval{
		&chatCompletionEval{streaming: false},
		&chatCompletionEval{streaming: true},
	}
}

// chatCompletionEval verifies that the model returns non-empty content.
type chatCompletionEval struct {
	streaming bool
}

func (e *chatCompletionEval) Name() string {
	if e.streaming {
		return "chat_completion_streaming"
	}
	return "chat_completion"
}

func (e *chatCompletionEval) Category() string {
	return basicCategory
}

func (e *chatCompletionEval) Class() string {
	return ClassStandard
}

func (e *chatCompletionEval) Run(ctx context.Context, c *client.Client) Result {
	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "Say hello."},
		},
	}

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
		content = resp.Choices[0].Message.Content
	}

	if strings.TrimSpace(content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "content is empty",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
