package main

import (
	"testing"

	"github.com/cucumber/godog"
)

// The map is unguarded: godog must run the scenarios one at a time.
type perScenario[T any] map[*godog.ScenarioContext]*T

func (p perScenario[T]) of(sc *godog.ScenarioContext, create func() *T) *T {
	state, ok := p[sc]
	if !ok {
		state = create()
		p[sc] = state
	}
	return state
}

func TestPerScenario(t *testing.T) {
	first, second := &godog.ScenarioContext{}, &godog.ScenarioContext{}
	tests := []struct {
		name        string
		asking      *godog.ScenarioContext
		shared      bool
		wantCreated int
	}{
		{"WHEN the same scenario asks again THEN it gets the same state AND nothing new is created", first, true, 1},
		{"WHEN another scenario asks THEN it gets its own state", second, false, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			states, created := perScenario[int]{}, 0
			create := func() *int { created++; return new(int) }
			initial := states.of(first, create)

			got := states.of(tc.asking, create)

			if (got == initial) != tc.shared || created != tc.wantCreated {
				t.Errorf("shared = %t, created = %d, want %t and %d", got == initial, created, tc.shared, tc.wantCreated)
			}
		})
	}
}
