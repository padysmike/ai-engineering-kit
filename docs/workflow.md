# Changing approved tests and scenarios

Approved scenarios (`scenarios/`) and existing tests (`*_test.go`) are the agent's fixed requirements. Protected paths (`Makefile`, `GNUmakefile`, `makefile`, `scripts/`, `gate/`, `skills/`, `.golangci.yml`, `.github/`, `AGENTS.md`, `CLAUDE.md`) are fixed too. The gate does not let a branch change them without a human. Adding new tests and scenarios is always allowed.

## What the agent does
- A requirement changes: propose the new or replacement scenario as an **added** file and say which old scenario it replaces. Do not edit or delete the old one.
- A test is wrong: report it in the pull request and leave it as it is.
- If the gate answers `NEEDS APPROVAL`, do not retry in a loop and do not work around it. Continue with other tickets, and call the gate again later.

## Approval flow (does not block other work)
1. The agent or the orchestrator calls `gate_merge` (or `gate_check`).
2. The gate finds an approvable change and every other check is green. It posts the status `gate` as `pending` on the pull request, stores a request on the host, prints `approval requested: ...` in the service terminal, and returns `NEEDS APPROVAL` at once. Other jobs keep running.
3. You decide in another terminal:

        bin/gate approvals                 # list requests with files and kinds
        bin/gate approve ticket-7          # by branch, or by pull request number: bin/gate approve 7
        bin/gate deny ticket-7 --reason "keep the old wording"

4. The next `gate_merge` finds the decision. After approval it runs all checks again and merges. After denial it returns `BLOCKED` with your reason.

The approval belongs to the pull request **and to its head commit**. A new push to the PR is a new request, so nothing can be added after approval.

If other checks fail too, no request is created. The agent has to fix those first.

## Where the state lives
The service stores requests in the directory given by `--state` (default `~/.gate`, or `GATE_STATE_DIR`). Keep it on the host, outside everything mounted into the sandbox. The MCP server has no tool that approves or denies, so the agent cannot decide for itself.

## One-off approval without the flow
    bin/gate pr --sandbox-image ai-engineering-checks --github OWNER/REPO --branch ticket-7 --allow-test-changes --merge

The same flags `--allow-test-changes` and `--allow-protected` exist for the local mode. They are only for a human at a terminal.

## Why not a label or a comment
Both tokens belong to one GitHub account, so a label, comment or review cannot prove that a human approved. The decision is a command on the host.
