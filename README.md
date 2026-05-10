# noor

A terminal chat client powered by [OpenRouter](https://openrouter.ai), built in Go with [Charm](https://charm.sh) libraries.

## Features

- Beautiful markdown rendering via **glamour**
- Styled UI via **lipgloss**
- Interactive model picker with categories via **bubbletea**
- Multi-line input — `Ctrl+N` for new line, `Enter` to send
- Always-on streaming — spinner → live text → glamour render
- Auto-retry on network errors (silent, one attempt)
- Image analysis — send local images with `/image`
- Image generation & editing — `/imagine` and `/image` with image models
- MCP (Model Context Protocol) support — connect any stdio MCP server
- Built-in web search via TinyFish
- Built-in weather via Open-Meteo
- Per-model context window tracking with countdown
- Cost tracking per reply + session total on exit
- Session history — browse and resume past sessions with `/history`
- Persistent model preference — remembers your last model across sessions
- Single compiled binary

## Requirements

- Go 1.21+
- An OpenRouter API key — get one at [openrouter.ai/keys](https://openrouter.ai/keys)

## Installation

```bash
git clone <repo>
cd noor
make install
```

This builds and copies the binary to `~/.local/bin/noor`.

## Configuration

Create `~/.config/noor/config` once:

```
OPENROUTER_API_KEY=your-openrouter-api-key
TINYFISH_API_KEY=sk-tinyfish-...   # optional, enables web search
```

Then just run:

```bash
noor
```

## Usage

```bash
noor                                          # default
noor -m 4000 -t 0.9                          # custom max tokens + temperature
noor -S concise                               # response style
noor -p "You are a Linux expert"             # custom system prompt
noor --mcp-server "python3 mcp_security.py"  # with MCP server
```

## Options

| Flag | Description |
|---|---|
| `-h, --help` | Show help |
| `--version` | Show version |
| `-c, --config FILE` | Custom config file |
| `-S, --style STYLE` | Response style: `markdown`, `plain`, `concise`, `raw` |
| `-p, --system-prompt STR` | Set system prompt |
| `-t, --temp FLOAT` | Temperature (0.0–2.0, default: 0.7) |
| `-m, --max-tokens INT` | Max tokens (default: 2000) |
| `--mcp-server CMD` | MCP stdio server command |
| `--no-history` | Disable session history |
| `--debug` | Enable debug logging to stderr |

## Slash Commands

| Command | Description |
|---|---|
| `/help` | Show available commands |
| `/clear` | Clear screen and reset conversation |
| `/reset` | Reset conversation context |
| `/model [name]` | Interactive picker or switch model directly (saves preference) |
| `/style <name>` | Switch response style |
| `/theme [name]` | Switch color theme (default/cyberpunk/ocean/forest/sunset/minimal) |
| `/system <prompt>` | Set custom system prompt |
| `/history` | Browse and resume a past session |
| `/history clear` | Delete all saved sessions (with confirmation) |
| `/search <query>` | Search past sessions for text and resume one |
| `/tools` | List loaded tools |
| `/mcp [cmd\|stop]` | Start/stop an MCP server live (e.g. `/mcp python3 mcp.py`) |
| `/image <path> [prompt]` | Analyze image or edit it (image models) |
| `/imagine <prompt>` | Generate an image (image models) |
| `/export [file]` | Export — default `.html`, or `.py`/`.js`/`.go` to extract code |
| `/copy` | Copy last response to clipboard |
| `/retry` | Regenerate last response with same prompt |
| `/edit` | Edit your last message and regenerate |
| `/budget [amount]` | Show today's spend, or set daily limit (e.g. `/budget 2.00`) |
| `/compress` | Summarize older messages to free up context window |
| `/freeze` | Export last code block to PNG (requires [`freeze`](https://github.com/charmbracelet/freeze)) |
| `/attach` | Pick a file via filepicker to include with your next message |
| `weather <city>` | Live weather for any location |
| `@<ref>` | Include file/URL/git as context (see References below) |
| `exit` | Quit (shows session cost) |

### References (`@` syntax)

Type `@<something>` inside any message to inline its contents:

| Reference | What it does |
|---|---|
| `@path/to/file.go` | Read and include a local file |
| `@~/notes.md` | `~` expands to your home directory |
| `@https://...` | Fetch URL, strip HTML, include text |
| `@diff` | Current `git diff` |
| `@diff-staged` | `git diff --staged` |
| `@status` | `git status --short` |
| `@log` | Last 10 commits (`git log -10 --oneline`) |
| `@branch` | Current branch name |

Example:
```
explain what @main.go does and how it differs from @diff
```

## Models

### Chat

| Model | Context | Notes |
|---|---|---|
| `anthropic/claude-opus-4.7` | 1M | Most powerful Claude |
| `anthropic/claude-sonnet-4.6` | 1M | Best balance |
| `anthropic/claude-haiku-4.5` | 200K | Fastest, cheapest Claude ← default |
| `openai/gpt-5.5` | 1.05M | OpenAI GPT-5.5 |
| `openai/gpt-5.4` | 1.05M | OpenAI GPT-5.4 |
| `openai/gpt-5.4-nano` | 400K | OpenAI GPT-5.4 lightweight |
| `openai/gpt-4o` | 128K | OpenAI flagship |
| `openai/gpt-4o-mini` | 128K | OpenAI lightweight |
| `google/gemini-2.5-pro` | 1M | Google flagship |
| `google/gemini-2.5-flash` | 1M | Google fast/cheap |
| `deepseek/deepseek-v4-pro` | 1M | DeepSeek latest, most powerful |
| `deepseek/deepseek-v4-flash` | 1M | DeepSeek fast/cheap |
| `deepseek/deepseek-v3.2` | 131K | DeepSeek V3 updated |
| `mistralai/mistral-large` | 128K | Mistral flagship |
| `z-ai/glm-5.1` | 202K | GLM latest |
| `z-ai/glm-5-turbo` | 202K | GLM fast |
| `moonshotai/kimi-k2.6` | 262K | Moonshot AI |
| `minimax/minimax-m2.7` | 196K | MiniMax |
| `x-ai/grok-4.3` | 1M | xAI Grok 4.3 |

### Images

| Model | Context | Notes |
|---|---|---|
| `google/gemini-2.5-flash-image` | 32K | Stable image generation & editing |
| `google/gemini-3.1-flash-image-preview` | 64K | Latest, higher quality (preview) |

> Generated/edited images are saved to `~/Pictures/noor/`.

## Footer

After each reply:
```
1.2s  ·  342 tokens  ·  $0.0003
████░░░░░░░░  4% · 8K used · 192K left
```

On `exit`:
```
session cost: $0.0012
goodbye
```

## Development

```bash
make build     # build locally
make install   # build + install to ~/.local/bin
make uninstall # remove from PATH
make clean     # delete local binary
```

## Session History

Sessions are saved to `~/.cache/noor/history/` as JSON files. The last 10 sessions are kept automatically. Use `/history` to browse and resume any past session.
