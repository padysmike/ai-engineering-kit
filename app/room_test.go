package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreate_GeneratedCodeIsTaken_NewCodeIsDrawn(t *testing.T) {
	codes := []string{"AAAAAA", "AAAAAA", "BBBBBB"}
	rooms := newRoomStore(func() string {
		code := codes[0]
		codes = codes[1:]
		return code
	})
	rooms.create()

	second := rooms.create()

	if second.code != "BBBBBB" {
		t.Errorf("code = %q, want %q", second.code, "BBBBBB")
	}
}

func TestRoomStateChange(t *testing.T) {
	tests := []struct {
		name    string
		earlier []string
		action  string
	}{
		{"WHEN a draft room is closed THEN the change is rejected", nil, "close"},
		{"WHEN an open room is opened THEN the change is rejected", []string{"open"}, "open"},
		{"WHEN a closed room is opened THEN the change is rejected", []string{"open", "close"}, "open"},
		{"WHEN a closed room is closed THEN the change is rejected", []string{"open", "close"}, "close"},
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

			host.server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rooms/"+host.rooms[0].Code+"/"+tc.action, nil))

			if rec.Code != http.StatusConflict {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
			}
		})
	}
}

func TestRoomStateChange_UnknownRoom_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()

	newServer().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rooms/NOROOM/open", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
