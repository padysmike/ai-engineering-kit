package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJoinBodyBytes = 4096

var errUnreadableJoin = errors.New("join body must be empty or JSON of at most 4096 bytes")

type joinRequest struct {
	QRToken *string `json:"qrToken"`
}

func (j joinRequest) entryCheck() entryCheck {
	if j.QRToken == nil {
		return byRoomCode
	}
	return byQRToken(*j.QRToken)
}

func decodeJoin(w http.ResponseWriter, r *http.Request) (joinRequest, error) {
	var request joinRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJoinBodyBytes))
	if err != nil {
		return request, errUnreadableJoin
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return request, nil
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return request, errUnreadableJoin
	}
	return request, nil
}

// The room's existence and state are reported before the input, as for every
// request.
func refuseUnreadableJoin(w http.ResponseWriter, rooms *roomStore, code string, unreadable error) {
	found, err := rooms.find(code)
	if err == nil && found.state != roomOpen {
		err = errRoomNotOpen
	}
	if err != nil {
		writeRoomError(w, err)
		return
	}
	http.Error(w, unreadable.Error(), http.StatusBadRequest)
}
