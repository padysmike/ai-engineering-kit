# gate

A small Go program that decides whether a branch may be merged into `main`.

## Trust model
- It runs on the host, outside the agent's sandbox. The agent has no merge token.
- The checks execute the branch's own code (tests, build, lint), so they never run in the gate's process or environment. Each `make` target runs in a throwaway container with no network, no capabilities, none of the gate's environment variables, a read-only root filesystem and only three mounts: the worktree (read-only, so one target cannot rewrite the Makefile for the next), the diff, and a read-only module cache. Everything the checks write goes to a tmpfs that disappears with the container. The gate reads the exit code and the output. The gate's tokens and the approval state directory are never visible to the branch's code.
- It trusts only the base branch. It restores `Makefile` and `scripts/` from the base branch before running any check, so a branch cannot weaken its own checks.
- It blocks when a branch changes a protected path (`Makefile`, `scripts/`, `gate/`, `skills/`, `.claude/`, `.github/`, `.golangci.yml`, `AGENTS.md`, `CLAUDE.md`).
- It blocks when a branch modifies, renames or deletes an approved test or scenario (`scenarios/`, `*_test.go`). Adding new ones is allowed.
- With `--merge` it merges (`--no-ff`) only if every check passes.

## Check image
Build the image the checks run in once, and rebuild it when the Go or linter version changes:

    docker build -t ai-engineering-checks -f gate/checks.Dockerfile gate

Every command that runs checks needs `--sandbox-image ai-engineering-checks`. The gate downloads modules on the host first (`go mod download` executes no module code), then runs the targets with `GOPROXY=off`. `--host-checks` runs the targets on the host instead, with only an allowlisted environment. The branch's code then runs as the host user, so it can read host files, rewrite the shared worktree between targets and write into the approval state. The gate refuses `--host-checks` together with `--state`, `--github` or `--allow-merge`; use it only for local experiments on code you trust.

## Usage
    make gate-build
    bin/gate --sandbox-image ai-engineering-checks --branch ticket-1-health            # check only
    bin/gate --sandbox-image ai-engineering-checks --branch ticket-1-health --merge    # check, then merge
    bin/gate --sandbox-image ai-engineering-checks --branch x --allow-test-changes     # a human approved test changes
    bin/gate --branch x --allow-protected        # a human approved changes to protected paths

Exit codes: 0 passed, 1 blocked, 2 usage or internal error.

## Limits
A new or changed `TestMain` is held for human approval, because it controls the test process and can hide failing tests. Code that runs inside the test process can still forge its own output, so tests that pass are evidence, not proof; review and mutation testing cover the rest. The image pins the Go and golangci-lint versions; golangci-lint must be built with a Go version at least as new as the image's, otherwise `REQUIRE_LINT=1` blocks every pull request.

The gate catches weakened checks, not necessarily vacuous tests. Review and mutation testing cover that. Keep the binary and the repository checkout outside the agent's reach.

## MCP server
`gate mcp` serves the gate over MCP (stdio) using the official Go SDK, `github.com/modelcontextprotocol/go-sdk`, so an agent can request a check or a merge without any rights of its own.

    bin/gate mcp --repo /path/to/checkout                 # tool: gate_check
    bin/gate mcp --repo /path/to/checkout --allow-merge   # adds gate_merge

- Run it on the host, outside the agent's sandbox, and give the agent only the connection to it.
- The agent supplies only a branch name, validated against a strict pattern. The repository, base branch, check list, `--allow-test-changes` and `--allow-protected` are set by whoever starts the server.
- `gate_merge` is not offered unless `--allow-merge` is passed. A merge still happens only when every check passes.
- Example client entry: `{"mcpServers":{"gate":{"command":"/path/to/bin/gate","args":["mcp","--repo","/path/to/checkout"]}}}`.

## GitHub mode
With `--github owner/repo` and `GATE_GITHUB_TOKEN`, the gate works on pull requests instead of local branches: it fetches the branch and the base from the remote, runs the checks, posts the commit status `gate` on the exact head SHA, and with `--merge` merges the pull request through the API with that SHA as a guard. The CLI form is `gate pr`; the MCP server takes the same flags. See `docs/github-setup.md` for tokens and the ruleset.

## Running as a service
`gate mcp --listen 127.0.0.1:8765` serves MCP over HTTP instead of stdio. It requires `GATE_MCP_TOKEN` and checks it as a bearer token on every request.

### Who is asking

Every request is logged to stderr, one line when it arrives and one when it finishes:

```
2026-12-03T13:41:07Z request tool=gate_check branch=ticket-12 caller="builder-1" client="claude-code/2.1" agent="..."
2026-12-03T13:41:19Z finished tool=gate_check branch=ticket-12 caller="builder-1" exit=1 took=12s
```

`caller` is the name of the bearer token that was used. Give each agent its own token with `GATE_MCP_TOKENS=builder-1=<secret>,reviewer=<secret>`; `GATE_MCP_TOKEN` alone is named `default`. Over stdio the caller is `stdio`. `client` is the name and version the MCP client announced about itself and `agent` is its `User-Agent`; both are self-reported, so only `caller` can be trusted.

## Checks
| Target | What it enforces |
|---|---|
| `fmt-check`, `vet` | formatting and `go vet` |
| `lint` | `golangci-lint` with `.golangci.yml`: errcheck, function length (40 lines, 30 statements), cyclomatic complexity (12) |
| `test` | `go test -race ./...` |
| `bdd` | the Gherkin scenarios |
| `coverage` | total statement coverage, minimum 70 percent |
| `diff-coverage` | coverage of statements changed against the base branch, minimum 80 percent (`scripts/diffcover`) |
| `validate-skills` | structure of `skills/*/SKILL.md` |

The gate exports `GATE_BASE_SHA` so `diff-coverage` compares against the trusted base commit. `make vuln` (govulncheck) is not part of the gate because it needs network access.

## Human approval
Changes to approved tests, scenarios and protected paths need a human. With `--state DIR` the gate does not just block: it stores a request tied to the pull request and head commit, posts the status `pending`, and returns `NEEDS APPROVAL` (exit code 3) without waiting.

    bin/gate approvals
    bin/gate approve <branch or PR number>
    bin/gate deny <branch or PR number> --reason "..."

Only these commands, run on the host, decide. See `docs/workflow.md`.
