package main

import (
	"crypto/rand"
	"errors"
	"slices"
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
	errRoomNotOpen     = errors.New("room is not open")
)

type room struct {
	code         string
	state        roomState
	hostToken    string
	participants []participant
	posts        []post
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
	created := room{code: code, state: roomDraft, hostToken: rand.Text()}
	s.byCode[code] = created
	return created
}

func (s *roomStore) find(code string) (room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return room{}, errRoomNotFound
	}
	return found.snapshot(), nil
}

func (r room) snapshot() room {
	r.participants = slices.Clone(r.participants)
	r.posts = slices.Clone(r.posts)
	return r
}

func (s *roomStore) join(code string) (participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return participant{}, errRoomNotFound
	}
	if found.state != roomOpen {
		return participant{}, errRoomNotOpen
	}
	joined := participant{id: len(found.participants) + 1, alias: randomAlias(), token: rand.Text()}
	found.participants = append(found.participants, joined)
	s.byCode[code] = found
	return joined, nil
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
