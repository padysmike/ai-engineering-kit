# Lecture runbook

How to run the kit live: from an empty product idea to merged, gate-checked code.

## Moving parts
| Part | Where it runs | What it holds |
|---|---|---|
| Agent (Claude Code in harness-docker) | sandbox container | agent GitHub token, gate MCP token, Jira OAuth login |
| Gate (`bin/gate mcp`) | host, `~/gate/` | gate GitHub token, approval state `~/.gate` |
| aimebu | host service | message bus between agents and humans |
| GitHub ruleset | GitHub | requires status `gate` on the default branch and `main` |

The base branch is always the **GitHub default branch** (see `AGENTS.md`). The ruleset targets `~DEFAULT_BRANCH` too, so switching base means: change the default branch, restart the gate with `--base`.

## One-time setup
1. Tokens (github.com, Settings, Developer settings, Fine-grained tokens), both limited to this repository, short expiry. Permissions are in `docs/github-setup.md`.
   - Agent token: Contents RW, Pull requests RW. Save to `~/.config/ai-engineering-kit/agent-github-token`.
   - Gate token: Contents RW, Pull requests RW, Commit statuses RW. Save to `~/.config/ai-engineering-kit/gate-github-token`.
2. Check image: `docker build -t ai-engineering-checks -f gate/checks.Dockerfile gate`.
3. Gate checkout outside the sandbox mount: `~/gate/ai-engineering-kit` (a separate clone), binary built with `make gate-build` there.
4. Restart harness-docker with the agent token, so the sandbox never sees the broad one:

        GITHUB_TOKEN=$(cat ~/.config/ai-engineering-kit/agent-github-token) harness-docker-ctrl stop
        GITHUB_TOKEN=$(cat ~/.config/ai-engineering-kit/agent-github-token) harness-docker-ctrl start

5. MCP servers for this project (local scope, in the Claude config the sandbox mounts): `gate` (HTTP on the docker bridge, bearer token) and `atlassian` (Jira, OAuth). Log in to Atlassian once on the host with `claude` then `/mcp`; the sandbox reuses the login.

## Before each session
    ~/gate/start-gate.sh test        # or the lecture branch; leave this terminal open
    claude-docker                    # in the repo, in another terminal

In Claude, `/mcp` must show `gate`, `atlassian` and `aimebu` as connected.

## Switching to a fresh lecture branch
Start from the clean setup commit (tag `kit-ready`), not from a rehearsed `test`:

    git push origin kit-ready^{commit}:refs/heads/lecture
    gh api -X PATCH repos/padysmike/ai-engineering-kit -f default_branch=lecture
    ~/gate/start-gate.sh lecture

## The demo, step by step
The base branch must always be green: the gate requires a pass for every test and scenario that is already on it. So red scenarios never land alone; they land together with the code that makes them pass.

1. **Product.** With the audience, write `docs/product.md` (agent drafts, humans edit). Branch `ticket-0-product`, PR, ask the agent to call `gate_merge`. Docs only, so every check passes: the first, boring, green merge.
2. **Spec.** `write-spec` turns the product into `scenarios/*.feature`, `write-steps` adds `app/*_steps_test.go`. `make bdd` is red, as intended. Push as a **draft** PR: this is where humans review and agree on the requirements. Do not merge it.
3. **Decisions.** `record-decision` for anything costly to change (storage, API shape). ADRs can merge on their own (docs only).
4. **Roadmap.** `plan-roadmap` reads the spec branch and creates epics and tickets in Jira project `QAAT`. Each ticket carries its scenarios verbatim. Keep one ticket = one feature file, so a ticket can add its whole file and make it green. Close the spec PR.
5. **Implement.** Per ticket: `implement-ticket` (branch from the default branch, add the ticket's `.feature`, steps and code, `make check` green), then `open-pr`.
6. **Review.** `review` skill on the PR (optional, a second agent over aimebu works well).
7. **Gate.** The agent calls `gate_merge` with the branch name. The gate checks in a container, posts status `gate`, merges. Show the gate terminal log.
8. **Show the hard rule.** Ask the agent to "fix" an existing scenario (for example change `health.feature`). The gate answers `NEEDS APPROVAL`. In the host terminal: `bin/gate approvals`, then `bin/gate approve <PR>` or `bin/gate deny <PR> --reason ...`.

## When the gate blocks
- Adding tests and scenarios never needs approval. Modifying, renaming or deleting ones already on the base does, and so do protected paths.
- So "before we start" means: whatever is on the base branch when the lecture begins is the approved baseline. Anything you want gone (for example the health scenario) must be removed on the setup commit, not during the demo.
- Coverage gates: 70 percent total, 80 percent of changed lines. Ask the agent to run `make check` before `open-pr`.
- A red default branch blocks every PR. Never merge red scenarios.
