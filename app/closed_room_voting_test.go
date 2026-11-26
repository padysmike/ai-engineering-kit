package main

import (
	"net/http"
	"testing"
)

func TestVote_ClosedRoom(t *testing.T) {
	server := newServer()
	room, voter := openRoomWithParticipant(t, server)
	other := decoded[participantView](t, serve(server, http.MethodPost, "/rooms/"+room.Code+"/participants", "", ""))
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, voter.Token, `{"text":"one"}`)
	serve(server, http.MethodPost, posts+"/1/vote", voter.Token, "")
	serve(server, http.MethodPost, "/rooms/"+room.Code+"/close", "", "")
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN a new vote is cast THEN it conflicts", posts + "/1/vote", other.Token, http.StatusConflict},
		{"WHEN a vote is removed THEN it conflicts", posts + "/1/vote", voter.Token, http.StatusConflict},
		{"WHEN the post is unknown THEN it is not found", posts + "/2/vote", voter.Token, http.StatusNotFound},
		{"WHEN the token is unknown THEN it is unauthorized", posts + "/1/vote", "nope", http.StatusUnauthorized},
		{"WHEN the host token votes THEN it is forbidden", posts + "/1/vote", room.HostToken, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodPost, tc.path, tc.token, "")
			feed := decoded[feedView](t, serve(server, http.MethodGet, posts, voter.Token, ""))

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if len(feed.Posts) != 1 || feed.Posts[0].Votes != 1 {
				t.Errorf("feed = %+v, want one post with 1 vote", feed.Posts)
			}
		})
	}
}
