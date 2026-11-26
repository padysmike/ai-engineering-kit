package main

import (
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"time"
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
	code              string
	state             roomState
	hostToken         string
	qrKey             string
	qrGeneration      int
	participants      []participant
	kicked            []participant
	lastParticipantID int
	usedAliases       []string
	posts             []post
	lastPostID        int
}

type roomStore struct {
	mu          sync.Mutex
	byCode      map[string]room
	subscribers map[string]map[subscriber]viewer
	newCode     func() string
	newAlias    func() string
	now         func() time.Time
}

func newRoomStore(newCode func() string) *roomStore {
	return &roomStore{
		byCode:      map[string]room{},
		subscribers: map[string]map[subscriber]viewer{},
		newCode:     newCode,
		newAlias:    randomAlias,
		now:         time.Now,
	}
}

func randomRoomCode() string { return rand.Text()[:roomCodeLength] }

func (s *roomStore) create() room {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := s.newCode()
	for _, taken := s.byCode[code]; taken; _, taken = s.byCode[code] {
		code = s.newCode()
	}
	created := room{code: code, state: roomDraft, hostToken: rand.Text(), qrKey: rand.Text()}
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
	r.kicked = slices.Clone(r.kicked)
	r.usedAliases = slices.Clone(r.usedAliases)
	r.posts = slices.Clone(r.posts)
	for i := range r.posts {
		r.posts[i] = r.posts[i].snapshot()
	}
	return r
}

type entryCheck func(room, time.Time) error

func byRoomCode(room, time.Time) error { return nil }

func byQRToken(token string) entryCheck {
	return func(r room, now time.Time) error { return r.checkQRToken(token, now) }
}

func (s *roomStore) join(code string) (participant, error) {
	return s.admit(code, byRoomCode)
}

func (s *roomStore) admit(code string, checkEntry entryCheck) (participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return participant{}, errRoomNotFound
	}
	if err := checkEntry(found, s.now()); err != nil {
		return participant{}, err
	}
	if found.state != roomOpen {
		return participant{}, errRoomNotOpen
	}
	alias, err := drawUnusedAlias(s.newAlias, found.usedAliases)
	if err != nil {
		return participant{}, err
	}
	found.lastParticipantID++
	joined := participant{id: found.lastParticipantID, alias: alias, token: rand.Text()}
	found.participants = append(found.participants, joined)
	found.usedAliases = append(found.usedAliases, alias)
	s.byCode[code] = found
	s.publishLocked(code, eventRoom, viewOfRoom(found))
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
	s.publishLocked(code, eventRoom, viewOfRoom(found))
	return found, nil
}
