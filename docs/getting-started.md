# Getting started

From `git clone` to merged, gate-checked code.

## Branches
- `main` is the kit: skills, gate, scripts, docs. You never build your product on it.
- Your product lives on a **project branch** (for example `project`) that is the repository's **default branch** on GitHub. Agents branch from it and open pull requests against it (see `AGENTS.md`).
- A ruleset named `gate` covers the default branch and `main`: nothing lands there without a pull request and a green `gate` status, and nobody can bypass it.

## Setup
You need git, Go, Docker, the GitHub CLI (`gh auth login`), make and curl.

    git clone <this repository>
    cd ai-engineering-kit
    scripts/setup.sh

The script walks you through every step and skips what is already done. `scripts/setup.sh --check` only reports. What it does:

1. **Your own repository.** If `origin` is someone else's, it creates `<you>/ai-engineering-kit` (public by default, because rulesets on private repositories need GitHub Pro), renames the old remote to `upstream` and pushes `main`.
2. **Project branch.** Creates it from `main` and makes it the default branch.
3. **Ruleset `gate`** on the default branch and `main`.
4. **Two fine-grained tokens**, limited to your repository. It shows what to click and checks each token before saving it:

   | | Agent token | Gate token |
   |---|---|---|
   | Contents | read and write | read and write |
   | Pull requests | read and write | read and write |
   | Commit statuses | **no access** | read and write |

   The agent can push branches and open pull requests, but cannot post the `gate` status, so it cannot get its own work merged. Details: `docs/github-setup.md`.
5. **Check image** `ai-engineering-checks`, in which the gate runs every check without network or secrets.
6. **Where your agent runs**: on this machine, or in a Docker sandbox (the gate then listens on the Docker bridge; a firewall may need a rule, the script prints it).
7. **The gate**: its own checkout and binary, plus three helper scripts, all in `~/.ai-engineering-kit/<owner>__<repo>/`, outside the repository.
8. **MCP**: registers the gate in Claude Code for this project; Jira (Atlassian) is optional.

Everything secret stays in `~/.ai-engineering-kit/`. If your agent runs in a sandbox, do not mount that directory into it.

## Every session
    ~/.ai-engineering-kit/<owner>__<repo>/start-gate.sh          # terminal 1, leave it running
    ~/.ai-engineering-kit/<owner>__<repo>/agent-env.sh claude    # terminal 2, in the repository

`agent-env.sh` runs the agent with only the agent token. In a sandbox, pass the agent token as `GITHUB_TOKEN` instead. In Claude, `/mcp` must show `gate` as connected (and `atlassian` if you use Jira).

If your git remote uses SSH, pushes go through your own SSH key, not the agent token. The ruleset still protects the default branch and `main`; switch the remote to HTTPS if you want the token to be the only credential.

A fresh project later (for example a new course run): `scripts/setup.sh --new-project` creates another branch from `main` and makes it the default; restart the gate.

## The workflow
The default branch must always be green: the gate requires a pass for every test and scenario that is already on it. So red scenarios never land alone; they land together with the code that makes them pass.

1. **Product.** Write `docs/product.md` (agent drafts, humans edit). Branch `ticket-0-product`, pull request, the agent calls `gate_merge`. Docs only, so every check passes: the first, boring, green merge.
2. **Spec.** `write-spec` turns the product into `scenarios/*.feature`, `write-steps` adds `app/*_steps_test.go`. `make bdd` is red, as intended. Push as a **draft** pull request: this is where humans review and agree on the requirements. Do not merge it.
3. **Decisions.** `record-decision` for anything costly to change (storage, API shape). ADRs can merge on their own (docs only).
4. **Roadmap.** `plan-roadmap` reads the spec branch and creates epics and tickets in Jira (tell it the project key), or writes `docs/roadmap.md` without Jira. Each ticket carries its scenarios verbatim. Keep one ticket = one feature file, so a ticket can add its whole file and make it green. Close the spec pull request.
5. **Implement.** Per ticket: `implement-ticket` (branch from the default branch, add the ticket's `.feature`, steps and code, `make check` green), then `open-pr`.
6. **Review.** The `review` skill on the pull request, ideally by a second agent.
7. **Gate.** The agent calls `gate_merge` with the branch name. The gate checks in a container, posts the status `gate` and merges. Watch the gate's terminal.
8. **The hard rule.** Ask the agent to "fix" an existing scenario (for example `health.feature`). The gate answers `NEEDS APPROVAL`. You decide: `approve.sh approvals`, then `approve.sh approve <PR>` or `approve.sh deny <PR> --reason "..."`.

## When the gate blocks
- Adding tests and scenarios never needs approval. Modifying, renaming or deleting ones already on the default branch does, and so do protected paths (`Makefile`, `scripts/`, `gate/`, `skills/`, `.claude/`, `.github/`, `.golangci.yml`, `AGENTS.md`, `CLAUDE.md`).
- So whatever is on the project branch when you start is the approved baseline. Remove what you do not want (for example the health scenario) through a pull request you approve, before the real work starts.
- To relax the gate for a while, a human can turn single checks off: `approve.sh checks`, `approve.sh disable lint`, `approve.sh enable lint` (see `gate/README.md`).
- Coverage: 70 percent total, 80 percent of changed lines. Ask the agent to run `make check` before `open-pr`.
- A red default branch blocks every pull request. Never merge red scenarios.
- Inside some sandboxes `make check` cannot pass the gate's own tests (a git wrapper that refuses pushes to `main`, even to a throwaway local repository). The gate's container is the authority.
