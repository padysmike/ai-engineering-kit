package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

type liveFixture struct {
	rooms     *roomStore
	code      string
	token     string
	hostToken string
}

func newLiveFixture(t *testing.T) liveFixture {
	t.Helper()
	rooms := newRoomStore(randomRoomCode)
	created := rooms.create()
	if _, err := rooms.transition(created.code, roomDraft, roomOpen); err != nil {
		t.Fatal(err)
	}
	joined, err := rooms.join(created.code)
	if err != nil {
		t.Fatal(err)
	}
	return liveFixture{rooms: rooms, code: created.code, token: joined.token, hostToken: created.hostToken}
}

func (f liveFixture) subscribe(t *testing.T) (subscriber, room) {
	t.Helper()
	events, current, err := f.rooms.subscribe(f.code, f.token)
	if err != nil {
		t.Fatal(err)
	}
	return events, current
}

func (f liveFixture) addPosts(t *testing.T, count int) {
	t.Helper()
	for range count {
		if _, err := f.rooms.addPost(f.code, "Blue Fox", "question"); err != nil {
			t.Error(err)
		}
	}
}

// serve runs the stream handler on a real server and reports each time a
// handler returns, so tests can wait for the end of a stream without sleeping.
func (f liveFixture) serve(t *testing.T, heartbeats heartbeatSource) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	returned := make(chan struct{}, 1)
	mux := http.NewServeMux()
	registerEventRoutes(mux, f.rooms, heartbeats)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		returned <- struct{}{}
	}))
	t.Cleanup(server.Close)
	return server, returned
}

func (f liveFixture) open(t *testing.T, server *httptest.Server) (*bufio.Reader, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), streamDeadline)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/rooms/"+f.code+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return bufio.NewReader(response.Body), cancel
}

func (f liveFixture) subscriberCount() int {
	f.rooms.mu.Lock()
	defer f.rooms.mu.Unlock()
	return len(f.rooms.subscribers[f.code])
}

func silentHeartbeat() (<-chan time.Time, func()) { return nil, func() {} }

func awaitReturn(t *testing.T, returned <-chan struct{}) {
	t.Helper()
	select {
	case <-returned:
	case <-time.After(streamDeadline):
		t.Fatal("the stream handler did not return")
	}
}

func TestPublish_SubscriberNeverReads_PublishingContinuesAndItIsDropped(t *testing.T) {
	f := newLiveFixture(t)
	slow, _ := f.subscribe(t)
	fast, _ := f.subscribe(t)
	received := 0

	for range subscriberBacklog + 10 {
		f.addPosts(t, 1)
		<-fast
		received++
	}

	buffered := 0
	for range slow {
		buffered++
	}
	if buffered != subscriberBacklog || received != subscriberBacklog+10 || f.subscriberCount() != 1 {
		t.Errorf("slow got %d, fast got %d, subscribers = %d", buffered, received, f.subscriberCount())
	}
}

func TestSubscribe_PostsArriveMeanwhile_EachIsInTheSnapshotOrAnEventExactlyOnce(t *testing.T) {
	const posts = 100
	f := newLiveFixture(t)
	published := make(chan struct{})
	go func() {
		defer close(published)
		f.addPosts(t, posts)
	}()

	events, current := f.subscribe(t)
	<-published

	var ids []int
	for _, p := range current.posts {
		ids = append(ids, p.id)
	}
	for range len(events) {
		_, data, _ := strings.Cut(string(<-events), "data: ")
		var changed postView
		if err := json.Unmarshal([]byte(data), &changed); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, changed.ID)
	}
	slices.Sort(ids)
	if len(ids) != posts || len(slices.Compact(ids)) != posts {
		t.Errorf("saw %d posts, %d distinct, want %d of each", len(ids), len(slices.Compact(ids)), posts)
	}
}

func TestPublish(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, liveFixture)
		want   string
	}{
		{"WHEN the room closes THEN a room event carries the new state", func(t *testing.T, f liveFixture) {
			if _, err := f.rooms.transition(f.code, roomOpen, roomClosed); err != nil {
				t.Fatal(err)
			}
		}, "event: room\ndata: {\"code\":\"CODE\",\"state\":\"closed\",\"participantCount\":1}\n\n"},
		{"WHEN a post is removed THEN a post-removed event carries its id", func(_ *testing.T, f liveFixture) {
			f.rooms.mu.Lock()
			defer f.rooms.mu.Unlock()
			f.rooms.publishPostRemovedLocked(f.code, 7)
		}, "event: post-removed\ndata: {\"id\":7}\n\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newLiveFixture(t)
			events, _ := f.subscribe(t)

			tc.change(t, f)

			if got := strings.Replace(string(<-events), f.code, "CODE", 1); got != tc.want {
				t.Errorf("event = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEventsRequest(t *testing.T) {
	f := newLiveFixture(t)
	otherRoom := newLiveFixture(t)
	tests := []struct {
		name, code, token string
		status            int
	}{
		{"WHEN the room is unknown THEN it is not found", "NOROOM", f.token, http.StatusNotFound},
		{"WHEN the token is missing THEN it is unauthorized", f.code, "", http.StatusUnauthorized},
		{"WHEN the token belongs to another room THEN it is unauthorized", f.code, otherRoom.token, http.StatusUnauthorized},
		{"WHEN the host asks THEN the stream opens", f.code, f.hostToken, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ended, cancel := context.WithCancel(context.Background())
			cancel()
			request := httptest.NewRequestWithContext(ended, http.MethodGet, "/rooms/"+tc.code+"/events", nil)
			request.Header.Set("Authorization", "Bearer "+tc.token)
			request.SetPathValue("code", tc.code)
			rec := httptest.NewRecorder()

			eventsHandler(f.rooms, silentHeartbeat)(rec, request)

			if rec.Code != tc.status || f.subscriberCount() != 0 {
				t.Errorf("status = %d, want %d; subscribers = %d", rec.Code, tc.status, f.subscriberCount())
			}
		})
	}
}

func TestEventStream_SnapshotAndEvents_CarryNoToken(t *testing.T) {
	f := newLiveFixture(t)
	f.addPosts(t, 1)
	server, _ := f.serve(t, silentHeartbeat)
	lines, _ := f.open(t, server)
	first, err := nextEvent(lines)
	if err != nil {
		t.Fatal(err)
	}
	f.addPosts(t, 1)
	second, err := nextEvent(lines)
	if err != nil {
		t.Fatal(err)
	}

	sent := first.data + second.data

	if strings.Contains(sent, f.token) || strings.Contains(sent, f.hostToken) {
		t.Errorf("stream contains a token: %s", sent)
	}
}

func TestEventStream_HeartbeatIsDue_CommentLineIsSent(t *testing.T) {
	f := newLiveFixture(t)
	beats := make(chan time.Time)
	server, _ := f.serve(t, func() (<-chan time.Time, func()) { return beats, func() {} })
	lines, _ := f.open(t, server)
	if _, err := nextEvent(lines); err != nil {
		t.Fatal(err)
	}

	beats <- time.Time{}

	if line, err := lines.ReadString('\n'); err != nil || line != ": heartbeat\n" {
		t.Errorf("line = %q, err = %v", line, err)
	}
}

func TestEventStream_ClientDisconnects_HandlerReturnsAndUnsubscribes(t *testing.T) {
	f := newLiveFixture(t)
	stopped := make(chan struct{})
	server, returned := f.serve(t, func() (<-chan time.Time, func()) { return nil, func() { close(stopped) } })
	lines, disconnect := f.open(t, server)
	if _, err := nextEvent(lines); err != nil {
		t.Fatal(err)
	}

	disconnect()

	awaitReturn(t, returned)
	<-stopped
	if left := f.subscriberCount(); left != 0 {
		t.Errorf("subscribers = %d, want 0", left)
	}
}

func TestEventStream_SubscriberIsDropped_StreamEnds(t *testing.T) {
	f := newLiveFixture(t)
	server, returned := f.serve(t, silentHeartbeat)
	lines, _ := f.open(t, server)
	if _, err := nextEvent(lines); err != nil {
		t.Fatal(err)
	}
	f.rooms.mu.Lock()
	for dropped := range f.rooms.subscribers[f.code] {
		delete(f.rooms.subscribers[f.code], dropped)
		close(dropped)
	}
	f.rooms.mu.Unlock()

	awaitReturn(t, returned)

	if _, err := lines.ReadString('\n'); err == nil {
		t.Error("the stream is still open")
	}
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	d.deadlines = append(d.deadlines, deadline)
	return nil
}

func TestEventStreamSend_Write_IsLimitedAndTheLimitIsLiftedAfterwards(t *testing.T) {
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	stream := eventStream{writer: rec, control: http.NewResponseController(rec)}

	err := stream.send(heartbeatFrame)

	if err != nil || len(rec.deadlines) != 2 || rec.deadlines[0].IsZero() || !rec.deadlines[1].IsZero() {
		t.Errorf("err = %v, deadlines = %v", err, rec.deadlines)
	}
}

type timedOutWriter struct{ *httptest.ResponseRecorder }

var errWriteTimedOut = errors.New("write timed out")

func (timedOutWriter) Write([]byte) (int, error) { return 0, errWriteTimedOut }

func TestEventStreamRun_WriteFails_StreamEndsWithTheError(t *testing.T) {
	writer := timedOutWriter{ResponseRecorder: httptest.NewRecorder()}
	stream := eventStream{writer: writer, control: http.NewResponseController(writer)}

	err := stream.run(context.Background(), heartbeatFrame, nil, nil)

	if !errors.Is(err, errWriteTimedOut) {
		t.Errorf("error = %v, want %v", err, errWriteTimedOut)
	}
}
