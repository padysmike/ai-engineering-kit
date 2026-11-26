package main

import (
	"crypto/rand"
	"errors"
	"sync"
)

const roomCodeLength = 6

type roomState string

const (
	roomDraft  roomState = "draft"
	roomOpen   roomState = "open"
	roomClosed roomState = "closed"
)

var (
	errRoomNotFound    = errors.New("room not found")
	errStateTransition = errors.New("room state does not allow this change")
)

type room struct {
	code  string
	state roomState
}

type roomStore struct {
	mu      sync.Mutex
	byCode  map[string]room
	newCode func() string
}

func newRoomStore(newCode func() string) *roomStore {
	return &roomStore{byCode: map[string]room{}, newCode: newCode}
}

func randomRoomCode() string { return rand.Text()[:roomCodeLength] }

func (s *roomStore) create() room {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := s.newCode()
	for _, taken := s.byCode[code]; taken; _, taken = s.byCode[code] {
		code = s.newCode()
	}
	created := room{code: code, state: roomDraft}
	s.byCode[code] = created
	return created
}

func (s *roomStore) transition(code string, from, to roomState) (room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return room{}, errRoomNotFound
	}
	if found.state != from {
		return room{}, errStateTransition
	}
	found.state = to
	s.byCode[code] = found
	return found, nil
}
