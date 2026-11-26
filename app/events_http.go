package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"
)

const (
	heartbeatInterval  = 15 * time.Second
	streamWriteTimeout = 10 * time.Second
)

var heartbeatFrame = eventFrame(": heartbeat\n\n")

type heartbeatSource func() (beats <-chan time.Time, stop func())

func tickingHeartbeat() (<-chan time.Time, func()) {
	ticker := time.NewTicker(heartbeatInterval)
	return ticker.C, ticker.Stop
}

func registerEventRoutes(mux *http.ServeMux, rooms *roomStore, heartbeats heartbeatSource) {
	mux.HandleFunc("GET /rooms/{code}/events", eventsHandler(rooms, heartbeats))
}

func eventsHandler(rooms *roomStore, heartbeats heartbeatSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		events, current, err := rooms.subscribe(code, bearerToken(r))
		if err != nil {
			writeRoomError(w, err)
			return
		}
		defer rooms.unsubscribe(code, events)
		first, err := snapshotFrame(current)
		if err != nil {
			log.Print(err)
			http.Error(w, "snapshot failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		beats, stop := heartbeats()
		defer stop()
		stream := eventStream{writer: w, control: http.NewResponseController(w)}
		if err := stream.run(r.Context(), first, events, beats); err != nil {
			log.Printf("event stream of room %s: %v", code, err)
		}
	}
}

func snapshotFrame(current room) (eventFrame, error) {
	slices.SortFunc(current.posts, newestFirst)
	return newEventFrame(eventSnapshot, snapshotView{Room: viewOfRoom(current), Posts: viewsOfPosts(current.posts)})
}

type eventStream struct {
	writer  http.ResponseWriter
	control *http.ResponseController
}

func (s eventStream) run(ctx context.Context, first eventFrame, events subscriber, beats <-chan time.Time) error {
	if err := s.send(first); err != nil {
		return err
	}
	for {
		var next eventFrame
		select {
		case <-ctx.Done():
			return nil
		case frame, subscribed := <-events:
			if !subscribed {
				return nil
			}
			next = frame
		case <-beats:
			next = heartbeatFrame
		}
		if err := s.send(next); err != nil {
			return err
		}
	}
}

// send limits each write on its own, so a client that stops reading cannot
// hold the stream open while an idle stream is never cut off.
func (s eventStream) send(frame eventFrame) error {
	if err := s.limitWrite(time.Now().Add(streamWriteTimeout)); err != nil {
		return err
	}
	if _, err := s.writer.Write(frame); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	if err := s.control.Flush(); err != nil {
		return fmt.Errorf("flush event: %w", err)
	}
	return s.limitWrite(time.Time{})
}

func (s eventStream) limitWrite(deadline time.Time) error {
	err := s.control.SetWriteDeadline(deadline)
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("set write deadline: %w", err)
	}
	return nil
}
