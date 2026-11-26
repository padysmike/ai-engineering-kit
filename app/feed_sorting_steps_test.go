package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"

	"github.com/cucumber/godog"
)

var quotedText = regexp.MustCompile(`"([^"]*)"`)

type feedSorting struct {
	posting  *posting
	viewer   string
	response *httptest.ResponseRecorder
}

var feedSortings = perScenario[feedSorting]{}

func feedSortingFor(sc *godog.ScenarioContext) *feedSorting {
	return feedSortings.of(sc, func() *feedSorting { return &feedSorting{posting: postingFor(sc)} })
}

func (f *feedSorting) sortBy(name, order string) error {
	viewer, err := f.posting.attendee(name)
	if err != nil {
		return err
	}
	f.viewer = name
	f.response = f.posting.send(http.MethodGet, "/posts?sort="+order, viewer.Token, "")
	return nil
}

func (f *feedSorting) expectOrder(name, quotedTexts string) error {
	if f.response == nil || f.viewer != name {
		return fmt.Errorf("attendee %s has not sorted the feed", name)
	}
	if f.response.Code != http.StatusOK {
		return fmt.Errorf("feed status = %d, want %d", f.response.Code, http.StatusOK)
	}
	var feed feedView
	if err := json.Unmarshal(f.response.Body.Bytes(), &feed); err != nil {
		return fmt.Errorf("decode feed: %w", err)
	}
	var got, want []string
	for _, shown := range feed.Posts {
		got = append(got, shown.Text)
	}
	for _, match := range quotedText.FindAllStringSubmatch(quotedTexts, -1) {
		want = append(want, match[1])
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("feed = %q, want %q", got, want)
	}
	return nil
}

func (p *posting) hasSubmittedAfter(name, text, earlier string) error {
	posts, err := p.feed(p.hostToken(p.host.rooms[0].Code))
	if err != nil {
		return err
	}
	if err := p.show(posts, earlier); err != nil {
		return err
	}
	return p.hasSubmitted(name, text)
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		f := feedSortingFor(sc)

		sc.Step(`^attendee (\w+) has submitted the post "([^"]*)" after the post "([^"]*)"$`, f.posting.hasSubmittedAfter)
		sc.Step(`^attendee (\w+) sorts the feed by (\w+)$`, f.sortBy)
		sc.Step(`^the feed of attendee (\w+) lists (".*") in that order$`, f.expectOrder)
	})
}
