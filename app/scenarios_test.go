package main

import (
	"testing"

	"github.com/cucumber/godog"
)

var stepRegistrars []func(*godog.ScenarioContext)

func registerSteps(r func(*godog.ScenarioContext)) { stepRegistrars = append(stepRegistrars, r) }

func initializeScenario(sc *godog.ScenarioContext) {
	for _, r := range stepRegistrars {
		r(sc)
	}
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../scenarios"},
			Strict:   true,
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("scenarios failed")
	}
}
