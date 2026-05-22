# orka

A TUI kanban board for orchestrating AI coding agents.

## Install

```bash
go install github.com/agrajgarg/orka@latest
```

## Usage

```bash
orka          # open kanban board
orka new      # initialize orka in current project
orka mcp      # start MCP server for agent use
```

## Supported agents

- `claude-code` — Claude Code CLI
- `claude-bedrock` — Claude on AWS Bedrock
- `codex` — OpenAI Codex CLI
- `codex-foundry` — Codex on Azure Foundry

## Phases

```
To Be Picked → Research (optional) → Planning → Running → Review → Done
```

Research is optional per task — set at creation time, not editable after.

## Plugin support

Set a plugin per task at creation to use that plugin's slash commands:

| Plugin | Commands used |
|---|---|
| `none` | plain prose prompts |
| `superpowers` | `/research`, `/plan`, `/implement`, `/review` |
| `gsd` | `/gsd research`, `/gsd plan`, `/gsd run`, `/gsd review` |
| `custom` | prompts from `~/.config/orka/prompts.toml` |

## Config

Override any prompt in `~/.config/orka/prompts.toml` (auto-generated on first run).

## MCP

`orka new` writes `.mcp.json` to your project directory so agents can call:
- `list_tasks` — list all tasks
- `get_task` — get task details
- `complete_phase` — signal phase done (triggers auto-advance)
- `report_error` — mark task as errored with a message
- `update_notes` — append to task notes

## Board keys

| Key | Action |
|---|---|
| `n` | new task |
| `enter` | open task view |
| `L` | advance phase |
| `H` | retreat phase |
| `/` | search tasks |
| `j/k` | navigate cards |
| `h/l` | navigate columns |
| `?` | help |
| `q` | quit |

## Task view keys

| Key | Action |
|---|---|
| `L` | advance phase |
| `H` | retreat phase |
| `r` | restart agent |
| `s` | stop agent |
| `e` | edit notes |
| `j/k` | scroll output |
| `esc` | back to board |
