package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

func (p *posting) hasSubmitted(name, text string) error {
	if err := p.submit(name, text); err != nil {
		return err
	}
	if p.submitStatus != http.StatusCreated {
		return fmt.Errorf("submit status = %d, want %d", p.submitStatus, http.StatusCreated)
	}
	return nil
}

func (p *posting) expectFeedTexts(token string, want []string) error {
	posts, err := p.feed(token)
	if err != nil {
		return err
	}
	got := make([]string, 0, len(posts))
	for _, shown := range posts {
		got = append(got, shown.Text)
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("feed = %q, want %q", got, want)
	}
	return nil
}

func (p *posting) expectPostRejected() error {
	if p.submitStatus != http.StatusBadRequest && p.submitStatus != http.StatusConflict {
		return fmt.Errorf("submit status = %d, want %d or %d", p.submitStatus, http.StatusBadRequest, http.StatusConflict)
	}
	return nil
}

func (p *posting) expectPostCount(want int) error {
	posts, err := p.feed(p.hostToken(p.host.rooms[0].Code))
	if err != nil {
		return err
	}
	if len(posts) != want {
		return fmt.Errorf("feed has %d posts, want %d", len(posts), want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		p := postingFor(sc)
		var longPost string

		sc.Step(`^attendee (\w+) has submitted the post "([^"]*)"$`, p.hasSubmitted)
		sc.Step(`^the host has closed the room$`, p.host.closeRoom)
		sc.Step(`^attendee (\w+) submits a post of (\d+) characters$`, func(name string, length int) error {
			longPost = strings.Repeat("a", length)
			return p.submit(name, longPost)
		})
		sc.Step(`^the post is rejected$`, p.expectPostRejected)
		sc.Step(`^the feed contains that post$`, func() error {
			return p.expectFeedTexts(p.hostToken(p.host.rooms[0].Code), []string{longPost})
		})
		sc.Step(`^the feed contains (\d+) posts?$`, p.expectPostCount)
		sc.Step(`^the feed of attendee (\w+) contains only the post "([^"]*)"$`, func(name, text string) error {
			viewer, err := p.attendee(name)
			if err != nil {
				return err
			}
			return p.expectFeedTexts(viewer.Token, []string{text})
		})
	})
}
