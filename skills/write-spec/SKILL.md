---
name: write-spec
description: Turn a plain-language product idea or client conversation into BDD scenarios (Gherkin, English) stored in scenarios/. Use when requirements need to be formalised before planning or coding.
license: MIT
metadata:
  invocation: manual
---

# Write a spec

1. Read `docs/product.md` and the existing files in `scenarios/` so you do not duplicate or contradict them.
2. Split the idea into features. One `.feature` file per feature, named in kebab-case.
3. For each behaviour write a `Scenario` with `Given` / `When` / `Then`. Use concrete values, no vague words like "quickly".
4. Keep scenarios independent and testable. If something cannot be tested, say why in a comment.
5. List open questions at the end of your answer instead of guessing.
6. Do not modify existing scenarios. If one seems wrong, report it to a human.
7. Then use the `write-steps` skill so every new scenario has step definitions. A scenario with undefined steps fails `make check`, which is intended: it is the red state before implementation.
8. Prefer one concrete scenario per example over Scenario Outline unless the table is genuinely clearer.
