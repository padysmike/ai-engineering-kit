# Conventions

- Language: English in code, docs, specs, ADRs, skills and commit messages.
- Go: standard `gofmt`, `go vet`; small packages; errors wrapped with context. Style: `docs/code-style.md`. Tests: `docs/testing.md`.
- One ticket = one branch = one pull request. Branch name: `ticket-<id>-<slug>`; `<id>` is the issue key if you use a tracker.
- Commits: imperative mood, subject up to 72 characters, body explains why.
- Scenarios in `scenarios/` are approved requirements. Add new ones; never edit existing ones without a human.
- Decisions that are hard to reverse get an ADR in `docs/adr/`.
- No secrets in the repository. Configuration comes from environment variables; see `.env.example` when it exists.
