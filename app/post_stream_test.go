package main

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Approved step files are frozen, so the shared server's recording wrapper
// must already work for the live-update stream that a later slice adds.
func TestRecordDataToHost_StreamStillOpen_DataArrivesAndIsRecorded(t *testing.T) {
	release := make(chan struct{})
	p := &posting{hostTokens: map[string]string{}, roomsHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "data: first\n")
		<-release
	})}
	server := httptest.NewServer(http.HandlerFunc(p.recordDataToHost))
	defer server.Close()
	defer close(release)
	client := &http.Client{Timeout: 5 * time.Second}

	response, err := client.Get(server.URL + "/rooms/ABCDEF/events")
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	line, err := bufio.NewReader(response.Body).ReadString('\n')

	if err != nil || line != "data: first\n" {
		t.Fatalf("line = %q, err = %v", line, err)
	}
	if sent := p.sentToHost(); !strings.Contains(sent, "data: first") {
		t.Errorf("recorded %q lacks the streamed line", sent)
	}
}
