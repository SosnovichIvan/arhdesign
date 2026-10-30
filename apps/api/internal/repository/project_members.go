package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectMembers(ctx context.Context, actor project.Actor, projectID string) ([]project.Member, bool, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return nil, false, err
	}
	canManage, err := canManageProjectMembers(ctx, repository.pool, actor, projectID)
	if err != nil {
		return nil, false, err
	}
	items, err := listProjectMembers(ctx, repository.pool, projectID)
	return items, canManage, err
}

func (repository *Postgres) SearchProjectMemberCandidates(ctx context.Context, query project.MemberCandidateQuery) ([]project.MemberCandidate, error) {
	if err := requireProjectAccess(ctx, repository.pool, query.Actor, query.ProjectID, false); err != nil {
		return nil, err
	}
	canManage, err := canManageProjectMembers(ctx, repository.pool, query.Actor, query.ProjectID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, project.ErrForbidden
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name,profile.middle_name,
		       role.code,role.name
		FROM users
		JOIN profiles profile ON profile.user_id=users.id
		JOIN professional_roles role ON role.id=profile.professional_role_id
		WHERE users.status='active'
		  AND (users.login_normalized LIKE $2 || '%' OR users.email_normalized LIKE $2 || '%')
		  AND NOT EXISTS (
		      SELECT 1 FROM project_memberships membership
		      WHERE membership.project_id=$1 AND membership.user_id=users.id AND membership.revoked_at IS NULL
		  )
		ORDER BY CASE WHEN users.login_normalized=$2 THEN 0 ELSE 1 END,users.login_normalized,users.id
		LIMIT $3
	`, query.ProjectID, query.Query, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("search project member candidates: %w", err)
	}
	defer rows.Close()
	items := make([]project.MemberCandidate, 0, query.Limit)
	for rows.Next() {
		var item project.MemberCandidate
		if err = rows.Scan(&item.UserID, &item.Login, &item.Email, &item.FirstName, &item.LastName, &item.MiddleName, &item.ProfessionalRoleCode, &item.ProfessionalRoleName); err != nil {
			return nil, fmt.Errorf("scan project member candidate: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project member candidates: %w", err)
	}
	return items, nil
}

func (repository *Postgres) AddProjectMember(ctx context.Context, command project.MemberCommand) (project.Member, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.Member{}, fmt.Errorf("begin project member addition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := projectMemberAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return project.Member{}, err
	}
	if !canManage {
		return project.Member{}, project.ErrForbidden
	}
	if err = requireActiveProjectAccounts(ctx, tx, []string{command.UserID}); err != nil {
		return project.Member{}, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO project_memberships (project_id,user_id,project_role,joined_at)
		SELECT $1,$2,'executor',$3
		WHERE NOT EXISTS (
			SELECT 1 FROM project_memberships
			WHERE project_id=$1 AND user_id=$2 AND revoked_at IS NULL
		)
	`, command.ProjectID, command.UserID, command.Now)
	if err != nil {
		return project.Member{}, mapProjectWriteError("insert project member", err)
	}
	if tag.RowsAffected() != 1 {
		return project.Member{}, project.ErrConflict
	}
	if _, err = tx.Exec(ctx, `
		WITH default_chat AS (
			SELECT id FROM chats WHERE project_id=$1 AND kind='project' AND archived_at IS NULL ORDER BY created_at,id LIMIT 1
		)
		INSERT INTO chat_memberships(chat_id,project_id,user_id,added_by_user_id,joined_at)
		SELECT id,$1,$2,$3,$4 FROM default_chat
		ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING
	`, command.ProjectID, command.UserID, command.Actor.UserID, command.Now); err != nil {
		return project.Member{}, mapProjectWriteError("add member to default project chat", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,subject_user_id,project_id,metadata,occurred_at) VALUES ('project.member_added','success',$1,$2,'project_member',$3,$4,jsonb_build_object('projectRole','executor'),$5)`, command.RequestID, command.Actor.UserID, command.UserID, command.ProjectID, command.Now); err != nil {
		return project.Member{}, fmt.Errorf("audit project member addition: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.Actor.UserID, command.RequestID, command.UserID, command.UserID, notificationScopeMemberAdd, command.Notification, command.Now); err != nil {
		return project.Member{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.Member{}, mapProjectWriteError("commit project member addition", err)
	}
	items, err := listProjectMembers(ctx, repository.pool, command.ProjectID)
	if err != nil {
		return project.Member{}, err
	}
	for _, item := range items {
		if item.UserID == command.UserID {
			return item, nil
		}
	}
	return project.Member{}, project.ErrNotFound
}

func (repository *Postgres) RemoveProjectMember(ctx context.Context, command project.MemberCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin project member removal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := projectMemberAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return err
	}
	if !canManage {
		return project.ErrForbidden
	}
	var hasCustomer bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_memberships WHERE project_id=$1 AND user_id=$2 AND project_role='customer' AND revoked_at IS NULL)`, command.ProjectID, command.UserID).Scan(&hasCustomer); err != nil {
		return fmt.Errorf("check protected customer membership: %w", err)
	}
	if hasCustomer {
		return project.ErrConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE project_memberships SET revoked_at=$3,version=version+1 WHERE project_id=$1 AND user_id=$2 AND project_role IN ('executor','project_admin') AND revoked_at IS NULL`, command.ProjectID, command.UserID, command.Now)
	if err != nil {
		return mapProjectWriteError("revoke project member", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE chat_memberships SET removed_at=$3,removed_by_user_id=$4,version=version+1 WHERE project_id=$1 AND user_id=$2 AND removed_at IS NULL`, command.ProjectID, command.UserID, command.Now, command.Actor.UserID); err != nil {
		return mapProjectWriteError("revoke removed member chat access", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,subject_user_id,project_id,occurred_at) VALUES ('project.member_removed','success',$1,$2,'project_member',$3,$4,$5)`, command.RequestID, command.Actor.UserID, command.UserID, command.ProjectID, command.Now); err != nil {
		return fmt.Errorf("audit project member removal: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.Actor.UserID, command.RequestID, command.UserID, "", notificationScopeAdmins, command.Notification, command.Now); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return mapProjectWriteError("commit project member removal", err)
	}
	return nil
}

func canManageProjectMembers(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID string) (bool, error) {
	var canManage bool
	if err := querier.QueryRow(ctx, `
		SELECT $3 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=$1 AND membership.user_id=$2
			  AND membership.project_role='project_admin' AND membership.revoked_at IS NULL
		)
	`, projectID, actor.UserID, actor.SuperAdmin).Scan(&canManage); err != nil {
		return false, fmt.Errorf("check project member management access: %w", err)
	}
	return canManage, nil
}

func projectMemberAccess(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID string, lock bool) (bool, error) {
	query := `
		SELECT $3 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=project.id AND membership.user_id=$2
			  AND membership.project_role='project_admin' AND membership.revoked_at IS NULL
		),
		$3 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=project.id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		)
		FROM projects project
		WHERE project.id=$1 AND project.archived_at IS NULL`
	if lock {
		query += " FOR UPDATE"
	}
	var canManage, canView bool
	if err := querier.QueryRow(ctx, query, projectID, actor.UserID, actor.SuperAdmin).Scan(&canManage, &canView); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, project.ErrNotFound
		}
		return false, fmt.Errorf("authorize project membership access: %w", err)
	}
	if !canView {
		return false, project.ErrNotFound
	}
	return canManage, nil
}

type projectMemberQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listProjectMembers(ctx context.Context, querier projectMemberQuerier, projectID string) ([]project.Member, error) {
	rows, err := querier.Query(ctx, `
		SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name,profile.middle_name,
		       role.code,role.name,array_agg(membership.project_role ORDER BY membership.project_role),
		       min(membership.joined_at),NOT bool_or(membership.project_role='customer')
		FROM project_memberships membership
		JOIN users ON users.id=membership.user_id
		JOIN profiles profile ON profile.user_id=users.id
		JOIN professional_roles role ON role.id=profile.professional_role_id
		WHERE membership.project_id=$1 AND membership.revoked_at IS NULL
		GROUP BY users.id,profile.first_name,profile.last_name,profile.middle_name,role.code,role.name
		ORDER BY min(membership.joined_at),users.login
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()
	items := make([]project.Member, 0)
	for rows.Next() {
		var item project.Member
		if err = rows.Scan(&item.UserID, &item.Login, &item.Email, &item.FirstName, &item.LastName, &item.MiddleName, &item.ProfessionalRoleCode, &item.ProfessionalRoleName, &item.ProjectRoles, &item.JoinedAt, &item.Removable); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project members: %w", err)
	}
	return items, nil
}
