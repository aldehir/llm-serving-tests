package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// parseSSEStream parses an SSE stream and accumulates the result.
// Returns the accumulated result and raw chunk data for logging.
func parseSSEStream(r io.Reader) (*StreamResult, []byte, error) {
	result := &StreamResult{}
	toolCallBuilders := make(map[int]*toolCallBuilder)

	var rawChunks bytes.Buffer
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		rawChunks.WriteString(line)
		rawChunks.WriteString("\n")

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, rawChunks.Bytes(), fmt.Errorf("unmarshal chunk: %w", err)
		}

		result.Chunks = append(result.Chunks, chunk)

		// Accumulate usage if present
		if chunk.Usage != nil {
			result.Usage = chunk.Usage
		}

		// Process choices
		for _, choice := range chunk.Choices {
			delta := choice.Delta

			// Accumulate content
			result.Content += delta.Content
			result.ReasoningContent += delta.ReasoningContent

			// Accumulate tool calls
			for _, tc := range delta.ToolCalls {
				builder, ok := toolCallBuilders[tc.Index]
				if !ok {
					builder = &toolCallBuilder{}
					toolCallBuilders[tc.Index] = builder
				}
				builder.Accumulate(tc)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, rawChunks.Bytes(), fmt.Errorf("scan stream: %w", err)
	}

	// Build final tool calls
	for i := 0; i < len(toolCallBuilders); i++ {
		if builder, ok := toolCallBuilders[i]; ok {
			result.ToolCalls = append(result.ToolCalls, builder.Build())
		}
	}

	return result, rawChunks.Bytes(), nil
}

// CompletionStreamResult holds the result of a streaming /completions call.
type CompletionStreamResult struct {
	// Full concatenated output
	Text string
	// Individual token strings from each delta
	Tokens []string
	Usage  *Usage
	// Raw chunks for inspection
	Chunks []CompletionChunk
}

// parseCompletionSSEStream parses an SSE stream from /completions and accumulates the result.
func parseCompletionSSEStream(r io.Reader) (*CompletionStreamResult, []byte, error) {
	result := &CompletionStreamResult{}

	var rawChunks bytes.Buffer
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		rawChunks.WriteString(line)
		rawChunks.WriteString("\n")

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk CompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, rawChunks.Bytes(), fmt.Errorf("unmarshal chunk: %w", err)
		}

		result.Chunks = append(result.Chunks, chunk)

		if chunk.Usage != nil {
			result.Usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice.Text != "" {
				result.Text += choice.Text
				result.Tokens = append(result.Tokens, choice.Text)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, rawChunks.Bytes(), fmt.Errorf("scan stream: %w", err)
	}

	return result, rawChunks.Bytes(), nil
}

// toolCallBuilder accumulates tool call deltas.
type toolCallBuilder struct {
	id        string
	typ       string
	name      string
	arguments strings.Builder
}

func (b *toolCallBuilder) Accumulate(delta ToolCallDelta) {
	if delta.ID != "" {
		b.id = delta.ID
	}
	if delta.Type != "" {
		b.typ = delta.Type
	}
	if delta.Function.Name != "" {
		b.name = delta.Function.Name
	}
	b.arguments.WriteString(delta.Function.Arguments)
}

func (b *toolCallBuilder) Build() ToolCall {
	return ToolCall{
		ID:   b.id,
		Type: b.typ,
		Function: ToolCallFunction{
			Name:      b.name,
			Arguments: b.arguments.String(),
		},
	}
}
