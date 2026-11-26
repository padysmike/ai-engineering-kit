package main

import (
	"net/http"
	"sync"
	"testing"
)

func TestVoteRequest_Refused(t *testing.T) {
	server := newServer()
	own, member := openRoomWithParticipant(t, server)
	_, stranger := openRoomWithParticipant(t, server)
	posts := "/rooms/" + own.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN a vote has no token THEN it is unauthorized", posts + "/1/vote", "", http.StatusUnauthorized},
		{"WHEN a vote has another room's session token THEN it is unauthorized", posts + "/1/vote", stranger.Token, http.StatusUnauthorized},
		{"WHEN a vote has the host token THEN it is forbidden", posts + "/1/vote", own.HostToken, http.StatusForbidden},
		{"WHEN a vote targets an unknown post THEN it is not found", posts + "/2/vote", member.Token, http.StatusNotFound},
		{"WHEN a vote targets a post id that is not a number THEN it is not found", posts + "/one/vote", member.Token, http.StatusNotFound},
		{"WHEN the token is unknown AND the post is unknown THEN it is unauthorized", posts + "/2/vote", "nope", http.StatusUnauthorized},
		{"WHEN a vote targets an unknown room THEN it is not found", "/rooms/NOROOM/posts/1/vote", member.Token, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodPost, tc.path, tc.token, "")

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestVote_TwiceBySameParticipant_ResponsesShowAddedThenRemoved(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)

	added := decoded[postView](t, serve(server, http.MethodPost, posts+"/1/vote", member.Token, ""))
	removed := decoded[postView](t, serve(server, http.MethodPost, posts+"/1/vote", member.Token, ""))

	if added.Votes != 1 || removed.Votes != 0 {
		t.Errorf("votes = %d then %d, want 1 then 0", added.Votes, removed.Votes)
	}
}

func TestVote_WhileFeedIsRead_NoRace(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)
	var group sync.WaitGroup

	for range 50 {
		group.Go(func() { serve(server, http.MethodPost, posts+"/1/vote", member.Token, "") })
		group.Go(func() { serve(server, http.MethodGet, posts, member.Token, "") })
	}
	group.Wait()

	feed := decoded[feedView](t, serve(server, http.MethodGet, posts, member.Token, ""))
	if len(feed.Posts) != 1 || feed.Posts[0].Votes != 0 {
		t.Errorf("feed = %+v, want one post with 0 votes after an even number of toggles", feed.Posts)
	}
}

func TestFind_VotersOfResultAreChanged_StoreIsNot(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	code := rooms.create().code
	if _, err := rooms.transition(code, roomDraft, roomOpen); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.addPost(code, "Blue Fox", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.toggleVote(code, 1, 7); err != nil {
		t.Fatal(err)
	}
	found, err := rooms.find(code)
	if err != nil {
		t.Fatal(err)
	}

	found.posts[0].voterIDs[0] = 8

	stored, err := rooms.find(code)
	if err != nil {
		t.Fatal(err)
	}
	if stored.posts[0].voterIDs[0] != 7 {
		t.Errorf("voters = %v, want [7]", stored.posts[0].voterIDs)
	}
}
