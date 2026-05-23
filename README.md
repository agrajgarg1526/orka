# orka

A terminal kanban board for orchestrating AI coding agents. Each task gets its own git worktree and tmux session — agents work in isolation, you stay in control.

## Requirements

- Go 1.21+
- [tmux](https://github.com/tmux/tmux) — `brew install tmux`
- [claude](https://claude.ai/code) or [codex](https://github.com/openai/codex) CLI installed and authenticated

## Install

**Homebrew (recommended):**

```bash
brew tap agrajgarg1526/orka
brew install agrajgarg1526/orka/orka
```

**Manual (macOS arm64 — e.g. macOS 26 beta where Homebrew CLT is unsupported):**

```bash
curl -L https://github.com/agrajgarg1526/orka/releases/download/v0.1.0/orka_darwin_arm64.tar.gz | tar xz
sudo mv orka /usr/local/bin/orka
```

Use `orka_darwin_amd64.tar.gz` for Intel Macs.

**Go:**

```bash
go install github.com/agrajgarg/orka@latest
```

## Quick start

```bash
cd your-project
orka new        # register project + write .mcp.json
orka            # open the kanban board
```

Press `n` to create your first task, fill in the title, branch, and agent, then press `r` on the task to launch the agent.

## How it works

1. **`orka new`** registers the current directory as a project and writes `.mcp.json` so agents can report back via MCP.
2. **`orka`** opens the board. Tasks move left to right through phases; each phase sends a different prompt to the agent.
3. Pressing **`r`** on a task creates a git worktree at `.worktrees/<branch>`, starts a tmux session running the agent with the phase prompt, then attaches you to it.
4. Detach with **`ctrl+q`** to return to the board. The agent keeps running in the background.
5. Press **`r`** again to reattach. Advance to the next phase with **`l`** — this resets the session so the next `r` sends the correct prompt for the new phase.

## Phases

```
To Be Picked → Research → Planning → Running → Review → Done
```

- **To Be Picked** — backlog, not yet started
- **Research** — agent investigates and summarises findings
- **Planning** — agent produces an implementation plan
- **Running** — agent implements the task
- **Review** — agent reviews its own changes
- **Done** — complete

Research is optional — skip it per task at creation time with the "skip research" toggle.

## Creating a task

Press `n` on the board to open the task wizard. Steps:

| Field | Description |
|---|---|
| Title | Short name shown on the card |
| Branch | Git branch — a worktree is created at `.worktrees/<branch>` |
| Agent | `claude-code` or `codex` |
| Skip research | Jump straight to Planning |
| Description | Full brief sent in the agent prompt |

## Board keys

| Key | Action |
|---|---|
| `n` | new task |
| `enter` | open task view |
| `l` | advance selected task to next phase |
| `h` | retreat selected task to previous phase |
| `d` | delete task and clean up worktree |
| `↑ / ↓` | navigate cards within a column |
| `← / →` | navigate between columns |
| `/` | search tasks by title |
| `?` | help overlay |
| `esc` | back to project selector |
| `q` | quit |

## Task view keys

| Key | Action |
|---|---|
| `r` | launch agent / reattach to session |
| `l` | advance phase |
| `h` | retreat phase |
| `d` | open diff viewer |
| `e` | edit notes |
| `↑ / ↓` | scroll card content |
| `esc` | back to board |

Inside the agent session, **`ctrl+q`** detaches and returns to orka.

## Diff viewer

Press `d` in the task view to browse changed files.

| Key | Action |
|---|---|
| `↑ / ↓` | select file (file list) or scroll diff (diff pane) |
| `→` | focus diff pane |
| `←` | focus file list |
| `e` | open selected file in vim |
| `d / esc` | close |

Additions are green, deletions red, hunk headers blue.

## Agent sessions

- Each task has one tmux session named `orka-<id>`.
- Detach with `ctrl+q` — the session keeps running.
- `r` reattaches to the existing session, or starts a fresh resume if the session is gone (e.g. after a reboot).
- Advancing or retreating a phase resets the session so the next launch sends the correct phase prompt.

## Custom prompts

Default prompts are sent per phase. Override them in `~/.config/orka/prompts.toml` (auto-created on first run):

```toml
[claude-code]
research = "Research this task: {title}\n{description}"
planning = "Write a detailed plan for: {title}\n{description}"
running  = "Implement this: {title}\n{description}"
review   = "Review your changes for: {title}\n{description}"
```

Available placeholders: `{title}`, `{description}`, `{notes}`.

## MCP integration

`orka new` writes `.mcp.json` so agents inside a task session can call back into orka:

| Tool | Description |
|---|---|
| `list_tasks` | list all tasks for the project |
| `get_task` | get details of a specific task |
| `complete_phase` | signal phase done |
| `report_error` | mark task as errored with a message |
| `update_notes` | append to task notes |

Run `orka mcp` to start the MCP server manually (`.mcp.json` does this automatically).

## State

All state is stored in `~/.config/orka/state.json`.
