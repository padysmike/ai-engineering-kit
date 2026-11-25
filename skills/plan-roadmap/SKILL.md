---
name: plan-roadmap
description: Turn the BDD scenarios into a phased roadmap (PoC, MVP, finished product) with epics and tickets in the project tracker, normally Jira through an MCP server. Use after the scenarios are written and before implementation starts.
license: MIT
metadata:
  invocation: manual
---

# Plan the roadmap

1. Read `scenarios/` and `docs/adr/`.
2. Group scenarios into phases: PoC (proves the riskiest assumption), MVP (smallest useful product), finished product.
3. For each phase create epics, and for each epic small tickets. A ticket should fit in one pull request.
4. Every ticket links the scenarios it satisfies, copied verbatim, plus acceptance criteria and dependencies.
5. Create the items through the tracker tools available to you (for example a Jira MCP server). If none is available, write `docs/roadmap.md` instead and say so.
6. Show the plan to a human before creating more than ten items.
