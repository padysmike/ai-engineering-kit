# gate

A small Go program that decides whether a branch may be merged into `main`.

## Trust model
- It runs on the host, outside the agent's sandbox. The agent has no merge token.
- The checks execute the branch's own code (tests, build, lint), so they never run in the gate's process or environment. Each `make` target runs in a throwaway container with no network, no capabilities, none of the gate's environment variables, a read-only root filesystem and only three mounts: the worktree (read-only, so one target cannot rewrite the Makefile for the next), the diff, and a read-only module cache. Everything the checks write goes to a tmpfs that disappears with the container. The gate reads the exit code and the output. The gate's tokens and the approval state directory are never visible to the branch's code.
- It trusts only the base branch. It restores `Makefile` and `scripts/` from the base branch before running any check, so a branch cannot weaken its own checks.
- It blocks when a branch changes a protected path (`Makefile`, `scripts/`, `gate/`, `skills/`, `.claude/`, `.github/`, `.golangci.yml`, `AGENTS.md`, `CLAUDE.md`).
- It blocks when a branch modifies, renames or deletes an approved test or scenario (`scenarios/`, `*_test.go`). Adding new ones is allowed.
- With `--merge` it merges (`--no-ff`) only if every check passes.

## A branch behind its base
The gate checks the branch as it is. It does not merge the base into it first, yet every test and scenario on the base must report a pass. So when the base gained tests after the branch split off, and the branch lacks them, the gate fails at once, without running the make targets:

    [FAIL] 7 approved tests missing on head: rebase onto test

Rebase the branch onto that base, push and run the gate again. A branch that is only behind, with no tests missing, is checked as usual. Disabling `approved-tests-pass` turns this check off too.

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
The exit code of a test run is not trusted: code in the branch can end the test process with status 0 before a single test runs (an `init()` that calls `os.Exit(0)` is enough). So the gate also runs `go test -json` itself and requires a `pass` event for every test function and every scenario that exists on the base branch (`TestFeatures/<scenario name>`). The list comes from parsing the base branch, not from the branch under check. Files with a `//go:build` line are skipped, because the default run does not compile them. A test removed with human approval is no longer required.

Added lines in any Go file that take control of the test process (`TestMain`, `syscall.Exit`, `testing.Testing()`, `flag.Lookup("test.`) are held for human approval. This is a tripwire for review, not a guarantee: the same effect can be written in ways no pattern matches. When a branch adds test files or scenarios, `test`, `bdd` and the pass check run on a copy of the worktree without those new files, so the approved tests run alone and the new files cannot hook into them. Code that runs inside the test process can still forge the result: printing `--- PASS: TestHealthy (0.00s)` lines and exiting with status 0 produces genuine-looking pass events. The check raises the bar from an accidental or lazy exit to a deliberate forgery that is plain to see in review. Tests that pass are evidence, not proof; review and mutation testing cover the rest. The image pins the Go and golangci-lint versions; golangci-lint must be built with a Go version at least as new as the image's, otherwise `REQUIRE_LINT=1` blocks every pull request.

The gate catches weakened checks, not necessarily vacuous tests. Review and mutation testing cover that. Keep the binary and the repository checkout outside the agent's reach.

## MCP server
`gate mcp` serves the gate over MCP (stdio) using the official Go SDK, `github.com/modelcontextprotocol/go-sdk`, so an agent can request a check or a merge without any rights of its own.

    bin/gate mcp --repo /path/to/checkout                 # tools: gate_check, gate_result
    bin/gate mcp --repo /path/to/checkout --allow-merge   # adds gate_merge

A gate run can take longer than an MCP client waits for a tool call. So `gate_check` and `gate_merge` run in the background: they answer with the result if the run ends within 10 seconds, and with `RESULT: RUNNING` otherwise. `gate_result` with the same branch waits up to 10 seconds more and answers the same way, so call it until the result arrives. It starts nothing.

- A branch has one run at a time. Repeating the running request joins it, so it cannot start a second merge. The other request is refused until the run ends.
- Once a run has ended, `gate_check` or `gate_merge` starts a new one. `gate_result` keeps answering with the latest run, which may be older than the latest commit.
- Runs are kept in memory. After a restart, `gate_result` knows of no run: call `gate_check` or `gate_merge` again. A run cut off by the restart may or may not have merged; look at the pull request.

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

`make check SKIP_GATE_TESTS=1` leaves `gate/` out of `test`, `coverage` and `diff-coverage` on a machine where these tests cannot run, for example an agent sandbox that refuses local git pushes. `vet` and `lint` still cover `gate/`. It is a local switch only: the gate does not pass the variable to its checks. After a change in `gate/`, run `go test ./gate` where it works.

The gate exports `GATE_BASE_SHA` so `diff-coverage` compares against the trusted base commit. `make vuln` (govulncheck) is not part of the gate because it needs network access.

## Turning checks off
A human on the host can turn single checks off, for example to relax the gate for a demo:

    bin/gate checks                 # list the checks and whether each is on
    bin/gate disable lint
    bin/gate enable lint

- The checks are the make targets `fmt-check`, `vet`, `lint`, `test`, `bdd`, `coverage`, `diff-coverage` and `validate-skills`, plus `approved-tests-pass` (the check that every test and scenario on the base reports a pass).
- `lint` includes the function length and complexity limits.
- Disabling `test` or `bdd` also skips its run with the approved tests alone. `coverage`, `diff-coverage` and `approved-tests-pass` run tests too, so a failing test can still block through them.
- The checks for protected paths, changed approved tests and test process control cannot be turned off. Human approval covers their exceptions.
- The setting is a file in the state directory, so it applies only to runs started with `--state`, and the next run reads it without a restart. The commands use `--state`, else `GATE_STATE_DIR`, else `~/.gate`. Put `--state DIR` before the check name. If you used `scripts/setup.sh`, run them through `approve.sh`, which points at the right directory.
- A skipped check prints `[skip]`, and the GitHub status names it, for example `gate passed (skipped: lint)`.
- If the file cannot be read or names an unknown check, the gate does not run.
- There is no MCP tool for this, so an agent cannot change it. As with approvals, keep the state directory outside everything the agent can reach.

## Human approval
Changes to approved tests, scenarios and protected paths need a human. With `--state DIR` the gate does not just block: it stores a request tied to the pull request and head commit, posts the status `pending`, and returns `NEEDS APPROVAL` (exit code 3) without waiting.

    bin/gate approvals
    bin/gate approve <branch or PR number>
    bin/gate deny <branch or PR number> --reason "..."

Only these commands, run on the host, decide. See `docs/workflow.md`.
