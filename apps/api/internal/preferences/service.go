package preferences

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

const (
	DefaultTheme        = "light"
	DefaultUpcomingDays = 7
)

var (
	ErrInvalidInput = errors.New("invalid settings input")
	ErrNotFound     = errors.New("settings resource not found")
)

type Actor struct {
	UserID     string
	SuperAdmin bool
}

type UserSettings struct {
	Theme string
}

type ProjectUserSettings struct {
	UpcomingDays int
}

type Store interface {
	GetUserSettings(context.Context, string) (UserSettings, error)
	SaveUserSettings(context.Context, string, UserSettings) (UserSettings, error)
	GetProjectUserSettings(context.Context, Actor, string) (ProjectUserSettings, error)
	SaveProjectUserSettings(context.Context, Actor, string, ProjectUserSettings) (ProjectUserSettings, error)
}

type Service struct {
	store Store
}

func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("settings service requires store")
	}
	return &Service{store: store}, nil
}

func (service *Service) GetUser(ctx context.Context, userID string) (UserSettings, error) {
	if !validID(userID) {
		return UserSettings{}, ErrInvalidInput
	}
	return service.store.GetUserSettings(ctx, userID)
}

func (service *Service) SaveUser(ctx context.Context, userID string, settings UserSettings) (UserSettings, error) {
	if !validID(userID) || !validTheme(settings.Theme) {
		return UserSettings{}, ErrInvalidInput
	}
	return service.store.SaveUserSettings(ctx, userID, settings)
}

func (service *Service) GetProject(ctx context.Context, actor Actor, projectID string) (ProjectUserSettings, error) {
	if !validID(actor.UserID) || !validID(projectID) {
		return ProjectUserSettings{}, ErrInvalidInput
	}
	return service.store.GetProjectUserSettings(ctx, actor, projectID)
}

func (service *Service) SaveProject(ctx context.Context, actor Actor, projectID string, settings ProjectUserSettings) (ProjectUserSettings, error) {
	if !validID(actor.UserID) || !validID(projectID) || settings.UpcomingDays < 1 || settings.UpcomingDays > 90 {
		return ProjectUserSettings{}, ErrInvalidInput
	}
	return service.store.SaveProjectUserSettings(ctx, actor, projectID, settings)
}

func validID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil
}

func validTheme(theme string) bool {
	return theme == "light" || theme == "dark"
}
