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

type participantView struct {
	Alias string `json:"alias"`
}

func registerRoomRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms", func(w http.ResponseWriter, _ *http.Request) {
		writeRoom(w, http.StatusCreated, rooms.create())
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
		alias, err := rooms.join(r.PathValue("code"))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, participantView{Alias: alias})
	}
}

func transitionHandler(rooms *roomStore, from, to roomState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		changed, err := rooms.transition(r.PathValue("code"), from, to)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeRoom(w, http.StatusOK, changed)
	}
}

func writeRoomError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if errors.Is(err, errRoomNotFound) {
		status = http.StatusNotFound
	}
	http.Error(w, err.Error(), status)
}

func writeRoom(w http.ResponseWriter, status int, r room) {
	writeJSON(w, status, roomView{Code: r.code, State: r.state, ParticipantCount: len(r.aliases)})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write %T response: %v", body, err)
	}
}
