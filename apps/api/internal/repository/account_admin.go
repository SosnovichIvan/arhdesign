package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListAdminUsers(ctx context.Context, query account.AdminUserQuery) ([]account.AdminUserView, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT users.id::text, users.login, users.email, profile.first_name, profile.last_name,
		       profile.middle_name, role.code, role.name, users.status, users.global_role,
		       users.version, users.created_at, users.last_login_at
		  FROM users
		  JOIN profiles profile ON profile.user_id = users.id
		  JOIN professional_roles role ON role.id = profile.professional_role_id
		 WHERE ($1 = '' OR users.status = $1)
		   AND ($2 = '' OR users.login_normalized = $2 OR users.email_normalized = $2)
		   AND ($3::timestamptz IS NULL OR (users.created_at, users.id) < ($3, $4::uuid))
		 ORDER BY users.created_at DESC, users.id DESC
		 LIMIT $5`, query.Status, query.Identifier, query.BeforeRegisteredAt, query.BeforeID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	defer rows.Close()
	items := make([]account.AdminUserView, 0, query.Limit)
	for rows.Next() {
		view, err := scanAdminUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		items = append(items, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin users: %w", err)
	}
	return items, nil
}

func (repository *Postgres) SetAdminUserStatus(ctx context.Context, command account.AdminUserStatusCommand) (account.AdminUserView, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return account.AdminUserView{}, fmt.Errorf("begin admin user status: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var globalRole *string
	var emailVerified bool
	var version int64
	err = tx.QueryRow(ctx, `
		SELECT status, global_role, email_verified_at IS NOT NULL, version
		  FROM users WHERE id = $1 FOR UPDATE`, command.TargetUserID).Scan(&status, &globalRole, &emailVerified, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return account.AdminUserView{}, account.ErrAdminUserNotFound
	}
	if err != nil {
		return account.AdminUserView{}, fmt.Errorf("lock admin user: %w", err)
	}
	targetStatus := "disabled"
	if !command.Disable {
		targetStatus = "pending_verification"
		if emailVerified {
			targetStatus = "active"
		}
	}
	if status == targetStatus || !command.Disable && status != "disabled" {
		return loadAdminUser(ctx, tx, command.TargetUserID)
	}
	if version != command.ExpectedVersion {
		return account.AdminUserView{}, account.ErrAdminUserConflict
	}
	if command.Disable && globalRole != nil && *globalRole == "super_admin" {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('arhdesign.active-super-admin'))`); err != nil {
			return account.AdminUserView{}, fmt.Errorf("lock super administrator invariant: %w", err)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status='active' AND global_role='super_admin'`).Scan(&count); err != nil {
			return account.AdminUserView{}, fmt.Errorf("count active super administrators: %w", err)
		}
		if count <= 1 {
			return account.AdminUserView{}, account.ErrLastSuperAdmin
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		   SET status=$2, security_version=security_version+1, version=version+1, updated_at=$3
		 WHERE id=$1`, command.TargetUserID, targetStatus, command.Now); err != nil {
		return account.AdminUserView{}, fmt.Errorf("update admin user status: %w", err)
	}
	if command.Disable {
		if _, err := tx.Exec(ctx, `
			UPDATE sessions SET revoked_at=$2, revoke_reason='account_disabled'
			 WHERE user_id=$1 AND revoked_at IS NULL`, command.TargetUserID, command.Now); err != nil {
			return account.AdminUserView{}, fmt.Errorf("revoke disabled user sessions: %w", err)
		}
	}
	eventType := "account.restored"
	if command.Disable {
		eventType = "account.disabled"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events (event_type, result, request_id, actor_user_id, subject_type, subject_user_id, occurred_at)
		VALUES ($1, 'success', $2, $3, 'user', $4, $5)`, eventType, command.RequestID, command.ActorUserID, command.TargetUserID, command.Now); err != nil {
		return account.AdminUserView{}, fmt.Errorf("append admin user audit: %w", err)
	}
	view, err := loadAdminUser(ctx, tx, command.TargetUserID)
	if err != nil {
		return account.AdminUserView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return account.AdminUserView{}, fmt.Errorf("commit admin user status: %w", err)
	}
	return view, nil
}

type adminUserScanner interface{ Scan(...any) error }

func scanAdminUser(row adminUserScanner) (account.AdminUserView, error) {
	var view account.AdminUserView
	err := row.Scan(&view.ID, &view.Login, &view.Email, &view.FirstName, &view.LastName, &view.MiddleName,
		&view.ProfessionalRoleCode, &view.ProfessionalRoleName, &view.Status, &view.GlobalRole,
		&view.Version, &view.RegisteredAt, &view.LastInteractiveLoginAt)
	return view, err
}

func loadAdminUser(ctx context.Context, tx pgx.Tx, userID string) (account.AdminUserView, error) {
	view, err := scanAdminUser(tx.QueryRow(ctx, `
		SELECT users.id::text, users.login, users.email, profile.first_name, profile.last_name,
		       profile.middle_name, role.code, role.name, users.status, users.global_role,
		       users.version, users.created_at, users.last_login_at
		  FROM users
		  JOIN profiles profile ON profile.user_id=users.id
		  JOIN professional_roles role ON role.id=profile.professional_role_id
		 WHERE users.id=$1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return account.AdminUserView{}, account.ErrAdminUserNotFound
	}
	if err != nil {
		return account.AdminUserView{}, fmt.Errorf("load admin user: %w", err)
	}
	return view, nil
}
