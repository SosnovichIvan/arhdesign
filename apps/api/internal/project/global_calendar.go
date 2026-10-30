package project

import (
	"context"
	"time"
)

const (
	MaxGlobalCalendarProjects = 100
	MaxGlobalCalendarEvents   = 1000
)

type GlobalCalendarProject struct {
	ID, Name, Status                string
	PlannedStartOn, PlannedFinishOn *time.Time
	Items                           []UpcomingItem
}

type GlobalCalendarFeed struct {
	RangeStart, RangeEnd time.Time
	Projects             []GlobalCalendarProject
	HasMoreProjects      bool
	TruncatedEvents      bool
}

type GlobalCalendarQuery struct {
	Actor
	RangeStart, RangeEnd time.Time
	ProjectIDs           []string
}

func (service *Service) ListGlobalCalendar(ctx context.Context, actor Actor, start, end time.Time, projectIDs []string) (GlobalCalendarFeed, error) {
	if !validUUID(actor.UserID) || start.IsZero() || end.IsZero() || !end.After(start) || end.Sub(start) > 370*24*time.Hour || len(projectIDs) > 50 {
		return GlobalCalendarFeed{}, ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(projectIDs))
	for _, projectID := range projectIDs {
		if !validUUID(projectID) {
			return GlobalCalendarFeed{}, ErrInvalidInput
		}
		if _, duplicate := seen[projectID]; duplicate {
			return GlobalCalendarFeed{}, ErrInvalidInput
		}
		seen[projectID] = struct{}{}
	}
	start, end = start.UTC(), end.UTC()
	projects, hasMore, truncated, err := service.store.ListGlobalCalendar(ctx, GlobalCalendarQuery{Actor: actor, RangeStart: start, RangeEnd: end, ProjectIDs: projectIDs})
	if err != nil {
		return GlobalCalendarFeed{}, err
	}
	return GlobalCalendarFeed{RangeStart: start, RangeEnd: end, Projects: projects, HasMoreProjects: hasMore, TruncatedEvents: truncated}, nil
}
