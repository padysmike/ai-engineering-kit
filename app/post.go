package main

import (
	"errors"
	"slices"
)

var (
	errUnknownToken   = errors.New("token is missing or unknown")
	errNotParticipant = errors.New("only a participant can do this")
)

type participant struct {
	id    int
	alias string
	token string
}

type post struct {
	id    int
	alias string
	text  string
}

func (r room) participantByToken(token string) (participant, error) {
	if token == r.hostToken {
		return participant{}, errNotParticipant
	}
	i := slices.IndexFunc(r.participants, func(p participant) bool { return p.token == token })
	if i < 0 {
		return participant{}, errUnknownToken
	}
	return r.participants[i], nil
}

func (r room) checkMayRead(token string) error {
	if _, err := r.participantByToken(token); errors.Is(err, errUnknownToken) {
		return err
	}
	return nil
}

func (s *roomStore) addPost(code, alias, text string) (post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return post{}, errRoomNotFound
	}
	added := post{id: len(found.posts) + 1, alias: alias, text: text}
	found.posts = append(found.posts, added)
	s.byCode[code] = found
	return added, nil
}
