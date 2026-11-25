---
name: triage-logs
description: Read application logs or error output, group the errors, find the likely cause and propose a fix as a ticket or a patch. Use when the running app misbehaves.
license: MIT
metadata:
  invocation: manual
---

# Triage logs

1. Read the log excerpt the human provides. Do not fetch logs from production unless told to.
2. Group similar errors, count them, note the first occurrence.
3. For each group state the likely cause with the evidence (log line, code location).
4. Propose the smallest fix, and a regression test. Create a ticket or branch only when asked.
