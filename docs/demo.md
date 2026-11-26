# Running the demo

How to present the kit live: from a one-page product idea to a ticket merged by the gate. Setup and the workflow are explained in `docs/getting-started.md`; this page is the script.

Commands below use `STATE=~/.ai-engineering-kit/<owner>__<repo>`.

## Before the day: rehearse
- Run the whole script once, on its own project branch (see "Reset for the next run").
- Write down how long each step took and correct the timings below.
- Keep what the rehearsal produced: the merged pull requests and the spec branch. They are your fallback.
- Reset afterwards, so the live run starts clean.

## Pre-flight checklist
On the same machine and network that you will present from:

- [ ] `main` on GitHub holds the version of the kit you want to show. A new project branch starts from it.
- [ ] `scripts/setup.sh --check` shows no `[todo]` or `[fail]`. It also tests both tokens, so an expired one shows here.
- [ ] The gate was rebuilt from a version of the kit that includes KIT-4f, so a branch cannot replace the checks with its own `GNUmakefile`.
- [ ] If `gate/` on the project branch changed since the gate was built, run `scripts/setup.sh` again. It rebuilds the gate; `--check` does not tell you that it is old.
- [ ] If `gate/checks.Dockerfile` changed since you built the check image, rebuild it (`gate/README.md`).
- [ ] `$STATE/start-gate.sh` runs in its own terminal and names your new project branch.
- [ ] `$STATE/approve.sh checks` shows every check on.
- [ ] The local checkout is on the project branch, up to date, with no uncommitted changes.
- [ ] `make check` passes where the agent runs. In a sandbox where the gate's own tests cannot run, `make check SKIP_GATE_TESTS=1` passes (see `docs/getting-started.md`).
- [ ] A new agent session: `/mcp` shows `gate` and your tracker (`linear` or `atlassian`) as connected.
- [ ] The agent has no memory of the rehearsal. If it keeps memory between sessions, clear it for this project.
- [ ] The tracker has an **empty** project for this run, and you know its name.
- [ ] No `ticket-*` branch from an earlier run is left on GitHub or in your checkout.
- [ ] No token or secret is on any screen you will share. Terminals and the browser use a font readable from the back row.
- [ ] Browser tabs are open: the repository's pull requests, the tracker project and the rehearsal results.

## Live script
Planned for 60 minutes. These timings are estimates until you replace them with your rehearsal's. The agent works for minutes at a time: use that time to explain what it is doing and why it cannot cheat.

Type one prompt per step and read the result before you go on.

| Time | Step | You type | The audience sees |
|---|---|---|---|
| 0:00–0:05 | Intro | | Skills, gate, soft and hard rules (`docs/guardrails.md`) |
| 0:05–0:10 | Product | "Write `docs/product.md` from `docs/examples/live-qa-product.md`. Use branch `ticket-0-product`, open a pull request and call `gate_merge`." | The first green merge, and the gate's log |
| 0:10–0:18 | Spec | "/write-spec for `docs/product.md`. Commit on branch `ticket-0-spec` and push it." | Numbered feature files tagged PoC and MVP, and open questions. Answer one or two. |
| 0:18–0:26 | Contract and UI | "/record-decision: the API contract and the UI, based on the spec on `ticket-0-spec`." Read both, then: "I accept them. Set them to accepted, open a pull request and call `gate_merge`." | A human decides, the agent writes it down |
| 0:26–0:32 | Roadmap | "/plan-roadmap in the tracker project `<name>`." Read the plan, then approve it. | Milestones and issues, each with its feature file |
| 0:32–0:47 | First ticket | "/implement-ticket `<issue key>`." Approve its plan if it asks. Then "/open-pr". | The issue moves to In Progress, a red scenario turns green, the issue gets the PR link |
| 0:47–0:52 | Review and gate | In a second agent session: "/review pull request `<number>`." Then: "Call `gate_merge`." | Findings, the checks in the gate's terminal, the merge, the issue set to Done |
| 0:52–0:57 | The hard rule | See below. | `NEEDS APPROVAL`, and you deny it |
| 0:57–1:00 | Wrap-up | | What the agent could do, and what only a human could |

To use the audience's ideas instead, collect them on one page and give that to the agent in the product step. Plan extra time, and note that your rehearsal results then show a different product.

If you are late, shorten the spec and the review. Keep the gate and the hard rule: they are the point.

### The hard rule
This needs `scenarios/health.feature` on the project branch. Ask the agent:

> I want this for a demo. On a new branch from the latest default branch, delete the line `And the body contains "status":"ok"` from `scenarios/health.feature`. Open a pull request and call `gate_merge`.

The agent may object, because `AGENTS.md` forbids the edit. That is the soft rule; tell it to go on. The scenario still passes with the weaker check, so every other check can be green. The gate answers `NEEDS APPROVAL` all the same. If it answers `BLOCKED`, something else failed too: show the reason.

    $STATE/approve.sh approvals
    $STATE/approve.sh deny --reason "the scenario must keep checking the body" <PR>

Put `--reason` before the pull request number: written after it, the reason is silently lost.

Ask the agent to call `gate_merge` again. It gets `BLOCKED` with your reason. Close the pull request.

## Fallbacks
Decide in advance how long you will wait, for example three minutes without progress. Then show the rehearsal's result and say that it is from the rehearsal.

| Problem | What to do |
|---|---|
| The agent is slow, stuck or rate limited | Show the rehearsal's pull request for the same step. Let the agent work on in the background. |
| The tracker is not available | `plan-roadmap` writes `docs/roadmap.md` when no tracker tools are connected. Give the agent the feature file from the spec branch as the ticket. |
| `gate` is not connected in `/mcp` | Check that `start-gate.sh` is running. For a sandbox, check the firewall rule that `setup.sh` printed. Or run the gate yourself (below). |
| The gate answers `RUNNING` for a long time | That is normal for a slow run: the agent calls `gate_result` until the result arrives. Watch for the `finished` line in the gate's terminal. If you restarted the gate, look at the pull request before you call `gate_merge` again (`gate/README.md`). |
| A check fails for a reason you do not want to debug live | Read the reason aloud. As a last resort, `$STATE/approve.sh disable <check>`, say that you did it, and turn it on again right after with `enable`. |
| No network | Show the rehearsal from local material: screenshots or a recording that you made in advance. `make bdd` and `make run` still work if the modules are already downloaded. |

To run the gate by hand (a human only, in a terminal the agent cannot see, because it reads the gate token):

    GATE_GITHUB_TOKEN="$(cat $STATE/secrets/gate-github-token)" $STATE/gate pr \
      --repo $STATE/checkout --github OWNER/REPO --base <project-branch> --branch <branch> \
      --sandbox-image ai-engineering-checks --state $STATE/approvals --merge

## Reset for the next run
A person does this, not an agent.

1. Stop the gate. Close the pull requests that are still open.
2. Move the old run's branches out of the way, or the new run will collide with them. `ticket-0-product` and `ticket-0-spec` have the same names every time. For each branch you want to keep, copy it first:

       git fetch origin
       git push origin origin/ticket-0-spec:refs/heads/rehearsal/ticket-0-spec
       git push origin --delete ticket-0-spec

   Delete the other old `ticket-*` branches, also locally. Merged pull requests stay readable on GitHub.
3. Commit or discard local changes, then run:

       scripts/setup.sh --new-project

   Give a name that has **never been used**, for example one with the date. If a branch with that name exists, the script reuses it as it is. A new name creates a branch from `main`, makes it the default and rebuilds the gate from it.
4. Create a new, empty project in the tracker.
5. Start the gate again with `$STATE/start-gate.sh`, and start a new agent session.
6. Go through the pre-flight checklist.
