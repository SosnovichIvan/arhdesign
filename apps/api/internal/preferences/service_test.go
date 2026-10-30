package preferences

import (
	"context"
	"errors"
	"testing"
)

const testUserID = "00000000-0000-0000-0000-000000000001"
const testProjectID = "00000000-0000-0000-0000-000000000010"

type fakeStore struct {
	user         UserSettings
	project      ProjectUserSettings
	actor        Actor
	userID       string
	projectID    string
	err          error
	savedUser    UserSettings
	savedProject ProjectUserSettings
}

func (store *fakeStore) GetUserSettings(_ context.Context, userID string) (UserSettings, error) {
	store.userID = userID
	return store.user, store.err
}
func (store *fakeStore) SaveUserSettings(_ context.Context, userID string, settings UserSettings) (UserSettings, error) {
	store.userID, store.savedUser = userID, settings
	return settings, store.err
}
func (store *fakeStore) GetProjectUserSettings(_ context.Context, actor Actor, projectID string) (ProjectUserSettings, error) {
	store.actor, store.projectID = actor, projectID
	return store.project, store.err
}
func (store *fakeStore) SaveProjectUserSettings(_ context.Context, actor Actor, projectID string, settings ProjectUserSettings) (ProjectUserSettings, error) {
	store.actor, store.projectID, store.savedProject = actor, projectID, settings
	return settings, store.err
}

func TestServiceReadsAndSavesUserAndProjectSettings(t *testing.T) {
	store := &fakeStore{user: UserSettings{Theme: "dark"}, project: ProjectUserSettings{UpcomingDays: 14}}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.GetUser(context.Background(), testUserID)
	if err != nil || user.Theme != "dark" || store.userID != testUserID {
		t.Fatalf("user=%#v userID=%s error=%v", user, store.userID, err)
	}
	actor := Actor{UserID: testUserID, SuperAdmin: true}
	project, err := service.GetProject(context.Background(), actor, testProjectID)
	if err != nil || project.UpcomingDays != 14 || store.actor != actor || store.projectID != testProjectID {
		t.Fatalf("project=%#v actor=%#v projectID=%s error=%v", project, store.actor, store.projectID, err)
	}
	if _, err = service.SaveUser(context.Background(), testUserID, UserSettings{Theme: "light"}); err != nil || store.savedUser.Theme != "light" {
		t.Fatalf("saved user=%#v error=%v", store.savedUser, err)
	}
	if _, err = service.SaveProject(context.Background(), actor, testProjectID, ProjectUserSettings{UpcomingDays: 30}); err != nil || store.savedProject.UpcomingDays != 30 {
		t.Fatalf("saved project=%#v error=%v", store.savedProject, err)
	}
}

func TestServiceRejectsInvalidSettingsAndPreservesStoreErrors(t *testing.T) {
	service, err := NewService(&fakeStore{})
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range []func() error{
		func() error { _, err := service.GetUser(context.Background(), "bad"); return err },
		func() error {
			_, err := service.SaveUser(context.Background(), testUserID, UserSettings{Theme: "system"})
			return err
		},
		func() error {
			_, err := service.GetProject(context.Background(), Actor{UserID: "bad"}, testProjectID)
			return err
		},
		func() error {
			_, err := service.SaveProject(context.Background(), Actor{UserID: testUserID}, testProjectID, ProjectUserSettings{UpcomingDays: 0})
			return err
		},
		func() error {
			_, err := service.SaveProject(context.Background(), Actor{UserID: testUserID}, testProjectID, ProjectUserSettings{UpcomingDays: 91})
			return err
		},
	} {
		if err := run(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("error=%v", err)
		}
	}
	storeErr := errors.New("database unavailable")
	service, _ = NewService(&fakeStore{err: storeErr})
	if _, err = service.GetUser(context.Background(), testUserID); !errors.Is(err, storeErr) {
		t.Fatalf("store error=%v", err)
	}
	if _, err = NewService(nil); err == nil {
		t.Fatal("nil store was accepted")
	}
}
