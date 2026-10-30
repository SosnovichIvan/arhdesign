package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) GetUserSettings(ctx context.Context, userID string) (preferences.UserSettings, error) {
	settings := preferences.UserSettings{Theme: preferences.DefaultTheme}
	err := repository.pool.QueryRow(ctx, `
		SELECT COALESCE((SELECT theme FROM user_settings WHERE user_id=$1), $2)
	`, userID, preferences.DefaultTheme).Scan(&settings.Theme)
	if err != nil {
		return preferences.UserSettings{}, fmt.Errorf("get user settings: %w", err)
	}
	return settings, nil
}

func (repository *Postgres) SaveUserSettings(ctx context.Context, userID string, settings preferences.UserSettings) (preferences.UserSettings, error) {
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO user_settings (user_id,theme,updated_at)
		VALUES ($1,$2,now())
		ON CONFLICT (user_id) DO UPDATE SET theme=EXCLUDED.theme,updated_at=EXCLUDED.updated_at
		RETURNING theme
	`, userID, settings.Theme).Scan(&settings.Theme)
	if err != nil {
		return preferences.UserSettings{}, fmt.Errorf("save user settings: %w", err)
	}
	return settings, nil
}

func (repository *Postgres) GetProjectUserSettings(ctx context.Context, actor preferences.Actor, projectID string) (preferences.ProjectUserSettings, error) {
	settings := preferences.ProjectUserSettings{UpcomingDays: preferences.DefaultUpcomingDays}
	err := repository.pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT personal.upcoming_days
			FROM project_user_settings personal
			WHERE personal.project_id=project.id AND personal.user_id=$2
		), $4)
		FROM projects project
		WHERE project.id=$1 AND project.archived_at IS NULL
		  AND ($3 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=project.id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		  ))
	`, projectID, actor.UserID, actor.SuperAdmin, preferences.DefaultUpcomingDays).Scan(&settings.UpcomingDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return preferences.ProjectUserSettings{}, preferences.ErrNotFound
	}
	if err != nil {
		return preferences.ProjectUserSettings{}, fmt.Errorf("get project user settings: %w", err)
	}
	return settings, nil
}

func (repository *Postgres) SaveProjectUserSettings(ctx context.Context, actor preferences.Actor, projectID string, settings preferences.ProjectUserSettings) (preferences.ProjectUserSettings, error) {
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO project_user_settings (project_id,user_id,upcoming_days,updated_at)
		SELECT project.id,$2,$4,now()
		FROM projects project
		WHERE project.id=$1 AND project.archived_at IS NULL
		  AND ($3 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=project.id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		  ))
		ON CONFLICT (project_id,user_id) DO UPDATE
		SET upcoming_days=EXCLUDED.upcoming_days,updated_at=EXCLUDED.updated_at
		RETURNING upcoming_days
	`, projectID, actor.UserID, actor.SuperAdmin, settings.UpcomingDays).Scan(&settings.UpcomingDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return preferences.ProjectUserSettings{}, preferences.ErrNotFound
	}
	if err != nil {
		return preferences.ProjectUserSettings{}, fmt.Errorf("save project user settings: %w", err)
	}
	return settings, nil
}
