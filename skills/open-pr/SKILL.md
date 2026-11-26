---
name: open-pr
description: Open a pull request for the current ticket branch with a clear description, and link it to the tracker issue. Use after implement-ticket has finished and make check passes.
license: MIT
metadata:
  invocation: manual
---

# Open a pull request

1. Confirm the branch is neither the base branch (see `AGENTS.md`) nor `main`, and `make check` passes.
2. Title: the ticket id (the issue key, if there is one) and a short imperative summary.
3. Description: what changed, why, how it was tested, scenarios covered, anything a reviewer should look at first.
4. Open the PR against the base branch. Do not merge it. The gate and a human decide.
5. Add the PR link to the tracker issue: as a link if the tracker tools can do that, otherwise as a comment or in the issue text.
6. Set the issue to Done (the tracker's "completed" state) only after the gate reports that it merged the PR. `BLOCKED` or `NEEDS APPROVAL` is not a merge.
