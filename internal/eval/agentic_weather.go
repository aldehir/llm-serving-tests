package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

// cityWeatherResponses maps city names (lowercase) to canned weather JSON responses.
var cityWeatherResponses = map[string]string{
	"tokyo": `{"city": "Tokyo", "temperature_f": 54, "temperature_c": 12, "conditions": "partly cloudy", "humidity": 55, "wind_mph": 8}`,
	"london": `{"city": "London", "temperature_f": 48, "temperature_c": 9, "conditions": "overcast with light drizzle", "humidity": 82, "wind_mph": 12}`,
	"new york": `{"city": "New York", "temperature_f": 35, "temperature_c": 2, "conditions": "clear skies", "humidity": 40, "wind_mph": 15}`,
	"paris": `{"city": "Paris", "temperature_f": 50, "temperature_c": 10, "conditions": "foggy", "humidity": 78, "wind_mph": 5}`,
	"sydney": `{"city": "Sydney", "temperature_f": 79, "temperature_c": 26, "conditions": "sunny", "humidity": 60, "wind_mph": 10}`,
	"cairo": `{"city": "Cairo", "temperature_f": 88, "temperature_c": 31, "conditions": "hot and dry", "humidity": 20, "wind_mph": 7}`,
	"mumbai": `{"city": "Mumbai", "temperature_f": 91, "temperature_c": 33, "conditions": "humid and hazy", "humidity": 75, "wind_mph": 6}`,
	"são paulo": `{"city": "São Paulo", "temperature_f": 82, "temperature_c": 28, "conditions": "thunderstorms", "humidity": 85, "wind_mph": 9}`,
	"sao paulo": `{"city": "São Paulo", "temperature_f": 82, "temperature_c": 28, "conditions": "thunderstorms", "humidity": 85, "wind_mph": 9}`,
	"moscow": `{"city": "Moscow", "temperature_f": 18, "temperature_c": -8, "conditions": "heavy snow", "humidity": 90, "wind_mph": 20}`,
	"dubai": `{"city": "Dubai", "temperature_f": 95, "temperature_c": 35, "conditions": "clear and hot", "humidity": 30, "wind_mph": 4}`,
	"toronto": `{"city": "Toronto", "temperature_f": 28, "temperature_c": -2, "conditions": "light snow", "humidity": 70, "wind_mph": 18}`,
	"berlin": `{"city": "Berlin", "temperature_f": 41, "temperature_c": 5, "conditions": "cloudy", "humidity": 65, "wind_mph": 11}`,
	"singapore": `{"city": "Singapore", "temperature_f": 88, "temperature_c": 31, "conditions": "partly cloudy with afternoon showers", "humidity": 80, "wind_mph": 5}`,
	"nairobi": `{"city": "Nairobi", "temperature_f": 72, "temperature_c": 22, "conditions": "mild and pleasant", "humidity": 50, "wind_mph": 8}`,
	"mexico city": `{"city": "Mexico City", "temperature_f": 68, "temperature_c": 20, "conditions": "partly sunny", "humidity": 45, "wind_mph": 6}`,
}

// lookupWeatherResponse returns the canned response for a city, matching loosely.
func lookupWeatherResponse(location string) string {
	loc := strings.ToLower(location)
	for city, resp := range cityWeatherResponses {
		if strings.Contains(loc, city) {
			return resp
		}
	}
	return `{"error": "weather data not available for this location"}`
}

// cities whose names we expect to find in the final summary.
var expectedCities = []string{
	"Tokyo", "London", "New York", "Paris", "Sydney",
	"Cairo", "Mumbai", "Paulo", "Moscow", "Dubai",
	"Toronto", "Berlin", "Singapore", "Nairobi", "Mexico",
}

// agenticMultiCityWeatherEval tests a long multi-turn agentic conversation
// where the model must call get_weather for 15 different cities, receive
// canned responses, and produce a summary covering all of them.
type agenticMultiCityWeatherEval struct {
	streaming         bool
	parallelToolCalls bool
}

func (e *agenticMultiCityWeatherEval) Name() string {
	if e.parallelToolCalls {
		return "agentic_multi_city_weather_parallel"
	}
	return "agentic_multi_city_weather"
}

func (e *agenticMultiCityWeatherEval) Category() string {
	return agenticCategory
}

func (e *agenticMultiCityWeatherEval) Class() string {
	return ClassStandard
}

func (e *agenticMultiCityWeatherEval) SetStreaming(streaming bool) { e.streaming = streaming }
func (e *agenticMultiCityWeatherEval) Streaming() bool             { return e.streaming }

func (e *agenticMultiCityWeatherEval) IsDefaultDisabled() bool {
	return true
}

func (e *agenticMultiCityWeatherEval) Run(ctx context.Context, c *client.Client) Result {
	const maxIterations = 30

	userPrompt := `I need you to check the weather in 15 cities around the world and give me a summary of each.

Use the get_weather tool for each of these cities:
1. Tokyo
2. London
3. New York
4. Paris
5. Sydney
6. Cairo
7. Mumbai
8. São Paulo
9. Moscow
10. Dubai
11. Toronto
12. Berlin
13. Singapore
14. Nairobi
15. Mexico City

After you have retrieved the weather for ALL 15 cities, provide a summary listing each city with its temperature and conditions. You may call the tool for multiple cities in parallel.`

	messages := []client.Message{
		{Role: "user", Content: userPrompt},
	}

	totalToolCalls := 0

	for i := range maxIterations {
		req := client.ChatCompletionRequest{
			Messages:          messages,
			Tools:             []client.Tool{weatherTool},
			ToolChoice:        "auto",
			ParallelToolCalls: e.parallelToolCalls,
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

		// No tool calls means the model is done
		if len(toolCalls) == 0 {
			return e.validateFinalResponse(content, totalToolCalls)
		}

		totalToolCalls += len(toolCalls)

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
			var response string
			if tc.Function.Name == "get_weather" {
				response = lookupWeatherResponse(tc.Function.Arguments)
			} else {
				response = `{"error": "unknown tool"}`
			}
			messages = append(messages, client.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    response,
			})
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   false,
		Message:  fmt.Sprintf("reached max iterations (%d) without completing weather summary", maxIterations),
	}
}

func (e *agenticMultiCityWeatherEval) validateFinalResponse(content string, totalToolCalls int) Result {
	if strings.TrimSpace(content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "final response is empty",
		}
	}

	// Must have made at least 15 total tool calls (one per city)
	if totalToolCalls < 15 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("model made only %d tool calls, expected at least 15", totalToolCalls),
		}
	}

	// Check that the final summary mentions each city
	contentLower := strings.ToLower(content)
	missing := []string{}
	for _, city := range expectedCities {
		if !strings.Contains(contentLower, strings.ToLower(city)) {
			missing = append(missing, city)
		}
	}

	if len(missing) > 2 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("final summary missing %d cities: %s", len(missing), strings.Join(missing, ", ")),
		}
	}

	// Sanity check: response should be substantial
	if len(content) < 300 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  fmt.Sprintf("final response too short (%d chars, expected at least 300 for 15-city summary)", len(content)),
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
