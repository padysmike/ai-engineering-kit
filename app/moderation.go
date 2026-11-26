package main

import (
	"errors"
	"slices"
)

var errParticipantNotFound = errors.New("participant not found")

func (r room) postsVisibleTo(token string) []post {
	if token == r.hostToken {
		return r.posts
	}
	return slices.DeleteFunc(slices.Clone(r.posts), func(p post) bool { return p.hidden })
}

func (r room) postIndex(postID int) (int, error) {
	i := slices.IndexFunc(r.posts, func(p post) bool { return p.id == postID })
	if i < 0 {
		return 0, errPostNotFound
	}
	return i, nil
}

func (r room) wasKicked(matches func(participant) bool) bool {
	return slices.ContainsFunc(r.kicked, matches)
}

func (s *roomStore) hidePost(code string, postID int) (post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return post{}, errRoomNotFound
	}
	i, err := found.postIndex(postID)
	if err != nil {
		return post{}, err
	}
	found.posts[i].hidden = true
	s.publishToLocked(code, viewer.isHost, eventPost, viewOfPost(found.posts[i]))
	s.publishToLocked(code, viewer.isParticipant, eventPostRemoved, removedPostView{ID: postID})
	return found.posts[i].snapshot(), nil
}

func (s *roomStore) deletePost(code string, postID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return errRoomNotFound
	}
	i, err := found.postIndex(postID)
	if err != nil {
		return err
	}
	found.posts = slices.Delete(found.posts, i, i+1)
	s.byCode[code] = found
	s.publishPostRemovedLocked(code, postID)
	return nil
}

// kick keeps the participant in the room's kicked list, so that a change whose
// caller was authenticated before the kick can still be refused.
func (s *roomStore) kick(code string, participantID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found, ok := s.byCode[code]
	if !ok {
		return errRoomNotFound
	}
	i := slices.IndexFunc(found.participants, func(p participant) bool { return p.id == participantID })
	if i < 0 {
		return errParticipantNotFound
	}
	found.kicked = append(found.kicked, found.participants[i])
	found.participants = slices.Delete(found.participants, i, i+1)
	s.byCode[code] = found
	s.closeStreamsOfLocked(code, participantID)
	s.publishLocked(code, eventRoom, viewOfRoom(found))
	return nil
}
