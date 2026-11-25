# ai-engineering-kit

Materials for the lecture "Ze života developera v AI době" (a day in the life of a developer in the AI era). A small, tool-neutral kit for building software with AI agents under hard guardrails.

## What is here
| Path | Purpose |
|---|---|
| `skills/` | Tool-neutral skills (Agent Skills format): spec, ADR, roadmap, scaffold, implement, PR, review, logs |
| `gate/` | Go merge gate that runs on the host and trusts only `main` |
| `scenarios/` | BDD requirements (approved; agents may not edit them) |
| `app/*_steps_test.go` | godog step definitions for the scenarios, next to the code they exercise |
| `docs/` | conventions, guardrails, ADRs, product description |
| `app/` | skeleton of the product built during the lecture |
| `scripts/` | check scripts used by the Makefile |

## Quick start
    go mod tidy         # first time only; commit go.mod and go.sum
    make check          # needs Go 1.26+ (required by the MCP SDK); golangci-lint is optional
    make gate-build
    docker build -t ai-engineering-checks -f gate/checks.Dockerfile gate
    bin/gate --sandbox-image ai-engineering-checks --branch <branch>

## Skills in other tools
Skills live once, in `skills/`. Point your tool at that directory (symlink or copy into `.claude/skills`, `.codex/skills`, `.github/skills`, `.gemini/skills`, or use the `skills` CLI). Validate with `make validate-skills`.

## License
MIT
