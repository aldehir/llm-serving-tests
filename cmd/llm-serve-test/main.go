package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
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
	all     bool
	extra   []string

	replayDelay time.Duration
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

var replayCmd = &cobra.Command{
	Use:   "replay <jsonl-file>",
	Short: "Replay streaming response from JSONL capture",
	Long:  "Visualize a captured streaming response by rendering each chunk with simulated delay.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReplay,
}

var replayAllCmd = &cobra.Command{
	Use:   "replay-all <log-dir>",
	Short: "Replay all streaming responses from a log directory",
	Long:  "Replay all .stream.jsonl files from a log directory in sequence.",
	Args:  cobra.ExactArgs(1),
	RunE:  runReplayAll,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Server base URL (required for run)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (optional)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to test (required for run)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full request/response for all tests")
	rootCmd.PersistentFlags().StringVar(&filter, "filter", "", "Run only tests matching pattern")
	rootCmd.PersistentFlags().StringVar(&class, "class", "", "Run only tests of specified class (standard, reasoning, interleaved)")
	rootCmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "Include tests that are disabled by default")
	rootCmd.PersistentFlags().StringArrayVarP(&extra, "extra", "e", nil, "Extra request field (key=value or key:=json), can be repeated")

	replayCmd.Flags().DurationVar(&replayDelay, "delay", 10*time.Millisecond, "Delay between chunks")
	replayAllCmd.Flags().DurationVar(&replayDelay, "delay", 10*time.Millisecond, "Delay between chunks")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(replayCmd)
	rootCmd.AddCommand(replayAllCmd)
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
		All:     all,
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

		// Skip disabled-by-default tests unless --all is set
		isDisabled := eval.IsDefaultDisabled(t)
		if !all && isDisabled {
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

		// Show disabled indicator
		disabledMarker := ""
		if isDisabled {
			disabledMarker = " (disabled by default)"
		}
		fmt.Printf("  %-45s [%s]%s\n", t.Name(), t.Class(), disabledMarker)
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

// runReplay replays a streaming response from a JSONL capture file.
func runReplay(cmd *cobra.Command, args []string) error {
	return replayFile(args[0])
}

// runReplayAll replays all streaming responses from a log directory.
func runReplayAll(cmd *cobra.Command, args []string) error {
	dir := args[0]

	pattern := filepath.Join(dir, "*.stream.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .stream.jsonl files found in %s", dir)
	}

	sort.Strings(files)

	headerStyle := color.New(color.FgCyan, color.Bold)

	for i, file := range files {
		// Extract eval name from filename (remove .stream.jsonl suffix)
		base := filepath.Base(file)
		evalName := strings.TrimSuffix(base, ".stream.jsonl")

		if i > 0 {
			fmt.Println()
		}
		headerStyle.Printf("=== %s ===\n", evalName)

		if err := replayFile(file); err != nil {
			return fmt.Errorf("replay %s: %w", evalName, err)
		}
	}

	return nil
}

// replayFile replays a single JSONL file.
func replayFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	renderer := newStreamRenderer()

	for scanner.Scan() {
		var chunk client.ChatCompletionChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return fmt.Errorf("parse chunk: %w", err)
		}

		renderer.render(chunk)
		time.Sleep(replayDelay)
	}

	renderer.finish()
	return scanner.Err()
}

// streamRenderer handles progressive display of streaming chunks.
type streamRenderer struct {
	inReasoning bool
	inContent   bool
	toolCalls   map[int]*toolState

	thinkingStyle *color.Color
	toolStyle     *color.Color
}

type toolState struct {
	headerPrinted bool
	name          string
}

func newStreamRenderer() *streamRenderer {
	return &streamRenderer{
		toolCalls:     make(map[int]*toolState),
		thinkingStyle: color.New(color.FgHiBlack, color.Italic),
		toolStyle:     color.New(color.FgYellow),
	}
}

func (r *streamRenderer) render(chunk client.ChatCompletionChunk) {
	if len(chunk.Choices) == 0 {
		return
	}

	delta := chunk.Choices[0].Delta

	// Handle reasoning content
	if delta.ReasoningContent != "" {
		if !r.inReasoning {
			r.thinkingStyle.Print("[thinking] ")
			r.inReasoning = true
		}
		r.thinkingStyle.Print(delta.ReasoningContent)
	}

	// Handle regular content
	if delta.Content != "" {
		if r.inReasoning {
			fmt.Println() // End reasoning line
			fmt.Println() // Blank line before content
			r.inReasoning = false
		}
		r.inContent = true
		fmt.Print(delta.Content)
	}

	// Handle tool calls
	for _, tc := range delta.ToolCalls {
		state := r.toolCalls[tc.Index]
		if state == nil {
			state = &toolState{}
			r.toolCalls[tc.Index] = state
		}

		// Store name when we first see it
		if tc.Function.Name != "" {
			state.name = tc.Function.Name
		}

		// Print header before first argument chunk
		if tc.Function.Arguments != "" && !state.headerPrinted {
			if r.inReasoning || r.inContent {
				fmt.Println()
				fmt.Println()
				r.inReasoning = false
				r.inContent = false
			} else if len(r.toolCalls) > 1 {
				// Separate multiple tool calls with a newline
				fmt.Println()
			}
			r.toolStyle.Printf("[tool: %s]\n", state.name)
			state.headerPrinted = true
		}

		// Stream arguments
		if tc.Function.Arguments != "" {
			r.toolStyle.Print(tc.Function.Arguments)
		}
	}
}

func (r *streamRenderer) finish() {
	fmt.Println()
}
