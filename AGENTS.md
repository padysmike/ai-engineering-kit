# AGENTS.md

Instructions for any AI coding agent working in this repository.

## Commands
- `make check` runs everything: format, vet, lint (including function length and complexity), tests with the race detector, scenarios, total coverage, coverage of changed lines, skill validation.
- `make vuln` runs govulncheck (needs network); run it before a release.
- `make bdd` runs only the scenarios in `scenarios/`.
- `make run` starts the app on `:8080`.

## Base branch
The base branch is the repository's default branch on GitHub: `gh repo view --json defaultBranchRef -q .defaultBranchRef.name`. Branch from its latest commit and open pull requests against it. `main` is the kit's source; target it only when it is the default branch.

## Rules
- One ticket = one branch (`ticket-<id>-<slug>`) = one pull request. Never commit to the base branch or to `main`.
- Never merge. The gate (`gate/`) merges, run by a human on the host.
- Do not edit approved scenarios in `scenarios/` or existing tests. Adding new ones is fine.
- Do not edit `Makefile`, `scripts/`, `gate/`, `skills/`, `.golangci.yml`, `.github/`, `AGENTS.md`, `CLAUDE.md`. If you think they need a change, ask a human.
- Specs, ADRs, code, comments and commit messages are in English.

## Where things are
- `docs/conventions.md`: workflow and general conventions.
- `docs/code-style.md`: code style (Effective Go, Clean Code, KISS, YAGNI, duplication rules) and comments policy. Read before writing code.
- `docs/architecture.md`: hexagonal and DDD rules; start flat, add layers only when a trigger appears.
- `docs/testing.md`: Go test naming (WHEN/THEN) and structure. Read before writing tests.
- `docs/workflow.md`: how approved tests and scenarios are changed. `docs/guardrails.md`: soft vs hard rules. `docs/github-setup.md`: tokens and ruleset for the gate.
- `docs/adr/`: design decisions. `scenarios/`: BDD requirements. `skills/`: reusable procedures.
- `app/`: the product. `gate/`: the merge gate.
- Each scenario file needs step definitions: see `skills/write-steps/SKILL.md` and `docs/scenarios.md`. Scenarios run with godog.
