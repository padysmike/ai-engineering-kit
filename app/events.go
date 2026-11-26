package main

import (
	"encoding/json"
	"fmt"
	"log"
)

const subscriberBacklog = 256

const (
	eventSnapshot    = "snapshot"
	eventPost        = "post"
	eventPostRemoved = "post-removed"
	eventRoom        = "room"
)

type snapshotView struct {
	Room  roomView   `json:"room"`
	Posts []postView `json:"posts"`
}

type removedPostView struct {
	ID int `json:"id"`
}

type eventFrame []byte

func newEventFrame(name string, payload any) (eventFrame, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode %s event: %w", name, err)
	}
	return fmt.Appendf(nil, "event: %s\ndata: %s\n\n", name, data), nil
}

type subscriber chan eventFrame

type viewer struct{ participantID int }

const hostViewerID = 0

func (v viewer) isHost() bool { return v.participantID == hostViewerID }

func (v viewer) isParticipant() bool { return !v.isHost() }

func anyViewer(viewer) bool { return true }

func (r room) viewerOf(token string) (viewer, error) {
	if token == r.hostToken {
		return viewer{participantID: hostViewerID}, nil
	}
	caller, err := r.participantByToken(token)
	return viewer{participantID: caller.id}, err
}

func (s *roomStore) subscribe(code, token string) (subscriber, room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return nil, room{}, errRoomNotFound
	}
	watching, err := found.viewerOf(token)
	if err != nil {
		return nil, room{}, err
	}
	joined := make(subscriber, subscriberBacklog)
	if s.subscribers[code] == nil {
		s.subscribers[code] = map[subscriber]viewer{}
	}
	s.subscribers[code][joined] = watching
	current := found.snapshot()
	current.posts = current.postsVisibleTo(token)
	return joined, current, nil
}

func (s *roomStore) unsubscribe(code string, leaving subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers[code], leaving)
}

func (s *roomStore) publishLocked(code, name string, payload any) {
	s.publishToLocked(code, anyViewer, name, payload)
}

// publishToLocked needs s.mu held, so that events leave in the order of the
// changes and none falls between a new subscriber and its snapshot. It never
// waits: a subscriber with a full backlog is dropped and its channel closed.
func (s *roomStore) publishToLocked(code string, receives func(viewer) bool, name string, payload any) {
	if len(s.subscribers[code]) == 0 {
		return
	}
	frame, err := newEventFrame(name, payload)
	if err != nil {
		log.Print(err)
		return
	}
	for receiver, watching := range s.subscribers[code] {
		if !receives(watching) {
			continue
		}
		select {
		case receiver <- frame:
		default:
			s.dropLocked(code, receiver)
		}
	}
}

func (s *roomStore) dropLocked(code string, dropped subscriber) {
	delete(s.subscribers[code], dropped)
	close(dropped)
}

func (s *roomStore) closeStreamsOfLocked(code string, participantID int) {
	for receiver, watching := range s.subscribers[code] {
		if watching.participantID == participantID {
			s.dropLocked(code, receiver)
		}
	}
}

func (s *roomStore) publishPostRemovedLocked(code string, postID int) {
	s.publishLocked(code, eventPostRemoved, removedPostView{ID: postID})
}
