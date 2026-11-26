package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type newPostRequest struct {
	Text string `json:"text"`
}

type postView struct {
	ID    int    `json:"id"`
	Alias string `json:"alias"`
	Text  string `json:"text"`
	Votes int    `json:"votes"`
}

type feedView struct {
	Posts []postView `json:"posts"`
}

func registerPostRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms/{code}/posts", submitPostHandler(rooms))
	mux.HandleFunc("GET /rooms/{code}/posts", feedHandler(rooms))
}

func submitPostHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		author, err := authorOfRequest(rooms, code, bearerToken(r))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		var request newPostRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "malformed JSON", http.StatusBadRequest)
			return
		}
		added, err := rooms.addPost(code, author.alias, request.Text)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, viewOfPost(added))
	}
}

func authorOfRequest(rooms *roomStore, code, token string) (participant, error) {
	found, err := rooms.find(code)
	if err != nil {
		return participant{}, err
	}
	return found.participantByToken(token)
}

func feedHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		found, err := rooms.find(r.PathValue("code"))
		if err == nil {
			err = found.checkMayRead(bearerToken(r))
		}
		if err != nil {
			writeRoomError(w, err)
			return
		}
		feed := feedView{Posts: make([]postView, 0, len(found.posts))}
		for _, p := range found.posts {
			feed.Posts = append(feed.Posts, viewOfPost(p))
		}
		writeJSON(w, http.StatusOK, feed)
	}
}

func bearerToken(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return token
}

func viewOfPost(p post) postView {
	return postView{ID: p.id, Alias: p.alias, Text: p.text}
}
