package main

import (
	"net/http"
	"slices"
	"testing"
)

func TestMarkRequest_Refused_PostUnmarked(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	other, _ := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)
	for _, action := range []string{"pin", "answered"} {
		tests := []struct {
			name, path, token string
			status            int
		}{
			{"WHEN there is no token THEN it is unauthorized", posts + "/1/", "", http.StatusUnauthorized},
			{"WHEN the token is another room's host token THEN it is unauthorized", posts + "/1/", other.HostToken, http.StatusUnauthorized},
			{"WHEN the token is a participant's THEN it is forbidden", posts + "/1/", member.Token, http.StatusForbidden},
			{"WHEN the post is unknown THEN it is not found", posts + "/2/", room.HostToken, http.StatusNotFound},
			{"WHEN the post id is not a number THEN it is not found", posts + "/one/", room.HostToken, http.StatusNotFound},
			{"WHEN the room is unknown THEN it is not found", "/rooms/NOROOM/posts/1/", room.HostToken, http.StatusNotFound},
		}
		for _, tc := range tests {
			t.Run(action+" "+tc.name, func(t *testing.T) {
				rec := serve(server, http.MethodPost, tc.path+action, tc.token, "")

				if rec.Code != tc.status {
					t.Errorf("status = %d, want %d", rec.Code, tc.status)
				}
				feed := decoded[feedView](t, serve(server, http.MethodGet, posts, room.HostToken, ""))
				if feed.Posts[0].Pinned || feed.Posts[0].Answered {
					t.Errorf("post = %+v, want no marks", feed.Posts[0])
				}
			})
		}
	}
}

func TestMark_RepeatedInAClosedRoom_StaysMarked(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)
	serve(server, http.MethodPost, "/rooms/"+room.Code+"/close", room.HostToken, "")
	serve(server, http.MethodPost, posts+"/1/pin", room.HostToken, "")
	serve(server, http.MethodPost, posts+"/1/answered", room.HostToken, "")

	repeated := serve(server, http.MethodPost, posts+"/1/pin", room.HostToken, "")

	got := decoded[postView](t, repeated)
	if repeated.Code != http.StatusOK || !got.Pinned || !got.Answered {
		t.Errorf("status = %d, post = %+v, want 200 with both marks", repeated.Code, got)
	}
}

func TestMark_AnsweredPost_OrderUnchanged(t *testing.T) {
	server := newServer()
	room, member := openRoomWithParticipant(t, server)
	posts := "/rooms/" + room.Code + "/posts"
	serve(server, http.MethodPost, posts, member.Token, `{"text":"one"}`)
	serve(server, http.MethodPost, posts, member.Token, `{"text":"two"}`)

	serve(server, http.MethodPost, posts+"/1/answered", room.HostToken, "")

	feed := decoded[feedView](t, serve(server, http.MethodGet, posts, member.Token, ""))
	if got, want := postIDs(feed), []int{2, 1}; !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestMark_HiddenPost_OnlyTheHostGetsTheEvent(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	created := rooms.create()
	if _, err := rooms.transition(created.code, roomDraft, roomOpen); err != nil {
		t.Fatal(err)
	}
	member, err := rooms.join(created.code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.addPost(created.code, member.alias, "off topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.hidePost(created.code, 1); err != nil {
		t.Fatal(err)
	}
	hostEvents, _, err := rooms.subscribe(created.code, created.hostToken)
	if err != nil {
		t.Fatal(err)
	}
	memberEvents, snapshot, err := rooms.subscribe(created.code, member.token)
	if err != nil {
		t.Fatal(err)
	}

	for _, mark := range []postMark{pin, answer} {
		if _, err := rooms.markPost(created.code, 1, mark); err != nil {
			t.Fatal(err)
		}
	}

	if len(hostEvents) != 2 || len(memberEvents) != 0 || len(snapshot.posts) != 0 {
		t.Errorf("host events = %d, member events = %d, member snapshot = %d posts, want 2, 0, 0", len(hostEvents), len(memberEvents), len(snapshot.posts))
	}
}

func TestMarkPost_UnknownRoom_Error(t *testing.T) {
	_, err := newRoomStore(randomRoomCode).markPost("NOROOM", 1, pin)

	if err != errRoomNotFound {
		t.Errorf("err = %v, want %v", err, errRoomNotFound)
	}
}
