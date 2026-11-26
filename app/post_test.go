package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(server http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, request)
	return rec
}

func decoded[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(rec.Body).Decode(&value); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return value
}

func openRoomWithParticipant(t *testing.T, server http.Handler) (createdRoomView, participantView) {
	t.Helper()
	created := decoded[createdRoomView](t, serve(server, http.MethodPost, "/rooms", "", ""))
	serve(server, http.MethodPost, "/rooms/"+created.Code+"/open", "", "")
	joined := decoded[participantView](t, serve(server, http.MethodPost, "/rooms/"+created.Code+"/participants", "", ""))
	return created, joined
}

func TestPostsRequest_Refused(t *testing.T) {
	server := newServer()
	own, member := openRoomWithParticipant(t, server)
	other, stranger := openRoomWithParticipant(t, server)
	path := "/rooms/" + own.Code + "/posts"
	valid := `{"text":"hello"}`
	tests := []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"WHEN a post has no token THEN it is unauthorized", http.MethodPost, path, "", valid, http.StatusUnauthorized},
		{"WHEN a post has an unknown token THEN it is unauthorized", http.MethodPost, path, "nope", valid, http.StatusUnauthorized},
		{"WHEN a post has another room's session token THEN it is unauthorized", http.MethodPost, path, stranger.Token, valid, http.StatusUnauthorized},
		{"WHEN a post has the host token THEN it is forbidden", http.MethodPost, path, own.HostToken, valid, http.StatusForbidden},
		{"WHEN a post is malformed JSON THEN it is a bad request", http.MethodPost, path, member.Token, "{", http.StatusBadRequest},
		{"WHEN the token is unknown AND the JSON is malformed THEN it is unauthorized", http.MethodPost, path, "nope", "{", http.StatusUnauthorized},
		{"WHEN a post targets an unknown room THEN it is not found", http.MethodPost, "/rooms/NOROOM/posts", member.Token, valid, http.StatusNotFound},
		{"WHEN the feed is read without a token THEN it is unauthorized", http.MethodGet, path, "", "", http.StatusUnauthorized},
		{"WHEN the feed is read with another room's host token THEN it is unauthorized", http.MethodGet, path, other.HostToken, "", http.StatusUnauthorized},
		{"WHEN the feed of an unknown room is read THEN it is not found", http.MethodGet, "/rooms/NOROOM/posts", member.Token, "", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, tc.method, tc.path, tc.token, tc.body)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestFeed_NoPosts_EmptyList(t *testing.T) {
	server := newServer()
	created, _ := openRoomWithParticipant(t, server)

	rec := serve(server, http.MethodGet, "/rooms/"+created.Code+"/posts", created.HostToken, "")

	if got, want := rec.Body.String(), `{"posts":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestSubmitPost_TwoPosts_ReturnedWithIncreasingIDsAndNoVotes(t *testing.T) {
	server := newServer()
	created, member := openRoomWithParticipant(t, server)
	path := "/rooms/" + created.Code + "/posts"

	first := decoded[postView](t, serve(server, http.MethodPost, path, member.Token, `{"text":"one"}`))
	second := decoded[postView](t, serve(server, http.MethodPost, path, member.Token, `{"text":"two"}`))

	if want := (postView{ID: 1, Alias: member.Alias, Text: "one"}); first != want {
		t.Errorf("first = %+v, want %+v", first, want)
	}
	if want := (postView{ID: 2, Alias: member.Alias, Text: "two"}); second != want {
		t.Errorf("second = %+v, want %+v", second, want)
	}
}

func TestJoin_TwoParticipants_DifferentIDsAndTokens(t *testing.T) {
	server := newServer()
	created, first := openRoomWithParticipant(t, server)

	second := decoded[participantView](t, serve(server, http.MethodPost, "/rooms/"+created.Code+"/participants", "", ""))

	if first.ID != 1 || second.ID != 2 {
		t.Errorf("ids = %d, %d, want 1, 2", first.ID, second.ID)
	}
	if first.Token == "" || first.Token == second.Token {
		t.Errorf("tokens %q and %q, want two different tokens", first.Token, second.Token)
	}
}

func TestHostToken_LaterRoomResponses_NotIncluded(t *testing.T) {
	server := newServer()
	created, _ := openRoomWithParticipant(t, server)

	read := serve(server, http.MethodGet, "/rooms/"+created.Code, "", "")
	closed := serve(server, http.MethodPost, "/rooms/"+created.Code+"/close", "", "")

	if created.HostToken == "" {
		t.Fatal("create returned no host token")
	}
	for _, rec := range []*httptest.ResponseRecorder{read, closed} {
		if body := rec.Body.String(); strings.Contains(body, "hostToken") || strings.Contains(body, created.HostToken) {
			t.Errorf("body %q exposes the host token", body)
		}
	}
}
