---
name: implement-ticket
description: Implement one ticket on its own branch following the project's rules, with a plan first and tests alongside, and keep its tracker issue up to date. Use when a human assigns a single ticket.
license: MIT
metadata:
  invocation: manual
---

# Implement a ticket

A ticket is usually an issue in the tracker, for example Linear. Without a tracker, skip the tracker steps.

1. Read `AGENTS.md`, `docs/conventions.md`, the ADRs, the ticket and its scenarios. If the ADR for the API contract or the UI is missing or not accepted, tell a human before you write code. A product without an API or a UI does not need that ADR.
2. Write a short plan and wait for approval when running in manual mode.
3. Set the issue to In Progress (the tracker's "started" state).
4. Create branch `ticket-<id>-<slug>` from the latest base branch (see `AGENTS.md`). `<id>` is the issue key, for example `ticket-ABC-12-room-lifecycle`.
5. If the ticket carries a feature file, add it to `scenarios/` word for word and write its steps with the `write-steps` skill.
6. Implement in small steps. Add tests for new behaviour. Never edit existing tests or scenarios to make them pass.
7. Run `make check` until it passes. Never change the Makefile, scripts or gate to make it pass.
8. Commit. Before you hand off, fetch, rebase onto the latest base branch and run `make check` again. Do this at every handoff, also after review fixes.
9. Stop. Opening the pull request is a separate step; never merge.
