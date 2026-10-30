package repository

import (
	"context"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
)

func (repository *Postgres) ListGlobalCalendar(ctx context.Context, query project.GlobalCalendarQuery) ([]project.GlobalCalendarProject, bool, bool, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT project.id::text,project.name,project.status,project.planned_start_on,project.planned_finish_on
		FROM projects project
		WHERE project.archived_at IS NULL
		  AND ($2 OR EXISTS (
			SELECT 1 FROM project_memberships membership
			WHERE membership.project_id=project.id AND membership.user_id=$1 AND membership.revoked_at IS NULL
		  ))
		  AND (COALESCE(cardinality($3::uuid[]), 0) = 0 OR project.id=ANY($3::uuid[]))
		ORDER BY CASE project.status WHEN 'active' THEN 0 WHEN 'draft' THEN 1 WHEN 'paused' THEN 2 ELSE 3 END,
		         project.planned_start_on NULLS LAST,lower(project.name),project.id
		LIMIT $4
	`, query.UserID, query.SuperAdmin, query.ProjectIDs, project.MaxGlobalCalendarProjects+1)
	if err != nil {
		return nil, false, false, fmt.Errorf("list global calendar projects: %w", err)
	}
	defer rows.Close()
	projects := make([]project.GlobalCalendarProject, 0)
	for rows.Next() {
		var lane project.GlobalCalendarProject
		if err = rows.Scan(&lane.ID, &lane.Name, &lane.Status, &lane.PlannedStartOn, &lane.PlannedFinishOn); err != nil {
			return nil, false, false, fmt.Errorf("scan global calendar project: %w", err)
		}
		lane.Items = make([]project.UpcomingItem, 0)
		projects = append(projects, lane)
	}
	if err = rows.Err(); err != nil {
		return nil, false, false, fmt.Errorf("iterate global calendar projects: %w", err)
	}
	hasMore := len(projects) > project.MaxGlobalCalendarProjects
	if hasMore {
		projects = projects[:project.MaxGlobalCalendarProjects]
	}
	if len(projects) == 0 {
		return projects, hasMore, false, nil
	}

	projectIDs := make([]string, len(projects))
	projectIndexes := make(map[string]int, len(projects))
	for index := range projects {
		projectIDs[index] = projects[index].ID
		projectIndexes[projects[index].ID] = index
	}
	eventRows, err := repository.pool.Query(ctx, `
		SELECT id::text,project_id::text,kind,title,description,status,location,effective_at,ends_at,created_at,version
		FROM (
			SELECT task.id,task.project_id,'task'::text AS kind,task.title,task.description,task.status,NULL::text AS location,
			       task.due_at AS effective_at,NULL::timestamptz AS ends_at,task.created_at,task.version
			FROM project_tasks task
			WHERE task.project_id=ANY($1::uuid[]) AND task.due_at >= $2 AND task.due_at < $3
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
			SELECT meeting.id,meeting.project_id,'meeting'::text,meeting.title,meeting.description,NULL::text,meeting.location,
			       meeting.starts_at,meeting.ends_at,meeting.created_at,meeting.version
			FROM project_meetings meeting
			WHERE meeting.project_id=ANY($1::uuid[]) AND meeting.starts_at >= $2 AND meeting.starts_at < $3
		) events
		ORDER BY effective_at,kind,id
		LIMIT $6
	`, projectIDs, query.RangeStart, query.RangeEnd, query.UserID, query.SuperAdmin, project.MaxGlobalCalendarEvents+1)
	if err != nil {
		return nil, false, false, fmt.Errorf("list global calendar events: %w", err)
	}
	defer eventRows.Close()
	eventCount := 0
	truncated := false
	for eventRows.Next() {
		item, scanErr := scanUpcoming(eventRows)
		if scanErr != nil {
			return nil, false, false, fmt.Errorf("scan global calendar event: %w", scanErr)
		}
		if eventCount == project.MaxGlobalCalendarEvents {
			truncated = true
			continue
		}
		eventCount++
		if index, exists := projectIndexes[item.ProjectID]; exists {
			projects[index].Items = append(projects[index].Items, item)
		}
	}
	if err = eventRows.Err(); err != nil {
		return nil, false, false, fmt.Errorf("iterate global calendar events: %w", err)
	}
	return projects, hasMore, truncated, nil
}
