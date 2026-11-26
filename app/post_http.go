package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

const maxPostBodyBytes = 4096

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
	mux.HandleFunc("POST /rooms/{code}/posts/{postID}/vote", voteHandler(rooms))
}

func submitPostHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		author, err := authorOfRequest(rooms, code, bearerToken(r))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		request, err := decodeNewPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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

func decodeNewPost(w http.ResponseWriter, r *http.Request) (newPostRequest, error) {
	var request newPostRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPostBodyBytes)).Decode(&request)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return request, errors.New("request body is too large")
	}
	if err != nil {
		return request, errors.New("malformed JSON")
	}
	return request, nil
}

func authorOfRequest(rooms *roomStore, code, token string) (participant, error) {
	found, author, err := participantOfRequest(rooms, code, token)
	if err != nil {
		return participant{}, err
	}
	if found.state != roomOpen {
		return participant{}, errRoomNotOpen
	}
	return author, nil
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
		order, err := requestedOrder(r)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		slices.SortFunc(found.posts, order)
		writeJSON(w, http.StatusOK, feedView{Posts: viewsOfPosts(found.posts)})
	}
}

func requestedOrder(r *http.Request) (postOrder, error) {
	query := r.URL.Query()
	if !query.Has("sort") {
		return newestFirst, nil
	}
	order, ok := postOrders[query.Get("sort")]
	if !ok {
		return nil, errUnknownSort
	}
	return order, nil
}

func voteHandler(rooms *roomStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		_, voter, err := participantOfRequest(rooms, code, bearerToken(r))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		postID, err := strconv.Atoi(r.PathValue("postID"))
		if err != nil {
			writeRoomError(w, errPostNotFound)
			return
		}
		voted, err := rooms.toggleVote(code, postID, voter.id)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, viewOfPost(voted))
	}
}

func participantOfRequest(rooms *roomStore, code, token string) (room, participant, error) {
	found, err := rooms.find(code)
	if err != nil {
		return room{}, participant{}, err
	}
	caller, err := found.participantByToken(token)
	return found, caller, err
}

func bearerToken(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return token
}

func viewsOfPosts(posts []post) []postView {
	views := make([]postView, 0, len(posts))
	for _, p := range posts {
		views = append(views, viewOfPost(p))
	}
	return views
}

func viewOfPost(p post) postView {
	return postView{ID: p.id, Alias: p.alias, Text: p.text, Votes: len(p.voterIDs)}
}
