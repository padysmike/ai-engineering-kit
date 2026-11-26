package main

import (
	"cmp"
	"errors"
	"slices"
	"unicode/utf8"
)

const maxPostCharacters = 280

var (
	errUnknownToken   = errors.New("token is missing or unknown")
	errNotParticipant = errors.New("only a participant can do this")
	errNotHost        = errors.New("only the host can do this")
	errPostTooLong    = errors.New("post is longer than 280 characters")
	errPostNotFound   = errors.New("post not found")
	errUnknownSort    = errors.New("unknown sort")
)

type postOrder func(a, b post) int

var postOrders = map[string]postOrder{"newest": newestFirst, "top": mostVotedFirst}

func newestFirst(a, b post) int { return cmp.Compare(b.id, a.id) }

func mostVotedFirst(a, b post) int {
	return cmp.Or(cmp.Compare(len(b.voterIDs), len(a.voterIDs)), cmp.Compare(a.id, b.id))
}

type participant struct {
	id    int
	alias string
	token string
}

type post struct {
	id       int
	alias    string
	text     string
	voterIDs []int
}

func (p post) snapshot() post {
	p.voterIDs = slices.Clone(p.voterIDs)
	return p
}

func (p *post) toggleVote(participantID int) {
	if i := slices.Index(p.voterIDs, participantID); i >= 0 {
		p.voterIDs = slices.Delete(p.voterIDs, i, i+1)
		return
	}
	p.voterIDs = append(p.voterIDs, participantID)
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

func (r room) checkIsHost(token string) error {
	if token == r.hostToken {
		return nil
	}
	if _, err := r.participantByToken(token); err != nil {
		return err
	}
	return errNotHost
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
	if found.state != roomOpen {
		return post{}, errRoomNotOpen
	}
	if utf8.RuneCountInString(text) > maxPostCharacters {
		return post{}, errPostTooLong
	}
	added := post{id: len(found.posts) + 1, alias: alias, text: text}
	found.posts = append(found.posts, added)
	s.byCode[code] = found
	return added, nil
}

func (s *roomStore) toggleVote(code string, postID, participantID int) (post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return post{}, errRoomNotFound
	}
	i := slices.IndexFunc(found.posts, func(p post) bool { return p.id == postID })
	if i < 0 {
		return post{}, errPostNotFound
	}
	if found.state != roomOpen {
		return post{}, errRoomNotOpen
	}
	found.posts[i].toggleVote(participantID)
	return found.posts[i].snapshot(), nil
}
