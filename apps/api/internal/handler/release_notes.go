package handler

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"slices"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
)

//go:embed release-notes.json
var releaseNotesJSON []byte

var releaseCatalog = loadReleaseCatalog()

func loadReleaseCatalog() []accountgenerated.ReleaseNote {
	var notes []accountgenerated.ReleaseNote
	if err := json.Unmarshal(releaseNotesJSON, &notes); err != nil {
		panic(err)
	}
	return notes
}

type releaseNoteReadStore interface {
	ReadReleaseVersions(ctx context.Context, userID string, versions []string) ([]string, error)
	MarkReleaseVersionRead(ctx context.Context, userID, version string) error
}

func (endpoint *AccountEndpoint) ConfigureReleaseNoteReads(store releaseNoteReadStore) {
	endpoint.releaseNotes = store
}

func (endpoint *AccountEndpoint) ListReleaseNotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	requestID := accountRequestID(w, r)
	viewer, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return
	}
	versions := make([]string, 0, len(releaseCatalog))
	for _, note := range releaseCatalog {
		versions = append(versions, note.Version)
	}
	read, err := endpoint.releaseNotes.ReadReleaseVersions(r.Context(), viewer.ID, versions)
	if err != nil {
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось загрузить обновления", requestID)
		return
	}
	response := slices.Clone(releaseCatalog)
	for index := range response {
		response[index].Read = slices.Contains(read, response[index].Version)
	}
	writeAccountJSON(w, http.StatusOK, response)
}

func (endpoint *AccountEndpoint) MarkReleaseNoteRead(w http.ResponseWriter, r *http.Request, version string) {
	w.Header().Set("Cache-Control", "private, no-store")
	requestID := accountRequestID(w, r)
	viewer, ok := endpoint.authenticatedAccount(w, r, requestID)
	if !ok {
		return
	}
	if _, ok := endpoint.validAuthenticatedMutation(r); !ok {
		writeAccountError(w, http.StatusForbidden, "csrf_rejected", "Запрос отклонён", requestID)
		return
	}
	if !slices.ContainsFunc(releaseCatalog, func(note accountgenerated.ReleaseNote) bool { return note.Version == version }) {
		writeAccountError(w, http.StatusNotFound, "resource_hidden", "Выпуск не найден", requestID)
		return
	}
	if err := endpoint.releaseNotes.MarkReleaseVersionRead(r.Context(), viewer.ID, version); err != nil {
		writeAccountError(w, http.StatusInternalServerError, "internal_error", "Не удалось отметить обновление прочитанным", requestID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
