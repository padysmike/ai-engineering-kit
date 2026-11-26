package main

import (
	"net/http"
	"net/url"
	"testing"
)

func qrTokenIn(t *testing.T, joinURL string) string {
	t.Helper()
	parsed, err := url.Parse(joinURL)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	return parameters.Get("qr")
}

func TestRotateQR_OldTokenRefusedAndNobodySignedOut(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	path := "/rooms/" + room.Code
	old := qrTokenIn(t, decoded[qrView](t, serve(server, http.MethodGet, path+"/qr", room.HostToken, "")).URL)

	rotation := serve(server, http.MethodPost, path+"/qr/rotate", room.HostToken, "")

	fresh := qrTokenIn(t, decoded[qrView](t, rotation).URL)
	if got := serve(server, http.MethodPost, path+"/participants", "", `{"qrToken":"`+old+`"}`).Code; got != http.StatusForbidden {
		t.Errorf("old token: status = %d, want %d", got, http.StatusForbidden)
	}
	if got := serve(server, http.MethodPost, path+"/participants", "", `{"qrToken":"`+fresh+`"}`).Code; got != http.StatusCreated {
		t.Errorf("new token: status = %d, want %d", got, http.StatusCreated)
	}
	if got := serve(server, http.MethodGet, path+"/posts", member.Token, "").Code; got != http.StatusOK {
		t.Errorf("earlier participant: status = %d, want %d", got, http.StatusOK)
	}
	if got := rotation.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestRotateQR_Refused_EarlierTokenStillWorks(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	other, _ := openRoomWithParticipant(t, server)
	path := "/rooms/" + room.Code
	token := qrTokenIn(t, decoded[qrView](t, serve(server, http.MethodGet, path+"/qr", room.HostToken, "")).URL)
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN there is no token THEN it is unauthorized", path + "/qr/rotate", "", http.StatusUnauthorized},
		{"WHEN the token is another room's host token THEN it is unauthorized", path + "/qr/rotate", other.HostToken, http.StatusUnauthorized},
		{"WHEN the token is a participant's THEN it is forbidden", path + "/qr/rotate", member.Token, http.StatusForbidden},
		{"WHEN the room is unknown THEN it is not found", "/rooms/NOROOM/qr/rotate", room.HostToken, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(server, http.MethodPost, tc.path, tc.token, "")

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := serve(server, http.MethodPost, path+"/participants", "", `{"qrToken":"`+token+`"}`).Code; got != http.StatusCreated {
				t.Errorf("earlier QR token: status = %d, want %d", got, http.StatusCreated)
			}
		})
	}
}

func TestRotateQR_UnknownRoom_Error(t *testing.T) {
	_, err := newRoomStore(randomRoomCode).rotateQR("NOROOM")

	if err != errRoomNotFound {
		t.Errorf("err = %v, want %v", err, errRoomNotFound)
	}
}
