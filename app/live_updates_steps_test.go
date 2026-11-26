package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

const streamDeadline = 5 * time.Second

type streamEvent struct {
	name string
	data string
}

func nextEvent(lines *bufio.Reader) (streamEvent, error) {
	var next streamEvent
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			return next, fmt.Errorf("read event stream: %w", err)
		}
		line = strings.TrimSuffix(line, "\n")
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			next.name = name
		}
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			next.data = data
		}
		if line == "" && next.name != "" {
			return next, nil
		}
	}
}

type streamViewer struct {
	lines        *bufio.Reader
	close        func()
	order        func(a, b postView) int
	room         roomView
	posts        map[int]postView
	postsOnEntry map[int]postView
	countOnEntry int
}

func openStream(url, token string) (*streamViewer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), streamDeadline)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		cancel()
		return nil, err
	}
	viewer := &streamViewer{
		lines: bufio.NewReader(response.Body),
		close: func() { cancel(); _ = response.Body.Close() },
		order: func(a, b postView) int { return cmp.Compare(b.ID, a.ID) },
		posts: map[int]postView{},
	}
	if kind := response.Header.Get("Content-Type"); response.StatusCode != http.StatusOK || kind != "text/event-stream" {
		viewer.close()
		return nil, fmt.Errorf("open stream: status = %d, content type = %q", response.StatusCode, kind)
	}
	return viewer, viewer.readSnapshot()
}

func (v *streamViewer) readSnapshot() error {
	first, err := nextEvent(v.lines)
	if err != nil {
		return err
	}
	var snapshot snapshotView
	if first.name != "snapshot" {
		return fmt.Errorf("first event = %q, want snapshot", first.name)
	}
	if err := json.Unmarshal([]byte(first.data), &snapshot); err != nil {
		return fmt.Errorf("decode snapshot: %w", err)
	}
	v.room, v.countOnEntry = snapshot.Room, snapshot.Room.ParticipantCount
	for _, shown := range snapshot.Posts {
		v.posts[shown.ID] = shown
	}
	v.postsOnEntry = map[int]postView{}
	for id, shown := range v.posts {
		v.postsOnEntry[id] = shown
	}
	return nil
}

func (v *streamViewer) apply(next streamEvent) error {
	switch next.name {
	case "room":
		return json.Unmarshal([]byte(next.data), &v.room)
	case "post":
		var changed postView
		err := json.Unmarshal([]byte(next.data), &changed)
		v.posts[changed.ID] = changed
		return err
	default:
		return fmt.Errorf("unexpected event %q", next.name)
	}
}

func (v *streamViewer) readUntil(reached func() bool) error {
	for !reached() {
		next, err := nextEvent(v.lines)
		if err != nil {
			return err
		}
		if err := v.apply(next); err != nil {
			return err
		}
	}
	return nil
}

func (v *streamViewer) feed() []postView {
	shown := make([]postView, 0, len(v.posts))
	for _, p := range v.posts {
		shown = append(shown, p)
	}
	slices.SortFunc(shown, v.order)
	return shown
}

func (v *streamViewer) feedTexts() []string {
	var texts []string
	for _, shown := range v.feed() {
		texts = append(texts, shown.Text)
	}
	return texts
}

const hostViewer = "host"

type liveRoom struct {
	posting    *posting
	server     *httptest.Server
	viewers    map[string]*streamViewer
	joinStatus int
}

var liveRooms = perScenario[liveRoom]{}

func liveRoomFor(sc *godog.ScenarioContext) *liveRoom {
	return liveRooms.of(sc, func() *liveRoom {
		return &liveRoom{posting: postingFor(sc), viewers: map[string]*streamViewer{}}
	})
}

func (l *liveRoom) viewer(name string) (*streamViewer, error) {
	if open, ok := l.viewers[name]; ok {
		return open, nil
	}
	code := l.posting.host.rooms[0].Code
	token := l.posting.hostToken(code)
	if name != hostViewer {
		attendee, err := l.posting.attendee(name)
		if err != nil {
			return nil, err
		}
		token = attendee.Token
	}
	if l.server == nil {
		l.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.posting.host.server.ServeHTTP(w, r)
		}))
	}
	opened, err := openStream(l.server.URL+"/rooms/"+code+"/events", token)
	if err != nil {
		return nil, fmt.Errorf("viewer %s: %w", name, err)
	}
	l.viewers[name] = opened
	return opened, nil
}

func (l *liveRoom) connectEveryone() error {
	names := []string{hostViewer}
	for name := range l.posting.attendees {
		names = append(names, name)
	}
	for _, name := range names {
		if _, err := l.viewer(name); err != nil {
			return err
		}
	}
	return nil
}

func (l *liveRoom) leave() {
	for _, open := range l.viewers {
		open.close()
	}
	if l.server != nil {
		l.server.Close()
	}
}

func (l *liveRoom) join(name string) {
	rec := l.posting.send(http.MethodPost, "/participants", "", "")
	l.joinStatus = rec.Code
	var joined participantView
	if json.Unmarshal(rec.Body.Bytes(), &joined) == nil {
		l.posting.attendees[name] = joined
	}
}

func (l *liveRoom) expectPostArrives(name, text string) error {
	viewer, err := l.viewer(name)
	if err != nil {
		return err
	}
	arrived := func() bool { return slices.Contains(viewer.feedTexts(), text) }
	if err := viewer.readUntil(arrived); err != nil {
		return fmt.Errorf("waiting for the post %q: %w", text, err)
	}
	if err := l.posting.show(viewer.feed(), text); err != nil {
		return err
	}
	if _, reloaded := viewer.postsOnEntry[l.posting.shown.ID]; reloaded {
		return fmt.Errorf("post %q was already there when attendee %s connected", text, name)
	}
	return nil
}

func (l *liveRoom) expectHostCount(want int) error {
	viewer, err := l.viewer(hostViewer)
	if err != nil {
		return err
	}
	return viewer.readUntil(func() bool { return viewer.room.ParticipantCount == want })
}

func (l *liveRoom) expectHostCountArrives(want int) error {
	if l.joinStatus != http.StatusCreated {
		return fmt.Errorf("join status = %d, want %d", l.joinStatus, http.StatusCreated)
	}
	if err := l.expectHostCount(want); err != nil {
		return fmt.Errorf("waiting for a participant count of %d: %w", want, err)
	}
	if entry := l.viewers[hostViewer].countOnEntry; entry == want {
		return fmt.Errorf("participant count was already %d when the host connected", entry)
	}
	return nil
}

func (l *liveRoom) viewSortedByTop(name string) error {
	viewer, err := l.viewer(name)
	if err != nil {
		return err
	}
	viewer.order = func(a, b postView) int {
		return cmp.Or(cmp.Compare(b.Votes, a.Votes), cmp.Compare(a.ID, b.ID))
	}
	return nil
}

func (l *liveRoom) expectLiveOrder(name, quotedTexts string) error {
	viewer, err := l.viewer(name)
	if err != nil {
		return err
	}
	var want []string
	for _, match := range quotedText.FindAllStringSubmatch(quotedTexts, -1) {
		want = append(want, match[1])
	}
	if err := viewer.readUntil(func() bool { return slices.Equal(viewer.feedTexts(), want) }); err != nil {
		return fmt.Errorf("feed = %q, want %q: %w", viewer.feedTexts(), want, err)
	}
	return nil
}

func expectsLiveDelivery(scenario *godog.Scenario) bool {
	return slices.ContainsFunc(scenario.Steps, func(step *godog.Step) bool {
		return strings.HasSuffix(step.Text, "without reloading")
	})
}

// Whoever must see a change without reloading has to be connected before it
// happens, and no step says so, so everyone connects before the first When.
func connectBeforeFirstAction(sc *godog.ScenarioContext, live *liveRoom) {
	var expectsLive bool
	sc.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		expectsLive = expectsLiveDelivery(scenario)
		return ctx, nil
	})
	sc.StepContext().Before(func(ctx context.Context, step *godog.Step) (context.Context, error) {
		if expectsLive && step.Type == "Action" {
			return ctx, live.connectEveryone()
		}
		return ctx, nil
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		live.leave()
		return ctx, nil
	})
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		live := liveRoomFor(sc)
		connectBeforeFirstAction(sc, live)

		sc.Step(`^attendee (\w+) joins the room$`, live.join)
		sc.Step(`^attendee (\w+) sees the post "([^"]*)" in the feed without reloading$`, live.expectPostArrives)
		sc.Step(`^the host sees a participant count of (\d+)$`, live.expectHostCount)
		sc.Step(`^the host sees a participant count of (\d+) without reloading$`, live.expectHostCountArrives)
		sc.Step(`^attendee (\w+) is viewing the feed sorted by top$`, live.viewSortedByTop)
		sc.Step(`^the feed of attendee (\w+) lists (".*") in that order without reloading$`, live.expectLiveOrder)
	})
}
