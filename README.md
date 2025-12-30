# llm-serving-tests

A test suite for validating LLM inference server implementations. Checks that OpenAI-compatible API responses are correctly structured—reasoning fields populated, tool calls parsed, JSON schemas respected.

## Install

```bash
go install github.com/aldehir/llm-serving-tests/cmd/llm-serve-test@latest
```

Or build from source:

```bash
git clone https://github.com/aldehir/llm-serving-tests
cd llm-serving-tests
go build -o llm-serve-test ./cmd/llm-serve-test
```

## Usage

```bash
llm-serve-test --base-url http://localhost:8080/v1 --model deepseek-r1
```

Required flags:
- `--base-url` - Server base URL (include `/v1` if needed)
- `--model` - Model name to test

Optional flags:
- `--api-key` - API key if your server requires auth
- `--timeout` - Request timeout (default: 30s)
- `--response-header-timeout` - Time to wait for response headers, useful for slow prompt processing (default: 5m)
- `--verbose` / `-v` - Show full request/response for all tests
- `--filter` - Run only tests matching a pattern (e.g. `--filter streaming`)
- `--class` - Run only tests of a specific class: `standard`, `reasoning`, or `interleaved`
- `--all` / `-a` - Include tests that are disabled by default
- `--extra` / `-e` - Add custom fields to request payloads (repeatable)

## Test Classes

Not all models support all features. Use `--class` to run tests appropriate for your model type. Classes are hierarchical (standard < reasoning < interleaved):

- **standard** - Basic functionality: tool calling, JSON schema. Works with any model.
- **reasoning** - Includes standard tests, plus tests requiring `reasoning_content` support. For reasoning models like DeepSeek R1.
- **interleaved** - Includes all tests. Adds multi-turn agentic flows where reasoning must be sent back to the server.

```bash
# Test a standard model
llm-serve-test --base-url http://localhost:8080/v1 --model llama-3 --class standard

# Test a reasoning model
llm-serve-test --base-url http://localhost:8080/v1 --model deepseek-r1 --class reasoning
```

## List Available Tests

```bash
llm-serve-test list
```

Filter the list:

```bash
llm-serve-test list --filter tool
llm-serve-test list --class reasoning
```

## Custom Request Fields

Some servers need extra parameters. Use `--extra` to add fields to the request body:

```bash
# String value
llm-serve-test --base-url ... --model ... --extra "custom_param=value"

# JSON value (use := instead of =)
llm-serve-test --base-url ... --model ... --extra "temperature:=0.7"
llm-serve-test --base-url ... --model ... --extra 'stop:=["\n"]'
```

## What Gets Tested

**Basic**
- `chat_completion` - Verifies model returns non-empty content

**Reasoning**
- `reasoning_present` - Verifies `reasoning_content` is populated
- `reasoning_not_leaked` - Confirms reasoning doesn't leak into main `content`

**Tool Calling**
- `single_tool_call` - Basic tool call parsing
- `parallel_tool_calls` - Multiple concurrent tool calls
- `required_tool_call` - `tool_choice: "required"` behavior
- `required_tool_call_with_reasoning` - Tool calls don't suppress reasoning output

**Structured Output**
- `json_schema` - Response conforms to requested JSON schema

**Agentic (Multi-Turn)** - all agentic tests use streaming
- `agentic_tool_call` - Full tool use loop with reasoning
- `agentic_reasoning_in_template` - Reasoning included when continuing from tool result
- `agentic_reasoning_not_in_user_template` - Reasoning excluded when last message is from user
- `agentic_long_response` - Long text generation after tool call (disabled by default, use `--all` to include)

Most tests have streaming variants (e.g. `single_tool_call_streaming`).

## Logs

Request/response logs are grouped by model and timestamped:

```
logs/
└── deepseek-r1/
    ├── 2025-01-15_143022/
    │   ├── reasoning_present.log
    │   ├── single_tool_call.log
    │   └── ...
    └── 2025-01-15_152301/
        └── ...
```

The path is printed at the end of each run:

```
Logs written to: ./logs/deepseek-r1/2025-01-15_143022/
```

Use `--verbose` to also print full request/response details to the terminal.

Streaming tests also generate `.stream.jsonl` files for replay (see below).

## Replay Streaming Responses

Streaming tests capture chunks to JSONL files for later visualization. This helps verify streaming output is coherent.

Replay a single file:

```bash
llm-serve-test replay logs/deepseek-r1/2025-01-15_143022/reasoning_present_streaming.stream.jsonl
```

Replay all streaming captures from a log directory:

```bash
llm-serve-test replay-all logs/deepseek-r1/2025-01-15_143022/
```

Options:
- `--delay` - Time between chunks (default: 10ms)

The output is styled:
- **Reasoning** - Dark gray, italic, prefixed with `[thinking]`
- **Content** - Regular text
- **Tool calls** - Yellow, with `[tool: name]` header

## Example Output

```
LLM Serving Tests
=================
Server: http://localhost:8080/v1
Model: deepseek-r1

Reasoning
  ✓ reasoning_present (512ms)
  ✓ reasoning_not_leaked (487ms)
  ✓ reasoning_present_streaming (534ms)
  ✓ reasoning_not_leaked_streaming (501ms)

Tool Calling
  ✓ single_tool_call (623ms)
  ✓ single_tool_call_streaming (645ms)
  ✓ parallel_tool_calls (701ms)
  ✗ parallel_tool_calls_streaming - expected at least 2 tool calls, got 1

Results: 7/8 passed

Logs written to: ./logs/deepseek-r1/2025-01-15_143022/
```

## License

MIT
