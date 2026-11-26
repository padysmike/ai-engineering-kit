package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type roomView struct {
	Code             string    `json:"code"`
	State            roomState `json:"state"`
	ParticipantCount int       `json:"participantCount"`
}

type createdRoomView struct {
	roomView
	HostToken string `json:"hostToken"`
}

type participantView struct {
	ID    int    `json:"id"`
	Alias string `json:"alias"`
	Token string `json:"token"`
}

func registerRoomRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms", func(w http.ResponseWriter, _ *http.Request) {
		created := rooms.create()
		writeJSON(w, http.StatusCreated, createdRoomView{roomView: viewOfRoom(created), HostToken: created.hostToken})
	})
	mux.HandleFunc("GET /rooms/{code}", findHandler(rooms))
	mux.HandleFunc("POST /rooms/{code}/participants", joinHandler(rooms))
	mux.HandleFunc("POST /rooms/{code}/open", transitionHandler(rooms, roomDraft, roomOpen))
	mux.HandleFunc("POST /rooms/{code}/close", transitionHandler(rooms, roomOpen, roomClosed))
}

func findHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		found, err := rooms.find(r.PathValue("code"))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeRoom(w, http.StatusOK, found)
	}
}

func joinHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		joined, err := rooms.join(r.PathValue("code"))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, participantView{ID: joined.id, Alias: joined.alias, Token: joined.token})
	}
}

func transitionHandler(rooms *roomStore, from, to roomState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if err := checkHostRequest(rooms, code, bearerToken(r)); err != nil {
			writeRoomError(w, err)
			return
		}
		changed, err := rooms.transition(code, from, to)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeRoom(w, http.StatusOK, changed)
	}
}

func checkHostRequest(rooms *roomStore, code, token string) error {
	found, err := rooms.find(code)
	if err != nil {
		return err
	}
	return found.checkIsHost(token)
}

func writeRoomError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), roomErrorStatus(err))
}

func roomErrorStatus(err error) int {
	switch {
	case errors.Is(err, errRoomNotFound), errors.Is(err, errPostNotFound):
		return http.StatusNotFound
	case errors.Is(err, errUnknownToken):
		return http.StatusUnauthorized
	case errors.Is(err, errNotParticipant), errors.Is(err, errNotHost):
		return http.StatusForbidden
	case errors.Is(err, errPostTooLong), errors.Is(err, errUnknownSort):
		return http.StatusBadRequest
	default:
		return http.StatusConflict
	}
}

func writeRoom(w http.ResponseWriter, status int, r room) {
	writeJSON(w, status, viewOfRoom(r))
}

func viewOfRoom(r room) roomView {
	return roomView{Code: r.code, State: r.state, ParticipantCount: len(r.participants)}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write %T response: %v", body, err)
	}
}
