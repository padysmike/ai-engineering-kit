package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type roomView struct {
	Code  string    `json:"code"`
	State roomState `json:"state"`
}

func registerRoomRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms", func(w http.ResponseWriter, _ *http.Request) {
		writeRoom(w, http.StatusCreated, rooms.create())
	})
	mux.HandleFunc("POST /rooms/{code}/open", transitionHandler(rooms, roomDraft, roomOpen))
	mux.HandleFunc("POST /rooms/{code}/close", transitionHandler(rooms, roomOpen, roomClosed))
}

func transitionHandler(rooms *roomStore, from, to roomState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		changed, err := rooms.transition(r.PathValue("code"), from, to)
		if err != nil {
			http.Error(w, err.Error(), roomErrorStatus(err))
			return
		}
		writeRoom(w, http.StatusOK, changed)
	}
}

func roomErrorStatus(err error) int {
	if errors.Is(err, errRoomNotFound) {
		return http.StatusNotFound
	}
	return http.StatusConflict
}

func writeRoom(w http.ResponseWriter, status int, r room) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(roomView{Code: r.code, State: r.state}); err != nil {
		log.Printf("write room %s: %v", r.code, err)
	}
}
