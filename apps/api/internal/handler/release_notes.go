package handler

import (
	_ "embed"
	"net/http"
)

//go:embed release-notes.json
var releaseNotes []byte

func (endpoint *AccountEndpoint) ListReleaseNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if _, ok := endpoint.authenticatedAccount(w, r, accountRequestID(w, r)); !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(releaseNotes)
}
