package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/cucumber/godog"
	"rsc.io/qr"
)

type qrLogin struct {
	join      *attendeeJoin
	displayed *httptest.ResponseRecorder
	image     *httptest.ResponseRecorder
	scanned   *httptest.ResponseRecorder
}

var qrLogins = perScenario[qrLogin]{}

func qrLoginFor(sc *godog.ScenarioContext) *qrLogin {
	return qrLogins.of(sc, func() *qrLogin { return &qrLogin{join: joinFor(sc)} })
}

// display records the image together with the URL it shows. Both carry a
// token that changes every second, so it asks again until the URL is the same
// before and after the image.
func (q *qrLogin) display() error {
	for range 5 {
		before := q.getAsHost("/qr")
		q.image = q.getAsHost("/qr.png")
		q.displayed = q.getAsHost("/qr")
		if before.Body.String() == q.displayed.Body.String() {
			return nil
		}
	}
	return fmt.Errorf("the QR URL kept changing while the image was fetched")
}

func (q *qrLogin) getAsHost(path string) *httptest.ResponseRecorder {
	host := q.join.host
	request := httptest.NewRequest(http.MethodGet, "/rooms/"+host.rooms[0].Code+path, nil)
	host.authorizeAsHost(request)
	rec := httptest.NewRecorder()
	host.server.ServeHTTP(rec, request)
	return rec
}

func (q *qrLogin) displayedQR() (qrView, error) {
	var view qrView
	if q.displayed == nil || q.displayed.Code != http.StatusOK {
		return view, fmt.Errorf("the host has no QR code: %+v", q.displayed)
	}
	if err := json.Unmarshal(q.displayed.Body.Bytes(), &view); err != nil {
		return view, fmt.Errorf("decode QR code: %w", err)
	}
	return view, nil
}

func joinPageParameters(joinURL string) (url.Values, error) {
	parsed, err := url.Parse(joinURL)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", joinURL, err)
	}
	return url.ParseQuery(parsed.Fragment)
}

// open does what a phone and then the join page do: it loads the URL without
// its fragment, and the page joins with what the fragment carries.
func (q *qrLogin) open(joinURL string) error {
	parameters, err := joinPageParameters(joinURL)
	if err != nil {
		return err
	}
	address, _, _ := strings.Cut(joinURL, "#")
	page := httptest.NewRecorder()
	q.join.host.server.ServeHTTP(page, httptest.NewRequest(http.MethodGet, address, nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `src="app.js"`) {
		return fmt.Errorf("GET %s: status = %d, want the join page", address, page.Code)
	}
	return q.joinWithToken(parameters.Get("room"), parameters.Get("qr"))
}

func (q *qrLogin) joinWithToken(roomCode, token string) error {
	body, err := json.Marshal(joinRequest{QRToken: &token})
	if err != nil {
		return err
	}
	q.join.roomCode, q.join.requestBody, q.join.alias = roomCode, string(body), ""
	q.join.request = httptest.NewRequest(http.MethodPost, "/rooms/"+roomCode+"/participants", bytes.NewReader(body))
	q.scanned = httptest.NewRecorder()
	q.join.host.server.ServeHTTP(q.scanned, q.join.request)
	var joined participantView
	if q.scanned.Code == http.StatusCreated {
		if err := json.Unmarshal(q.scanned.Body.Bytes(), &joined); err != nil {
			return fmt.Errorf("decode participant: %w", err)
		}
	}
	q.join.alias = joined.Alias
	return nil
}

func (q *qrLogin) openFromRoomQR() error {
	if err := q.display(); err != nil {
		return err
	}
	view, err := q.displayedQR()
	if err != nil {
		return err
	}
	return q.open(view.URL)
}

func (q *qrLogin) expectQROfRoomURL() error {
	view, err := q.displayedQR()
	if err != nil {
		return err
	}
	encoded, err := qr.Encode(view.URL, qr.M)
	if err != nil {
		return fmt.Errorf("encode %q: %w", view.URL, err)
	}
	if q.image.Code != http.StatusOK || !bytes.Equal(q.image.Body.Bytes(), encoded.PNG()) {
		return fmt.Errorf("the QR image does not encode %q", view.URL)
	}
	parameters, err := joinPageParameters(view.URL)
	if err != nil {
		return err
	}
	if got, want := parameters.Get("room"), q.join.host.rooms[0].Code; got != want {
		return fmt.Errorf("URL %q is for room %q, want %q", view.URL, got, want)
	}
	return nil
}

func (q *qrLogin) expectURLToken() error {
	view, err := q.displayedQR()
	if err != nil {
		return err
	}
	parameters, err := joinPageParameters(view.URL)
	if err != nil {
		return err
	}
	if parameters.Get("qr") == "" {
		return fmt.Errorf("URL %q contains no token", view.URL)
	}
	return nil
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		q := qrLoginFor(sc)

		sc.Step(`^the host displays the room's QR code$`, q.display)
		sc.Step(`^the QR code encodes a URL of that room$`, q.expectQROfRoomURL)
		sc.Step(`^the URL contains a token$`, q.expectURLToken)
		sc.Step(`^a new attendee opens the URL from the room's QR code$`, q.openFromRoomQR)
	})
}
