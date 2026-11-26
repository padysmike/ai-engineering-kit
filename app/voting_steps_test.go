package main

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/cucumber/godog"
)

type voting struct {
	posting    *posting
	voteStatus int
}

var votings = perScenario[voting]{}

func votingFor(sc *godog.ScenarioContext) *voting {
	return votings.of(sc, func() *voting { return &voting{posting: postingFor(sc)} })
}

func (v *voting) postByText(text string) (postView, error) {
	posts, err := v.posting.feed(v.posting.hostToken(v.posting.host.rooms[0].Code))
	if err != nil {
		return postView{}, err
	}
	i := slices.IndexFunc(posts, func(candidate postView) bool { return candidate.Text == text })
	if i < 0 {
		return postView{}, fmt.Errorf("feed %+v lacks the post %q", posts, text)
	}
	return posts[i], nil
}

func (v *voting) upvote(name, text string) error {
	voter, err := v.posting.attendee(name)
	if err != nil {
		return err
	}
	target, err := v.postByText(text)
	if err != nil {
		return err
	}
	path := "/posts/" + strconv.Itoa(target.ID) + "/vote"
	v.voteStatus = v.posting.send(http.MethodPost, path, voter.Token, "").Code
	return nil
}

func (v *voting) hasUpvoted(name, text string) error {
	if err := v.upvote(name, text); err != nil {
		return err
	}
	if v.voteStatus != http.StatusOK {
		return fmt.Errorf("vote status = %d, want %d", v.voteStatus, http.StatusOK)
	}
	return nil
}

func (v *voting) expectVotes(text string, want int) error {
	target, err := v.postByText(text)
	if err != nil {
		return err
	}
	if target.Votes != want {
		return fmt.Errorf("post %q has %d votes, want %d", text, target.Votes, want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		v := votingFor(sc)

		sc.Step(`^attendee (\w+) upvotes the post "([^"]*)"$`, v.upvote)
		sc.Step(`^attendee (\w+) has upvoted the post "([^"]*)"$`, v.hasUpvoted)
		sc.Step(`^the post "([^"]*)" has (\d+) votes?$`, v.expectVotes)
	})
}
