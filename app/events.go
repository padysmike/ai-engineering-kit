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

func (s *roomStore) subscribe(code, token string) (subscriber, room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return nil, room{}, errRoomNotFound
	}
	if err := found.checkMayRead(token); err != nil {
		return nil, room{}, err
	}
	joined := make(subscriber, subscriberBacklog)
	if s.subscribers[code] == nil {
		s.subscribers[code] = map[subscriber]struct{}{}
	}
	s.subscribers[code][joined] = struct{}{}
	return joined, found.snapshot(), nil
}

func (s *roomStore) unsubscribe(code string, leaving subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers[code], leaving)
}

// publishLocked needs s.mu held, so that events leave in the order of the
// changes and none falls between a new subscriber and its snapshot. It never
// waits: a subscriber with a full backlog is dropped and its channel closed.
func (s *roomStore) publishLocked(code, name string, payload any) {
	if len(s.subscribers[code]) == 0 {
		return
	}
	frame, err := newEventFrame(name, payload)
	if err != nil {
		log.Print(err)
		return
	}
	for receiver := range s.subscribers[code] {
		select {
		case receiver <- frame:
		default:
			delete(s.subscribers[code], receiver)
			close(receiver)
		}
	}
}

func (s *roomStore) publishPostRemovedLocked(code string, postID int) {
	s.publishLocked(code, eventPostRemoved, removedPostView{ID: postID})
}
