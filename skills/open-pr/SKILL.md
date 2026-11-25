---
name: open-pr
description: Open a pull request for the current ticket branch with a clear description. Use after implement-ticket has finished and make check passes.
license: MIT
metadata:
  invocation: manual
---

# Open a pull request

1. Confirm the branch is not `main` and `make check` passes.
2. Title: the ticket id and a short imperative summary.
3. Description: what changed, why, how it was tested, scenarios covered, anything a reviewer should look at first.
4. Open the PR against `main`. Do not merge it. The gate and a human decide.
