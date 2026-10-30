//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
)

func TestUserAndProjectSettingsPersistPerUserAndEnforceScope(t *testing.T) {
	pool := newMigrationSchemaPool(t, "user_project_settings")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx := context.Background()
	ownerID := insertProjectTestUser(t, pool, "settings.owner", "settings.owner@example.com")
	otherID := insertProjectTestUser(t, pool, "settings.other", "settings.other@example.com")

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var projectID string
	if err = tx.QueryRow(ctx, `INSERT INTO projects (created_by_user_id,customer_user_id,name,type) VALUES ($1,$1,'Настройки проекта','interior_design') RETURNING id::text`, ownerID).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO project_memberships (project_id,user_id,project_role) VALUES ($1,$2,'customer')`, projectID, ownerID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	user, err := store.GetUserSettings(ctx, ownerID)
	if err != nil || user.Theme != preferences.DefaultTheme {
		t.Fatalf("default user settings=%#v error=%v", user, err)
	}
	user, err = store.SaveUserSettings(ctx, ownerID, preferences.UserSettings{Theme: "dark"})
	if err != nil || user.Theme != "dark" {
		t.Fatalf("saved user settings=%#v error=%v", user, err)
	}
	user, err = store.GetUserSettings(ctx, ownerID)
	if err != nil || user.Theme != "dark" {
		t.Fatalf("restored user settings=%#v error=%v", user, err)
	}

	actor := preferences.Actor{UserID: ownerID}
	projectSettings, err := store.GetProjectUserSettings(ctx, actor, projectID)
	if err != nil || projectSettings.UpcomingDays != preferences.DefaultUpcomingDays {
		t.Fatalf("default project settings=%#v error=%v", projectSettings, err)
	}
	projectSettings, err = store.SaveProjectUserSettings(ctx, actor, projectID, preferences.ProjectUserSettings{UpcomingDays: 21})
	if err != nil || projectSettings.UpcomingDays != 21 {
		t.Fatalf("saved project settings=%#v error=%v", projectSettings, err)
	}
	projectSettings, err = store.GetProjectUserSettings(ctx, actor, projectID)
	if err != nil || projectSettings.UpcomingDays != 21 {
		t.Fatalf("restored project settings=%#v error=%v", projectSettings, err)
	}

	if _, err = store.GetProjectUserSettings(ctx, preferences.Actor{UserID: otherID}, projectID); !errors.Is(err, preferences.ErrNotFound) {
		t.Fatalf("foreign read error=%v", err)
	}
	if _, err = store.SaveProjectUserSettings(ctx, preferences.Actor{UserID: otherID}, projectID, preferences.ProjectUserSettings{UpcomingDays: 30}); !errors.Is(err, preferences.ErrNotFound) {
		t.Fatalf("foreign write error=%v", err)
	}

	if _, err = pool.Exec(ctx, `INSERT INTO user_settings (user_id,theme) VALUES ($1,'system')`, otherID); err == nil {
		t.Fatal("invalid theme was accepted")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO project_user_settings (project_id,user_id,upcoming_days) VALUES ($1,$2,91)`, projectID, otherID); err == nil {
		t.Fatal("invalid upcoming period was accepted")
	}
}
