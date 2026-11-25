package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cucumber/godog"
)

func init() {
	registerSteps(func(sc *godog.ScenarioContext) {
		var rec *httptest.ResponseRecorder

		sc.Step(`^a client requests GET (\S+)$`, func(path string) {
			rec = httptest.NewRecorder()
			newServer().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		})
		sc.Step(`^the response status is (\d+)$`, func(want int) error {
			if rec.Code != want {
				return fmt.Errorf("status = %d, want %d", rec.Code, want)
			}
			return nil
		})
		sc.Step(`^the body contains (.+)$`, func(want string) error {
			if !strings.Contains(rec.Body.String(), want) {
				return fmt.Errorf("body %q lacks %s", rec.Body.String(), want)
			}
			return nil
		})
	})
}
