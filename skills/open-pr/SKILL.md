---
name: open-pr
description: Open a pull request for the current ticket branch with a clear description, and link it to the tracker issue. Use after implement-ticket has finished and make check passes.
license: MIT
metadata:
  invocation: manual
---

# Open a pull request

1. Confirm the branch is neither the base branch (see `AGENTS.md`) nor `main`.
2. Fetch and rebase onto the latest base branch, then confirm `make check` passes. The gate expects every test and scenario on the base branch to pass on your branch. If you rebase a branch that you already pushed, push it with `--force-with-lease`. Do this again before every later handoff.
3. Title: the ticket id (the issue key, if there is one) and a short imperative summary.
4. Description: what changed, why, how it was tested, scenarios covered, anything a reviewer should look at first.
5. Open the PR against the base branch. Do not merge it. The gate and a human decide.
6. Add the PR link to the tracker issue: as a link if the tracker tools can do that, otherwise as a comment or in the issue text.
7. Set the issue to Done (the tracker's "completed" state) only after the gate reports that it merged the PR. `BLOCKED` or `NEEDS APPROVAL` is not a merge.
