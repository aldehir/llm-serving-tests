package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aldehir/llm-serving-tests/internal/client"
	"github.com/aldehir/llm-serving-tests/internal/eval"
	evallog "github.com/aldehir/llm-serving-tests/internal/log"
)

var (
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
	verbose bool
	filter  string
	class   string
	extra   []string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "llm-serve-test",
	Short: "LLM inference server test suite",
	Long:  "A tool for testing LLM inference server implementations against OpenAI-compatible APIs.",
	RunE:  runEvals,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tests",
	Long:  "List all available tests that can be filtered with --filter.",
	Run:   listTests,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Server base URL (required for run)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (optional)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to test (required for run)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full request/response for all tests")
	rootCmd.PersistentFlags().StringVar(&filter, "filter", "", "Run only tests matching pattern")
	rootCmd.PersistentFlags().StringVar(&class, "class", "", "Run only tests of specified class (standard, reasoning, interleaved)")
	rootCmd.PersistentFlags().StringArrayVarP(&extra, "extra", "e", nil, "Extra request field (key=value or key:=json), can be repeated")

	rootCmd.AddCommand(listCmd)
}

func runEvals(cmd *cobra.Command, args []string) error {
	if baseURL == "" {
		return fmt.Errorf("--base-url is required")
	}

	if model == "" {
		return fmt.Errorf("--model is required")
	}

	// Validate class if specified
	if class != "" {
		validClasses := eval.AllClasses()
		valid := false
		for _, c := range validClasses {
			if class == c {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid --class %q (valid: %s)", class, strings.Join(validClasses, ", "))
		}
	}

	// Parse extra fields
	extraFields, err := parseExtraFields(extra)
	if err != nil {
		return fmt.Errorf("invalid --extra flag: %w", err)
	}

	// Initialize logger
	logger, err := evallog.New(model)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Close()

	// Initialize client
	c := client.New(client.Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: timeout,
		Logger:  logger,
		Extra:   extraFields,
	})

	// Run evals
	runner := eval.NewRunner(c, eval.RunnerConfig{
		Verbose: verbose,
		Filter:  filter,
		Class:   class,
		Logger:  logger,
	})

	fmt.Println("LLM Serving Tests")
	fmt.Println("=================")
	fmt.Printf("Server: %s\n", baseURL)
	fmt.Printf("Model: %s\n", model)
	fmt.Println()

	results := runner.Run()

	// Print summary
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}

	fmt.Printf("\nResults: %d/%d passed\n", passed, len(results))
	fmt.Printf("\nLogs written to: %s\n", logger.Dir())

	if passed < len(results) {
		os.Exit(1)
	}

	return nil
}

func listTests(cmd *cobra.Command, args []string) {
	tests := eval.AllEvals()

	currentCategory := ""
	for _, t := range tests {
		// Apply name filter if specified
		if filter != "" && !strings.Contains(t.Name(), filter) {
			continue
		}

		// Apply class filter if specified
		if !eval.ClassMatches(t.Class(), class) {
			continue
		}

		// Print category header
		if t.Category() != currentCategory {
			if currentCategory != "" {
				fmt.Println()
			}
			currentCategory = t.Category()
			fmt.Println(currentCategory)
		}

		fmt.Printf("  %-45s [%s]\n", t.Name(), t.Class())
	}
}

// parseExtraFields parses --extra flags into a map.
// Supports two formats:
//   - key=value  (value is a string)
//   - key:=json  (value is parsed as JSON)
func parseExtraFields(extras []string) (map[string]any, error) {
	if len(extras) == 0 {
		return nil, nil
	}

	result := make(map[string]any)
	for _, e := range extras {
		// Check for JSON value format (key:=value)
		if idx := strings.Index(e, ":="); idx > 0 {
			key := e[:idx]
			value := e[idx+2:]
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err != nil {
				return nil, fmt.Errorf("invalid JSON for %q: %w", key, err)
			}
			result[key] = parsed
			continue
		}

		// Check for string value format (key=value)
		if idx := strings.Index(e, "="); idx > 0 {
			key := e[:idx]
			value := e[idx+1:]
			result[key] = value
			continue
		}

		return nil, fmt.Errorf("invalid format %q (expected key=value or key:=json)", e)
	}

	return result, nil
}
