package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

type posting struct {
	host         *roomHost
	attendees    map[string]participantView
	hostTokens   map[string]string
	dataToHost   bytes.Buffer
	shown        postView
	roomsHandler http.Handler
}

var postings = perScenario[posting]{}

func postingFor(sc *godog.ScenarioContext) *posting {
	return postings.of(sc, func() *posting {
		p := &posting{host: hostFor(sc), attendees: map[string]participantView{}, hostTokens: map[string]string{}}
		p.roomsHandler, p.host.server = p.host.server, http.HandlerFunc(p.recordDataToHost)
		return p
	})
}

func (p *posting) recordDataToHost(w http.ResponseWriter, r *http.Request) {
	rec := httptest.NewRecorder()
	p.roomsHandler.ServeHTTP(rec, r)
	if p.isFromHost(r) {
		fmt.Fprintln(&p.dataToHost, rec.Header(), rec.Body.String())
	}
	var created createdRoomView
	if r.URL.Path == "/rooms" && json.Unmarshal(rec.Body.Bytes(), &created) == nil {
		p.hostTokens[created.Code] = created.HostToken
	}
	for name, values := range rec.Header() {
		w.Header()[name] = values
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}

func (p *posting) isFromHost(r *http.Request) bool {
	token := bearerToken(r)
	if token == "" {
		return !strings.HasSuffix(r.URL.Path, "/participants")
	}
	return slices.Contains(slices.Collect(maps.Values(p.hostTokens)), token)
}

func (p *posting) call(method, path, token, body string, wantStatus int, response any) error {
	request := httptest.NewRequest(method, "/rooms/"+p.host.rooms[0].Code+path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	p.host.server.ServeHTTP(rec, request)
	if rec.Code != wantStatus {
		return fmt.Errorf("%s %s: status = %d, want %d", method, request.URL.Path, rec.Code, wantStatus)
	}
	if err := json.NewDecoder(rec.Body).Decode(response); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, request.URL.Path, err)
	}
	return nil
}

func (p *posting) attendee(name string) (participantView, error) {
	joined, ok := p.attendees[name]
	if !ok || joined.Token == "" {
		return participantView{}, fmt.Errorf("attendee %s has no session token", name)
	}
	return joined, nil
}

func (p *posting) joinRoom(name string) error {
	var joined participantView
	if err := p.call(http.MethodPost, "/participants", "", "", http.StatusCreated, &joined); err != nil {
		return err
	}
	p.attendees[name] = joined
	return nil
}

func (p *posting) submit(name, text string) error {
	author, err := p.attendee(name)
	if err != nil {
		return err
	}
	body := `{"text":` + strconv.Quote(text) + `}`
	return p.call(http.MethodPost, "/posts", author.Token, body, http.StatusCreated, &postView{})
}

func (p *posting) feed(token string) ([]postView, error) {
	var feed feedView
	err := p.call(http.MethodGet, "/posts", token, "", http.StatusOK, &feed)
	return feed.Posts, err
}

func (p *posting) feedOf(name string) ([]postView, error) {
	viewer, err := p.attendee(name)
	if err != nil {
		return nil, err
	}
	return p.feed(viewer.Token)
}

func (p *posting) show(posts []postView, text string) error {
	i := slices.IndexFunc(posts, func(candidate postView) bool { return candidate.Text == text })
	if i < 0 {
		return fmt.Errorf("feed %+v lacks the post %q", posts, text)
	}
	p.shown = posts[i]
	return nil
}

func (p *posting) expectSeenByAttendee(name, text string) error {
	posts, err := p.feedOf(name)
	if err != nil {
		return err
	}
	return p.show(posts, text)
}

func (p *posting) expectShownAlias(name string) error {
	author, err := p.attendee(name)
	if err != nil {
		return err
	}
	if author.Alias == "" || p.shown.Alias != author.Alias {
		return fmt.Errorf("post alias = %q, want %q", p.shown.Alias, author.Alias)
	}
	return nil
}

func (p *posting) expectSeenByHost(text, name string) error {
	posts, err := p.feed(p.hostTokens[p.host.rooms[0].Code])
	if err != nil {
		return err
	}
	if err := p.show(posts, text); err != nil {
		return err
	}
	return p.expectShownAlias(name)
}

func (p *posting) expectNoSessionTokenSentToHost(name string) error {
	author, err := p.attendee(name)
	if err != nil {
		return err
	}
	sent := p.dataToHost.String()
	if !strings.Contains(sent, p.shown.Text) {
		return fmt.Errorf("data sent to the host %q lacks the post it was shown", sent)
	}
	if strings.Contains(sent, author.Token) {
		return fmt.Errorf("data sent to the host contains the session token of attendee %s", name)
	}
	return nil
}

func (p *posting) expectAllPostsUnderJoinAlias(name string) error {
	posts, err := p.feedOf(name)
	if err != nil {
		return err
	}
	if len(posts) != 2 {
		return fmt.Errorf("feed has %d posts, want 2", len(posts))
	}
	for _, shown := range posts {
		p.shown = shown
		if err := p.expectShownAlias(name); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		p := postingFor(sc)

		sc.Step(`^attendee (\w+) has joined the room$`, p.joinRoom)
		sc.Step(`^attendee (\w+) submits the post "([^"]*)"$`, p.submit)
		sc.Step(`^attendee (\w+) sees the post "([^"]*)" in the feed$`, p.expectSeenByAttendee)
		sc.Step(`^the post is shown with the alias of attendee (\w+)$`, p.expectShownAlias)
		sc.Step(`^the host sees the post "([^"]*)" with the alias of attendee (\w+)$`, p.expectSeenByHost)
		sc.Step(`^the data sent to the host does not contain the session token of attendee (\w+)$`, p.expectNoSessionTokenSentToHost)
		sc.Step(`^both posts are shown with the alias attendee (\w+) received on joining$`, p.expectAllPostsUnderJoinAlias)
	})
}
