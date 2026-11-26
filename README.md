# ai-engineering-kit

Materials for the lecture "Ze života developera v AI době" (a day in the life of a developer in the AI era). A small, tool-neutral kit for building software with AI agents under hard guardrails.

## What is here
| Path | Purpose |
|---|---|
| `skills/` | Tool-neutral skills (Agent Skills format): spec, ADR, roadmap, scaffold, implement, PR, review, logs |
| `gate/` | Go merge gate that runs on the host and trusts only the base branch |
| `scenarios/` | BDD requirements (approved; agents may not edit them) |
| `app/*_steps_test.go` | godog step definitions for the scenarios, next to the code they exercise |
| `docs/` | conventions, guardrails, ADRs, product description |
| `app/` | skeleton of the product built during the lecture |
| `scripts/` | `setup.sh` for your copy, and the check scripts used by the Makefile |

## Start here
    git clone https://github.com/padysmike/ai-engineering-kit.git
    cd ai-engineering-kit
    scripts/setup.sh

The script walks you through everything: your own copy of the repository, a project branch, the GitHub ruleset, two tokens, the merge gate and the MCP connections for your agent: the gate and, if you like, a tracker (Linear by default, or Jira). Run it again at any time; `scripts/setup.sh --check` only reports what is missing. Then read `docs/getting-started.md`. To present the kit live, follow `docs/demo.md`.

`main` holds the kit. Your product lives on its own project branch, which setup makes the default branch.

## Working on the kit itself
    make check          # needs Go 1.26+ (required by the MCP SDK); golangci-lint is optional
    make gate-build
    docker build -t ai-engineering-checks -f gate/checks.Dockerfile gate
    bin/gate --sandbox-image ai-engineering-checks --branch <branch>

## Skills in other tools
Skills live once, in `skills/`; Claude Code finds them through `.claude/skills`. Point other tools at that directory (symlink or copy into `.claude/skills`, `.codex/skills`, `.github/skills`, `.gemini/skills`, or use the `skills` CLI). Validate with `make validate-skills`.

## License
MIT
