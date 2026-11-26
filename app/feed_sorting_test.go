package main

import (
	"net/http"
	"slices"
	"testing"
)

func postIDs(feed feedView) []int {
	ids := make([]int, 0, len(feed.Posts))
	for _, shown := range feed.Posts {
		ids = append(ids, shown.ID)
	}
	return ids
}

func TestFeedOrder(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	for _, body := range []string{`{"text":"one"}`, `{"text":"two"}`, `{"text":"three"}`, `{"text":"four"}`} {
		serve(server, http.MethodPost, posts, member.Token, body)
	}
	serve(server, http.MethodPost, posts+"/2/vote", member.Token, "")
	serve(server, http.MethodPost, posts+"/3/vote", member.Token, "")
	tests := []struct {
		name, query string
		want        []int
	}{
		{"WHEN top is asked for THEN ties are listed oldest first", "?sort=top", []int{2, 3, 1, 4}},
		{"WHEN newest is asked for THEN the latest post is first", "?sort=newest", []int{4, 3, 2, 1}},
		{"WHEN no sort is given THEN the latest post is first", "", []int{4, 3, 2, 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, token := range []string{member.Token, room.HostToken} {
				rec := serve(server, http.MethodGet, posts+tc.query, token, "")

				if got := postIDs(decoded[feedView](t, rec)); !slices.Equal(got, tc.want) {
					t.Errorf("ids = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestFeedSort_Refused(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN the sort is unknown THEN it is a bad request", posts + "?sort=oldest", member.Token, http.StatusBadRequest},
		{"WHEN the sort is empty THEN it is a bad request", posts + "?sort=", member.Token, http.StatusBadRequest},
		{"WHEN the token is unknown AND the sort is unknown THEN it is unauthorized", posts + "?sort=oldest", "nope", http.StatusUnauthorized},
		{"WHEN the room is unknown AND the sort is unknown THEN it is not found", "/rooms/NOROOM/posts?sort=oldest", member.Token, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodGet, tc.path, tc.token, "")

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}
