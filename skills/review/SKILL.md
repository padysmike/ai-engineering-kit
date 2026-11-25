---
name: review
description: Review a pull request in phases (context, per-PR reviewers, cross-cutting lenses, synthesis), verify every finding against the repository, and report only majors and blockers using semantic labels. Use before the gate and before a human review.
license: MIT
metadata:
  invocation: manual
---

# Review

Report findings only. Do not edit code. Post comments only in the synthesis phase, and only when a human asked for posting.

## Phase 0: context
- Read the originating context first (ticket, thread or incident that prompted the PR). Note the exact behaviour the change must deliver, what broke, for whom, on which code path.
- Fetch the PR metadata and head SHA (needed to anchor comments) before starting any parallel work.
- Read `AGENTS.md`, `docs/conventions.md` and the linked scenarios.

## Phase 1: per-PR reviewers
- Review each PR separately, ideally with a separate agent or pass per distinct angle (security and reliability, fail-fast pragmatism, CI and build minimalism).
- The very first action of each reviewer is to fetch the actual diff. Never reason from assumptions.
- Reviewers report findings back; they do not post.

## Phase 2: cross-cutting lenses
- Take 3 to 5 lenses across all PRs: concurrency and distributed systems, API and contract compatibility, edge cases and data integrity, tests and observability. Give each the Phase 0 context.
- This phase surfaces incomplete remediation and sibling-path gaps. Do not skip it when a PR fixes a specific incident or touches shared code.
- If you cannot run parallel agents, run the lenses one after another yourself.

## Phase 3: synthesis
- Dedupe findings across lenses and drop what fails verification.
- Rank by severity and record consensus: how many lenses found each finding independently.
- Post survivors as inline comments on file and line. If a real major lies outside the diff (a pre-existing sibling path left unfixed, an incomplete remediation, a paired-PR inconsistency), post it as one clearly labelled PR-level comment. Never invent an inline anchor.
- Report only majors and blockers. No nits, no style opinions.

## Verify before flagging
- Check each finding against the repository, not only the diff. Is it the established convention? Does a comparable module do it differently? Flag only deviations you can defend, and cite the comparable file and line.
- Label every finding INTRODUCED (caused by this diff) or PRE-EXISTING (already there). Pre-existing issues that share the root cause the PR set out to fix are worth raising as follow-ups, not blockers.
- Incomplete-remediation check (highest value): does the change resolve the originating problem, and does a parallel code path share the same root cause? Trace where the bad value originates and every producer and consumer. Confirm that a flagged path is reachable.
- Paired-PR consistency: if PRs span both sides of a contract (producer and consumer), check they agree.
- Compute the line number in the new version of the file by walking the hunk headers: count added and context lines, skip removed lines.

## Semantic labels
Begin every comment with one of:
- Remark: no change or answer expected.
- Question: an honest question, an answer is expected.
- Hint: nothing to change now, worth knowing for the future.
- Suggestion: consider it; accepting is the author's call.
- Important: a change is expected; if you strongly disagree, discuss.
- Crucial: must not be merged; severe issue.

## Comment style
- Formal, compact, a few sentences. No emoji.
- State the mechanism (what breaks, and under what conditions) and, where useful, a direction for the fix. Say plainly when an issue is pre-existing or outside the diff.
- Be fair: credit a real improvement before naming a residual gap.
- End with a sign-off naming the model that wrote the review, for example `Reviewed by <model name>`. Do not invent a human persona.

## Mechanics
- Use whatever GitHub access the environment provides (a CLI, an MCP server or the REST API with a token). Never print tokens.
- Inline comment: pull request review comment with body, head commit SHA, path, line and side RIGHT. PR-level comment: issue comment on the pull request.
- Write JSON payloads to a temporary file to avoid quoting problems. Confirm each post from the returned URL. If the API rejects the line, retry on a nearby changed line, or fall back to a PR-level comment.

## Final report
- Per PR: label, file and line (or PR-level), comment URL, and consensus per finding.
- State plainly whether the PRs fix the originating problem, and whether a sibling path leaves it partly open.
- List what you deliberately did not flag and why (repo convention, pre-existing and out of scope, improvement with no residual gap).
- If there are no majors or blockers, say so and post nothing. Do not invent findings.
