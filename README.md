# llm-evals

A test suite for validating LLM inference server implementations. Checks that OpenAI-compatible API responses are correctly structured—reasoning fields populated, tool calls parsed, JSON schemas respected.

## Install

```bash
go install github.com/aldehir/llm-evals/cmd/llm-eval@latest
```

Or build from source:

```bash
git clone https://github.com/aldehir/llm-evals
cd llm-evals
go build -o llm-eval ./cmd/llm-eval
```

## Usage

```bash
llm-eval --base-url http://localhost:8080/v1 --model deepseek-r1
```

Required flags:
- `--base-url` - Server base URL (include `/v1` if needed)
- `--model` - Model name to test

Optional flags:
- `--api-key` - API key if your server requires auth
- `--timeout` - Request timeout (default: 30s)
- `--verbose` / `-v` - Show full request/response for all evals
- `--filter` - Run only evals matching a pattern (e.g. `--filter streaming`)
- `--class` - Run only evals of a specific class: `standard`, `reasoning`, or `interleaved`
- `--extra` / `-e` - Add custom fields to request payloads (repeatable)

## Test Classes

Not all models support all features. Use `--class` to run only relevant tests:

- **standard** - Basic functionality: tool calling, JSON schema. Works with any model.
- **reasoning** - Requires `reasoning_content` support (thinking tokens). For reasoning models like DeepSeek R1.
- **interleaved** - Multi-turn agentic flows where reasoning must be sent back to the server. Requires interleaved reasoning support in the chat template.

```bash
# Test a standard model
llm-eval --base-url http://localhost:8080/v1 --model llama-3 --class standard

# Test a reasoning model
llm-eval --base-url http://localhost:8080/v1 --model deepseek-r1 --class reasoning
```

## List Available Evals

```bash
llm-eval list
```

Filter the list:

```bash
llm-eval list --filter tool
llm-eval list --class reasoning
```

## Custom Request Fields

Some servers need extra parameters. Use `--extra` to add fields to the request body:

```bash
# String value
llm-eval --base-url ... --model ... --extra "custom_param=value"

# JSON value (use := instead of =)
llm-eval --base-url ... --model ... --extra "temperature:=0.7"
llm-eval --base-url ... --model ... --extra 'stop:=["\\n"]'
```

## What Gets Tested

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

**Agentic (Multi-Turn)**
- `agentic_tool_call` - Full tool use loop with reasoning
- `agentic_reasoning_in_template` - Reasoning included when continuing from tool result
- `agentic_reasoning_not_in_user_template` - Reasoning excluded when last message is from user

All evals have streaming variants (e.g. `single_tool_call_streaming`).

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

## Example Output

```
LLM Eval Suite
==============
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
