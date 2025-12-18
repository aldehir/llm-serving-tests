package client

import "encoding/json"

// ChatCompletionRequest represents a chat completion request.
type ChatCompletionRequest struct {
	Model             string          `json:"model"`
	Messages          []Message       `json:"messages"`
	Tools             []Tool          `json:"tools,omitempty"`
	ToolChoice        any             `json:"tool_choice,omitempty"`
	ParallelToolCalls bool            `json:"parallel_tool_calls,omitempty"`
	ResponseFormat    *ResponseFormat `json:"response_format,omitempty"`
	Stream            bool            `json:"stream,omitempty"`
	StreamOptions     *StreamOptions  `json:"stream_options,omitempty"`
	MaxTokens         int             `json:"max_tokens,omitempty"`

	// Extra contains additional fields to include in the request JSON.
	// These are flattened into the root of the request object.
	Extra map[string]any `json:"-"`
}

// MarshalJSON implements custom JSON marshaling to flatten Extra fields.
func (r ChatCompletionRequest) MarshalJSON() ([]byte, error) {
	// Create a map with all the standard fields
	m := make(map[string]any)

	m["model"] = r.Model
	m["messages"] = r.Messages

	if len(r.Tools) > 0 {
		m["tools"] = r.Tools
	}
	if r.ToolChoice != nil {
		m["tool_choice"] = r.ToolChoice
	}
	if r.ParallelToolCalls {
		m["parallel_tool_calls"] = r.ParallelToolCalls
	}
	if r.ResponseFormat != nil {
		m["response_format"] = r.ResponseFormat
	}
	if r.Stream {
		m["stream"] = r.Stream
	}
	if r.StreamOptions != nil {
		m["stream_options"] = r.StreamOptions
	}
	if r.MaxTokens > 0 {
		m["max_tokens"] = r.MaxTokens
	}

	// Merge extra fields (they can override standard fields if needed)
	for k, v := range r.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

// Message represents a chat message.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// Tool represents a function tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function definition.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall represents a tool call in a response.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction represents the function call details.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseFormat specifies the format of the response.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema defines a JSON schema for structured output.
type JSONSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema"`
	Strict      bool            `json:"strict,omitempty"`
}

// StreamOptions configures streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatCompletionResponse represents a non-streaming chat completion response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// ResponseMessage represents the message in a response.
type ResponseMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk represents a streaming response chunk.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ChunkChoice represents a choice in a streaming chunk.
type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

// ChunkDelta represents the delta content in a streaming chunk.
type ChunkDelta struct {
	Role             string          `json:"role,omitempty"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta represents a partial tool call in a streaming chunk.
type ToolCallDelta struct {
	Index    int                   `json:"index"`
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function ToolCallFunctionDelta `json:"function,omitempty"`
}

// ToolCallFunctionDelta represents partial function call details.
type ToolCallFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ApplyTemplateRequest represents a request to the /apply-template endpoint.
type ApplyTemplateRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ApplyTemplateResponse represents a response from the /apply-template endpoint.
type ApplyTemplateResponse struct {
	Prompt string `json:"prompt"`
}
