package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

var aliasShape = regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`)

type attendeeJoin struct {
	host        *roomHost
	roomCode    string
	request     *http.Request
	requestBody string
	alias       string
}

var attendeeJoins = perScenario[attendeeJoin]{}

func joinFor(sc *godog.ScenarioContext) *attendeeJoin {
	return attendeeJoins.of(sc, func() *attendeeJoin { return &attendeeJoin{host: hostFor(sc)} })
}

func (j *attendeeJoin) send(roomCode, path, body string) error {
	j.roomCode, j.requestBody = roomCode, body
	j.request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	j.host.server.ServeHTTP(rec, j.request)
	if rec.Code != http.StatusCreated {
		return fmt.Errorf("POST %s: status = %d, want %d", path, rec.Code, http.StatusCreated)
	}
	var view participantView
	if err := json.NewDecoder(rec.Body).Decode(&view); err != nil {
		return fmt.Errorf("POST %s: decode participant: %w", path, err)
	}
	j.alias = view.Alias
	return nil
}

func (j *attendeeJoin) enterRoomCode(roomCode string) error {
	return j.send(roomCode, "/rooms/"+roomCode+"/participants", "")
}

func (j *attendeeJoin) expectInRoom() error {
	return j.host.expectParticipantCount(j.roomCode, 1)
}

func (j *attendeeJoin) expectAnonymousAlias() error {
	if !aliasShape.MatchString(j.alias) {
		return fmt.Errorf("alias = %q, want two capitalised words", j.alias)
	}
	return nil
}

func (j *attendeeJoin) expectNoCredentials() error {
	if j.request == nil {
		return fmt.Errorf("no join request was sent")
	}
	sent := strings.ToLower(j.request.URL.String() + j.requestBody)
	for _, credential := range []string{"password", "email"} {
		if strings.Contains(sent, credential) {
			return fmt.Errorf("join request %q contains %q", sent, credential)
		}
	}
	if header := j.request.Header.Get("Authorization"); header != "" {
		return fmt.Errorf("join request has an Authorization header")
	}
	return nil
}

func (h *roomHost) createOpenRooms(count int) error {
	if err := h.createRooms(count); err != nil {
		return err
	}
	for i, created := range h.rooms {
		opened, err := h.post("/rooms/"+created.Code+"/open", http.StatusOK)
		if err != nil {
			return err
		}
		h.rooms[i] = opened
	}
	return nil
}

func (h *roomHost) expectParticipantCount(roomCode string, want int) error {
	rec := httptest.NewRecorder()
	h.server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rooms/"+roomCode, nil))
	if rec.Code != http.StatusOK {
		return fmt.Errorf("GET room %s: status = %d, want %d", roomCode, rec.Code, http.StatusOK)
	}
	var view roomView
	if err := json.NewDecoder(rec.Body).Decode(&view); err != nil {
		return fmt.Errorf("GET room %s: decode room: %w", roomCode, err)
	}
	if view.ParticipantCount != want {
		return fmt.Errorf("participant count of room %s = %d, want %d", roomCode, view.ParticipantCount, want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		host := hostFor(sc)
		join := joinFor(sc)
		roomIndex := map[string]int{"first": 0, "second": 1}

		sc.Step(`^(\d+) open rooms$`, host.createOpenRooms)
		enterFirstRoomCode := func() error { return join.enterRoomCode(host.rooms[0].Code) }

		sc.Step(`^a new attendee enters the room's 6-character room code$`, enterFirstRoomCode)
		sc.Step(`^the attendee is in the room$`, join.expectInRoom)
		sc.Step(`^the attendee is signed in with an anonymous alias$`, join.expectAnonymousAlias)
		sc.Step(`^the join request contained no password or email$`, join.expectNoCredentials)
		sc.Step(`^attendee \w+ joins the first room$`, enterFirstRoomCode)
		sc.Step(`^the participant count of the (first|second) room is (\d+)$`, func(which string, want int) error {
			return host.expectParticipantCount(host.rooms[roomIndex[which]].Code, want)
		})
	})
}
