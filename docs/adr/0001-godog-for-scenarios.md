# 0001. godog runs the BDD scenarios

- Status: accepted
- Date: 2026-11-25

## Context
Scenarios must be executable, otherwise they are documentation that drifts. godog is the standard Cucumber implementation for Go and runs inside `go test`.

Maintenance check on 2026-11-25 (repository history): latest release v0.16.0 on 2026-08-01, before that v0.15.1 (2025-07), v0.15.0 (2024-10) and v0.14.1 (2024-04), so about one release a year. Since January 2026 there are 19 non-bot commits from a handful of maintainers; the rest is dependency bumps. The project lives in the official `cucumber` organisation, is still below 1.0 (the API may change) and its own CLI is deprecated in favour of `go test`.

## Decision
Use godog, pinned in `go.mod` and `go.sum`. Feature files stay in `scenarios/`; step definitions are Go test files next to the code, registered through a small registry so each feature gets its own file. The suite runs in strict mode.

## Consequences
- Scenario Outline, tables, doc strings and tags are available.
- One external dependency with transitive ones; it must be reachable on first download, so commit `go.sum`.
- Existing step files are protected by the gate (`*_test.go`), so new features add new step files.
- Slow release cadence and a small maintainer group are a long-term risk. The `.feature` files are plain Gherkin, so switching runner would only touch the step registration.
