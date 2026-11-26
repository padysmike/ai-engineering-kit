---
name: record-decision
description: Record a hard-to-reverse design decision as an Architecture Decision Record in docs/adr/. Use when choosing a database, framework, protocol or any structure that is costly to change, and for the API contract and the UI before the first implementation ticket.
license: MIT
metadata:
  invocation: manual
---

# Record a decision

1. Copy `docs/adr/0000-template.md` to `docs/adr/NNNN-short-title.md`, where NNNN is the next free number.
2. Fill Context, Decision and Consequences. Name at least one rejected alternative and why.
3. Set the status to `proposed`. A human changes it to `accepted`.
4. Link the ADR from the ticket or pull request that needs it.

## Before the first implementation ticket
Decide both of these in advance, so that tickets do not each invent their own. Check them against every feature file in the spec.

- **API contract:** every operation with its request, response and errors, how a caller is identified, and how live updates arrive, if there are any.
- **UI:** the screens, what each one shows, and the flows between them, including error and empty states.

Write one ADR for each. Leave one out only if the product has no API or no UI. A human accepts them before the first ticket starts.
