package project

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Member struct {
	UserID, Login, Email, FirstName, ProfessionalRoleCode, ProfessionalRoleName string
	LastName, MiddleName                                                        *string
	ProjectRoles                                                                []string
	JoinedAt                                                                    time.Time
	Removable                                                                   bool
}

type MemberCandidate struct {
	UserID, Login, Email, FirstName, ProfessionalRoleCode, ProfessionalRoleName string
	LastName, MiddleName                                                        *string
}

type MemberList struct {
	Items     []Member
	CanManage bool
}

type MemberCandidateQuery struct {
	Actor
	ProjectID string
	Query     string
	Limit     int
}

type MemberCommand struct {
	Actor
	ProjectID, UserID, RequestID string
	Now                          time.Time
	Notification                 *EventNotification
}

func (service *Service) ListMembers(ctx context.Context, actor Actor, projectID string) (MemberList, error) {
	if !validUUID(actor.UserID) {
		return MemberList{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return MemberList{}, ErrNotFound
	}
	items, canManage, err := service.store.ListProjectMembers(ctx, actor, projectID)
	return MemberList{Items: items, CanManage: canManage}, err
}

func (service *Service) SearchMemberCandidates(ctx context.Context, actor Actor, projectID, query string) ([]MemberCandidate, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if !validUUID(actor.UserID) || len([]rune(query)) < 2 || len([]rune(query)) > 100 {
		return nil, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return nil, ErrNotFound
	}
	return service.store.SearchProjectMemberCandidates(ctx, MemberCandidateQuery{Actor: actor, ProjectID: projectID, Query: query, Limit: 10})
}

func (service *Service) AddMember(ctx context.Context, actor Actor, projectID, userID, requestID string) (Member, error) {
	if !validUUID(actor.UserID) || !validUUID(userID) || len(requestID) < 8 {
		return Member{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return Member{}, ErrNotFound
	}
	notification, err := service.eventNotification("project.member.added", "В проект добавлен новый участник")
	if err != nil {
		return Member{}, err
	}
	return service.store.AddProjectMember(ctx, MemberCommand{Actor: actor, ProjectID: projectID, UserID: userID, RequestID: requestID, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) RemoveMember(ctx context.Context, actor Actor, projectID, userID, requestID string) error {
	if !validUUID(actor.UserID) || !validUUID(userID) || len(requestID) < 8 {
		return ErrInvalidInput
	}
	if !validUUID(projectID) {
		return ErrNotFound
	}
	notification, err := service.eventNotification("project.member.removed", "Участник удалён из проекта")
	if err != nil {
		return err
	}
	return service.store.RemoveProjectMember(ctx, MemberCommand{Actor: actor, ProjectID: projectID, UserID: userID, RequestID: requestID, Now: service.now().UTC(), Notification: notification})
}

func validUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil
}
