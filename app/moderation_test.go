package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type moderatedRoom struct {
	server        http.Handler
	path          string
	host          createdRoomView
	author, other participantView
}

func newModeratedRoom(t *testing.T) moderatedRoom {
	t.Helper()
	server := newServer()
	host, author := openRoomWithParticipant(t, server)
	path := "/rooms/" + host.Code
	other := decoded[participantView](t, serve(server, http.MethodPost, path+"/participants", "", ""))
	serve(server, http.MethodPost, path+"/posts", author.Token, `{"text":"one"}`)
	return moderatedRoom{server: server, path: path, host: host, author: author, other: other}
}

func (m moderatedRoom) status(method, path, token, body string) int {
	return serve(m.server, method, m.path+path, token, body).Code
}

func (m moderatedRoom) feedBody(token string) string {
	return strings.TrimSpace(serve(m.server, http.MethodGet, m.path+"/posts", token, "").Body.String())
}

func TestModerationRequest_Refused(t *testing.T) {
	m := newModeratedRoom(t)
	stranger, _ := openRoomWithParticipant(t, m.server)
	actions := []struct{ method, path string }{
		{http.MethodPost, "/posts/1/hide"},
		{http.MethodDelete, "/posts/1"},
		{http.MethodDelete, "/participants/1"},
		{http.MethodGet, "/participants"},
	}
	callers := []struct {
		name, token string
		status      int
	}{
		{"has no token", "", http.StatusUnauthorized},
		{"has another room's host token", stranger.HostToken, http.StatusUnauthorized},
		{"has a participant token", m.author.Token, http.StatusForbidden},
	}
	for _, action := range actions {
		for _, caller := range callers {
			name := "WHEN " + action.method + " " + action.path + " " + caller.name + " THEN it is refused"
			t.Run(name, func(t *testing.T) {
				got := m.status(action.method, action.path, caller.token, "")

				if got != caller.status || !strings.Contains(m.feedBody(m.other.Token), `"one"`) {
					t.Errorf("status = %d, want %d; feed = %s", got, caller.status, m.feedBody(m.other.Token))
				}
			})
		}
	}
}

func TestModerationRequest_UnknownTarget_NotFound(t *testing.T) {
	m := newModeratedRoom(t)
	tests := []struct{ name, method, path string }{
		{"WHEN an unknown post is hidden THEN it is not found", http.MethodPost, "/posts/9/hide"},
		{"WHEN a post id that is not a number is hidden THEN it is not found", http.MethodPost, "/posts/one/hide"},
		{"WHEN an unknown post is deleted THEN it is not found", http.MethodDelete, "/posts/9"},
		{"WHEN an unknown participant is kicked THEN it is not found", http.MethodDelete, "/participants/9"},
		{"WHEN a participant id that is not a number is kicked THEN it is not found", http.MethodDelete, "/participants/x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.status(tc.method, tc.path, m.host.HostToken, ""); got != http.StatusNotFound {
				t.Errorf("status = %d, want %d", got, http.StatusNotFound)
			}
		})
	}
	if got := serve(m.server, http.MethodDelete, "/rooms/NOROOM/posts/1", "", "").Code; got != http.StatusNotFound {
		t.Errorf("unknown room: status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestHidePost_ParticipantsLoseItAndTheHostSeesItMarked(t *testing.T) {
	m := newModeratedRoom(t)

	hidden := decoded[postView](t, serve(m.server, http.MethodPost, m.path+"/posts/1/hide", m.host.HostToken, ""))
	again := m.status(http.MethodPost, "/posts/1/hide", m.host.HostToken, "")
	vote := m.status(http.MethodPost, "/posts/1/vote", m.other.Token, "")

	if !hidden.Hidden || again != http.StatusOK || vote != http.StatusNotFound {
		t.Errorf("hidden = %+v, second hide = %d, vote = %d", hidden, again, vote)
	}
	if got := m.feedBody(m.author.Token); got != `{"posts":[]}` {
		t.Errorf("author's feed = %s, want no posts", got)
	}
	if got := m.feedBody(m.host.HostToken); !strings.Contains(got, `"hidden":true`) {
		t.Errorf("host's feed = %s, want the post marked hidden", got)
	}
}

func TestFeed_PostIsNotHidden_HasNoHiddenField(t *testing.T) {
	m := newModeratedRoom(t)

	for name, token := range map[string]string{"participant": m.other.Token, "host": m.host.HostToken} {
		if got := m.feedBody(token); strings.Contains(got, "hidden") {
			t.Errorf("%s feed = %s, want no hidden field", name, got)
		}
	}
}

func TestDeletePost_ItIsGoneForGoodAndItsIDIsNotReused(t *testing.T) {
	m := newModeratedRoom(t)

	deleted := m.status(http.MethodDelete, "/posts/1", m.host.HostToken, "")
	again := m.status(http.MethodDelete, "/posts/1", m.host.HostToken, "")
	vote := m.status(http.MethodPost, "/posts/1/vote", m.other.Token, "")
	added := decoded[postView](t, serve(m.server, http.MethodPost, m.path+"/posts", m.author.Token, `{"text":"two"}`))

	if deleted != http.StatusNoContent || again != http.StatusNotFound || vote != http.StatusNotFound || added.ID != 2 {
		t.Errorf("delete = %d, again = %d, vote = %d, next id = %d", deleted, again, vote, added.ID)
	}
}

func TestKick_TokenIsRevokedEverywhere(t *testing.T) {
	m := newModeratedRoom(t)
	if got := m.status(http.MethodDelete, "/participants/2", m.host.HostToken, ""); got != http.StatusNoContent {
		t.Fatalf("kick status = %d", got)
	}
	tests := []struct{ name, method, path, body string }{
		{"WHEN a kicked participant reads the feed THEN it is unauthorized", http.MethodGet, "/posts", ""},
		{"WHEN a kicked participant posts THEN it is unauthorized", http.MethodPost, "/posts", `{"text":"again"}`},
		{"WHEN a kicked participant votes THEN it is unauthorized", http.MethodPost, "/posts/1/vote", ""},
		{"WHEN a kicked participant opens the stream THEN it is unauthorized", http.MethodGet, "/events", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.status(tc.method, tc.path, m.other.Token, tc.body); got != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", got, http.StatusUnauthorized)
			}
		})
	}
}

func TestKick_PostsAndVotesStayAndARejoinGetsANewID(t *testing.T) {
	m := newModeratedRoom(t)
	m.status(http.MethodPost, "/posts", m.other.Token, `{"text":"theirs"}`)
	m.status(http.MethodPost, "/posts/1/vote", m.other.Token, "")

	m.status(http.MethodDelete, "/participants/2", m.host.HostToken, "")
	again := m.status(http.MethodDelete, "/participants/2", m.host.HostToken, "")
	rejoined := decoded[participantView](t, serve(m.server, http.MethodPost, m.path+"/participants", "", ""))

	feed := m.feedBody(m.author.Token)
	if !strings.Contains(feed, `"theirs"`) || !strings.Contains(feed, `"votes":1`) {
		t.Errorf("feed = %s, want the kicked participant's post and vote", feed)
	}
	if again != http.StatusNotFound || rejoined.ID != 3 || rejoined.Alias == m.other.Alias {
		t.Errorf("second kick = %d, rejoined = %+v", again, rejoined)
	}
}

func TestParticipantList_NamesOnlyThoseStillInTheRoomWithoutTokens(t *testing.T) {
	m := newModeratedRoom(t)
	m.status(http.MethodDelete, "/participants/1", m.host.HostToken, "")

	body := serve(m.server, http.MethodGet, m.path+"/participants", m.host.HostToken, "").Body.String()

	want, err := json.Marshal(participantListView{Participants: []listedParticipantView{{ID: 2, Alias: m.other.Alias}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(body) != string(want) {
		t.Errorf("list = %s, want %s", body, want)
	}
}

func TestModeration_RoomIsClosed_StillWorks(t *testing.T) {
	m := newModeratedRoom(t)
	m.status(http.MethodPost, "/close", m.host.HostToken, "")

	hide := m.status(http.MethodPost, "/posts/1/hide", m.host.HostToken, "")
	remove := m.status(http.MethodDelete, "/posts/1", m.host.HostToken, "")
	kick := m.status(http.MethodDelete, "/participants/2", m.host.HostToken, "")

	if hide != http.StatusOK || remove != http.StatusNoContent || kick != http.StatusNoContent {
		t.Errorf("hide = %d, delete = %d, kick = %d", hide, remove, kick)
	}
}

// The handlers authenticate before they change anything, so a kick can fall
// in between: the change itself has to refuse a kicked participant.
func TestChange_CallerWasKickedAfterAuthentication_Refused(t *testing.T) {
	f := newLiveFixture(t)
	f.addPosts(t, 1)
	found, err := f.rooms.find(f.code)
	if err != nil {
		t.Fatal(err)
	}
	authenticated := found.participants[0]
	if err := f.rooms.kick(f.code, authenticated.id); err != nil {
		t.Fatal(err)
	}

	_, postErr := f.rooms.addPost(f.code, authenticated.alias, "late")
	_, voteErr := f.rooms.toggleVote(f.code, 1, authenticated.id)

	after, err := f.rooms.find(f.code)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(postErr, errUnknownToken) || !errors.Is(voteErr, errUnknownToken) {
		t.Errorf("post error = %v, vote error = %v, want %v", postErr, voteErr, errUnknownToken)
	}
	if len(after.posts) != 1 || len(after.posts[0].voterIDs) != 0 {
		t.Errorf("posts = %+v, want one post without votes", after.posts)
	}
}

type watchers struct {
	host, kicked, other subscriber
}

func watchModeration(t *testing.T) (liveFixture, watchers) {
	t.Helper()
	f := newLiveFixture(t)
	other, err := f.rooms.join(f.code)
	if err != nil {
		t.Fatal(err)
	}
	f.addPosts(t, 1)
	var w watchers
	for token, target := range map[string]*subscriber{f.hostToken: &w.host, f.token: &w.kicked, other.token: &w.other} {
		events, _, err := f.rooms.subscribe(f.code, token)
		if err != nil {
			t.Fatal(err)
		}
		*target = events
	}
	return f, w
}

func received(events subscriber) string {
	var all strings.Builder
	for range len(events) {
		all.Write(<-events)
	}
	return all.String()
}

func TestHidePost_Events_ParticipantsSeeARemovalAndTheHostAMarkedPost(t *testing.T) {
	f, w := watchModeration(t)

	if _, err := f.rooms.hidePost(f.code, 1); err != nil {
		t.Fatal(err)
	}

	if got := received(w.other); got != "event: post-removed\ndata: {\"id\":1}\n\n" {
		t.Errorf("participant got %q", got)
	}
	if got := received(w.host); !strings.HasPrefix(got, "event: post\n") || !strings.Contains(got, `"hidden":true`) {
		t.Errorf("host got %q", got)
	}
	if _, current, err := f.rooms.subscribe(f.code, f.token); err != nil || len(current.posts) != 0 {
		t.Errorf("participant snapshot = %+v, err = %v", current.posts, err)
	}
	if _, current, err := f.rooms.subscribe(f.code, f.hostToken); err != nil || len(current.posts) != 1 {
		t.Errorf("host snapshot = %+v, err = %v", current.posts, err)
	}
}

func TestDeletePost_Events_EveryoneSeesARemoval(t *testing.T) {
	f, w := watchModeration(t)

	if err := f.rooms.deletePost(f.code, 1); err != nil {
		t.Fatal(err)
	}

	for name, events := range map[string]subscriber{"host": w.host, "participant": w.other} {
		if got := received(events); got != "event: post-removed\ndata: {\"id\":1}\n\n" {
			t.Errorf("%s got %q", name, got)
		}
	}
}

func TestKick_Events_OnlyTheKickedStreamClosesAndOthersSeeTheCountDrop(t *testing.T) {
	f, w := watchModeration(t)

	if err := f.rooms.kick(f.code, 1); err != nil {
		t.Fatal(err)
	}

	if frame, open := <-w.kicked; open {
		t.Errorf("kicked stream is open and got %q", frame)
	}
	for name, events := range map[string]subscriber{"host": w.host, "other": w.other} {
		if got := received(events); !strings.Contains(got, "event: room\n") || !strings.Contains(got, `"participantCount":1`) {
			t.Errorf("%s got %q", name, got)
		}
	}
	if left := f.subscriberCount(); left != 2 {
		t.Errorf("subscribers = %d, want 2", left)
	}
}
