package main

import (
	"net/http"
	"testing"
)

func TestHostAction_Refused_StateUnchanged(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	other, _ := openRoomWithParticipant(t, server)
	path := "/rooms/" + room.Code
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN close has no token THEN it is unauthorized", path + "/close", "", http.StatusUnauthorized},
		{"WHEN close has an unknown token THEN it is unauthorized", path + "/close", "nope", http.StatusUnauthorized},
		{"WHEN close has another room's host token THEN it is unauthorized", path + "/close", other.HostToken, http.StatusUnauthorized},
		{"WHEN close has a participant token THEN it is forbidden", path + "/close", member.Token, http.StatusForbidden},
		{"WHEN open has no token AND the state does not allow it THEN it is unauthorized", path + "/open", "", http.StatusUnauthorized},
		{"WHEN open has a participant token AND the state does not allow it THEN it is forbidden", path + "/open", member.Token, http.StatusForbidden},
		{"WHEN the room is unknown AND there is no token THEN it is not found", "/rooms/NOROOM/close", "", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodPost, tc.path, tc.token, "")
			after := decoded[roomView](t, serve(server, http.MethodGet, path, "", ""))

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if after.State != roomOpen {
				t.Errorf("state = %q, want %q", after.State, roomOpen)
			}
		})
	}
}

func TestHostAction_DraftRoomWithoutToken_StaysDraft(t *testing.T) {
	server := newServer()
	created := decoded[createdRoomView](t, serve(server, http.MethodPost, "/rooms", "", ""))

	rec := serve(server, http.MethodPost, "/rooms/"+created.Code+"/open", "", "")

	after := decoded[roomView](t, serve(server, http.MethodGet, "/rooms/"+created.Code, "", ""))
	if rec.Code != http.StatusUnauthorized || after.State != roomDraft {
		t.Errorf("status = %d, state = %q, want %d and %q", rec.Code, after.State, http.StatusUnauthorized, roomDraft)
	}
}
