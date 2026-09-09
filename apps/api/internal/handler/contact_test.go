package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/notification"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/service"
)

type recordingNotifier struct{ submissions []notification.Submission }

func (notifier *recordingNotifier) Notify(_ context.Context, submission notification.Submission) {
	notifier.submissions = append(notifier.submissions, submission)
}

type fakeStore struct {
	createErr   error
	retryAfter  time.Duration
	retryErr    error
	submissions []repository.ContactSubmission
}

func (store *fakeStore) CooldownRetryAfter(context.Context, []byte, time.Time) (time.Duration, error) {
	return store.retryAfter, store.retryErr
}

func (store *fakeStore) CreateSubmissionAndSetCooldown(_ context.Context, submission repository.ContactSubmission, _ []byte, _ time.Time) error {
	store.submissions = append(store.submissions, submission)
	return store.createErr
}

func (*fakeStore) DeleteSubmissionsOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestContact(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{
		{"valid", `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры","consent":true}`, http.StatusCreated},
		{"valid without project details", `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","consent":true}`, http.StatusCreated},
		{"honeypot", `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры","consent":true,"website":"spam"}`, http.StatusBadRequest},
		{"no consent", `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры"}`, http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			handler := NewContact(service.NewCooldown(time.Now))
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(test.body))
			handler(response, request)
			if response.Code != test.status {
				t.Fatalf("got %d, want %d", response.Code, test.status)
			}
		})
	}
}

func TestContactRateLimitsRepeatedRequestsFromOneIP(t *testing.T) {
	now := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	handler := NewContactWithStoreAndRateLimit(
		service.NewCooldown(func() time.Time { return now }),
		nil,
		nil,
		service.NewRateLimiter(func() time.Time { return now }, time.Minute),
	)
	body := `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры","consent":true}`

	first := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(body))
	firstRequest.RemoteAddr = "198.51.100.10:34567"
	handler(first, firstRequest)
	if first.Code != http.StatusCreated {
		t.Fatalf("first request got %d, want %d", first.Code, http.StatusCreated)
	}

	second := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(body))
	secondRequest.RemoteAddr = "198.51.100.10:45678"
	handler(second, secondRequest)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request got %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", second.Header().Get("Retry-After"))
	}
	if !strings.Contains(second.Body.String(), `"rate_limited"`) {
		t.Fatalf("unexpected error body: %s", second.Body.String())
	}
}

func TestContactRejectsSecondSubmissionForOneHour(t *testing.T) {
	now := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	cooldown := service.NewCooldown(func() time.Time { return now })
	handler := NewContact(cooldown)
	body := `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры","consent":true}`

	firstResponse := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(body))
	handler(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first request got %d, want %d", firstResponse.Code, http.StatusCreated)
	}
	cookies := firstResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "contact_cooldown" {
		t.Fatal("successful submission must set the cooldown cookie")
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(body))
	secondRequest.AddCookie(cookies[0])
	handler(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second request got %d, want %d", secondResponse.Code, http.StatusTooManyRequests)
	}
	if secondResponse.Header().Get("Retry-After") != "3600" {
		t.Fatalf("Retry-After = %q, want 3600", secondResponse.Header().Get("Retry-After"))
	}
	if !strings.Contains(secondResponse.Body.String(), `"contact_cooldown"`) {
		t.Fatalf("unexpected error body: %s", secondResponse.Body.String())
	}
}

func TestContactEndpointNotifiesOnlyAfterAcceptedSubmission(t *testing.T) {
	notifier := &recordingNotifier{}
	endpoint := NewContactEndpoint(
		service.NewCooldown(time.Now), nil, nil, service.NewRateLimiter(time.Now, time.Minute), notifier,
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(`{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","projectDetails":"Нужен проект квартиры","consent":true}`))

	endpoint.CreateContactSubmission(response, request)

	if response.Code != http.StatusCreated || len(notifier.submissions) != 1 {
		t.Fatalf("status = %d, notifications = %d; want 201 and 1", response.Code, len(notifier.submissions))
	}
}

func TestContactWithStoreHandlesPersistenceOutcomes(t *testing.T) {
	body := `{"name":"Анна","contact":"anna@example.com","projectType":"Квартира","consent":true}`
	cases := []struct {
		name   string
		store  *fakeStore
		status int
	}{
		{"stored cooldown", &fakeStore{retryAfter: time.Minute}, http.StatusTooManyRequests},
		{"cooldown lookup error", &fakeStore{retryErr: context.DeadlineExceeded}, http.StatusInternalServerError},
		{"write error", &fakeStore{createErr: context.DeadlineExceeded}, http.StatusInternalServerError},
		{"stored", &fakeStore{}, http.StatusCreated},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			handler := NewContactWithStore(service.NewCooldown(time.Now), test.store, []byte("secret"))
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodPost, "/api/v1/contact-submissions", strings.NewReader(body)))
			if response.Code != test.status {
				t.Fatalf("got %d, want %d", response.Code, test.status)
			}
		})
	}
	if len(cases[3].store.submissions) != 1 || cases[3].store.submissions[0].ProjectDetails != "" {
		t.Fatal("optional project details must persist as an empty string")
	}
}
