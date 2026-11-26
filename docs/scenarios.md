# Scenarios and steps

Requirements live as Gherkin files in `scenarios/*.feature` and run with [godog](https://github.com/cucumber/godog) as an ordinary Go test (`TestFeatures` in `app/scenarios_test.go`). `make bdd` runs only the scenarios; `make check` and the gate run them too. The suite is strict: an undefined or pending step fails.

## Step definitions
- Live next to the code they exercise, in `app/<feature>_steps_test.go`, one file per feature.
- Register them in `init()` with `registerSteps(func(sc *godog.ScenarioContext){ ... })`. godog calls the initializer once per scenario.
- A step is an anchored regular expression (`^...$`). Capture groups are passed to the function as arguments (`string`, `int`, ...). A step function returns `error` or nothing.

## Per-scenario state
All step files share one helper, `perScenario` in `app/scenario_state_test.go`. For each kind of state, declare a type, a `perScenario` variable and an accessor. Any step file can then call the accessor and reach the same state within one scenario.

```go
type visit struct{ response *httptest.ResponseRecorder }

var visits = perScenario[visit]{}

func visitFor(sc *godog.ScenarioContext) *visit {
	return visits.of(sc, func() *visit { return &visit{} })
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		current := visitFor(sc)

		sc.Step(`^a visitor opens (\S+)$`, func(path string) {
			current.response = httptest.NewRecorder()
			newServer().ServeHTTP(current.response, httptest.NewRequest(http.MethodGet, path, nil))
		})
		sc.Step(`^the page is found$`, func() error {
			if current.response.Code != http.StatusOK {
				return fmt.Errorf("status = %d, want %d", current.response.Code, http.StatusOK)
			}
			return nil
		})
	})
}
```

## Rules for agents
- Never edit an existing `.feature` file or an existing `*_steps_test.go`. The gate blocks it. Add a new feature file and a new steps file instead.
- Steps must call real code. `When` steps record the result and `Then` steps assert it; a `Then` step that returns nil without checking is a vacuous test and will be caught in review.
- No buffering wrappers and no duplicate steps. The `write-steps` skill explains the rules.
- Use `godog` snippets from the failing run (`make bdd` prints suggested step stubs for undefined steps) as the starting point.

## Setup
`go get github.com/cucumber/godog@latest && go mod tidy`, then commit `go.mod` and `go.sum`.
