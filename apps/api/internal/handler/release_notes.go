package handler

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
)

//go:embed release-notes.json
var releaseNotes []byte

var releaseNotesRevision = fmt.Sprintf("%x", sha256.Sum256(releaseNotes))

func (endpoint *AccountEndpoint) ListReleaseNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if _, ok := endpoint.authenticatedAccount(w, r, accountRequestID(w, r)); !ok {
		return
	}
	w.Header().Set("X-Release-Notes-Revision", releaseNotesRevision)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(releaseNotes)
}
