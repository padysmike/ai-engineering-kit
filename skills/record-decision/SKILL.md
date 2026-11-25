---
name: record-decision
description: Record a hard-to-reverse design decision as an Architecture Decision Record in docs/adr/. Use when choosing a database, framework, protocol or any structure that is costly to change.
license: MIT
metadata:
  invocation: manual
---

# Record a decision

1. Copy `docs/adr/0000-template.md` to `docs/adr/NNNN-short-title.md`, where NNNN is the next free number.
2. Fill Context, Decision and Consequences. Name at least one rejected alternative and why.
3. Set the status to `proposed`. A human changes it to `accepted`.
4. Link the ADR from the ticket or pull request that needs it.
