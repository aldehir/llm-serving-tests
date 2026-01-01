package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const schemaCategory = "Structured Output"

// schemaEvals returns all JSON schema-related evals.
func schemaEvals() []Eval {
	return []Eval{
		&jsonSchemaEval{},
	}
}

// jsonSchemaEval verifies that structured output matches the requested schema.
type jsonSchemaEval struct {
	streaming bool
}

func (e *jsonSchemaEval) Name() string {
	return "json_schema"
}

func (e *jsonSchemaEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *jsonSchemaEval) Streaming() bool             { return e.streaming }

func (e *jsonSchemaEval) Category() string {
	return schemaCategory
}

func (e *jsonSchemaEval) Class() string {
	return ClassStandard
}

func (e *jsonSchemaEval) Run(ctx context.Context, c *client.Client) Result {
	// Define a simple schema for a person
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"},
			"occupation": {"type": "string"}
		},
		"required": ["name", "age", "occupation"],
		"additionalProperties": false
	}`)

	req := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "Generate a fictional person with a name, age, and occupation."},
		},
		ResponseFormat: &client.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &client.JSONSchema{
				Name:   "person",
				Schema: schema,
				Strict: true,
			},
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

	// Verify response is valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "response is not valid JSON: " + err.Error(),
		}
	}

	// Validate against schema
	if err := validatePersonSchema(parsed); err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  err.Error(),
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// validatePersonSchema validates the response against the expected person schema.
// This is a simple validation - for production use, consider a JSON Schema library.
func validatePersonSchema(data map[string]any) error {
	requiredFields := []string{"name", "age", "occupation"}

	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Check types
	if name, ok := data["name"]; ok {
		if reflect.TypeOf(name).Kind() != reflect.String {
			return fmt.Errorf("'name' must be a string, got %T", name)
		}
	}

	if age, ok := data["age"]; ok {
		// JSON numbers are float64 in Go
		switch v := age.(type) {
		case float64:
			if v != float64(int(v)) {
				return fmt.Errorf("'age' must be an integer, got float")
			}
		case int:
			// OK
		default:
			return fmt.Errorf("'age' must be an integer, got %T", age)
		}
	}

	if occupation, ok := data["occupation"]; ok {
		if reflect.TypeOf(occupation).Kind() != reflect.String {
			return fmt.Errorf("'occupation' must be a string, got %T", occupation)
		}
	}

	// Check for additional properties
	allowedFields := map[string]bool{
		"name":       true,
		"age":        true,
		"occupation": true,
	}
	for key := range data {
		if !allowedFields[key] {
			return fmt.Errorf("unexpected additional property: %s", key)
		}
	}

	return nil
}
