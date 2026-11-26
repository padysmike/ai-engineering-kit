package main

type postMark func(*post)

func pin(p *post) { p.pinned = true }

func answer(p *post) { p.answered = true }

func (s *roomStore) markPost(code string, postID int, mark postMark) (post, error) {
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
	mark(&found.posts[i])
	s.publishPostLocked(code, found.posts[i])
	return found.posts[i].snapshot(), nil
}

// publishPostLocked keeps a hidden post away from participants, who must not
// get it back through a later change to it.
func (s *roomStore) publishPostLocked(code string, changed post) {
	receives := anyViewer
	if changed.hidden {
		receives = viewer.isHost
	}
	s.publishToLocked(code, receives, eventPost, viewOfPost(changed))
}
