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
8. **MCP**: registers the gate in Claude Code for this project. It also offers a tracker: Linear (the default), Jira, or none. Log in once with `/mcp`.

Everything secret stays in `~/.ai-engineering-kit/`. If your agent runs in a sandbox, do not mount that directory into it.

## Every session
    ~/.ai-engineering-kit/<owner>__<repo>/start-gate.sh          # terminal 1, leave it running
    ~/.ai-engineering-kit/<owner>__<repo>/agent-env.sh claude    # terminal 2, in the repository

`agent-env.sh` runs the agent with only the agent token. In a sandbox, pass the agent token as `GITHUB_TOKEN` instead. In Claude, `/mcp` must show `gate` as connected, and your tracker if you use one (`linear`, or `atlassian` for Jira).

If your git remote uses SSH, pushes go through your own SSH key, not the agent token. The ruleset still protects the default branch and `main`; switch the remote to HTTPS if you want the token to be the only credential.

A fresh project later (for example a new course run): `scripts/setup.sh --new-project` creates another branch from `main` and makes it the default; restart the gate.

## The workflow
The default branch must always be green: the gate requires a pass for every test and scenario that is already on it. So red scenarios never land alone; they land together with the code that makes them pass.

1. **Product.** Write `docs/product.md` (agent drafts, humans edit). Branch `ticket-0-product`, pull request, the agent calls `gate_merge`. Docs only, so every check passes: the first, boring, green merge.
2. **Spec.** `write-spec` turns the product into `scenarios/*.feature`: 2 to 4 scenarios per file, numbered in build order and tagged PoC or MVP. Commit them on their own branch, for example `ticket-0-spec`, and push it. This is where humans review and agree on the requirements. The branch is **never merged** and needs no pull request: it has no steps or code, so its scenarios would be red.
3. **Contract and UI.** Before the first implementation ticket, `record-decision` writes one ADR for the API contract and one for the UI (screens and flows), based on the spec branch. Add ADRs for anything else that is costly to change, such as storage. A human accepts them. ADRs can merge on their own (docs only), from a branch off the default branch.
4. **Roadmap.** `plan-roadmap` reads the spec branch and the ADRs and creates the plan in your tracker (tell it which project), or writes `docs/roadmap.md` without one. In Linear, that is one project with the milestones PoC and MVP; in Jira, epics for the same phases. One issue = one feature file, and each issue carries its own file word for word.
5. **Implement.** Per ticket: `implement-ticket` (the issue moves to In Progress; branch from the default branch; add the feature file from the issue, its steps (`write-steps`) and the code; `make check` green; rebase onto the latest default branch before handing off), then `open-pr` (the issue gets the PR link). The issue key is the `<id>` in the branch name.
6. **Review.** The `review` skill on the pull request, ideally by a second agent.
7. **Gate.** The agent calls `gate_merge` with the branch name, and `gate_result` while the answer is `RUNNING`. The gate checks in a container, posts the status `gate` and merges. Watch the gate's terminal. After the merge, the agent sets the issue to Done.
8. **The hard rule.** Ask the agent to "fix" an existing scenario (for example `health.feature`). The gate answers `NEEDS APPROVAL`. You decide: `approve.sh approvals`, then `approve.sh approve <PR>` or `approve.sh deny <PR> --reason "..."`.

## When the gate blocks
- Adding tests and scenarios never needs approval. Modifying, renaming or deleting ones already on the default branch does, and so do protected paths (`Makefile`, `scripts/`, `gate/`, `skills/`, `.claude/`, `.github/`, `.golangci.yml`, `AGENTS.md`, `CLAUDE.md`).
- So whatever is on the project branch when you start is the approved baseline. Remove what you do not want (for example the health scenario) through a pull request you approve, before the real work starts.
- To relax the gate for a while, a human can turn single checks off: `approve.sh checks`, `approve.sh disable lint`, `approve.sh enable lint` (see `gate/README.md`).
- Coverage: 70 percent total, 80 percent of changed lines. Ask the agent to run `make check` before `open-pr`.
- `approved tests missing on head: rebase onto <base>`: the base gained tests since the branch split off. Rebase, push and run the gate again.
- A red default branch blocks every pull request. Never merge red scenarios.
- Inside some sandboxes `make check` cannot pass the gate's own tests (a git wrapper that refuses pushes to `main`, even to a throwaway local repository). The gate's container is the authority.
