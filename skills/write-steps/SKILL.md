---
name: write-steps
description: Write executable step definitions for BDD scenarios so they run with godog. Use while implementing a ticket that adds a feature file, and whenever a scenario reports an undefined step.
license: MIT
metadata:
  invocation: manual
---

# Write step definitions

1. Read `docs/scenarios.md`, the helper in `app/scenario_state_test.go` and the existing `app/*_steps_test.go` files.
2. Run `make bdd`. godog prints suggested stubs for undefined steps.
3. Create `app/<feature>_steps_test.go` for the new feature. Never edit an existing steps file or an existing `.feature` file.
4. Register steps inside `init()` with `registerSteps(func(sc *godog.ScenarioContext){...})`, using anchored patterns `^...$`.
5. Follow the rules below.
6. Every step must exercise real code. `Then` steps must fail when the behaviour is wrong.
7. Run `make bdd`. Before the feature is implemented it is expected to fail; report which scenarios are red and why.

## Rules
A merged step file is hard to change, because that needs a human. If you can see the later feature files, check that they can reuse your steps, for example when the same action is refused.

- **When steps record, Then steps assert.** A `When` step does the action and keeps the result, success or refusal, without judging it. A `Then` step checks it. A `Given` step prepares state and fails if it cannot.
- **Per-scenario state lives in one shared helper.** Keep it in a type of your own, stored with `perScenario` and reached through an accessor such as `roomFor(sc)`, so that other step files can use it. No state in local variables of the registered function, and no second helper of the same kind.
- **No buffering wrappers.** A test wrapper around a handler, writer or connection passes everything on at once, including flushes. A wrapper that collects output and hands it over later hides streaming bugs.
- **No duplicate steps.** Search all step files first and reuse a step that already matches. Do not add a pattern that overlaps another: an ambiguous step fails.
