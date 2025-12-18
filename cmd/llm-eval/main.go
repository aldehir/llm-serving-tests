package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aldehir/llm-evals/internal/client"
	"github.com/aldehir/llm-evals/internal/eval"
	evallog "github.com/aldehir/llm-evals/internal/log"
)

var (
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
	verbose bool
	filter  string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "llm-eval",
	Short: "LLM inference server evaluation suite",
	Long:  "A tool for testing LLM inference server implementations against OpenAI-compatible APIs.",
	RunE:  runEvals,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available evals",
	Long:  "List all available evals that can be filtered with --filter.",
	Run:   listEvals,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Server base URL (required for run)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (optional)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to test (required for run)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full request/response for all evals")
	rootCmd.PersistentFlags().StringVar(&filter, "filter", "", "Run only evals matching pattern")

	rootCmd.AddCommand(listCmd)
}

func runEvals(cmd *cobra.Command, args []string) error {
	if baseURL == "" {
		return fmt.Errorf("--base-url is required")
	}

	if model == "" {
		return fmt.Errorf("--model is required")
	}

	// Initialize logger
	logger, err := evallog.New()
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
	})

	// Run evals
	runner := eval.NewRunner(c, eval.RunnerConfig{
		Verbose: verbose,
		Filter:  filter,
		Logger:  logger,
	})

	fmt.Println("LLM Eval Suite")
	fmt.Println("==============")
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

func listEvals(cmd *cobra.Command, args []string) {
	evals := eval.AllEvals()

	currentCategory := ""
	for _, e := range evals {
		// Apply filter if specified
		if filter != "" && !strings.Contains(e.Name(), filter) {
			continue
		}

		// Print category header
		if e.Category() != currentCategory {
			if currentCategory != "" {
				fmt.Println()
			}
			currentCategory = e.Category()
			fmt.Println(currentCategory)
		}

		fmt.Printf("  %s\n", e.Name())
	}
}
