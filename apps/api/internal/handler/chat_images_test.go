package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/chatimage"
)

type chatImageTestStore struct {
	access chatimage.Access
	items  []chatimage.Image
	cursor *chatimage.Cursor
	upload chatimage.Upload
	pinned bool
	err    error
}

func (store *chatImageTestStore) ListChatImages(_ context.Context, access chatimage.Access, cursor *chatimage.Cursor) ([]chatimage.Image, error) {
	store.access, store.cursor = access, cursor
	return store.items, store.err
}

func (store *chatImageTestStore) StoreChatImage(_ context.Context, access chatimage.Access, upload chatimage.Upload) (chatimage.Image, error) {
	store.access, store.upload = access, upload
	return upload.Image, store.err
}

func (store *chatImageTestStore) ViewChatImage(_ context.Context, access chatimage.Access, _ string) (chatimage.Content, error) {
	store.access = access
	return chatimage.Content{MediaType: "image/png", Bytes: store.upload.Content}, store.err
}

func (store *chatImageTestStore) SetChatImagePin(_ context.Context, access chatimage.Access, _ string, pinned bool) error {
	store.access, store.pinned = access, pinned
	return store.err
}

func (store *chatImageTestStore) DeleteChatImage(_ context.Context, access chatimage.Access, _ string) error {
	store.access = access
	return store.err
}

func TestChatImageHTTPContract(t *testing.T) {
	role := "technical_admin"
	endpoint := accountEndpointForTest(t, activeAccount(&role))
	store := &chatImageTestStore{}
	server := accountgenerated.HandlerWithOptions(endpoint, accountgenerated.StdHTTPServerOptions{BaseURL: "/api"})
	listURL := "/api/v1/chat-images/" + endpointProjectID
	projectQuery := "?projectId=" + endpointProjectID
	imageURL := listURL + "/" + endpointMemberID
	png := "\x89PNG\r\n\x1a\n"
	payload, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+X2GQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, url, body string, expected int, update func(*http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		req := authenticatedRequest(method, url, body)
		if method == http.MethodPost {
			req.Header.Set("X-File-Name", "UTF-8''%D0%9F%D0%BB%D0%B0%D0%BD.png")
			req.Header.Set("Idempotency-Key", endpointMemberID)
		}
		if update != nil {
			update(req)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, req)
		if response.Code != expected {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, url, response.Code, expected, response.Body)
		}
		return response
	}
	request(http.MethodGet, listURL, "", 503, nil)
	endpoint.ConfigureChatImages(chatimage.NewService(store))
	request(http.MethodGet, listURL, "", 401, func(r *http.Request) { r.Header.Del("Cookie") })
	request(http.MethodDelete, imageURL, "", 403, func(r *http.Request) { r.Header.Del("X-CSRF-Token") })
	request(http.MethodPost, listURL, string(payload), 400, func(r *http.Request) { r.ContentLength = chatimage.MaxBytes + 1 })
	request(http.MethodPost, listURL, string(payload), 400, func(r *http.Request) { r.Header.Set("X-File-Name", "UTF-8''%xx") })
	request(http.MethodPost, listURL, string(payload), 400, func(r *http.Request) { r.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000000") })
	request(http.MethodPost, listURL, "", 400, nil)
	request(http.MethodPost, listURL, png, 400, nil)
	request(http.MethodPost, listURL, string(payload), 400, func(r *http.Request) { r.Header.Set("X-File-Name", "../plan.png") })

	upload := request(http.MethodPost, listURL+projectQuery, string(payload), 201, nil)
	if store.upload.Name != "План.png" || store.upload.Width != 1 || !store.access.SuperAdmin || store.access.ProjectID != endpointProjectID || store.access.UserID != endpointActorID {
		t.Fatalf("upload contract: %s %+v", upload.Body, store.access)
	}
	download := request(http.MethodGet, imageURL, "", 200, nil)
	if !bytes.Equal(download.Body.Bytes(), payload) || download.Header().Get("Content-Type") != "image/png" || download.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("download contract: %+v", download.Header())
	}
	request(http.MethodPut, imageURL, `{}`, 400, nil)
	request(http.MethodPut, imageURL, `{"pinned":false}`, 204, nil)
	if store.pinned {
		t.Fatal("explicit false did not unpin")
	}
	request(http.MethodPut, imageURL, `{"pinned":true}`, 204, nil)
	if !store.pinned {
		t.Fatal("image was not pinned")
	}
	request(http.MethodDelete, imageURL, "", 204, nil)

	item := store.upload.Image
	item.CreatedAt = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	item.Pinned = true
	for range chatimage.PageSize + 1 {
		store.items = append(store.items, item)
	}
	list := request(http.MethodGet, listURL, "", 200, nil)
	var page accountgenerated.ChatImagePage
	if err = json.NewDecoder(list.Body).Decode(&page); err != nil || len(page.Items) != chatimage.PageSize || page.NextCursor == nil {
		t.Fatalf("list = %+v error=%v", page, err)
	}
	request(http.MethodGet, listURL+"?cursor="+*page.NextCursor, "", 200, nil)
	if store.cursor == nil || !store.cursor.Pinned || store.cursor.ID != endpointMemberID || !store.cursor.CreatedAt.Equal(item.CreatedAt) {
		t.Fatalf("cursor: %+v", store.cursor)
	}
	for _, invalid := range []string{"!", strings.Repeat("a", 129), "YQ", "dHJ1ZTowOmJhZA"} {
		request(http.MethodGet, listURL+"?cursor="+invalid, "", 400, nil)
	}

	store.err = chatimage.ErrNotFound
	request(http.MethodGet, listURL, "", 404, nil)
	request(http.MethodGet, imageURL, "", 404, nil)
	request(http.MethodPut, imageURL, `{"pinned":true}`, 404, nil)
	store.err = chatimage.ErrForbidden
	request(http.MethodDelete, imageURL, "", 403, nil)
	store.err = chatimage.ErrConflict
	request(http.MethodPost, listURL, string(payload), 409, nil)
	store.err = errors.New("database busy")
	request(http.MethodGet, listURL, "", 500, nil)
}

func TestContactFormatValidation(t *testing.T) {
	for _, valid := range []string{"anna@example.com", "имя@studio.example", "+7 (900) 123-45-67", "8 900 123-45-67", "+123456789012345"} {
		if !validContact(valid) {
			t.Errorf("valid format rejected: %s", valid)
		}
	}
	for _, invalid := range []string{"груша", "anna@localhost", "name@example.-com", "x@y", "12", "123456789", "+1234567890123456", "anna <anna@example.com>", "name@example.com/comment"} {
		if validContact(invalid) {
			t.Errorf("invalid format accepted: %s", invalid)
		}
	}
}
