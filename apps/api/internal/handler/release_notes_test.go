package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
)

type releaseNoteStoreStub struct {
	read []string
	err  error
}

func (store *releaseNoteStoreStub) ReadReleaseVersions(context.Context, string, []string) ([]string, error) {
	return store.read, store.err
}

func (store *releaseNoteStoreStub) MarkReleaseVersionRead(_ context.Context, _, version string) error {
	store.read = []string{version}
	return store.err
}

func TestReleaseNotesRequireAnAuthenticatedAccount(t *testing.T) {
	store := &endpointAccountStore{accessFound: true, accessAccount: account.AccountView{Status: "active"}}
	endpoint := accountEndpointForTest(t, store)
	reads := &releaseNoteStoreStub{}
	endpoint.ConfigureReleaseNoteReads(reads)
	mux := accountgenerated.HandlerWithOptions(endpoint, accountgenerated.StdHTTPServerOptions{BaseURL: "/api"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/release-notes", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "Артемий") {
		t.Fatalf("anonymous request status = %d", response.Code)
	}

	request.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("s", 43)})
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var notes []accountgenerated.ReleaseNote
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" || json.Unmarshal(response.Body.Bytes(), &notes) != nil {
		t.Fatalf("release notes response status = %d", response.Code)
	}
	if len(notes) == 0 || notes[0].Version != "1.0.19" || len(notes[0].Added) == 0 || len(notes[0].Fixed) < 2 || notes[0].Fixed[0].Source != "Артемий (@Artemy)" || notes[0].Fixed[1].Source != "Артемий (@Artemy)" {
		t.Fatal("expected a versioned, attributed release note")
	}
	if notes[0].Read {
		t.Fatal("new release must be unread")
	}
	for _, test := range []struct {
		version string
		useAuth bool
		useCSRF bool
		want    int
	}{
		{"1.0.19", false, false, 401},
		{"1.0.19", true, false, 403},
		{"9.9.9", true, true, 404},
		{"1.0.19", true, true, 204},
	} {
		mark := httptest.NewRequest(http.MethodPut, "/api/v1/release-notes/"+test.version+"/read", nil)
		if test.useAuth {
			mark.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("s", 43)})
		}
		if test.useCSRF {
			mark.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: "csrf"})
			mark.Header.Set("X-CSRF-Token", "csrf")
			mark.Header.Set("Origin", "https://designer-svetlana.ru")
		}
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, mark)
		if response.Code != test.want {
			t.Fatalf("mark status = %d, want %d", response.Code, test.want)
		}
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if err := json.Unmarshal(response.Body.Bytes(), &notes); err != nil || !notes[0].Read {
		t.Fatalf("read state did not persist: %v", err)
	}
	reads.err = errors.New("database unavailable")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("read-store error status = %d", response.Code)
	}
	mark := httptest.NewRequest(http.MethodPut, "/api/v1/release-notes/1.0.19/read", nil)
	mark.AddCookie(&http.Cookie{Name: "__Host-arhdesign_session", Value: strings.Repeat("s", 43)})
	mark.AddCookie(&http.Cookie{Name: "__Host-arhdesign_csrf", Value: "csrf"})
	mark.Header.Set("X-CSRF-Token", "csrf")
	mark.Header.Set("Origin", "https://designer-svetlana.ru")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, mark)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("mark-store error status = %d", response.Code)
	}
}
