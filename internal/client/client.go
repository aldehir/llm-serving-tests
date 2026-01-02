package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	evallog "github.com/aldehir/llm-serving-tests/internal/log"
)

// Config configures the client.
type Config struct {
	BaseURL               string
	APIKey                string
	Model                 string
	Timeout               time.Duration
	ResponseHeaderTimeout time.Duration
	// Extra contains additional fields to include in all request payloads.
	Extra map[string]any
}

// Client is an OpenAI-compatible API client.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	extra      map[string]any
	httpClient *http.Client
	logger     evallog.RequestLogger
}

// New creates a new Client.
func New(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		extra:   cfg.Extra,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
			},
		},
	}
}

// WithLogger returns a new Client that uses the given logger.
// This creates a shallow copy that shares the underlying http.Client.
func (c *Client) WithLogger(logger evallog.RequestLogger) *Client {
	return &Client{
		baseURL:    c.baseURL,
		apiKey:     c.apiKey,
		model:      c.model,
		extra:      c.extra,
		httpClient: c.httpClient,
		logger:     logger,
	}
}

// applyExtra merges the client's extra fields into the request.
func (c *Client) applyExtra(req *ChatCompletionRequest) {
	if len(c.extra) == 0 {
		return
	}
	if req.Extra == nil {
		req.Extra = make(map[string]any)
	}
	for k, v := range c.extra {
		// Don't override if the request already has this key
		if _, exists := req.Extra[k]; !exists {
			req.Extra[k] = v
		}
	}
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}

// ChatCompletion performs a non-streaming chat completion.
func (c *Client) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	req.Model = c.model
	req.Stream = false
	c.applyExtra(&req)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Log request/response
	if c.logger != nil {
		c.logger.LogRequest(httpReq.Method, httpReq.URL.String(), reqBody)
		c.logger.LogResponse(resp.StatusCode, respBody)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// StreamResult holds the result of a streaming completion.
type StreamResult struct {
	// Accumulated content from all chunks
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	Usage            *Usage
	// Raw chunks for inspection
	Chunks []ChatCompletionChunk
}

// ChatCompletionStream performs a streaming chat completion.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (*StreamResult, error) {
	req.Model = c.model
	req.Stream = true
	if req.StreamOptions == nil {
		req.StreamOptions = &StreamOptions{IncludeUsage: true}
	}
	c.applyExtra(&req)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Log request
	if c.logger != nil {
		c.logger.LogRequest(httpReq.Method, httpReq.URL.String(), reqBody)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if c.logger != nil {
			c.logger.LogResponse(resp.StatusCode, body)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	result, rawChunks, err := parseSSEStream(resp.Body)
	if err != nil {
		return nil, err
	}

	// Log streamed response
	if c.logger != nil {
		c.logger.LogStreamResponse(resp.StatusCode, rawChunks)

		// Write JSONL for replay
		if len(result.Chunks) > 0 {
			var jsonlBuf bytes.Buffer
			for _, chunk := range result.Chunks {
				line, _ := json.Marshal(chunk)
				jsonlBuf.Write(line)
				jsonlBuf.WriteByte('\n')
			}
			c.logger.LogStreamChunks(jsonlBuf.Bytes())
		}
	}

	return result, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// ApplyTemplate calls the /apply-template endpoint to render messages into a prompt.
// This is specific to llama.cpp servers.
// Note: This endpoint is at the root, not under /v1.
func (c *Client) ApplyTemplate(ctx context.Context, messages []Message) (string, error) {
	reqData := ApplyTemplateRequest{
		Model:    c.model,
		Messages: messages,
	}

	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Strip /v1 suffix if present - apply-template is at the root
	baseURL := strings.TrimSuffix(c.baseURL, "/v1")

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/apply-template", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// Log request/response
	if c.logger != nil {
		c.logger.LogRequest(httpReq.Method, httpReq.URL.String(), reqBody)
		c.logger.LogResponse(resp.StatusCode, respBody)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result ApplyTemplateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Prompt, nil
}
