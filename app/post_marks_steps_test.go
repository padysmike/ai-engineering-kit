package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"

	"github.com/cucumber/godog"
)

type postMarks struct {
	voting   *voting
	live     *liveRoom
	response *httptest.ResponseRecorder
}

var postMarkScenarios = perScenario[postMarks]{}

func postMarksFor(sc *godog.ScenarioContext) *postMarks {
	return postMarkScenarios.of(sc, func() *postMarks {
		return &postMarks{voting: votingFor(sc), live: liveRoomFor(sc)}
	})
}

func (m *postMarks) mark(text, action string) error {
	target, err := m.voting.postByText(text)
	if err != nil {
		return err
	}
	posting := m.voting.posting
	hostToken := posting.hostToken(posting.host.rooms[0].Code)
	m.response = posting.send(http.MethodPost, "/posts/"+strconv.Itoa(target.ID)+"/"+action, hostToken, "")
	return nil
}

func (m *postMarks) pin(text string) error { return m.mark(text, "pin") }

func (m *postMarks) markAnswered(text string) error { return m.mark(text, "answered") }

func (m *postMarks) hasPinned(text string) error {
	if err := m.pin(text); err != nil {
		return err
	}
	if m.response.Code != http.StatusOK {
		return fmt.Errorf("pin status = %d, want %d", m.response.Code, http.StatusOK)
	}
	return nil
}

func hasMark(shown postView, mark string) bool {
	return map[string]bool{"pinned": shown.Pinned, "answered": shown.Answered}[mark]
}

func (m *postMarks) expectMarkArrives(name, text, mark string) error {
	viewer, err := m.live.viewer(name)
	if err != nil {
		return err
	}
	marked := func() bool {
		return slices.ContainsFunc(viewer.feed(), func(p postView) bool { return p.Text == text && hasMark(p, mark) })
	}
	if err := viewer.readUntil(marked); err != nil {
		return fmt.Errorf("waiting for the post %q to be marked as %s: %w", text, mark, err)
	}
	for _, earlier := range viewer.postsOnEntry {
		if earlier.Text == text && hasMark(earlier, mark) {
			return fmt.Errorf("post %q was already %s when attendee %s connected", text, mark, name)
		}
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		m := postMarksFor(sc)

		sc.Step(`^the host pins the post "([^"]*)"$`, m.pin)
		sc.Step(`^the host marks the post "([^"]*)" as answered$`, m.markAnswered)
		sc.Step(`^the host has pinned the post "([^"]*)"$`, m.hasPinned)
		sc.Step(`^attendee (\w+) sees the post "([^"]*)" marked as (pinned|answered) without reloading$`, m.expectMarkArrives)
	})
}
