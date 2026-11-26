package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
)

func TestReleaseNotesRequireAnAuthenticatedAccount(t *testing.T) {
	store := &endpointAccountStore{accessFound: true, accessAccount: account.AccountView{Status: "active"}}
	endpoint := accountEndpointForTest(t, store)
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
}
