package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJoinWithQRToken(t *testing.T) {
	issued := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	rooms := newRoomStore(randomRoomCode)
	code, other := openRoomIn(t, rooms), openRoomIn(t, rooms)
	found, err := rooms.find(code)
	if err != nil {
		t.Fatal(err)
	}
	token := found.newQRToken(issued)
	parts := strings.Split(token, ".")
	generation, expiry, signature := parts[0], parts[1], parts[2]
	rotated := found
	rotated.qrGeneration++
	tests := []struct {
		name, code, token string
		age               time.Duration
		want              error
	}{
		{"WHEN the token is fresh THEN the join is accepted", code, token, 0, nil},
		{"WHEN the token is about to expire THEN the join is accepted", code, token, qrTokenLifetime - time.Second, nil},
		{"WHEN the token has expired THEN the join is refused", code, token, qrTokenLifetime, errBadQRToken},
		{"WHEN the token is for another room THEN the join is refused", other, token, 0, errBadQRToken},
		{"WHEN the expiry is altered THEN the join is refused", code, generation + ".9" + expiry + "." + signature, 0, errBadQRToken},
		{"WHEN the generation is altered THEN the join is refused", code, "9." + expiry + "." + signature, 0, errBadQRToken},
		{"WHEN the token is signed for another generation THEN the join is refused", code, rotated.newQRToken(issued), 0, errBadQRToken},
		{"WHEN the signature is altered THEN the join is refused", code, token + "x", 0, errBadQRToken},
		{"WHEN the token is empty THEN the join is refused", code, "", 0, errBadQRToken},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rooms.now = func() time.Time { return issued.Add(tc.age) }

			_, err := rooms.admit(tc.code, byQRToken(tc.token))

			if err != tc.want {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestJoinRequestBody(t *testing.T) {
	server := newServer()
	open, _ := openRoomWithParticipant(t, server)
	closed, _ := openRoomWithParticipant(t, server)
	serve(server, http.MethodPost, "/rooms/"+closed.Code+"/close", closed.HostToken, "")
	path := "/rooms/" + open.Code + "/participants"
	tests := []struct {
		name, path, body string
		status           int
	}{
		{"WHEN the body is empty THEN the code alone joins", path, "", http.StatusCreated},
		{"WHEN the body is an empty object THEN the code alone joins", path, "{}", http.StatusCreated},
		{"WHEN the QR token is bad THEN the join is forbidden", path, `{"qrToken":"nope"}`, http.StatusForbidden},
		{"WHEN the QR token is empty THEN the join is forbidden", path, `{"qrToken":""}`, http.StatusForbidden},
		{"WHEN the body is not JSON THEN it is a bad request", path, `{"qrToken":`, http.StatusBadRequest},
		{"WHEN the body is too large THEN it is a bad request", path, `{"qrToken":"` + strings.Repeat("a", maxJoinBodyBytes) + `"}`, http.StatusBadRequest},
		{"WHEN the QR token is bad AND the room is closed THEN the join is forbidden", "/rooms/" + closed.Code + "/participants", `{"qrToken":"nope"}`, http.StatusForbidden},
		{"WHEN the body is not JSON AND the room is closed THEN it conflicts", "/rooms/" + closed.Code + "/participants", "{", http.StatusConflict},
		{"WHEN the body is not JSON AND the room is unknown THEN it is not found", "/rooms/NOROOM/participants", "{", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := decoded[roomView](t, serve(server, http.MethodGet, "/rooms/"+open.Code, "", ""))

			rec := serve(server, http.MethodPost, tc.path, "", tc.body)

			after := decoded[roomView](t, serve(server, http.MethodGet, "/rooms/"+open.Code, "", ""))
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status != http.StatusCreated && after.ParticipantCount != before.ParticipantCount {
				t.Errorf("participants = %d, want %d", after.ParticipantCount, before.ParticipantCount)
			}
		})
	}
}

func TestQRRequest_Refused(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	tests := []struct {
		name, path, token string
		status            int
	}{
		{"WHEN there is no token THEN it is unauthorized", "/rooms/" + room.Code + "/qr", "", http.StatusUnauthorized},
		{"WHEN the token is a participant's THEN it is forbidden", "/rooms/" + room.Code + "/qr", member.Token, http.StatusForbidden},
		{"WHEN the image is asked for without a token THEN it is unauthorized", "/rooms/" + room.Code + "/qr.png", "", http.StatusUnauthorized},
		{"WHEN the image is asked for by a participant THEN it is forbidden", "/rooms/" + room.Code + "/qr.png", member.Token, http.StatusForbidden},
		{"WHEN the room is unknown THEN it is not found", "/rooms/NOROOM/qr", room.HostToken, http.StatusNotFound},
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

func TestQRURL_FollowsRequestSchemeAndHost(t *testing.T) {
	server := newServer()
	room, _ := openRoomWithParticipant(t, server)
	for _, base := range []string{"http://192.168.1.20:8080", "https://qa.example.org"} {
		request := httptest.NewRequest(http.MethodGet, base+"/rooms/"+room.Code+"/qr", nil)
		request.Header.Set("Authorization", "Bearer "+room.HostToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, request)

		if got, want := decoded[qrView](t, rec).URL, base+"/#room="+room.Code+"&qr="; !strings.HasPrefix(got, want) {
			t.Errorf("url = %q, want prefix %q", got, want)
		}
	}
}

func TestQRResponses_TypeSetAndNeverStored(t *testing.T) {
	server := newServer()
	room, _ := openRoomWithParticipant(t, server)
	for path, contentType := range map[string]string{"/qr": "application/json", "/qr.png": "image/png"} {
		rec := serve(server, http.MethodGet, "/rooms/"+room.Code+path, room.HostToken, "")

		if got := rec.Header().Get("Content-Type"); rec.Code != http.StatusOK || got != contentType {
			t.Errorf("%s: status = %d, type = %q, want 200 and %q", path, rec.Code, got, contentType)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", path, got)
		}
	}
}
