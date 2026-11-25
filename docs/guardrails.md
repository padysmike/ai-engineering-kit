# Guardrails: soft vs hard rules

Heuristics generate, determinism decides.

| | Soft rules | Hard rules |
|---|---|---|
| Examples | `AGENTS.md`, skills, prompts, model choice | gate on the host (checks in a throwaway container), sandbox, CODEOWNERS, no merge token for the agent |
| Enforced by | the model's cooperation | something the agent cannot change |
| Failure mode | ignored or misread | blocks the merge |

Tool-specific deny rules and hooks are convenient, but they only protect against that one tool. The gate does not depend on the tool.

See `gate/README.md` for the trust model.
