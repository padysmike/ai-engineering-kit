package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cucumber/godog"
)

type webClient struct{ response *httptest.ResponseRecorder }

var webClients = perScenario[webClient]{}

func webClientFor(sc *godog.ScenarioContext) *webClient {
	return webClients.of(sc, func() *webClient { return &webClient{} })
}

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		client := webClientFor(sc)

		sc.Step(`^a client requests GET (\S+)$`, func(path string) {
			client.response = httptest.NewRecorder()
			newServer().ServeHTTP(client.response, httptest.NewRequest(http.MethodGet, path, nil))
		})
		sc.Step(`^the response status is (\d+)$`, func(want int) error {
			if client.response.Code != want {
				return fmt.Errorf("status = %d, want %d", client.response.Code, want)
			}
			return nil
		})
		sc.Step(`^the body contains (.+)$`, func(want string) error {
			if !strings.Contains(client.response.Body.String(), want) {
				return fmt.Errorf("body %q lacks %s", client.response.Body.String(), want)
			}
			return nil
		})
	})
}
