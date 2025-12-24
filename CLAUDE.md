# llm-serving-tests

CLI tool for testing LLM inference server implementations against OpenAI-compatible APIs.

## Quick Reference

```bash
# Build
go build -o llm-serve-test ./cmd/llm-serve-test

# Verify
go vet ./...

# Run tests (requires a running LLM server)
./llm-serve-test --base-url http://localhost:8080/v1 --model <model-name>

# List available tests
./llm-serve-test list
```

## Project Structure

```
cmd/llm-serve-test/    CLI entry point (cobra-based)
internal/
  client/              HTTP client for OpenAI-compatible API
  eval/                Test implementations
    runner.go          Test runner and Eval interface
    basic.go           Basic completion tests
    reasoning.go       Reasoning content tests
    tools.go           Tool calling tests
    schema.go          JSON schema tests
    agentic.go         Multi-turn agentic tests
  log/                 Request/response logging
logs/                  Test run output (gitignored)
```

## Adding New Evals

1. Create eval in appropriate file under `internal/eval/` (or new file if new category)
2. Implement the `Eval` interface:
   - `Name()` - test name (lowercase, underscores)
   - `Category()` - display category
   - `Class()` - one of `standard`, `reasoning`, `interleaved`
   - `Run(ctx, client)` - returns `Result{Passed, Message}`
3. Register in the category's `*Evals()` function (e.g., `toolEvals()`)
4. Add streaming variant if applicable (append `_streaming` to name)
5. Update README.md if adding new tests, CLI flags, or changing behavior

## Class Hierarchy

- `standard` - works with any OpenAI-compatible model
- `reasoning` - requires `reasoning_content` field support
- `interleaved` - requires reasoning in multi-turn conversations
