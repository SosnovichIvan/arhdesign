package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectTasks(ctx context.Context, query project.TaskQuery) ([]project.Task, bool, error) {
	canViewAll, canCreate, err := taskCollectionAccess(ctx, repository.pool, query.Actor, query.ProjectID)
	if err != nil {
		return nil, false, err
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT task.id::text,task.project_id::text,task.title,task.description,task.status,
		       task.due_at,task.started_at,task.completed_at,task.created_at,task.version,
		       (SELECT chat.id::text FROM chats chat WHERE chat.project_id=task.project_id AND chat.kind='context' AND chat.context_type='task' AND chat.context_id=task.id AND chat.archived_at IS NULL),
		       $3 OR EXISTS (
		         SELECT 1 FROM project_memberships membership
		         WHERE membership.project_id=task.project_id AND membership.user_id=$2
		           AND membership.project_role='project_admin' AND membership.revoked_at IS NULL
		       ) AS can_edit,
		       $3 OR EXISTS (
		         SELECT 1 FROM project_memberships membership
		         WHERE membership.project_id=task.project_id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		           AND (membership.project_role='project_admin' OR membership.privileges @> '{"tasks.status":true}'::jsonb)
		       ) OR EXISTS (
		         SELECT 1 FROM project_memberships membership
		         WHERE membership.project_id=task.project_id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		           AND membership.project_role='customer' AND task.status='review'
		       ) OR EXISTS (
		         SELECT 1 FROM project_task_assignees own_assignment
		         WHERE own_assignment.task_id=task.id AND own_assignment.user_id=$2
		           AND task.status IN ('new','in_progress','changes_requested')
		       ) AS can_change_status,
		       assignee.user_id::text,users.login,profile.first_name,profile.last_name
		FROM project_tasks task
		LEFT JOIN project_task_assignees assignee ON assignee.task_id=task.id
		LEFT JOIN users ON users.id=assignee.user_id
		LEFT JOIN profiles profile ON profile.user_id=assignee.user_id
		WHERE task.project_id=$1
		  AND ($4 OR EXISTS (
		    SELECT 1 FROM project_task_assignees own_assignment
		    WHERE own_assignment.task_id=task.id AND own_assignment.user_id=$2
		  ))
		ORDER BY task.due_at,task.id,users.login
		LIMIT 2500
	`, query.ProjectID, query.UserID, query.SuperAdmin, canViewAll)
	if err != nil {
		return nil, false, fmt.Errorf("list project tasks: %w", err)
	}
	defer rows.Close()
	items := make([]project.Task, 0)
	positions := make(map[string]int)
	for rows.Next() {
		var item project.Task
		var assigneeID, login, firstName, lastName *string
		if err = rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.DueAt, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.Version, &item.ContextChatID, &item.CanEdit, &item.CanChangeStatus, &assigneeID, &login, &firstName, &lastName); err != nil {
			return nil, false, fmt.Errorf("scan project task: %w", err)
		}
		position, found := positions[item.ID]
		if !found {
			item.Assignees = make([]project.TaskAssignee, 0)
			positions[item.ID] = len(items)
			items = append(items, item)
			position = len(items) - 1
		}
		if assigneeID != nil && login != nil && firstName != nil {
			items[position].Assignees = append(items[position].Assignees, project.TaskAssignee{UserID: *assigneeID, Login: *login, FirstName: *firstName, LastName: lastName})
		}
	}
	if err = rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate project tasks: %w", err)
	}
	return items, canCreate, nil
}

func (repository *Postgres) UpdateProjectTask(ctx context.Context, command project.UpdateTaskCommand) (project.Task, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.Task{}, fmt.Errorf("begin project task update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, customer, assigned, err := taskMutationAccess(ctx, tx, command.Actor, command.ProjectID, command.TaskID)
	if err != nil {
		return project.Task{}, err
	}
	if !canManage && !(assigned && (command.Status == project.TaskInProgress || command.Status == project.TaskReview)) && !(customer && (command.Status == project.TaskAccepted || command.Status == project.TaskChangesRequested)) {
		return project.Task{}, project.ErrForbidden
	}
	var item project.Task
	err = tx.QueryRow(ctx, `
		UPDATE project_tasks
		SET status=$4,
		    started_at=CASE WHEN $4='in_progress' THEN COALESCE(started_at,$5) ELSE started_at END,
		    completed_at=CASE WHEN $4 IN ('review','accepted') THEN COALESCE(completed_at,$5) WHEN $4 IN ('in_progress','changes_requested') THEN NULL ELSE completed_at END,
		    updated_at=$5,version=version+1
		WHERE project_id=$1 AND id=$2 AND version=$3
		RETURNING id::text,project_id::text,title,description,status,due_at,started_at,completed_at,created_at,version
	`, command.ProjectID, command.TaskID, command.ExpectedVersion, command.Status, command.Now).Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.DueAt, &item.StartedAt, &item.CompletedAt, &item.CreatedAt, &item.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.Task{}, project.ErrConflict
	}
	if err != nil {
		return project.Task{}, mapProjectWriteError("update project task", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('task.status_changed','success',$1,$2,'project_task',$3,jsonb_build_object('taskId',$4::text,'status',$5::text,'version',$6::bigint),$7)`, command.RequestID, command.UserID, command.ProjectID, command.TaskID, command.Status, item.Version, command.Now); err != nil {
		return project.Task{}, fmt.Errorf("audit project task status update: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.UserID, fmt.Sprintf("%s:%d", item.ID, item.Version), item.ID, "", notificationScopeTask, command.Notification, command.Now); err != nil {
		return project.Task{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.Task{}, mapProjectWriteError("commit project task update", err)
	}
	listed, _, err := repository.ListProjectTasks(ctx, project.TaskQuery{Actor: command.Actor, ProjectID: command.ProjectID})
	if err != nil {
		return project.Task{}, err
	}
	for _, listedItem := range listed {
		if listedItem.ID == item.ID {
			return listedItem, nil
		}
	}
	return project.Task{}, project.ErrNotFound
}

func taskCollectionAccess(ctx context.Context, query projectAccessQuerier, actor project.Actor, projectID string) (bool, bool, error) {
	var canView, canViewAll, canCreate bool
	err := query.QueryRow(ctx, `
		SELECT $3 OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$2 AND m.revoked_at IS NULL),
		       $3 OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$2 AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges @> '{"tasks.view_all":true}'::jsonb)),
		       $3 OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$2 AND m.revoked_at IS NULL AND (m.project_role='project_admin' OR m.privileges @> '{"tasks.create":true}'::jsonb))
		FROM projects p WHERE p.id=$1 AND p.archived_at IS NULL
	`, projectID, actor.UserID, actor.SuperAdmin).Scan(&canView, &canViewAll, &canCreate)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !canView) {
		return false, false, project.ErrNotFound
	}
	if err != nil {
		return false, false, fmt.Errorf("authorize project task collection: %w", err)
	}
	return canViewAll, canCreate, nil
}

func taskMutationAccess(ctx context.Context, query projectAccessQuerier, actor project.Actor, projectID, taskID string) (bool, bool, bool, error) {
	var canView, canManage, customer, assigned bool
	err := query.QueryRow(ctx, `
		SELECT $4 OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$3 AND m.revoked_at IS NULL),
		       $4 OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$3 AND m.revoked_at IS NULL AND (m.project_role='project_admin' OR m.privileges @> '{"tasks.manage":true}'::jsonb)),
		       EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=p.id AND m.user_id=$3 AND m.project_role='customer' AND m.revoked_at IS NULL),
		       EXISTS (SELECT 1 FROM project_task_assignees a WHERE a.task_id=t.id AND a.user_id=$3)
		FROM projects p JOIN project_tasks t ON t.project_id=p.id
		WHERE p.id=$1 AND t.id=$2 AND p.archived_at IS NULL
	`, projectID, taskID, actor.UserID, actor.SuperAdmin).Scan(&canView, &canManage, &customer, &assigned)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !canView) {
		return false, false, false, project.ErrNotFound
	}
	if err != nil {
		return false, false, false, fmt.Errorf("authorize project task mutation: %w", err)
	}
	if !canManage && !customer && !assigned {
		return false, false, false, project.ErrNotFound
	}
	return canManage, customer, assigned, nil
}
