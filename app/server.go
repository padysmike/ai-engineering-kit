package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFS embed.FS

type server struct {
	http.Handler
	rooms *roomStore
}

func newServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	rooms := newRoomStore(randomRoomCode)
	registerRoomRoutes(mux, rooms)
	registerPostRoutes(mux, rooms)
	registerModerationRoutes(mux, rooms)
	registerEventRoutes(mux, rooms, tickingHeartbeat)
	registerQRRoutes(mux, rooms)
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return &server{Handler: mux, rooms: rooms}
}
