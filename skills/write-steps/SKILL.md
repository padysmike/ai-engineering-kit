---
name: write-steps
description: Write executable step definitions for BDD scenarios so they run with godog. Use after write-spec, and whenever a scenario reports an undefined step.
license: MIT
metadata:
  invocation: manual
---

# Write step definitions

1. Read `docs/scenarios.md` and one existing `app/*_steps_test.go` file as the pattern.
2. Run `make bdd`. godog prints suggested stubs for undefined steps.
3. Create `app/<feature>_steps_test.go` for the new feature. Never edit an existing steps file or an existing `.feature` file.
4. Register steps inside `init()` with `registerSteps(func(sc *godog.ScenarioContext){...})`, using anchored patterns `^...$`. Keep per-scenario state in variables declared inside the registered function.
5. Reuse a step that already matches before adding a new one, and avoid patterns that overlap, because ambiguous steps fail.
6. Every step must exercise real code and assert something. `Then` steps must fail when the behaviour is wrong.
7. Run `make bdd`. It is expected to fail until the feature is implemented; report which scenarios are red and why.
