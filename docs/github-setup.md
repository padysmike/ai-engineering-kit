# GitHub setup for the gate

Goal: the agent can push branches and open pull requests, but only the gate can get a pull request merged. Both tokens come from one GitHub account, so the separation rests on token scope and on a ruleset, not on two identities.

## Tokens
Create two fine-grained personal access tokens, both limited to this one repository, both with a short expiry.

| | Agent token (given to harness-docker) | Gate token (stays on the host) |
|---|---|---|
| Metadata | read | read |
| Contents | read and write | read and write |
| Pull requests | read and write | read and write |
| Commit statuses | no access | read and write |
| Workflows, Administration, others | no access | no access |

The gate token is exported only in the shell that starts the gate: `export GATE_GITHUB_TOKEN=...`. Never put either token in the repository, in `.env.example` or in a file mounted into the sandbox.

## Ruleset on `main`
Settings, Rules, Rulesets, new branch ruleset targeting `main`:
- Restrict deletions, block force pushes.
- Require a pull request before merging.
- Require status checks to pass: add the status `gate`.
- Bypass list: **empty**. With one account, any bypass entry for you would apply to the agent token too.

Why this works: the agent token has no Commit statuses permission, so it cannot post `gate`. The status is bound to a commit SHA, so a new push invalidates it. The gate posts `success` only for the exact SHA it checked, and merges with that SHA as a guard, so GitHub refuses the merge if the branch moved in between.

The agent token can technically call the merge endpoint, but GitHub refuses until the `gate` status is green. Keep `CODEOWNERS` review as a second requirement if you want a human on protected paths.

## Guarding a branch other than `main`
For a demo or a release line, start the gate with `--base BRANCH` (for example `--base demo`) and protect that branch the same way:
- Create the ruleset with the target branch `demo` (or a pattern such as `release/*`). The rules are the same: pull request required, status `gate` required, no force push, no deletion, empty bypass list.
- The gate checks against the trusted base it is configured with and refuses a pull request that targets any other branch, so a PR cannot borrow the gate's approval for an unguarded branch.
- One gate guards one base branch. To guard two branches, run two gates (different ports and state directories), or protect only the branch you demonstrate on.

## Running the gate as a service
The gate runs on the host, not in a container. Only the check commands run in containers; build the image first (see `gate/README.md`).

On the host, in a checkout the agent cannot write to:

    export GATE_GITHUB_TOKEN=...      # gate token
    export GATE_MCP_TOKEN=...         # random secret the agent presents
    # or one token per agent, so the log shows who asked:
    # export GATE_MCP_TOKENS=builder-1=...,reviewer=...
    bin/gate mcp --repo ~/gate-checkout --github OWNER/REPO --allow-merge --listen 127.0.0.1:8765 --state ~/.gate --sandbox-image ai-engineering-checks

Point harness-docker's MCP configuration at `http://host.docker.internal:8765/mcp` with the header `Authorization: Bearer <GATE_MCP_TOKEN>`. On Docker Desktop for Mac the container reaches a loopback-bound host service this way. On Linux the container needs the bridge address instead of `host.docker.internal`.

The checkout used by the gate is a separate clone. The gate fetches the pull request branch and `main` from the remote, checks them in a temporary worktree with the Makefile and scripts from `main`, then posts the status and merges through the API.

## Without MCP
    GATE_GITHUB_TOKEN=... bin/gate pr --sandbox-image ai-engineering-checks --github OWNER/REPO --branch ticket-1-health            # check, post status
    GATE_GITHUB_TOKEN=... bin/gate pr --sandbox-image ai-engineering-checks --github OWNER/REPO --branch ticket-1-health --merge    # and merge

## Limits
- A fine-grained token cannot be limited to certain branches. The ruleset does that.
- One account means one audit trail; commits and statuses look like the same user. A separate bot account is cleaner later.
- If `GATE_MCP_TOKEN` leaks, anyone who can reach the port can request checks and merges of branches that pass. Keep it bound to loopback.
