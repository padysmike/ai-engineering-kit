---
name: write-spec
description: Turn a plain-language product idea or client conversation into BDD scenarios (Gherkin, English) stored in scenarios/. Use when requirements need to be formalised before planning or coding.
license: MIT
metadata:
  invocation: manual
---

# Write a spec

1. Read `docs/product.md` and the existing files in `scenarios/` so you do not duplicate or contradict them.
2. Split the idea into features. One `.feature` file per feature, with 2 to 4 scenarios. If a feature needs more, split it: one file is one ticket and one pull request.
3. Put the files in build order and name them `NN-short-name.feature` (kebab-case), for example `01-create-room.feature`. A file may rely only on behaviour from earlier files.
4. Split the files into PoC and MVP with a tag on the line above `Feature:`, `@poc` or `@mvp`. The PoC files come first and prove the riskiest assumption. With the MVP files, the product is the smallest useful one. Use a third tag, `@finished`, only if a human asks for work after the MVP.
5. For each behaviour write a `Scenario` with `Given` / `When` / `Then`. Use concrete values, no vague words like "quickly".
6. Keep scenarios independent and testable: each one sets up its own state. If something cannot be tested, say why in a comment.
7. List open questions at the end of your answer instead of guessing.
8. Do not modify existing scenarios. If one seems wrong, report it to a human.
9. Do not write step definitions or code. The spec lives on its own branch (for example `ticket-0-spec`), which is never merged. Later, each ticket adds one feature file together with its steps and code.
10. Prefer one concrete scenario per example over Scenario Outline unless the table is genuinely clearer.
