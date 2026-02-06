# agate - AI Orchestrator CLI

A non-interactive CLI tool that orchestrates AI agents to transform GOAL.md into working software.

## Project Overview

`agate` takes a goal file and drives the full lifecycle: interview, design, sprint planning, implementation, and review -- using multiple AI agents (Claude, Codex).

## Commands

- `agate auto` - Run the full lifecycle until done (recommended)
- `agate next` - Advance one step (exit 0=done, 1=more work, 2=error, 255=human action needed)
- `agate status` - Show progress and relevant files
- `agate suggest 'text'` - Send a suggestion to guide the next step

## Key Principles

1. **One step at a time**: Each `next` call does exactly ONE thing
2. **Non-interactive**: No prompts, just exit codes
3. **Multi-agent**: Uses both Claude and Codex, parallel for independent work
4. **Resumable**: State is inferred from file existence and markdown checkboxes (no state.json)
5. **User-editable**: All generated files can be edited

## Project Structure

```
agate/
├── cmd/                 # Cobra commands
│   ├── root.go         # Root command setup
│   ├── auto.go         # Auto command (loops next)
│   ├── next.go         # Next command
│   ├── interrupt.go    # Suggest/interrupt command
│   └── status.go       # Status command
├── internal/
│   ├── agent/          # Agent abstraction
│   │   ├── agent.go    # Interface
│   │   ├── claude.go   # Claude CLI integration
│   │   ├── codex.go    # Codex CLI integration
│   │   ├── haiku.go    # Haiku agent
│   │   ├── dummy.go    # No-op agent for testing
│   │   ├── executor.go # Command execution
│   │   ├── multi.go    # Parallel execution
│   │   └── progress.go # Progress tracking
│   ├── project/        # Project handling
│   │   ├── project.go  # Directory structure
│   │   ├── goal.go     # GOAL.md parsing
│   │   └── skills.go   # Skill generation
│   └── workflow/       # Core logic
│       ├── plan.go     # Planning workflow
│       ├── next.go     # Step advancement
│       ├── sprint.go   # Sprint parsing with failure tracking
│       ├── state.go    # State computation
│       ├── status.go   # Status display
│       ├── retro.go    # Sprint retrospectives
│       └── interrupt.go # Suggestion handling
├── main.go             # Entry point
└── go.mod              # Go module
```

## Building

```bash
go build -o agate .
```

## Testing

```bash
go test ./...
```

## Usage

1. Create a GOAL.md describing what you want to build
2. Run `agate auto` -- it handles interview, design, planning, and implementation
3. When it stops (exit 255), answer questions in `.ai/interview.md`, then re-run `agate auto`
4. Run `agate status` to check progress at any time

## Tech Stack

- Go 1.21+
- Cobra for CLI
- Shells out to `claude` and `codex` CLIs

## Upgrading from rocks

If you previously used the `rocks` CLI, note that:
- The CLI command is now `agate` instead of `rocks`
- The internal data directory is now `.ai/` instead of `.rocks/`
- Module path is `github.com/strongdm/agate`
