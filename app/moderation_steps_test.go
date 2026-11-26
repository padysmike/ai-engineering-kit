package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"

	"github.com/cucumber/godog"
)

type moderation struct {
	posting      *posting
	voting       *voting
	response     *httptest.ResponseRecorder
	actionStatus int
}

var moderations = perScenario[moderation]{}

func moderationFor(sc *godog.ScenarioContext) *moderation {
	return moderations.of(sc, func() *moderation {
		return &moderation{posting: postingFor(sc), voting: votingFor(sc)}
	})
}

func (m *moderation) hostToken() string {
	return m.posting.hostToken(m.posting.host.rooms[0].Code)
}

func (m *moderation) actAsHost(method, path string, successStatus int) {
	m.response = m.posting.send(method, path, m.hostToken(), "")
	m.actionStatus = successStatus
}

func (m *moderation) actOnPost(method, text, action string, successStatus int) error {
	target, err := m.voting.postByText(text)
	if err != nil {
		return err
	}
	m.actAsHost(method, "/posts/"+strconv.Itoa(target.ID)+action, successStatus)
	return nil
}

func (m *moderation) hide(text string) error {
	return m.actOnPost(http.MethodPost, text, "/hide", http.StatusOK)
}

func (m *moderation) delete(text string) error {
	return m.actOnPost(http.MethodDelete, text, "", http.StatusNoContent)
}

func (m *moderation) kick(name string) error {
	attendee, err := m.posting.attendee(name)
	if err != nil {
		return err
	}
	var list participantListView
	if err := m.posting.call(http.MethodGet, "/participants", m.hostToken(), "", http.StatusOK, &list); err != nil {
		return err
	}
	i := slices.IndexFunc(list.Participants, func(p listedParticipantView) bool { return p.Alias == attendee.Alias })
	if i < 0 {
		return fmt.Errorf("participant list %+v lacks the alias of attendee %s", list.Participants, name)
	}
	m.actAsHost(http.MethodDelete, "/participants/"+strconv.Itoa(list.Participants[i].ID), http.StatusNoContent)
	return nil
}

func (m *moderation) expectActionSucceeded() error {
	if m.response == nil || m.response.Code != m.actionStatus {
		return fmt.Errorf("the host's action did not answer %d: %+v", m.actionStatus, m.response)
	}
	return nil
}

func (m *moderation) expectFeedLacks(token, text string) error {
	if err := m.expectActionSucceeded(); err != nil {
		return err
	}
	posts, err := m.posting.feed(token)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(posts, func(p postView) bool { return p.Text == text }) {
		return fmt.Errorf("feed %+v still contains the post %q", posts, text)
	}
	return nil
}

func (m *moderation) expectAccess(name string, want int) error {
	attendee, err := m.posting.attendee(name)
	if err != nil {
		return err
	}
	if got := m.posting.send(http.MethodGet, "/posts", attendee.Token, "").Code; got != want {
		return fmt.Errorf("attendee %s reading the feed: status = %d, want %d", name, got, want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		m := moderationFor(sc)

		sc.Step(`^the host hides the post "([^"]*)"$`, m.hide)
		sc.Step(`^the host deletes the post "([^"]*)"$`, m.delete)
		sc.Step(`^the host kicks attendee (\w+)$`, m.kick)
		sc.Step(`^the feed of attendee (\w+) does not contain the post "([^"]*)"$`, func(name, text string) error {
			viewer, err := m.posting.attendee(name)
			if err != nil {
				return err
			}
			return m.expectFeedLacks(viewer.Token, text)
		})
		sc.Step(`^the feed of the host does not contain the post "([^"]*)"$`, func(text string) error {
			return m.expectFeedLacks(m.hostToken(), text)
		})
		sc.Step(`^attendee (\w+) no longer has access to the room$`, func(name string) error {
			return m.expectAccess(name, http.StatusUnauthorized)
		})
		sc.Step(`^attendee (\w+) still has access to the room$`, func(name string) error {
			return m.expectAccess(name, http.StatusOK)
		})
	})
}
