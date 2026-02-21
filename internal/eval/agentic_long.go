package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

// Incident investigation tools.
var (
	searchLogsTool = client.Tool{
		Type: "function",
		Function: client.ToolFunction{
			Name:        "search_logs",
			Description: "Search application logs for a service within a time range",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service": {
						"type": "string",
						"description": "The service name to search logs for"
					},
					"query": {
						"type": "string",
						"description": "Search query or filter expression"
					},
					"start_time": {
						"type": "string",
						"description": "Start time in ISO 8601 format"
					},
					"end_time": {
						"type": "string",
						"description": "End time in ISO 8601 format"
					}
				},
				"required": ["service"]
			}`),
		},
	}

	getServiceStatusTool = client.Tool{
		Type: "function",
		Function: client.ToolFunction{
			Name:        "get_service_status",
			Description: "Get the current health status and dependency information for a service",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service": {
						"type": "string",
						"description": "The service name"
					}
				},
				"required": ["service"]
			}`),
		},
	}

	queryMetricsTool = client.Tool{
		Type: "function",
		Function: client.ToolFunction{
			Name:        "query_metrics",
			Description: "Query time-series metrics for a service",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service": {
						"type": "string",
						"description": "The service name"
					},
					"metric": {
						"type": "string",
						"description": "Metric name, e.g. error_rate, latency_p99, request_count"
					},
					"start_time": {
						"type": "string",
						"description": "Start time in ISO 8601 format"
					},
					"end_time": {
						"type": "string",
						"description": "End time in ISO 8601 format"
					}
				},
				"required": ["service", "metric"]
			}`),
		},
	}

	listRecentDeploymentsTool = client.Tool{
		Type: "function",
		Function: client.ToolFunction{
			Name:        "list_recent_deployments",
			Description: "List recent deployments for a service",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service": {
						"type": "string",
						"description": "The service name"
					},
					"limit": {
						"type": "integer",
						"description": "Maximum number of deployments to return"
					}
				},
				"required": ["service"]
			}`),
		},
	}

	getConfigTool = client.Tool{
		Type: "function",
		Function: client.ToolFunction{
			Name:        "get_config",
			Description: "Get the current runtime configuration for a service",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service": {
						"type": "string",
						"description": "The service name"
					},
					"section": {
						"type": "string",
						"description": "Optional config section to retrieve"
					}
				},
				"required": ["service"]
			}`),
		},
	}
)

// Fake tool responses for the incident investigation scenario.
const (
	searchLogsResponse = `{
		"results": [
			{
				"timestamp": "2024-01-15T14:35:12Z",
				"level": "ERROR",
				"service": "checkout-service",
				"message": "Payment processing failed: connection refused to payment-processor-v2.internal:8443",
				"trace_id": "abc-123-def"
			},
			{
				"timestamp": "2024-01-15T14:34:58Z",
				"level": "ERROR",
				"service": "checkout-service",
				"message": "HTTP 503 from upstream: payment-processor-v2.internal:8443 - connection refused",
				"trace_id": "abc-124-def"
			},
			{
				"timestamp": "2024-01-15T14:32:45Z",
				"level": "WARN",
				"service": "checkout-service",
				"message": "Feature flag evaluation: use_payment_v2 = true, routing to payment-processor-v2",
				"trace_id": "abc-125-def"
			},
			{
				"timestamp": "2024-01-15T14:32:30Z",
				"level": "INFO",
				"service": "checkout-service",
				"message": "Configuration reloaded: payment processor endpoint changed to payment-processor-v2.internal:8443",
				"trace_id": "abc-126-def"
			},
			{
				"timestamp": "2024-01-15T14:30:15Z",
				"level": "INFO",
				"service": "checkout-service",
				"message": "Deployment v2.14.0 rollout complete, feature flags refreshed",
				"trace_id": "abc-127-def"
			}
		],
		"total_count": 1847,
		"truncated": true
	}`

	getServiceStatusResponse = `{
		"service": "checkout-service",
		"status": "degraded",
		"uptime": "14d 3h 22m",
		"instances": {
			"total": 5,
			"healthy": 5,
			"unhealthy": 0
		},
		"dependencies": [
			{
				"name": "postgres-primary",
				"status": "healthy",
				"latency_ms": 2
			},
			{
				"name": "redis-cache",
				"status": "healthy",
				"latency_ms": 1
			},
			{
				"name": "payment-processor-v1",
				"status": "healthy",
				"latency_ms": 45
			},
			{
				"name": "payment-processor-v2",
				"status": "unreachable",
				"error": "connection refused: payment-processor-v2.internal:8443"
			},
			{
				"name": "inventory-service",
				"status": "healthy",
				"latency_ms": 12
			}
		],
		"error_rate_1m": 15.3,
		"last_deploy": "2024-01-15T14:30:00Z"
	}`

	queryMetricsResponse = `{
		"service": "checkout-service",
		"metric": "error_rate",
		"unit": "percent",
		"datapoints": [
			{"timestamp": "2024-01-15T14:00:00Z", "value": 0.1},
			{"timestamp": "2024-01-15T14:10:00Z", "value": 0.1},
			{"timestamp": "2024-01-15T14:20:00Z", "value": 0.1},
			{"timestamp": "2024-01-15T14:30:00Z", "value": 0.3},
			{"timestamp": "2024-01-15T14:32:00Z", "value": 8.7},
			{"timestamp": "2024-01-15T14:34:00Z", "value": 14.9},
			{"timestamp": "2024-01-15T14:36:00Z", "value": 15.3},
			{"timestamp": "2024-01-15T14:38:00Z", "value": 15.1},
			{"timestamp": "2024-01-15T14:40:00Z", "value": 15.3}
		]
	}`

	listRecentDeploymentsResponse = `{
		"service": "checkout-service",
		"deployments": [
			{
				"version": "v2.14.0",
				"deployed_at": "2024-01-15T14:30:00Z",
				"deployed_by": "ci-pipeline",
				"status": "completed",
				"changelog": "Enable payment processor v2 feature flag",
				"commit": "a1b2c3d",
				"rollback_available": true
			},
			{
				"version": "v2.13.2",
				"deployed_at": "2024-01-14T09:15:00Z",
				"deployed_by": "ci-pipeline",
				"status": "completed",
				"changelog": "Fix cart total rounding for JPY currency",
				"commit": "e4f5g6h",
				"rollback_available": true
			},
			{
				"version": "v2.13.1",
				"deployed_at": "2024-01-12T16:45:00Z",
				"deployed_by": "ci-pipeline",
				"status": "completed",
				"changelog": "Add retry logic for inventory checks",
				"commit": "i7j8k9l",
				"rollback_available": false
			}
		]
	}`

	getConfigResponse = `{
		"service": "checkout-service",
		"environment": "production",
		"config": {
			"payment": {
				"processor_version": "v2",
				"v1_endpoint": "payment-processor-v1.internal:8443",
				"v2_endpoint": "payment-processor-v2.internal:8443",
				"active_processor": "v2",
				"timeout_ms": 5000,
				"retry_count": 3
			},
			"feature_flags": {
				"use_payment_v2": true,
				"enable_new_cart_ui": false,
				"async_inventory_check": true
			},
			"rate_limits": {
				"checkout_per_minute": 1000,
				"payment_per_minute": 500
			}
		},
		"last_updated": "2024-01-15T14:30:00Z"
	}`
)

var incidentInvestigationTools = []client.Tool{
	searchLogsTool,
	getServiceStatusTool,
	queryMetricsTool,
	listRecentDeploymentsTool,
	getConfigTool,
}

// lookupIncidentToolResponse returns the fake response for a given tool name.
func lookupIncidentToolResponse(toolName string) string {
	switch toolName {
	case "search_logs":
		return searchLogsResponse
	case "get_service_status":
		return getServiceStatusResponse
	case "query_metrics":
		return queryMetricsResponse
	case "list_recent_deployments":
		return listRecentDeploymentsResponse
	case "get_config":
		return getConfigResponse
	default:
		return `{"error": "unknown tool"}`
	}
}

// agenticIncidentInvestigationEval tests a long multi-turn agentic conversation
// where the model investigates a production incident using multiple tools over
// several rounds of tool calls.
type agenticIncidentInvestigationEval struct {
	streaming bool
}

func (e *agenticIncidentInvestigationEval) Name() string {
	return "agentic_incident_investigation"
}

func (e *agenticIncidentInvestigationEval) Category() string {
	return agenticCategory
}

func (e *agenticIncidentInvestigationEval) Class() string {
	return ClassStandard
}

func (e *agenticIncidentInvestigationEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *agenticIncidentInvestigationEval) Streaming() bool             { return e.streaming }

func (e *agenticIncidentInvestigationEval) IsDefaultDisabled() bool {
	return true
}

func (e *agenticIncidentInvestigationEval) Run(ctx context.Context, c *client.Client) Result {
	const maxIterations = 25
	const minToolCallRounds = 3

	systemPrompt := `You are an experienced Site Reliability Engineer (SRE) investigating a production incident. ` +
		`You have access to tools for searching logs, checking service status, querying metrics, ` +
		`listing deployments, and viewing configuration. ` +
		`Investigate the issue systematically using these tools to gather evidence. ` +
		`Once you have enough information, provide a root cause analysis including: ` +
		`1. What is failing and the impact ` +
		`2. The root cause ` +
		`3. Recommended immediate fix ` +
		`4. Timeline of events`

	userPrompt := `URGENT: The checkout-service is returning HTTP 500 errors in production. ` +
		`Our monitoring shows the error rate spiked from 0.1% to approximately 15% starting at 14:32 UTC today. ` +
		`Customers are unable to complete purchases. Please investigate and determine the root cause.`

	messages := []client.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	toolCallRounds := 0

	for i := range maxIterations {
		req := client.ChatCompletionRequest{
			Messages:   messages,
			Tools:      incidentInvestigationTools,
			ToolChoice: "auto",
		}

		var content string
		var reasoningContent string
		var toolCalls []client.ToolCall

		if e.streaming {
			result, err := c.ChatCompletionStream(ctx, req)
			if err != nil {
				return Result{
					Name:     e.Name(),
					Category: e.Category(),
					Passed:   false,
					Message:  fmt.Sprintf("iteration %d: request failed: %s", i+1, err.Error()),
				}
			}
			content = result.Content
			reasoningContent = result.ReasoningContent
			toolCalls = result.ToolCalls
		} else {
			resp, err := c.ChatCompletion(ctx, req)
			if err != nil {
				return Result{
					Name:     e.Name(),
					Category: e.Category(),
					Passed:   false,
					Message:  fmt.Sprintf("iteration %d: request failed: %s", i+1, err.Error()),
				}
			}
			if len(resp.Choices) == 0 {
				return Result{
					Name:     e.Name(),
					Category: e.Category(),
					Passed:   false,
					Message:  fmt.Sprintf("iteration %d: no choices in response", i+1),
				}
			}
			content = resp.Choices[0].Message.Content
			reasoningContent = resp.Choices[0].Message.ReasoningContent
			toolCalls = resp.Choices[0].Message.ToolCalls
		}

		// No tool calls means the model is done investigating
		if len(toolCalls) == 0 {
			return e.validateFinalResponse(content, toolCallRounds)
		}

		toolCallRounds++

		// Append assistant message
		assistantMsg := client.Message{
			Role:             "assistant",
			ReasoningContent: reasoningContent,
			ToolCalls:        toolCalls,
		}
		if content != "" {
			assistantMsg.Content = content
		}
		messages = append(messages, assistantMsg)

		// Append tool responses for each tool call
		for _, tc := range toolCalls {
			messages = append(messages, client.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    lookupIncidentToolResponse(tc.Function.Name),
			})
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   false,
		Message:  fmt.Sprintf("reached max iterations (%d) without completing investigation", maxIterations),
	}
}

func (e *agenticIncidentInvestigationEval) validateFinalResponse(content string, toolCallRounds int) Result {
	if toolCallRounds < minToolCallRoundsIncident {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("model only used %d tool call round(s), expected at least %d", toolCallRounds, minToolCallRoundsIncident),
		}
	}

	if strings.TrimSpace(content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "final response is empty",
		}
	}

	if len(content) < 200 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("final response too short (%d chars, expected at least 200)", len(content)),
		}
	}

	// Check that the response mentions key investigation terms
	keywords := []string{"payment", "deploy", "feature", "error", "checkout"}
	contentLower := strings.ToLower(content)
	matched := 0
	for _, kw := range keywords {
		if strings.Contains(contentLower, kw) {
			matched++
		}
	}

	if matched < 3 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("final response only mentions %d/5 expected keywords (payment, deploy, feature, error, checkout); expected at least 3", matched),
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

const minToolCallRoundsIncident = 3
