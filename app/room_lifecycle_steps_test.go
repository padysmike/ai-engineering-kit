package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
)

type roomHost struct {
	server http.Handler
	rooms  []roomView
}

type perScenario[T any] map[*godog.ScenarioContext]*T

func (p perScenario[T]) of(sc *godog.ScenarioContext, create func() *T) *T {
	if _, ok := p[sc]; !ok {
		p[sc] = create()
	}
	return p[sc]
}

var roomHosts = perScenario[roomHost]{}

func hostFor(sc *godog.ScenarioContext) *roomHost {
	return roomHosts.of(sc, func() *roomHost { return &roomHost{server: newServer()} })
}

func (h *roomHost) post(path string, wantStatus int) (roomView, error) {
	rec := httptest.NewRecorder()
	h.server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
	var view roomView
	if rec.Code != wantStatus {
		return view, fmt.Errorf("POST %s: status = %d, want %d", path, rec.Code, wantStatus)
	}
	if err := json.NewDecoder(rec.Body).Decode(&view); err != nil {
		return view, fmt.Errorf("POST %s: decode room: %w", path, err)
	}
	return view, nil
}

func (h *roomHost) createRooms(count int) error {
	for range count {
		view, err := h.post("/rooms", http.StatusCreated)
		if err != nil {
			return err
		}
		h.rooms = append(h.rooms, view)
	}
	return nil
}

func (h *roomHost) changeRoom(action string) error {
	view, err := h.post("/rooms/"+h.rooms[0].Code+"/"+action, http.StatusOK)
	if err != nil {
		return err
	}
	h.rooms[0] = view
	return nil
}

func (h *roomHost) openRoom() error { return h.changeRoom("open") }

func (h *roomHost) closeRoom() error { return h.changeRoom("close") }

func (h *roomHost) createDraftRoom() error { return h.createRooms(1) }

func (h *roomHost) createOpenRoom() error {
	if err := h.createDraftRoom(); err != nil {
		return err
	}
	return h.openRoom()
}

func (h *roomHost) expectCodeLength(want int) error {
	if len(h.rooms) == 0 {
		return fmt.Errorf("no rooms were created")
	}
	for _, view := range h.rooms {
		if len(view.Code) != want {
			return fmt.Errorf("room code %q has length %d, want %d", view.Code, len(view.Code), want)
		}
	}
	return nil
}

func (h *roomHost) expectDifferentCodes(want int) error {
	codes := map[string]bool{}
	for _, view := range h.rooms {
		codes[view.Code] = true
	}
	if len(codes) != want {
		return fmt.Errorf("%d different room codes, want %d", len(codes), want)
	}
	return nil
}

func (h *roomHost) expectState(want string) error {
	if got := string(h.rooms[0].State); got != want {
		return fmt.Errorf("room state = %q, want %q", got, want)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		host := hostFor(sc)

		sc.Step(`^the host creates (\d+) rooms$`, host.createRooms)
		sc.Step(`^each room has a (\d+)-character room code$`, host.expectCodeLength)
		sc.Step(`^the (\d+) room codes are different$`, host.expectDifferentCodes)
		sc.Step(`^a draft room$`, host.createDraftRoom)
		sc.Step(`^an open room$`, host.createOpenRoom)
		sc.Step(`^the host opens the room$`, host.openRoom)
		sc.Step(`^the host closes the room$`, host.closeRoom)
		sc.Step(`^the room state is "([^"]*)"$`, host.expectState)
	})
}
