package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// probeServer answers one provider HTTP call with a fixed JSON body over plain
// HTTP (bindings build their own http.Client, so the fixture stays on a port
// the default transport trusts). status 0 means success.
func probeServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}
