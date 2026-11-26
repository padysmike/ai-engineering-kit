package main

import "net/http"

func registerMarkRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("POST /rooms/{code}/posts/{postID}/pin", hostOnly(rooms, markHandler(rooms, pin)))
	mux.HandleFunc("POST /rooms/{code}/posts/{postID}/answered", hostOnly(rooms, markHandler(rooms, answer)))
}

func markHandler(rooms *roomStore, mark postMark) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := pathID(r, "postID", errPostNotFound)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		marked, err := rooms.markPost(r.PathValue("code"), postID, mark)
		if err != nil {
			writeRoomError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, viewOfPost(marked))
	}
}
