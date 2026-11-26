package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestSubmitPost_Limits(t *testing.T) {
	longestEscaped := strings.Repeat(`😀`, maxPostCharacters)
	tests := []struct {
		name, body string
		status     int
	}{
		{"WHEN the text has 280 multi-byte characters THEN it is accepted", `{"text":"` + strings.Repeat("ž", maxPostCharacters) + `"}`, http.StatusCreated},
		{"WHEN the text has 281 multi-byte characters THEN it is a bad request", `{"text":"` + strings.Repeat("ž", maxPostCharacters+1) + `"}`, http.StatusBadRequest},
		{"WHEN 280 characters are sent in the longest escaping THEN it is accepted", `{"text":"` + longestEscaped + `"}`, http.StatusCreated},
		{"WHEN the body is over the size limit THEN it is a bad request", `{"text":"` + strings.Repeat("a", maxPostBodyBytes) + `"}`, http.StatusBadRequest},
		{"WHEN the text is empty THEN it is accepted", `{"text":""}`, http.StatusCreated},
	}
	server := newServer()
	created, member := openRoomWithParticipant(t, server)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodPost, "/rooms/"+created.Code+"/posts", member.Token, tc.body)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestSubmitPost_ClosedRoom_ConflictBeforeInputAndFeedUnchanged(t *testing.T) {
	server := newServer()
	created, member := openRoomWithParticipant(t, server)
	path := "/rooms/" + created.Code + "/posts"
	serve(server, http.MethodPost, "/rooms/"+created.Code+"/close", created.HostToken, "")

	valid := serve(server, http.MethodPost, path, member.Token, `{"text":"late"}`)
	malformed := serve(server, http.MethodPost, path, member.Token, "{")
	unknownToken := serve(server, http.MethodPost, path, "nope", `{"text":"late"}`)
	feed := serve(server, http.MethodGet, path, member.Token, "")

	if valid.Code != http.StatusConflict || malformed.Code != http.StatusConflict {
		t.Errorf("statuses = %d, %d, want %d", valid.Code, malformed.Code, http.StatusConflict)
	}
	if unknownToken.Code != http.StatusUnauthorized {
		t.Errorf("unknown token status = %d, want %d", unknownToken.Code, http.StatusUnauthorized)
	}
	if got, want := feed.Body.String(), `{"posts":[]}`+"\n"; got != want {
		t.Errorf("feed = %q, want %q", got, want)
	}
}

func TestAddPost_RoomIsNotOpen_Rejected(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	created := rooms.create()

	_, err := rooms.addPost(created.code, "Blue Fox", "late")

	if err != errRoomNotOpen {
		t.Errorf("err = %v, want %v", err, errRoomNotOpen)
	}
}

func TestFind_ResultIsChanged_StoreIsNot(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	code := rooms.create().code
	if _, err := rooms.transition(code, roomDraft, roomOpen); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.join(code); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.addPost(code, "Blue Fox", "original"); err != nil {
		t.Fatal(err)
	}
	found, err := rooms.find(code)
	if err != nil {
		t.Fatal(err)
	}

	found.posts[0].text = "changed"
	found.participants[0].alias = "changed"

	stored, err := rooms.find(code)
	if err != nil {
		t.Fatal(err)
	}
	if stored.posts[0].text != "original" || stored.participants[0].alias == "changed" {
		t.Errorf("store changed through a found room: %+v", stored)
	}
}
