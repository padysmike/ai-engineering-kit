package main

import (
	"net/http"
	"strconv"
)

type listedParticipantView struct {
	ID    int    `json:"id"`
	Alias string `json:"alias"`
}

type participantListView struct {
	Participants []listedParticipantView `json:"participants"`
}

func registerModerationRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms/{code}/posts/{postID}/hide", hostOnly(rooms, hidePostHandler(rooms)))
	mux.HandleFunc("DELETE /rooms/{code}/posts/{postID}", hostOnly(rooms, deletePostHandler(rooms)))
	mux.HandleFunc("DELETE /rooms/{code}/participants/{participantID}", hostOnly(rooms, kickHandler(rooms)))
	mux.HandleFunc("GET /rooms/{code}/participants", hostOnly(rooms, participantListHandler(rooms)))
}

func hostOnly(rooms *roomStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := hostedRoom(rooms, r.PathValue("code"), bearerToken(r)); err != nil {
			writeRoomError(w, err)
			return
		}
		next(w, r)
	}
}

func pathID(r *http.Request, name string, notFound error) (int, error) {
	id, err := strconv.Atoi(r.PathValue(name))
	if err != nil {
		return 0, notFound
	}
	return id, nil
}

func hidePostHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := pathID(r, "postID", errPostNotFound)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		hidden, err := rooms.hidePost(r.PathValue("code"), postID)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, viewOfPost(hidden))
	}
}

func deletePostHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := pathID(r, "postID", errPostNotFound)
		if err == nil {
			err = rooms.deletePost(r.PathValue("code"), postID)
		}
		if err != nil {
			writeRoomError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func kickHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		participantID, err := pathID(r, "participantID", errParticipantNotFound)
		if err == nil {
			err = rooms.kick(r.PathValue("code"), participantID)
		}
		if err != nil {
			writeRoomError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func participantListHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		found, err := rooms.find(r.PathValue("code"))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		list := participantListView{Participants: make([]listedParticipantView, 0, len(found.participants))}
		for _, p := range found.participants {
			list.Participants = append(list.Participants, listedParticipantView{ID: p.id, Alias: p.alias})
		}
		writeJSON(w, http.StatusOK, list)
	}
}
