package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type qrTokens struct {
	login         *qrLogin
	posting       *posting
	previousURL   string
	previousImage []byte
	rotation      *httptest.ResponseRecorder
}

var qrTokenScenarios = perScenario[qrTokens]{}

func qrTokensFor(sc *godog.ScenarioContext) *qrTokens {
	return qrTokenScenarios.of(sc, func() *qrTokens { return &qrTokens{login: qrLoginFor(sc), posting: postingFor(sc)} })
}

func (q *qrTokens) setServerTime(now time.Time) error {
	underTest, ok := q.posting.roomsHandler.(*server)
	if !ok {
		return fmt.Errorf("the server under test is a %T, so its clock cannot be set", q.posting.roomsHandler)
	}
	underTest.rooms.now = func() time.Time { return now }
	return nil
}

func (q *qrTokens) displayAndRemember() error {
	if err := q.login.display(); err != nil {
		return err
	}
	view, err := q.login.displayedQR()
	if err != nil {
		return err
	}
	q.previousURL, q.previousImage = view.URL, q.login.image.Body.Bytes()
	return nil
}

func (q *qrTokens) expireDisplayedToken() error {
	if err := q.displayAndRemember(); err != nil {
		return err
	}
	return q.setServerTime(time.Now().Add(qrTokenLifetime + time.Second))
}

// openRoomWithDisplayedQR stops the server's clock, so that only a rotation
// can make the next QR code differ from this one.
func (q *qrTokens) openRoomWithDisplayedQR() error {
	if err := q.login.join.host.createOpenRoom(); err != nil {
		return err
	}
	if err := q.setServerTime(time.Now()); err != nil {
		return err
	}
	return q.displayAndRemember()
}

func (q *qrTokens) openShownURL() error { return q.login.open(q.previousURL) }

func withLaterExpiry(joinURL string) (string, error) {
	parameters, err := joinPageParameters(joinURL)
	if err != nil {
		return "", err
	}
	token := parameters.Get("qr")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("token %q does not have three parts", token)
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("token %q has no expiry: %w", token, err)
	}
	parts[1] = strconv.FormatInt(expiry+int64(time.Hour/time.Second), 10)
	return strings.Replace(joinURL, token, strings.Join(parts, "."), 1), nil
}

func (q *qrTokens) openAlteredURL() error {
	if err := q.displayAndRemember(); err != nil {
		return err
	}
	altered, err := withLaterExpiry(q.previousURL)
	if err != nil {
		return err
	}
	return q.login.open(altered)
}

func (q *qrTokens) expectNotSignedIn() error {
	scanned := q.login.scanned
	if scanned == nil || scanned.Code != http.StatusForbidden {
		return fmt.Errorf("join response = %+v, want status %d", scanned, http.StatusForbidden)
	}
	var joined participantView
	if json.Unmarshal(scanned.Body.Bytes(), &joined) == nil && joined.Token != "" {
		return fmt.Errorf("the refused join returned a session token")
	}
	return nil
}

func (q *qrTokens) expectParticipants(want int) error {
	host := q.login.join.host
	return host.expectParticipantCount(host.rooms[0].Code, want)
}

func (q *qrTokens) rotate() error {
	host := q.login.join.host
	request := httptest.NewRequest(http.MethodPost, "/rooms/"+host.rooms[0].Code+"/qr/rotate", nil)
	host.authorizeAsHost(request)
	q.rotation = httptest.NewRecorder()
	host.server.ServeHTTP(q.rotation, request)
	return nil
}

func (q *qrTokens) expectDifferentQROnScreen() error {
	if q.rotation == nil || q.rotation.Code != http.StatusOK {
		return fmt.Errorf("rotation response = %+v, want status %d", q.rotation, http.StatusOK)
	}
	var rotated qrView
	if err := json.Unmarshal(q.rotation.Body.Bytes(), &rotated); err != nil {
		return fmt.Errorf("decode rotation: %w", err)
	}
	if err := q.login.display(); err != nil {
		return err
	}
	shown, err := q.login.displayedQR()
	if err != nil {
		return err
	}
	if shown.URL != rotated.URL || shown.URL == q.previousURL {
		return fmt.Errorf("screen shows %q after rotating to %q from %q", shown.URL, rotated.URL, q.previousURL)
	}
	if bytes.Equal(q.login.image.Body.Bytes(), q.previousImage) {
		return fmt.Errorf("the QR image did not change")
	}
	return nil
}

func (q *qrTokens) expectNewQROpensRoom() error {
	shown, err := q.login.displayedQR()
	if err != nil {
		return err
	}
	if err := q.login.open(shown.URL); err != nil {
		return err
	}
	if q.login.scanned.Code != http.StatusCreated {
		return fmt.Errorf("scanning the new QR code: status = %d, want %d", q.login.scanned.Code, http.StatusCreated)
	}
	return q.expectParticipants(1)
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		q := qrTokensFor(sc)

		sc.Step(`^the token in the room's QR code has expired$`, q.expireDisplayedToken)
		sc.Step(`^an open room with a displayed QR code$`, q.openRoomWithDisplayedQR)
		sc.Step(`^a new attendee opens the URL from that QR code$`, q.openShownURL)
		sc.Step(`^a new attendee opens the URL from the room's QR code with an altered token$`, q.openAlteredURL)
		sc.Step(`^the attendee is not signed in$`, q.expectNotSignedIn)
		sc.Step(`^the participant count of the room is (\d+)$`, q.expectParticipants)
		sc.Step(`^the host rotates the QR code$`, q.rotate)
		sc.Step(`^the host's screen shows a QR code that differs from the previous one$`, q.expectDifferentQROnScreen)
		sc.Step(`^scanning the new QR code opens the room$`, q.expectNewQROpensRoom)
	})
}
