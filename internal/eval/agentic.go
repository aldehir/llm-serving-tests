package eval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aldehir/llm-serving-tests/internal/client"
)

const agenticCategory = "Agentic"

// agenticEvals returns all agentic (multi-turn) evals.
func agenticEvals() []Eval {
	return []Eval{
		&agenticToolCallEval{},
		&agenticReasoningInTemplateEval{},
		&agenticReasoningNotInUserTemplateEval{},
		&agenticLongResponseEval{},
	}
}

// weatherTool is the tool definition used in agentic evals.
var weatherTool = client.Tool{
	Type: "function",
	Function: client.ToolFunction{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {
					"type": "string",
					"description": "The city and state, e.g. San Francisco, CA"
				}
			},
			"required": ["location"]
		}`),
	},
}

// agenticToolCallEval tests a multi-turn tool call flow with interleaved reasoning.
type agenticToolCallEval struct{}

func (e *agenticToolCallEval) Name() string {
	return "agentic_tool_call"
}

func (e *agenticToolCallEval) Category() string {
	return agenticCategory
}

func (e *agenticToolCallEval) Class() string {
	return ClassInterleaved
}

func (e *agenticToolCallEval) Run(ctx context.Context, c *client.Client) Result {
	// Turn 1: User asks question requiring tool use
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	result1, err := c.ChatCompletionStream(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1 request failed: " + err.Error(),
		}
	}

	// Verify we got a tool call
	if len(result1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool call, got none",
		}
	}

	tc := result1.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool 'get_weather', got '" + tc.Function.Name + "'",
		}
	}

	// Turn 2: Send assistant message with reasoning + tool result
	req2 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
			{
				Role:             "assistant",
				ReasoningContent: result1.ReasoningContent,
				ToolCalls:        result1.ToolCalls,
			},
			{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    `{"temperature": 72, "conditions": "sunny"}`,
			},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	result2, err := c.ChatCompletionStream(ctx, req2)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2 request failed: " + err.Error(),
		}
	}

	// Verify we got a final response with content
	if strings.TrimSpace(result2.Content) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2: expected content in response, got empty",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// agenticReasoningInTemplateEval verifies reasoning appears in the template
// when messages end with a tool result after an assistant message.
type agenticReasoningInTemplateEval struct{}

func (e *agenticReasoningInTemplateEval) Name() string {
	return "agentic_reasoning_in_template"
}

func (e *agenticReasoningInTemplateEval) Category() string {
	return agenticCategory
}

func (e *agenticReasoningInTemplateEval) Class() string {
	return ClassInterleaved
}

func (e *agenticReasoningInTemplateEval) Run(ctx context.Context, c *client.Client) Result {
	// First, get reasoning content from the model
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	result1, err := c.ChatCompletionStream(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "initial request failed: " + err.Error(),
		}
	}

	// Need reasoning content for this test
	if strings.TrimSpace(result1.ReasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return reasoning_content, cannot test template",
		}
	}

	if len(result1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return tool calls, cannot test template",
		}
	}

	tc := result1.ToolCalls[0]

	// Build messages ending with tool result (should include reasoning in template)
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in San Francisco?"},
		{
			Role:             "assistant",
			ReasoningContent: result1.ReasoningContent,
			ToolCalls:        result1.ToolCalls,
		},
		{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    `{"temperature": 72, "conditions": "sunny"}`,
		},
	}

	// Call /apply-template
	prompt, err := c.ApplyTemplate(ctx, messages)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "/apply-template failed: " + err.Error(),
		}
	}

	// Verify reasoning content appears in the prompt
	if !strings.Contains(prompt, result1.ReasoningContent) {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content not found in rendered template",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// agenticReasoningNotInUserTemplateEval verifies reasoning does NOT appear
// when messages end with a user message.
type agenticReasoningNotInUserTemplateEval struct{}

func (e *agenticReasoningNotInUserTemplateEval) Name() string {
	return "agentic_reasoning_not_in_user_template"
}

func (e *agenticReasoningNotInUserTemplateEval) Category() string {
	return agenticCategory
}

func (e *agenticReasoningNotInUserTemplateEval) Class() string {
	return ClassInterleaved
}

func (e *agenticReasoningNotInUserTemplateEval) Run(ctx context.Context, c *client.Client) Result {
	// First, get reasoning content from the model
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Tools:      []client.Tool{weatherTool},
		ToolChoice: "auto",
	}

	result1, err := c.ChatCompletionStream(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "initial request failed: " + err.Error(),
		}
	}

	// Need reasoning content for this test
	if strings.TrimSpace(result1.ReasoningContent) == "" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return reasoning_content, cannot test template",
		}
	}

	if len(result1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "model did not return tool calls, cannot test template",
		}
	}

	tc := result1.ToolCalls[0]

	// Build messages ending with USER message (reasoning should NOT appear)
	messages := []client.Message{
		{Role: "user", Content: "What's the weather in San Francisco?"},
		{
			Role:             "assistant",
			ReasoningContent: result1.ReasoningContent,
			ToolCalls:        result1.ToolCalls,
		},
		{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    `{"temperature": 72, "conditions": "sunny"}`,
		},
		{Role: "user", Content: "Thanks, what about New York?"},
	}

	// Call /apply-template
	prompt, err := c.ApplyTemplate(ctx, messages)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "/apply-template failed: " + err.Error(),
		}
	}

	// Verify reasoning content does NOT appear in the prompt
	if strings.Contains(prompt, result1.ReasoningContent) {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "reasoning_content found in template when it should not be (ends with user message)",
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}

// fetchDocsTool is the tool definition for fetching documentation.
var fetchDocsTool = client.Tool{
	Type: "function",
	Function: client.ToolFunction{
		Name:        "fetch_documentation",
		Description: "Fetch technical documentation for a given topic",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"topic": {
					"type": "string",
					"description": "The documentation topic to fetch"
				}
			},
			"required": ["topic"]
		}`),
	},
}

// detailedDocsResponse is a detailed technical document returned by the fetch tool.
// The model must summarize and explain this content, generating a long response.
const detailedDocsResponse = `{
	"title": "Garbage Collection in Modern Programming Languages: A Comprehensive Guide",
	"sections": [
		{
			"heading": "1. Overview and History",
			"content": "Garbage collection (GC) is a form of automatic memory management that attempts to reclaim memory occupied by objects that are no longer in use by the program. It was invented by John McCarthy around 1959 to simplify manual memory management in Lisp. Before GC, programmers had to manually allocate and free memory using functions like malloc() and free() in C, which led to common bugs like memory leaks (forgetting to free), double-free errors (freeing twice), and use-after-free vulnerabilities (accessing freed memory). GC eliminates these classes of bugs at the cost of some runtime overhead and less predictable pause times."
		},
		{
			"heading": "2. Reference Counting",
			"content": "Reference counting tracks the number of references to each object. When a reference is created, the count increments; when destroyed, it decrements. Objects with zero references are immediately freed. Advantages: (1) Deterministic destruction - objects are freed immediately when unused, enabling RAII patterns and predictable resource cleanup; (2) Incremental overhead - work is spread across program execution rather than concentrated in pauses; (3) Simple to implement and understand. Disadvantages: (1) Cannot handle circular references without additional cycle detection; (2) Overhead on every pointer operation (increment/decrement); (3) Cache-unfriendly due to scattered reference count updates; (4) Thread-safety requires atomic operations, adding overhead. Used by: Python (with cycle detection), Swift (ARC - Automatic Reference Counting), Rust (Rc for single-threaded, Arc for multi-threaded), Objective-C (ARC), PHP, Perl."
		},
		{
			"heading": "3. Mark and Sweep Algorithm",
			"content": "Mark-and-sweep operates in two distinct phases. The MARK phase starts from root references (stack variables, global variables, CPU registers) and traverses all reachable objects, marking them as alive using a bit flag or separate bitmap. This traversal can be done depth-first (using a stack) or breadth-first (using a queue). The SWEEP phase then scans the entire heap linearly and frees all unmarked objects, resetting marks for the next cycle. Advantages: (1) Correctly handles circular references; (2) No overhead during normal execution (no reference counting); (3) Can be optimized with bitmaps for cache efficiency. Disadvantages: (1) Requires stopping the program (stop-the-world pause) in basic implementations; (2) Pause time proportional to heap size; (3) Can cause memory fragmentation. Variations include: MARK-COMPACT which adds a compaction phase to defragment memory by sliding live objects together; MARK-COPY (also called semi-space or Cheney's algorithm) which copies live objects to a new memory region, automatically compacting and allowing simple bump-pointer allocation."
		},
		{
			"heading": "4. Generational Garbage Collection",
			"content": "The generational hypothesis, observed empirically across many programs, states that most objects die young (are short-lived). Studies show 80-98% of objects become garbage shortly after allocation. Generational collectors exploit this by dividing the heap into generations: YOUNG GENERATION (also called nursery or eden) holds newly allocated objects and is collected frequently with fast minor GCs since most objects are dead. SURVIVOR SPACES hold objects that survived one or more minor GCs, acting as a buffer. OLD GENERATION (also called tenured) holds long-lived objects and is collected infrequently with slower major GCs. Objects are promoted from young to old after surviving a threshold number of collections (typically 15). Write barriers track old-to-young pointers since we can not scan the entire old generation during minor GCs. The remembered set or card table records which old generation regions contain pointers to young generation. Benefits: (1) Most GC pauses are short minor collections; (2) Major collections are rare; (3) Young generation uses fast copying collection; (4) Better cache locality for recently allocated objects."
		},
		{
			"heading": "5. Concurrent and Parallel Collection",
			"content": "Modern GCs use concurrency and parallelism to reduce pause times. PARALLEL GC uses multiple threads during stop-the-world pauses to speed up collection. CONCURRENT GC allows mutator threads (application threads) to run simultaneously with GC threads, reducing pause times at the cost of complexity. Challenges include: (1) Mutators can modify object graph during marking; (2) New allocations during GC; (3) Synchronization overhead. Solutions include write barriers (track pointer modifications), read barriers (intercept object access), and safe points (locations where GC can safely pause threads). The TRI-COLOR ABSTRACTION models concurrent marking: WHITE objects are candidates for collection (not yet reached), GRAY objects are reached but children not scanned, BLACK objects are fully scanned. The invariant ensures no black object points directly to white. SATB (Snapshot-At-The-Beginning) records the object graph at GC start. INCREMENTAL UPDATE tracks new pointers created during marking."
		},
		{
			"heading": "6. Java Garbage Collectors",
			"content": "Java offers multiple GC implementations: SERIAL GC (-XX:+UseSerialGC) is single-threaded, suitable for small heaps and single-core machines. PARALLEL GC (-XX:+UseParallelGC) uses multiple threads for young and old generation collection, optimizing for throughput. G1 (Garbage First, -XX:+UseG1GC) divides heap into equal-sized regions (1-32MB), tracks garbage density per region, prioritizes collecting regions with most garbage first, supports concurrent marking, aims for configurable pause time targets (-XX:MaxGCPauseMillis). ZGC (-XX:+UseZGC) is designed for very large heaps (multi-terabyte), achieves sub-millisecond pauses regardless of heap size, uses colored pointers (metadata in pointer bits) and load barriers, performs concurrent compaction. SHENANDOAH (-XX:+UseShenandoahGC) also achieves low pause times with concurrent compaction, uses Brooks pointers (forwarding pointers) for concurrent relocation, available in OpenJDK."
		},
		{
			"heading": "7. Go Garbage Collector",
			"content": "Go uses a concurrent, tri-color mark-and-sweep collector optimized for low latency. Key characteristics: (1) NON-GENERATIONAL - all objects treated equally, simplifies implementation; (2) NON-COMPACTING - avoids complexity of moving objects; (3) CONCURRENT - marking runs concurrently with mutators; (4) PARALLEL - uses multiple GC threads. The GC cycle: (a) Mark Setup - STW pause to enable write barrier; (b) Concurrent Marking - scans objects while program runs; (c) Mark Termination - STW pause to finish marking; (d) Concurrent Sweep - returns memory while program runs. Write barriers use a hybrid approach combining Dijkstra insertion barrier and Yuasa deletion barrier. GOGC environment variable controls target heap growth (default 100 = heap can double before GC). Go prioritizes consistent low latency (typically <1ms pauses) over raw throughput. The pacer algorithm tries to complete GC before heap limit is reached."
		},
		{
			"heading": "8. Python Garbage Collection",
			"content": "Python primarily uses REFERENCE COUNTING for immediate cleanup of unreferenced objects. For circular references, Python adds a GENERATIONAL CYCLE DETECTOR with three generations (0, 1, 2). Only container objects (lists, dicts, sets, classes, instances) participate in cycle detection since only they can form cycles. The cycle detector runs automatically when generation thresholds are exceeded (default: 700, 10, 10). Algorithm: (1) For each container, copy reference count to gc_refs; (2) Traverse containers, decrementing gc_refs for each internal reference; (3) Objects with gc_refs > 0 are reachable from outside; (4) Objects with gc_refs == 0 and only internal references are cyclic garbage. The gc module allows manual control: gc.collect() forces collection, gc.disable() disables automatic collection, gc.get_threshold()/gc.set_threshold() configure thresholds. CPython's GIL (Global Interpreter Lock) simplifies GC but limits parallelism. PyPy uses a proper generational moving GC instead."
		},
		{
			"heading": "9. Memory Allocation Strategies",
			"content": "GC interacts closely with memory allocation. BUMP POINTER ALLOCATION simply increments a pointer for each allocation - extremely fast (single pointer increment) but requires contiguous free space, typically used with copying collectors. FREE LIST ALLOCATION maintains a list of free blocks, finds a suitable block for each allocation (first-fit, best-fit, or segregated by size class). SEGREGATED FREE LISTS group allocations by size class for fast allocation and reduced fragmentation - used by jemalloc, tcmalloc. THREAD-LOCAL ALLOCATION BUFFERS (TLABs) give each thread a private allocation region, eliminating synchronization for most allocations. LARGE OBJECT SPACES handle big objects separately since copying them is expensive. OBJECT POOLS pre-allocate and reuse objects of the same type."
		},
		{
			"heading": "10. GC Tuning and Troubleshooting",
			"content": "Common tuning goals: (1) REDUCE PAUSE TIMES - use concurrent/low-latency collectors (ZGC, Shenandoah, Go), reduce heap size, tune generation sizes; (2) INCREASE THROUGHPUT - use parallel collectors, increase heap size, increase survivor ratio; (3) REDUCE MEMORY FOOTPRINT - reduce heap size, tune generation ratios, use compressed pointers. Troubleshooting: (1) MEMORY LEAKS - objects referenced but unused (static collections, listeners, caches without eviction), use heap dumps and profilers; (2) LONG PAUSES - heap too large, too many live objects, fragmentation, check GC logs; (3) HIGH GC OVERHEAD - allocation rate too high, heap too small, premature promotion; (4) OUT OF MEMORY - actual leak, heap too small, metaspace/native memory issues. Tools: Java VisualVM, Eclipse MAT, async-profiler, Go pprof, Python tracemalloc and objgraph."
		}
	]
}`

// agenticLongResponseEval tests a multi-turn flow where the model must generate
// a long text response after receiving tool results. This tests the server's
// grammar trigger for <tool_call> token that never comes during long generation.
type agenticLongResponseEval struct{}

func (e *agenticLongResponseEval) Name() string {
	return "agentic_long_response"
}

func (e *agenticLongResponseEval) Category() string {
	return agenticCategory
}

func (e *agenticLongResponseEval) Class() string {
	return ClassStandard
}

func (e *agenticLongResponseEval) Run(ctx context.Context, c *client.Client) Result {
	// Turn 1: User asks to fetch and explain documentation
	req1 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{
				Role: "user",
				Content: `Fetch the documentation about garbage collection and then write a comprehensive
tutorial explaining how garbage collection works. Your explanation must cover ALL of the
following topics in detail:

1. The history and motivation for garbage collection
2. Reference counting - how it works, advantages, disadvantages, and which languages use it
3. Mark and sweep algorithm - the two phases, variations like mark-compact and mark-copy
4. Generational garbage collection - the hypothesis, how generations work, promotion
5. Concurrent and parallel collection - challenges, tri-color abstraction, write barriers
6. Specific implementations in Java (G1, ZGC, Shenandoah), Go, and Python
7. Memory allocation strategies that work with GC
8. Tuning and troubleshooting guidance

For each topic, explain the concepts thoroughly with specific technical details.
Do not just summarize - provide a complete educational explanation.`,
			},
		},
		Tools:      []client.Tool{fetchDocsTool},
		ToolChoice: "auto",
	}

	result1, err := c.ChatCompletionStream(ctx, req1)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1 request failed: " + err.Error(),
		}
	}

	// Verify we got a tool call
	if len(result1.ToolCalls) == 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool call, got none",
		}
	}

	tc := result1.ToolCalls[0]
	if tc.Function.Name != "fetch_documentation" {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 1: expected tool 'fetch_documentation', got '" + tc.Function.Name + "'",
		}
	}

	// Turn 2: Send tool result with detailed documentation
	req2 := client.ChatCompletionRequest{
		Messages: []client.Message{
			{
				Role: "user",
				Content: `Fetch the documentation about garbage collection and then write a comprehensive
tutorial explaining how garbage collection works. Your explanation must cover ALL of the
following topics in detail:

1. The history and motivation for garbage collection
2. Reference counting - how it works, advantages, disadvantages, and which languages use it
3. Mark and sweep algorithm - the two phases, variations like mark-compact and mark-copy
4. Generational garbage collection - the hypothesis, how generations work, promotion
5. Concurrent and parallel collection - challenges, tri-color abstraction, write barriers
6. Specific implementations in Java (G1, ZGC, Shenandoah), Go, and Python
7. Memory allocation strategies that work with GC
8. Tuning and troubleshooting guidance

For each topic, explain the concepts thoroughly with specific technical details.
Do not just summarize - provide a complete educational explanation.`,
			},
			{
				Role:             "assistant",
				ReasoningContent: result1.ReasoningContent,
				ToolCalls:        result1.ToolCalls,
			},
			{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    detailedDocsResponse,
			},
		},
		Tools:      []client.Tool{fetchDocsTool},
		ToolChoice: "auto",
	}

	result2, err := c.ChatCompletionStream(ctx, req2)
	if err != nil {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2 request failed: " + err.Error(),
		}
	}

	// Verify no additional tool calls (model should just explain)
	if len(result2.ToolCalls) > 0 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2: expected no tool calls, got " + result2.ToolCalls[0].Function.Name,
		}
	}

	// Verify we got substantial content (at least 2500 chars for comprehensive tutorial)
	if len(result2.Content) < 2500 {
		return Result{
			Name:     e.Name(),
			Category: e.Category(),
			Passed:   false,
			Message:  "turn 2: response too short (expected 2500+ chars for comprehensive tutorial)",
		}
	}

	// Verify content covers key topics from the documentation
	requiredTopics := []string{
		"reference",
		"mark",
		"generation",
	}

	contentLower := strings.ToLower(result2.Content)
	for _, topic := range requiredTopics {
		if !strings.Contains(contentLower, topic) {
			return Result{
				Name:     e.Name(),
				Category: e.Category(),
				Passed:   false,
				Message:  "turn 2: response missing expected topic: " + topic,
			}
		}
	}

	return Result{
		Name:     e.Name(),
		Category: e.Category(),
		Passed:   true,
	}
}
