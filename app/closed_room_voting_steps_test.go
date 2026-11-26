package main

import (
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

func (v *voting) expectVoteRejected() error {
	if v.voteStatus != http.StatusConflict {
		return fmt.Errorf("vote status = %d, want %d", v.voteStatus, http.StatusConflict)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		sc.Step(`^the vote is rejected$`, votingFor(sc).expectVoteRejected)
	})
}
