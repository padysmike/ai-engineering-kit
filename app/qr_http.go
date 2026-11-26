package main

import (
	"log"
	"net/http"
	"net/url"

	"rsc.io/qr"
)

type qrView struct {
	URL string `json:"url"`
}

type hostQRHandler func(w http.ResponseWriter, joinURL string)

func registerQRRoutes(mux *http.ServeMux, rooms *roomStore) {
	mux.HandleFunc("GET /rooms/{code}/qr", forHostQR(rooms, currentQR, writeQRURL))
	mux.HandleFunc("GET /rooms/{code}/qr.png", forHostQR(rooms, currentQR, writeQRImage))
	mux.HandleFunc("POST /rooms/{code}/qr/rotate", forHostQR(rooms, rotatedQR, writeQRURL))
}

type qrSource func(rooms *roomStore, hosted room) (room, error)

func currentQR(_ *roomStore, hosted room) (room, error) { return hosted, nil }

func rotatedQR(rooms *roomStore, hosted room) (room, error) { return rooms.rotateQR(hosted.code) }

func forHostQR(rooms *roomStore, source qrSource, write hostQRHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		found, err := hostedRoom(rooms, r.PathValue("code"), bearerToken(r))
		if err == nil {
			found, err = source(rooms, found)
		}
		if err != nil {
			writeRoomError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		write(w, joinPageURL(r, found.code, found.newQRToken(rooms.now())))
	}
}

func writeQRURL(w http.ResponseWriter, joinURL string) {
	writeJSON(w, http.StatusOK, qrView{URL: joinURL})
}

func writeQRImage(w http.ResponseWriter, joinURL string) {
	code, err := qr.Encode(joinURL, qr.M)
	if err != nil {
		log.Printf("draw QR code: %v", err)
		http.Error(w, "could not draw the QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	if _, err := w.Write(code.PNG()); err != nil {
		log.Printf("write QR image: %v", err)
	}
}

// The token sits in the fragment, which browsers do not send to servers, so
// opening the URL does not put it into request logs.
func joinPageURL(r *http.Request, code, qrToken string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	page := url.URL{Scheme: scheme, Host: r.Host, Path: "/", Fragment: "room=" + code + "&qr=" + qrToken}
	return page.String()
}
