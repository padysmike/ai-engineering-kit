# Code style

## Comments

Do not write comments unless genuinely needed. A comment is a sign of bad design or poor naming: rename or restructure instead.

Given the choice between a comment explaining a unit, a constraint or an invariant, and a longer identifier that states it, pick the identifier.

| Instead of | Write |
|---|---|
| `Timestamp` plus a comment "epoch millis" | `TimestampMillis` |
| a comment promising a reader is never reused | a function that builds its own reader, so reuse is impossible |

- Do not write comments that document the absence of something (a removed metric, a rejected approach). Git history and ADRs cover that.
- Keep a short comment only where naming cannot carry the meaning: a non-obvious ordering constraint, or a contract imposed by an external library.
- In a config format there is nothing to rename. Put the explanation in the prose file next to it, and keep beside the config only what a future editor would otherwise undo.

## Matching the surroundings

Existing code in a repository may be heavily commented. Do not copy that density; this rule wins over the neighbouring files' habits.

## Principles and where they come from

Sources, in order of authority for this repository: the Go standard (`gofmt`, `go vet`, [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)), then Clean Code, KISS, YAGNI and DRY. Effective Go dates from 2009 and does not cover generics or modules, so where it conflicts with Code Review Comments or current practice, the latter wins. Where Clean Code tolerates comments, the Comments section above wins.

### Go idioms (Effective Go)
- Let `gofmt` decide formatting. Never hand-format.
- Packages: short, lowercase, one word. Do not repeat the package name in exported identifiers (`bufio.Reader`, not `bufio.BufReader`). Constructor of the main type is `New`.
- No `Get` prefix on getters (`Owner()`, `SetOwner()`). MixedCaps, never underscores. One-method interfaces are named method + `er` (`Reader`).
- Handle errors and special cases first and return early. Do not write `else` after a branch that returns. The success path runs straight down the left edge.
- Scope temporaries with init statements: `if err := f(); err != nil { ... }`.
- Check every error; never discard with `_` without a reason a reviewer can see from the name or the code. Wrap with context (operation and subject), and let callers inspect with `errors.Is` and `errors.As`.
- `panic` only for impossible states. Libraries return errors.
- Make the zero value useful, so `var v T` works without a constructor.
- Pair acquire and release with `defer` immediately after acquiring.
- Share memory by communicating. A value is owned by one goroutine at a time. Bound concurrency with a fixed worker count, and make every goroutine's exit condition explicit.
- Small interfaces (one or two methods), defined by the consumer, accepted as parameters. Return concrete types from constructors unless a type exists only to implement an interface.
- Prefer composition and embedding over inheritance-like hierarchies.

### Readability (Clean Code)
- Names reveal intent and are pronounceable and searchable. A longer accurate name beats a comment (see Comments).
- Functions do one thing and stay at one level of abstraction. If you can extract a function whose name is not a restatement of its body, extract it.
- Few parameters. Zero to three is normal; more means a missing type. No boolean flag arguments: two functions, or a named option.
- Command-query separation: a function either changes state or returns information, not both.
- No hidden side effects. Name what a function does, including what it changes.
- Flatten nesting with guard clauses. Deep nesting is a signal to extract or invert a condition.
- Replace magic numbers and strings with named constants.
- Leave the code cleaner than you found it, in the lines you touch, and only those (see YAGNI on scope).
- Tests: fast, independent, repeatable, self-validating, written alongside the change (`docs/testing.md`).

### KISS
- Choose the simplest design that satisfies the scenarios. If two designs work, take the one a new reader understands first.
- Prefer the standard library and plain functions over frameworks, reflection and generics-for-their-own-sake. Introduce a dependency only when it removes more code than it adds, and record it in an ADR when it is hard to reverse.
- A clever one-liner that needs study is worse than three plain lines.

### YAGNI
- Build what the current ticket and its scenarios need. No unused parameters, options, config keys, hooks or extension points "for later".
- No interface until there are two implementations or a consumer that needs a test double. No generics until there are two concrete types that actually share the algorithm.
- Delete dead code instead of commenting it out. Git remembers.
- Do not refactor unrelated code in the same pull request. Note it as a follow-up.

### Duplication (DRY, applied with judgement)
- Duplicate knowledge is the problem, not duplicate text. Two blocks that look alike but change for different reasons stay separate.
- Rule of three: tolerate duplication twice, extract on the third occurrence, when the shape of the abstraction is visible.
- A wrong abstraction costs more than duplication. If an extracted function accumulates flags and special cases, inline it again and re-extract along the real seam.
- Extract with a name that states the shared concept, not the mechanics (`validateOrder`, not `processStuff`).
- Test helpers and table-driven tests are the preferred way to remove duplication in tests, within the naming rules of `docs/testing.md`.

### Review checklist
1. Does every function fit on one screen and do one thing?
2. Could a better name replace any comment?
3. Is there code for a need the scenarios do not state?
4. Is the same knowledge expressed in two places?
5. Is the simplest working design used?
6. Are errors handled once, with context, at the right level?
