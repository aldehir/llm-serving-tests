package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aldehir/llm-evals/internal/client"
	"github.com/aldehir/llm-evals/internal/eval"
	evallog "github.com/aldehir/llm-evals/internal/log"
)

func main() {
	var (
		baseURL = flag.String("base-url", "", "Server base URL (required)")
		apiKey  = flag.String("api-key", "", "API key (optional)")
		model   = flag.String("model", "", "Model to test (required)")
		timeout = flag.Duration("timeout", 30*time.Second, "Request timeout")
		verbose = flag.Bool("verbose", false, "Show full request/response for all evals")
		filter  = flag.String("filter", "", "Run only evals matching pattern")
	)

	flag.Parse()

	if *baseURL == "" {
		fmt.Fprintln(os.Stderr, "error: --base-url is required")
		flag.Usage()
		os.Exit(1)
	}

	if *model == "" {
		fmt.Fprintln(os.Stderr, "error: --model is required")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger, err := evallog.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Initialize client
	c := client.New(client.Config{
		BaseURL: *baseURL,
		APIKey:  *apiKey,
		Model:   *model,
		Timeout: *timeout,
		Logger:  logger,
	})

	// Run evals
	runner := eval.NewRunner(c, eval.RunnerConfig{
		Verbose: *verbose,
		Filter:  *filter,
		Logger:  logger,
	})

	fmt.Println("LLM Eval Suite")
	fmt.Println("==============")
	fmt.Printf("Server: %s\n", *baseURL)
	fmt.Printf("Model: %s\n", *model)
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
}
