# agate

An [attractor](https://github.com/strongdm/attractor) for software. You define a goal; AI agents converge on working code.

## What is an attractor?

In dynamical systems, an attractor is a state toward which a system tends to evolve regardless of starting conditions. Agate applies this idea to software development: given a goal, it pulls multiple non-deterministic AI agents through iterative plan/implement/review cycles until the goal is met. The agents are chaotic, but the loop is convergent.

## Quick start

```bash
go install github.com/strongdm/agate@latest

mkdir my-project && cd my-project

cat > GOAL.md << 'EOF'
Build a CLI tool that converts CSV files to JSON.
Language: Go
EOF

agate auto
```

That's it. `agate auto` drives the entire lifecycle:

1. **Interview** -- generates clarifying questions in `.ai/interview.md`, then stops (exit 255) so you can answer them
2. **Design** -- produces `.ai/design/overview.md` and `.ai/design/decisions.md`
3. **Sprint planning** -- breaks the work into tasks with skills assigned to each
4. **Implementation** -- agents write code, reviewers gate each task
5. **Assessment** -- when a sprint finishes, agate checks if the goal is met. If not, it plans the next sprint and keeps going

When `auto` stops for human input (exit 255), answer the questions and re-run `agate auto`. It picks up where it left off.

```
GOAL.md ──> interview ──> design ──> sprint plan ──> implement/review loop
                                          │                    │
                                          │    ┌───────────────┘
                                          │    │ sprint complete?
                                          │    ├── goal met ──> done
                                          │    └── more work ──> next sprint
                                          │                        │
                                          └────────────────────────┘
```

## Commands

| Command | Purpose | Exit codes |
|---------|---------|------------|
| `agate auto` | Run the full lifecycle until done | 0 = done, 255 = human action needed |
| `agate next` | Advance exactly one step | 0 = done, 1 = more work, 2 = error, 255 = human action needed |
| `agate status` | Show progress and relevant files | same as `next` |
| `agate suggest 'text'` | Send a hint to guide the next step | |

### `agate auto` (recommended)

Runs `agate next` in a loop. Stops when the project is complete (exit 0) or when human action is needed (exit 255). You can type suggestions on stdin between steps and they'll be forwarded automatically.

```bash
agate auto                  # use default agent (Claude Opus)
agate auto --agent haiku    # use Haiku (fast, cheap, good for testing)
```

### `agate next`

For manual control. Each call advances one step -- generating the interview, producing the design, implementing a single task, reviewing it, etc. Chain it in a shell loop if you prefer:

```bash
while agate next; [ $? -eq 1 ]; do :; done
```

### `agate suggest`

Sends a suggestion that gets picked up on the next `agate next` invocation. Useful for steering the agents without editing files directly.

```bash
agate suggest 'focus on error handling first'
```

## Agents

Use `--agent` with `auto` or `next` to select which AI drives the work:

| Agent | Model | Notes |
|-------|-------|-------|
| `claude` | Claude Opus 4.5 | Most capable, default |
| `haiku` | Claude 3.5 Haiku | Fast, cheap, good for testing |
| `codex` | GPT 5.2 | OpenAI alternative |
| `dummy` | No-op | For workflow testing |

## State and files

All state lives in plain markdown files -- no databases, no JSON blobs. Everything is human-readable and human-editable.

| Path | Contents |
|------|----------|
| `GOAL.md` | Your project description (you write this) |
| `.ai/interview.md` | Clarifying questions and your answers |
| `.ai/design/overview.md` | Architecture overview |
| `.ai/design/decisions.md` | Technical decisions |
| `.ai/sprints/sprint-NNN.md` | Sprint task lists with checkbox progress |
| `.ai/skills/*.md` | Agent skill prompts (auto-generated + custom) |
| `.ai/logs/` | Full agent invocation logs |

## Built-in skills

Agate generates language-specific skills automatically (e.g. `go-coder`, `go-reviewer` for Go projects). It also ships built-in skills prefixed with `_`:

| Skill | Purpose |
|-------|---------|
| `_planner` | Creates sprint plans and task breakdowns |
| `_reviewer` | Reviews implementation, gates task completion |
| `_recover` | Diagnoses and fixes environment after agent failures |
| `_replanner` | Rewrites failing tasks when review fails repeatedly |
| `_interviewer` | Generates clarifying questions during planning |
| `_retro` | Runs sprint retrospectives |

Sprint tasks reference skills by name (`- [ ] go-coder: implement X`). You can add custom skills as `.md` files in `.ai/skills/`.

## Install

```bash
go install github.com/strongdm/agate@latest
```

Or build from source: `go build -o agate .`

Requires [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli). [Codex CLI](https://github.com/openai/codex) is optional.

## License

Apache License 2.0. See [LICENSE](LICENSE).
