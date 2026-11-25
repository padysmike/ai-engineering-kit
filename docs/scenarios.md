# Scenarios and steps

Requirements live as Gherkin files in `scenarios/*.feature` and run with [godog](https://github.com/cucumber/godog) as an ordinary Go test (`TestFeatures` in `app/scenarios_test.go`). `make bdd` runs only the scenarios; `make check` and the gate run them too. The suite is strict: an undefined or pending step fails.

## Step definitions
- Live next to the code they exercise, in `app/<feature>_steps_test.go`, one file per feature.
- Register them in `init()` with `registerSteps(func(sc *godog.ScenarioContext){ ... })`. godog calls the initializer once per scenario, so variables declared inside it are fresh state for each scenario.
- A step is an anchored regular expression (`^...$`). Capture groups are passed to the function as arguments (`string`, `int`, ...). A step function returns `error` or nothing.

```go
func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		var rec *httptest.ResponseRecorder

		sc.Step(`^a client requests GET (\S+)$`, func(path string) {
			rec = httptest.NewRecorder()
			newServer().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		})
	})
}
```

## Rules for agents
- Never edit an existing `.feature` file or an existing `*_steps_test.go`. The gate blocks it. Add a new feature file and a new steps file instead.
- Steps must call real code and assert something. A step that returns nil without checking is a vacuous test and will be caught in review.
- Use `godog` snippets from the failing run (`make bdd` prints suggested step stubs for undefined steps) as the starting point.

## Setup
`go get github.com/cucumber/godog@latest && go mod tidy`, then commit `go.mod` and `go.sum`.
