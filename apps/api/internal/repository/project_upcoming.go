package repository

import (
	"context"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectUpcoming(ctx context.Context, query project.UpcomingQuery) ([]project.UpcomingItem, error) {
	if err := requireProjectAccess(ctx, repository.pool, query.Actor, query.ProjectID, false); err != nil {
		return nil, err
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text,project_id::text,kind,title,description,status,location,effective_at,ends_at,created_at,version
		FROM (
			SELECT id,project_id,'task'::text AS kind,title,description,status,NULL::text AS location,
			       due_at AS effective_at,NULL::timestamptz AS ends_at,created_at,version
			FROM project_tasks task
			WHERE task.project_id=$1 AND task.due_at >= $2 AND task.due_at < $3
			  AND ($5 OR EXISTS (
				SELECT 1 FROM project_memberships membership
				WHERE membership.project_id=task.project_id AND membership.user_id=$4
				  AND membership.revoked_at IS NULL
				  AND (membership.project_role IN ('customer','project_admin') OR membership.privileges @> '{"tasks.view_all": true}'::jsonb)
			  ) OR EXISTS (
				SELECT 1 FROM project_task_assignees assignee
				WHERE assignee.task_id=task.id AND assignee.user_id=$4
			  ))
			UNION ALL
			SELECT id,project_id,'meeting'::text,title,description,NULL::text,location,
			       starts_at,ends_at,created_at,version
			FROM project_meetings
			WHERE project_id=$1 AND starts_at >= $2 AND starts_at < $3
		) upcoming
		ORDER BY effective_at,kind,id
		LIMIT 500
	`, query.ProjectID, query.RangeStart, query.End, query.UserID, query.SuperAdmin)
	if err != nil {
		return nil, fmt.Errorf("list project upcoming: %w", err)
	}
	defer rows.Close()
	items := make([]project.UpcomingItem, 0)
	for rows.Next() {
		item, scanErr := scanUpcoming(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan project upcoming: %w", scanErr)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project upcoming: %w", err)
	}
	return items, nil
}

func (repository *Postgres) CreateProjectTask(ctx context.Context, command project.CreateTaskCommand) (project.UpcomingItem, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.UpcomingItem{}, fmt.Errorf("begin project task creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, canCreate, accessErr := taskCollectionAccess(ctx, tx, command.Actor, command.ProjectID)
	if accessErr != nil {
		return project.UpcomingItem{}, accessErr
	}
	if !canCreate {
		return project.UpcomingItem{}, project.ErrForbidden
	}
	if len(command.AssigneeUserIDs) > 0 {
		var activeAssignees int
		if err = tx.QueryRow(ctx, `SELECT count(DISTINCT user_id) FROM project_memberships WHERE project_id=$1 AND user_id=ANY($2::uuid[]) AND revoked_at IS NULL`, command.ProjectID, command.AssigneeUserIDs).Scan(&activeAssignees); err != nil {
			return project.UpcomingItem{}, fmt.Errorf("check project task assignees: %w", err)
		}
		if activeAssignees != len(command.AssigneeUserIDs) {
			return project.UpcomingItem{}, project.ErrAccountUnavailable
		}
	}
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, false); err != nil {
		return project.UpcomingItem{}, err
	}
	var item project.UpcomingItem
	item.Kind = project.UpcomingTask
	err = tx.QueryRow(ctx, `
		INSERT INTO project_tasks (project_id,created_by_user_id,title,description,due_at,created_at,updated_at)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$6)
		RETURNING id::text,project_id::text,title,description,status,due_at,created_at,version
	`, command.ProjectID, command.UserID, command.Title, command.Description, command.DueAt, command.Now).Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.EffectiveAt, &item.CreatedAt, &item.Version)
	if err != nil {
		return project.UpcomingItem{}, mapProjectWriteError("insert project task", err)
	}
	for _, assigneeID := range command.AssigneeUserIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO project_task_assignees (task_id,user_id,assigned_by_user_id,assigned_at) VALUES ($1,$2,$3,$4)`, item.ID, assigneeID, command.UserID, command.Now); err != nil {
			return project.UpcomingItem{}, mapProjectWriteError("insert project task assignee", err)
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('task.created','success',$1,$2,'project_task',$3,jsonb_build_object('taskId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, item.ID, command.Now); err != nil {
		return project.UpcomingItem{}, fmt.Errorf("audit project task creation: %w", err)
	}
	if err = enqueueTaskAssigneeNotification(ctx, tx, command.ProjectID, item.ID, command.UserID, command.Notification, command.Now); err != nil {
		return project.UpcomingItem{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.UpcomingItem{}, mapProjectWriteError("commit project task creation", err)
	}
	return item, nil
}

func (repository *Postgres) CreateProjectMeeting(ctx context.Context, command project.CreateMeetingCommand) (project.UpcomingItem, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.UpcomingItem{}, fmt.Errorf("begin project meeting creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, true); err != nil {
		return project.UpcomingItem{}, err
	}
	var item project.UpcomingItem
	item.Kind = project.UpcomingMeeting
	err = tx.QueryRow(ctx, `
		INSERT INTO project_meetings (project_id,created_by_user_id,title,description,location,starts_at,ends_at,created_at,updated_at)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,$8,$8)
		RETURNING id::text,project_id::text,title,description,location,starts_at,ends_at,created_at,version
	`, command.ProjectID, command.UserID, command.Title, command.Description, command.Location, command.StartsAt, command.EndsAt, command.Now).Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Location, &item.EffectiveAt, &item.EndsAt, &item.CreatedAt, &item.Version)
	if err != nil {
		return project.UpcomingItem{}, mapProjectWriteError("insert project meeting", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('meeting.created','success',$1,$2,'project_meeting',$3,jsonb_build_object('meetingId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, item.ID, command.Now); err != nil {
		return project.UpcomingItem{}, fmt.Errorf("audit project meeting creation: %w", err)
	}
	if err = enqueueProjectMemberNotification(ctx, tx, command.ProjectID, command.UserID, item.ID, command.Notification, command.Now); err != nil {
		return project.UpcomingItem{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.UpcomingItem{}, mapProjectWriteError("commit project meeting creation", err)
	}
	return item, nil
}

type projectAccessQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func requireProjectAccess(ctx context.Context, query projectAccessQuerier, actor project.Actor, projectID string, manage bool) error {
	var allowed bool
	err := query.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM projects project
			WHERE project.id=$1 AND project.archived_at IS NULL
			  AND ($3 OR EXISTS (
				SELECT 1 FROM project_memberships membership
				WHERE membership.project_id=project.id AND membership.user_id=$2
				  AND membership.revoked_at IS NULL
				  AND (NOT $4 OR membership.project_role IN ('customer','project_admin'))
			  ))
		)
	`, projectID, actor.UserID, actor.SuperAdmin, manage).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("check project access: %w", err)
	}
	if !allowed {
		return project.ErrNotFound
	}
	return nil
}

type upcomingScanner interface {
	Scan(...any) error
}

func scanUpcoming(row upcomingScanner) (project.UpcomingItem, error) {
	var item project.UpcomingItem
	err := row.Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Title, &item.Description, &item.Status, &item.Location, &item.EffectiveAt, &item.EndsAt, &item.CreatedAt, &item.Version)
	return item, err
}
