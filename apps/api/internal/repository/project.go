package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *Postgres) CreateProject(ctx context.Context, command project.CreateCommand) (project.View, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.View{}, fmt.Errorf("begin project creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	accountIDs := append([]string(nil), command.AdminUserIDs...)
	if command.CustomerUserID != nil {
		accountIDs = append(accountIDs, *command.CustomerUserID)
	}
	if err = requireActiveProjectAccounts(ctx, tx, uniqueStrings(accountIDs)); err != nil {
		return project.View{}, err
	}

	var projectID string
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (created_by_user_id, customer_user_id, name, type, address, status,
			planned_start_on, planned_finish_on, currency_code, description, auto_approve_expenses, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),'draft',$6,$7,$8,NULLIF($9,''),$10,$11,$11)
		RETURNING id::text
	`, command.UserID, command.CustomerUserID, command.Name, command.Type, command.Address, command.PlannedStartOn, command.PlannedFinishOn, command.CurrencyCode, command.Description, command.AutoApproveExpenses, command.Now).Scan(&projectID)
	if err != nil {
		return project.View{}, mapProjectWriteError("insert project", err)
	}
	var chatID string
	if err = tx.QueryRow(ctx, `INSERT INTO chats (project_id,kind,name,created_by_user_id,created_at) VALUES ($1,'project','Общий чат проекта',$2,$3) RETURNING id::text`, projectID, command.UserID, command.Now).Scan(&chatID); err != nil {
		return project.View{}, mapProjectWriteError("insert project chat", err)
	}
	if command.CustomerUserID != nil {
		if _, err = tx.Exec(ctx, `INSERT INTO project_memberships (project_id,user_id,project_role,joined_at) VALUES ($1,$2,'customer',$3)`, projectID, *command.CustomerUserID, command.Now); err != nil {
			return project.View{}, mapProjectWriteError("insert customer membership", err)
		}
	}
	for _, adminID := range command.AdminUserIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO project_memberships (project_id,user_id,project_role,joined_at) VALUES ($1,$2,'project_admin',$3) ON CONFLICT DO NOTHING`, projectID, adminID, command.Now); err != nil {
			return project.View{}, mapProjectWriteError("insert project administrator", err)
		}
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO chat_memberships (chat_id,project_id,user_id,added_by_user_id,joined_at)
		SELECT $1::uuid,$2::uuid,$3::uuid,$3::uuid,$4::timestamptz
		UNION
		SELECT $1::uuid,$2::uuid,membership.user_id,$3::uuid,$4::timestamptz FROM project_memberships membership
		WHERE membership.project_id=$2::uuid AND membership.revoked_at IS NULL
		ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING
	`, chatID, projectID, command.UserID, command.Now); err != nil {
		return project.View{}, mapProjectWriteError("insert initial chat memberships", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,occurred_at) VALUES ('project.created','success',$1,$2,'project',$3,$4)`, command.RequestID, command.UserID, projectID, command.Now); err != nil {
		return project.View{}, fmt.Errorf("audit project creation: %w", err)
	}
	if command.Notification != nil {
		if _, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (
				recipient_user_id,channel,message_type,idempotency_key,
				payload_ciphertext,payload_key_version,metadata,created_at,updated_at
			)
			SELECT users.id,'telegram',$2,'project.created:' || $1 || ':' || users.id,
			       $3,$4,jsonb_build_object('projectId',$1),$5,$5
			FROM users
			JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
			JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
			WHERE users.global_role='super_admin' AND users.status='active'
			ON CONFLICT (idempotency_key) DO NOTHING
		`, projectID, command.Notification.MessageType, command.Notification.PayloadCiphertext, command.Notification.PayloadKeyVersion, command.Now); err != nil {
			return project.View{}, fmt.Errorf("enqueue project creation notification: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return project.View{}, mapProjectWriteError("commit project creation", err)
	}
	return repository.GetProject(ctx, command.Actor, projectID)
}

func (repository *Postgres) ListProjects(ctx context.Context, query project.ListQuery) ([]project.View, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT project.id::text, project.name, project.type, project.address, project.status,
			project.customer_user_id::text, project.planned_start_on, project.planned_finish_on,
			project.currency_code, project.description, project.auto_approve_expenses,
			project.created_by_user_id::text, project.created_at, project.version,
			COALESCE(array_agg(member.user_id::text ORDER BY member.joined_at) FILTER (WHERE member.project_role='project_admin' AND member.revoked_at IS NULL), ARRAY[]::text[])
		FROM projects project
		LEFT JOIN project_memberships member ON member.project_id=project.id
		WHERE project.archived_at IS NULL
		  AND ($2 OR EXISTS (SELECT 1 FROM project_memberships visible WHERE visible.project_id=project.id AND visible.user_id=$1 AND visible.revoked_at IS NULL))
		  AND ($3='' OR project.status=$3)
		  AND ($4::timestamptz IS NULL OR (project.created_at,project.id) < ($4,$5::uuid))
		GROUP BY project.id
		ORDER BY project.created_at DESC, project.id DESC
		LIMIT $6
	`, query.UserID, query.SuperAdmin, query.Status, query.BeforeCreatedAt, query.BeforeID, query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	items := make([]project.View, 0, query.PageSize)
	for rows.Next() {
		view, scanErr := scanProject(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan project list: %w", scanErr)
		}
		items = append(items, view)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return items, nil
}

func (repository *Postgres) GetProject(ctx context.Context, actor project.Actor, projectID string) (project.View, error) {
	row := repository.pool.QueryRow(ctx, `
		SELECT project.id::text, project.name, project.type, project.address, project.status,
			project.customer_user_id::text, project.planned_start_on, project.planned_finish_on,
			project.currency_code, project.description, project.auto_approve_expenses,
			project.created_by_user_id::text, project.created_at, project.version,
			COALESCE(array_agg(member.user_id::text ORDER BY member.joined_at) FILTER (WHERE member.project_role='project_admin' AND member.revoked_at IS NULL), ARRAY[]::text[])
		FROM projects project
		LEFT JOIN project_memberships member ON member.project_id=project.id
		WHERE project.id=$1 AND project.archived_at IS NULL
		  AND ($3 OR EXISTS (SELECT 1 FROM project_memberships visible WHERE visible.project_id=project.id AND visible.user_id=$2 AND visible.revoked_at IS NULL))
		GROUP BY project.id
	`, projectID, actor.UserID, actor.SuperAdmin)
	view, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.View{}, project.ErrNotFound
	}
	if err != nil {
		return project.View{}, fmt.Errorf("get project: %w", err)
	}
	return view, nil
}

func (repository *Postgres) UpdateProject(ctx context.Context, command project.UpdateCommand) (project.View, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.View{}, fmt.Errorf("begin project update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name, projectType, status string
	var address, description *string
	var start, finish *time.Time
	var autoApprove bool
	var version int64
	err = tx.QueryRow(ctx, `
		SELECT project.name,project.type,project.address,project.status,project.planned_start_on,
		       project.planned_finish_on,project.description,project.auto_approve_expenses,project.version
		FROM projects project
		WHERE project.id=$1 AND project.archived_at IS NULL
		  AND ($3 OR EXISTS (
		      SELECT 1 FROM project_memberships membership
		      WHERE membership.project_id=project.id AND membership.user_id=$2
		        AND membership.revoked_at IS NULL AND membership.project_role IN ('customer','project_admin')
		  ))
		FOR UPDATE
	`, command.ProjectID, command.UserID, command.SuperAdmin).Scan(&name, &projectType, &address, &status, &start, &finish, &description, &autoApprove, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.View{}, project.ErrNotFound
	}
	if err != nil {
		return project.View{}, fmt.Errorf("lock project update: %w", err)
	}
	if version != command.ExpectedVersion {
		return project.View{}, project.ErrConflict
	}
	if command.Name != nil {
		name = *command.Name
	}
	if command.Type != nil {
		projectType = *command.Type
	}
	if command.Status != nil {
		status = *command.Status
	}
	if command.Address.Set {
		address = command.Address.Value
	}
	if command.Description.Set {
		description = command.Description.Value
	}
	if command.PlannedStartOn.Set {
		start = command.PlannedStartOn.Value
	}
	if command.PlannedFinishOn.Set {
		finish = command.PlannedFinishOn.Value
	}
	if command.AutoApproveExpenses != nil {
		autoApprove = *command.AutoApproveExpenses
	}
	if start != nil && finish != nil && finish.Before(*start) {
		return project.View{}, project.ErrInvalidInput
	}
	if _, err = tx.Exec(ctx, `
		UPDATE projects
		SET name=$2,type=$3,address=$4,status=$5,planned_start_on=$6,planned_finish_on=$7,
		    description=$8,auto_approve_expenses=$9,updated_at=$10,version=version+1
		WHERE id=$1
	`, command.ProjectID, name, projectType, address, status, start, finish, description, autoApprove, command.Now); err != nil {
		return project.View{}, mapProjectWriteError("update project", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,occurred_at) VALUES ('project.updated','success',$1,$2,'project',$3,$4)`, command.RequestID, command.UserID, command.ProjectID, command.Now); err != nil {
		return project.View{}, fmt.Errorf("audit project update: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return project.View{}, mapProjectWriteError("commit project update", err)
	}
	return repository.GetProject(ctx, command.Actor, command.ProjectID)
}

func (repository *Postgres) ArchiveProject(ctx context.Context, command project.ArchiveCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin project archive: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var version int64
	err = tx.QueryRow(ctx, `SELECT version FROM projects WHERE id=$1 AND archived_at IS NULL FOR UPDATE`, command.ProjectID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock project archive: %w", err)
	}
	if version != command.ExpectedVersion {
		return project.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE projects SET status='archived',archived_at=$2,updated_at=$2,version=version+1 WHERE id=$1`, command.ProjectID, command.Now); err != nil {
		return mapProjectWriteError("archive project", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,occurred_at) VALUES ('project.archived','success',$1,$2,'project',$3,$4)`, command.RequestID, command.UserID, command.ProjectID, command.Now); err != nil {
		return fmt.Errorf("audit project archive: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return mapProjectWriteError("commit project archive", err)
	}
	return nil
}

func (repository *Postgres) AssignProjectCustomer(ctx context.Context, command project.AssignCustomerCommand) (project.View, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.View{}, fmt.Errorf("begin customer assignment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var existingCustomer *string
	var version int64
	err = tx.QueryRow(ctx, `SELECT customer_user_id::text,version FROM projects WHERE id=$1 AND archived_at IS NULL FOR UPDATE`, command.ProjectID).Scan(&existingCustomer, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.View{}, project.ErrNotFound
	}
	if err != nil {
		return project.View{}, fmt.Errorf("lock project customer: %w", err)
	}
	if version != command.ExpectedVersion {
		return project.View{}, project.ErrConflict
	}
	if existingCustomer != nil {
		if *existingCustomer != command.CustomerUserID {
			return project.View{}, project.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return project.View{}, fmt.Errorf("commit repeated customer assignment: %w", err)
		}
		return repository.GetProject(ctx, command.Actor, command.ProjectID)
	}
	if err = requireActiveProjectAccounts(ctx, tx, []string{command.CustomerUserID}); err != nil {
		return project.View{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE projects SET customer_user_id=$2,updated_at=$3,version=version+1 WHERE id=$1`, command.ProjectID, command.CustomerUserID, command.Now); err != nil {
		return project.View{}, mapProjectWriteError("assign project customer", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO project_memberships (project_id,user_id,project_role,joined_at) VALUES ($1,$2,'customer',$3)`, command.ProjectID, command.CustomerUserID, command.Now); err != nil {
		return project.View{}, mapProjectWriteError("insert assigned customer membership", err)
	}
	if _, err = tx.Exec(ctx, `
		WITH default_chat AS (
			SELECT id FROM chats WHERE project_id=$1 AND kind='project' AND archived_at IS NULL ORDER BY created_at,id LIMIT 1
		)
		INSERT INTO chat_memberships(chat_id,project_id,user_id,added_by_user_id,joined_at)
		SELECT id,$1,$2,$3,$4 FROM default_chat
		ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING
	`, command.ProjectID, command.CustomerUserID, command.UserID, command.Now); err != nil {
		return project.View{}, mapProjectWriteError("add customer to default project chat", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,subject_user_id,project_id,occurred_at) VALUES ('project.customer_assigned','success',$1,$2,'user',$3,$4,$5)`, command.RequestID, command.UserID, command.CustomerUserID, command.ProjectID, command.Now); err != nil {
		return project.View{}, fmt.Errorf("audit customer assignment: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return project.View{}, mapProjectWriteError("commit customer assignment", err)
	}
	return repository.GetProject(ctx, command.Actor, command.ProjectID)
}

type projectScanner interface{ Scan(...any) error }

func scanProject(row projectScanner) (project.View, error) {
	var view project.View
	var address, description, customerID *string
	var start, finish *time.Time
	err := row.Scan(&view.ID, &view.Name, &view.Type, &address, &view.Status, &customerID, &start, &finish, &view.CurrencyCode, &description, &view.AutoApproveExpenses, &view.CreatedByUserID, &view.CreatedAt, &view.Version, &view.AdminUserIDs)
	view.Address, view.Description, view.CustomerUserID, view.PlannedStartOn, view.PlannedFinishOn = address, description, customerID, start, finish
	return view, err
}

func requireActiveProjectAccounts(ctx context.Context, tx pgx.Tx, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=ANY($1::uuid[]) AND status='active'`, ids).Scan(&active); err != nil {
		return fmt.Errorf("validate project accounts: %w", err)
	}
	if active != len(ids) {
		return project.ErrAccountUnavailable
	}
	return nil
}

func mapProjectWriteError(action string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "23514":
			return fmt.Errorf("%s: %w", action, project.ErrConflict)
		case "23503":
			return fmt.Errorf("%s: %w", action, project.ErrAccountUnavailable)
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
