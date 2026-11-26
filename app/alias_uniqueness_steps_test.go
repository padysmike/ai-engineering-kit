package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
)

type attendeeCrowd struct {
	host      *roomHost
	responses []*httptest.ResponseRecorder
}

var attendeeCrowds = perScenario[attendeeCrowd]{}

func crowdFor(sc *godog.ScenarioContext) *attendeeCrowd {
	return attendeeCrowds.of(sc, func() *attendeeCrowd { return &attendeeCrowd{host: hostFor(sc)} })
}

func (c *attendeeCrowd) join(count int) {
	path := "/rooms/" + c.host.rooms[0].Code + "/participants"
	for range count {
		rec := httptest.NewRecorder()
		c.host.server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		c.responses = append(c.responses, rec)
	}
}

func (c *attendeeCrowd) expectDifferentAliases(want int) error {
	aliases := map[string]bool{}
	for i, rec := range c.responses {
		if rec.Code != http.StatusCreated {
			return fmt.Errorf("join %d: status = %d, want %d", i+1, rec.Code, http.StatusCreated)
		}
		var view participantView
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			return fmt.Errorf("join %d: decode participant: %w", i+1, err)
		}
		if view.Alias == "" {
			return fmt.Errorf("join %d: alias is empty", i+1)
		}
		aliases[view.Alias] = true
	}
	if len(aliases) != want {
		return fmt.Errorf("%d different aliases from %d joins, want %d", len(aliases), len(c.responses), want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		crowd := crowdFor(sc)

		sc.Step(`^(\d+) attendees join the room$`, crowd.join)
		sc.Step(`^all (\d+) aliases are different$`, crowd.expectDifferentAliases)
	})
}
