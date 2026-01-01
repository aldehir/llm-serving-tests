package eval

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// StreamMode controls whether evals run in blocking, streaming, or both modes.
type StreamMode string

const (
	// ModeBlocking runs evals in blocking (non-streaming) mode only.
	ModeBlocking StreamMode = "blocking"
	// ModeStreaming runs evals in streaming mode only.
	ModeStreaming StreamMode = "streaming"
	// ModeBoth runs evals in both blocking and streaming modes.
	ModeBoth StreamMode = "both"
)

// AllModes returns all valid stream modes.
func AllModes() []string {
	return []string{string(ModeBlocking), string(ModeStreaming), string(ModeBoth)}
}

// AllClasses returns all valid eval classes.
func AllClasses() []string {
	return []string{ClassStandard, ClassReasoning, ClassInterleaved}
}

// ClassMatches returns true if the eval's class is compatible with the requested class.
// Class hierarchy: standard < reasoning < interleaved
// - standard: only standard tests
// - reasoning: standard + reasoning tests
// - interleaved: standard + reasoning + interleaved tests
func ClassMatches(evalClass, requestedClass string) bool {
	if requestedClass == "" {
		return true
	}
	if evalClass == requestedClass {
		return true
	}
	// reasoning class also runs standard tests
	if requestedClass == ClassReasoning && evalClass == ClassStandard {
		return true
	}
	// interleaved class runs standard + reasoning tests
	if requestedClass == ClassInterleaved && (evalClass == ClassStandard || evalClass == ClassReasoning) {
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

// StreamModeEval is an optional interface for evals that support both streaming and blocking modes.
// Evals implementing this interface can be run in either mode based on the --mode flag.
type StreamModeEval interface {
	Eval
	// SetStreaming configures whether the eval runs in streaming mode.
	SetStreaming(streaming bool)
	// Streaming returns the current streaming mode.
	Streaming() bool
}

// Result represents the result of an eval.
type Result struct {
	Name     string
	Category string
	Passed   bool
	Message  string
	Duration time.Duration
}

// DefaultDisabled is an optional interface for evals that are disabled by default.
// Evals implementing this interface with IsDefaultDisabled() returning true will
// only run when --all is specified.
type DefaultDisabled interface {
	IsDefaultDisabled() bool
}

// IsDefaultDisabled returns true if the eval is disabled by default.
// This checks if the eval implements the DefaultDisabled interface.
func IsDefaultDisabled(e Eval) bool {
	if dd, ok := e.(DefaultDisabled); ok {
		return dd.IsDefaultDisabled()
	}
	return false
}

// RunnerConfig configures the runner.
type RunnerConfig struct {
	Verbose bool
	Filter  string
	Class   string
	All     bool // Include evals that are disabled by default
	Logger  *evallog.Logger
	Jobs    int        // Number of parallel test executions (1 = sequential)
	Mode    StreamMode // Streaming mode: blocking, streaming, or both
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
	// Filter evals
	var evals []Eval
	for _, e := range r.evals {
		// Apply name filter
		if r.config.Filter != "" && !strings.Contains(e.Name(), r.config.Filter) {
			continue
		}

		// Apply class filter
		if !ClassMatches(e.Class(), r.config.Class) {
			continue
		}

		// Skip disabled-by-default tests unless --all is set
		if !r.config.All && IsDefaultDisabled(e) {
			continue
		}

		evals = append(evals, e)
	}

	if r.config.Jobs <= 1 {
		return r.runSequential(evals)
	}
	return r.runParallel(evals)
}

// runSequential executes evals one at a time (original behavior).
func (r *Runner) runSequential(evals []Eval) []Result {
	var results []Result
	currentCategory := ""

	for _, e := range evals {
		// Print category header
		if e.Category() != currentCategory {
			currentCategory = e.Category()
			fmt.Println(currentCategory)
		}

		// Run in configured mode(s)
		for _, result := range r.runEvalInModes(e) {
			r.printResult(result)
			results = append(results, result)
		}
	}

	return results
}

// evalJob represents a job for the worker pool.
type evalJob struct {
	eval      Eval
	streaming bool
}

// runParallel executes evals concurrently using a worker pool.
func (r *Runner) runParallel(evals []Eval) []Result {
	var results []Result
	jobs := make(chan evalJob)
	resultChan := make(chan Result)
	var wg sync.WaitGroup

	// Start workers
	for range r.config.Jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result := r.runSingleEval(job.eval, job.streaming)
				resultChan <- result
			}
		}()
	}

	// Collect results in a separate goroutine
	var resultWg sync.WaitGroup
	resultWg.Add(1)
	go func() {
		defer resultWg.Done()
		for result := range resultChan {
			r.printResultParallel(result)
			results = append(results, result)
		}
	}()

	// Send jobs based on mode
	mode := r.config.Mode
	if mode == "" {
		mode = ModeBoth
	}

	for _, e := range evals {
		switch mode {
		case ModeBlocking:
			jobs <- evalJob{eval: e, streaming: false}
		case ModeStreaming:
			jobs <- evalJob{eval: e, streaming: true}
		case ModeBoth:
			jobs <- evalJob{eval: e, streaming: false}
			jobs <- evalJob{eval: e, streaming: true}
		}
	}
	close(jobs)

	wg.Wait()
	close(resultChan)
	resultWg.Wait()

	return results
}

// runEvalInModes runs an eval in the configured mode(s) and returns results.
func (r *Runner) runEvalInModes(e Eval) []Result {
	mode := r.config.Mode
	if mode == "" {
		mode = ModeBoth
	}

	var results []Result

	switch mode {
	case ModeBlocking:
		results = append(results, r.runSingleEval(e, false))
	case ModeStreaming:
		results = append(results, r.runSingleEval(e, true))
	case ModeBoth:
		results = append(results, r.runSingleEval(e, false))
		results = append(results, r.runSingleEval(e, true))
	}

	return results
}

// runSingleEval executes a single eval with logging.
func (r *Runner) runSingleEval(e Eval, streaming bool) Result {
	// Set streaming mode if eval supports it
	if sme, ok := e.(StreamModeEval); ok {
		sme.SetStreaming(streaming)
	}

	// Build name with mode suffix
	name := e.Name()
	if streaming {
		name += " (streaming)"
	} else {
		name += " (blocking)"
	}

	if r.config.Logger != nil {
		r.config.Logger.StartEval(name)
	}

	start := time.Now()
	ctx := context.Background()
	result := e.Run(ctx, r.client)
	result.Duration = time.Since(start)
	result.Name = name
	result.Category = e.Category()

	if r.config.Logger != nil {
		r.config.Logger.LogResult(result.Passed, result.Message)
		r.config.Logger.EndEval()
	}

	return result
}

// printResult prints a result in sequential mode (indented under category).
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

// printResultParallel prints a result in parallel mode (with category prefix).
func (r *Runner) printResultParallel(result Result) {
	if result.Passed {
		fmt.Printf("✓ %s (%dms) [%s]\n", result.Name, result.Duration.Milliseconds(), result.Category)
	} else {
		fmt.Printf("✗ %s - %s [%s]\n", result.Name, result.Message, result.Category)
		if r.config.Verbose && r.config.Logger != nil {
			fmt.Printf("    See log: %s/%s.log\n", r.config.Logger.Dir(), result.Name)
		}
	}
}

// AllEvals returns all registered evals.
func AllEvals() []Eval {
	var evals []Eval

	// Basic evals
	evals = append(evals, basicEvals()...)

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
