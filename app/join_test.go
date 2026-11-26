package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJoin_RoomIsNotOpen_RejectedAndNobodyAdded(t *testing.T) {
	tests := []struct {
		name    string
		earlier []string
	}{
		{"WHEN the room is a draft THEN the join is rejected AND nobody is added", nil},
		{"WHEN the room is closed THEN the join is rejected AND nobody is added", []string{"open", "close"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := &roomHost{server: newServer()}
			if err := host.createDraftRoom(); err != nil {
				t.Fatal(err)
			}
			for _, action := range tc.earlier {
				if err := host.changeRoom(action); err != nil {
					t.Fatal(err)
				}
			}
			rec := httptest.NewRecorder()

			host.server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rooms/"+host.rooms[0].Code+"/participants", nil))

			if rec.Code != http.StatusConflict {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
			}
			if err := host.expectParticipantCount(host.rooms[0].Code, 0); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRoomRequest_UnknownRoom_NotFound(t *testing.T) {
	tests := []struct {
		name, method, path string
	}{
		{"WHEN an unknown room is joined THEN it is not found", http.MethodPost, "/rooms/NOROOM/participants"},
		{"WHEN an unknown room is read THEN it is not found", http.MethodGet, "/rooms/NOROOM"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			newServer().ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}
