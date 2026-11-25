# Go test conventions

## Naming

| Shape | Name |
|---|---|
| Case inside a table-driven test | `WHEN <condition> THEN <result>` |
| Extra condition or extra result | join with `AND` |
| Standalone test, one case | `TestName_Condition_ExpectedResult` |

```go
// table cases
{name: "WHEN the cache is empty THEN the lookup returns not found"}
{name: "WHEN the user is blocked AND the token is valid THEN access is denied"}

// standalone
func TestValidate_TimeoutIsZero_BootFails(t *testing.T)
```

The table function keeps a plain subject name (`TestInvalidateCache`), because the conditions live in the cases.

## Body

Three blocks separated by one blank line: arrange, act, assert. Nothing else divides them.

```go
func TestValidate_TimeoutAboveCeiling_BootFails(t *testing.T) {
	cfg := validConfig()
	cfg.Timeout = 400 * time.Millisecond

	err := cfg.Validate()

	require.Error(t, err)
}
```

No comments inside the body unless really needed. The name already states the condition and the result. Assertion messages carry extra context better than comments, because they appear only on failure.

A comment above the function is fine for the reason the test exists when that is not obvious: the incident it pins or the invariant it defends.

## Scenarios

Each BDD scenario in `scenarios/` reads Given / When / Then, with And continuing a line. Scenarios are approved requirements; agents add new ones and never edit existing ones.

## Why

Case names become the failure output. `--- FAIL: TestX/WHEN_the_cache_is_empty_THEN_the_lookup_returns_not_found` names the broken behaviour without opening the file. `--- FAIL: TestX/empty` does not.
