package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aldehir/llm-serving-tests/internal/client"
	evallog "github.com/aldehir/llm-serving-tests/internal/log"
)

// Eval class constants.
const (
	// ClassStandard is for evals that work on any OpenAI-compatible model.
	ClassStandard = "standard"
	// ClassReasoning is for evals that require reasoning_content extraction.
	ClassReasoning = "reasoning"
	// ClassInterleaved is for evals that require interleaved reasoning
	// (reasoning_content sent back in multi-turn conversations).
	ClassInterleaved = "interleaved"
)

// AllClasses returns all valid eval classes.
func AllClasses() []string {
	return []string{ClassStandard, ClassReasoning, ClassInterleaved}
}

// ClassMatches returns true if the eval's class is compatible with the requested class.
// Class hierarchy: interleaved > reasoning > standard
// - interleaved includes reasoning tests (interleaved models support reasoning)
// - reasoning includes only reasoning tests
// - standard includes only standard tests
func ClassMatches(evalClass, requestedClass string) bool {
	if requestedClass == "" {
		return true
	}
	if evalClass == requestedClass {
		return true
	}
	// interleaved class also runs reasoning tests
	if requestedClass == ClassInterleaved && evalClass == ClassReasoning {
		return true
	}
	return false
}

// Eval defines the interface for an evaluation.
type Eval interface {
	// Name returns the name of the eval.
	Name() string
	// Category returns the category (e.g., "Reasoning", "Tool Calling").
	Category() string
	// Class returns the model class required (e.g., "standard", "reasoning", "interleaved").
	Class() string
	// Run executes the eval and returns the result.
	Run(ctx context.Context, c *client.Client) Result
}

// Result represents the result of an eval.
type Result struct {
	Name     string
	Category string
	Passed   bool
	Message  string
	Duration time.Duration
}

// RunnerConfig configures the runner.
type RunnerConfig struct {
	Verbose bool
	Filter  string
	Class   string
	Logger  *evallog.Logger
}

// Runner executes evals.
type Runner struct {
	client *client.Client
	config RunnerConfig
	evals  []Eval
}

// NewRunner creates a new Runner with all registered evals.
func NewRunner(c *client.Client, cfg RunnerConfig) *Runner {
	return &Runner{
		client: c,
		config: cfg,
		evals:  AllEvals(),
	}
}

// Run executes all evals and returns results.
func (r *Runner) Run() []Result {
	var results []Result
	currentCategory := ""

	for _, e := range r.evals {
		// Apply name filter
		if r.config.Filter != "" && !strings.Contains(e.Name(), r.config.Filter) {
			continue
		}

		// Apply class filter
		if !ClassMatches(e.Class(), r.config.Class) {
			continue
		}

		// Print category header
		if e.Category() != currentCategory {
			currentCategory = e.Category()
			fmt.Println(currentCategory)
		}

		// Start logging
		if r.config.Logger != nil {
			r.config.Logger.StartEval(e.Name())
		}

		// Run eval
		start := time.Now()
		ctx := context.Background()
		result := e.Run(ctx, r.client)
		result.Duration = time.Since(start)

		// Log result
		if r.config.Logger != nil {
			r.config.Logger.LogResult(result.Passed, result.Message)
			r.config.Logger.EndEval()
		}

		// Print result
		r.printResult(result)
		results = append(results, result)
	}

	return results
}

func (r *Runner) printResult(result Result) {
	if result.Passed {
		fmt.Printf("  ✓ %s (%dms)\n", result.Name, result.Duration.Milliseconds())
	} else {
		fmt.Printf("  ✗ %s - %s\n", result.Name, result.Message)
		if r.config.Verbose && r.config.Logger != nil {
			fmt.Printf("    See log: %s/%s.log\n", r.config.Logger.Dir(), result.Name)
		}
	}
}

// AllEvals returns all registered evals.
func AllEvals() []Eval {
	var evals []Eval

	// Reasoning evals
	evals = append(evals, reasoningEvals()...)

	// Tool calling evals
	evals = append(evals, toolEvals()...)

	// Schema evals
	evals = append(evals, schemaEvals()...)

	// Agentic evals (multi-turn with interleaved reasoning)
	evals = append(evals, agenticEvals()...)

	return evals
}
