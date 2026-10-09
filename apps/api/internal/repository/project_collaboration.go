package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectChats(ctx context.Context, actor project.Actor, projectID string) (project.ChatList, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return project.ChatList{}, err
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT chat.id::text,chat.project_id::text,chat.kind,chat.name,chat.context_type,chat.context_id::text,
		       chat.created_at,chat.version,
		       (SELECT count(*)::int FROM chat_memberships member WHERE member.chat_id=chat.id AND member.removed_at IS NULL),
		       (SELECT count(*)::int
		        FROM chat_messages message
		        LEFT JOIN chat_memberships own_membership ON own_membership.chat_id=chat.id AND own_membership.user_id=$2 AND own_membership.removed_at IS NULL
		        WHERE message.chat_id=chat.id AND message.author_user_id<>$2
		          AND message.created_at>COALESCE(own_membership.last_read_at,chat.created_at)),
		       ($3 OR chat.created_by_user_id=$2 OR EXISTS (
		         SELECT 1 FROM project_memberships membership WHERE membership.project_id=chat.project_id
		         AND membership.user_id=$2 AND membership.project_role='project_admin' AND membership.revoked_at IS NULL))
		FROM chats chat
		WHERE chat.project_id=$1 AND chat.archived_at IS NULL
		  AND ($3 OR EXISTS (SELECT 1 FROM chat_memberships member WHERE member.chat_id=chat.id AND member.user_id=$2 AND member.removed_at IS NULL))
		ORDER BY chat.created_at,chat.id
	`, projectID, actor.UserID, actor.SuperAdmin)
	if err != nil {
		return project.ChatList{}, fmt.Errorf("list project chats: %w", err)
	}
	defer rows.Close()
	items := []project.ChatSummary{}
	for rows.Next() {
		var value project.ChatSummary
		if err = rows.Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Name, &value.ContextType, &value.ContextID, &value.CreatedAt, &value.Version, &value.MemberCount, &value.UnreadCount, &value.CanManage); err != nil {
			return project.ChatList{}, fmt.Errorf("scan project chat: %w", err)
		}
		if value.ContextType != nil && value.ContextID != nil {
			title, titleErr := contextEntityTitle(ctx, repository.pool, actor, projectID, *value.ContextType, *value.ContextID)
			if titleErr == nil {
				value.ContextTitle = &title
			}
		}
		items = append(items, value)
	}
	if err = rows.Err(); err != nil {
		return project.ChatList{}, fmt.Errorf("iterate project chats: %w", err)
	}
	return project.ChatList{Items: items, CanCreate: true}, nil
}

func (repository *Postgres) DeleteProjectChat(ctx context.Context, command project.DeleteChatCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chat deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := chatManagementAccess(ctx, tx, command.Actor, command.ProjectID, command.ChatID)
	if err != nil {
		return err
	}
	if !canManage {
		return project.ErrForbidden
	}
	tag, err := tx.Exec(ctx, `UPDATE chats SET archived_at=$4,version=version+1 WHERE id=$1 AND project_id=$2 AND version=$3 AND archived_at IS NULL`, command.ChatID, command.ProjectID, command.Version, command.Now)
	if err != nil {
		return mapProjectWriteError("delete chat", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('chat.deleted','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, command.ChatID, command.Now); err != nil {
		return fmt.Errorf("audit chat deletion: %w", err)
	}
	if err = enqueueChatMemberNotification(ctx, tx, command.ProjectID, command.ChatID, command.UserID, command.RequestID, command.Notification, command.Now); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return mapProjectWriteError("commit chat deletion", err)
	}
	return nil
}

func (repository *Postgres) CreateProjectChat(ctx context.Context, command project.CreateChatCommand) (project.ChatSummary, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatSummary{}, fmt.Errorf("begin chat creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, false); err != nil {
		return project.ChatSummary{}, err
	}
	members := append(append([]string{}, command.MemberUserIDs...), command.UserID)
	for _, userID := range members {
		var allowed bool
		if err = tx.QueryRow(ctx, `SELECT $3 OR EXISTS (SELECT 1 FROM project_memberships WHERE project_id=$1 AND user_id=$2 AND revoked_at IS NULL)`, command.ProjectID, userID, command.SuperAdmin && userID == command.UserID).Scan(&allowed); err != nil {
			return project.ChatSummary{}, fmt.Errorf("validate chat member: %w", err)
		}
		if !allowed {
			return project.ChatSummary{}, project.ErrNotFound
		}
	}
	var value project.ChatSummary
	err = tx.QueryRow(ctx, `INSERT INTO chats(project_id,kind,name,created_by_user_id,created_at) VALUES($1,'project',$2,$3,$4) RETURNING id::text,project_id::text,kind,name,created_at,version`, command.ProjectID, command.Name, command.UserID, command.Now).Scan(&value.ID, &value.ProjectID, &value.Kind, &value.Name, &value.CreatedAt, &value.Version)
	if err != nil {
		return project.ChatSummary{}, mapProjectWriteError("create project chat", err)
	}
	for _, userID := range members {
		if _, err = tx.Exec(ctx, `INSERT INTO chat_memberships(chat_id,project_id,user_id,added_by_user_id,joined_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING`, value.ID, command.ProjectID, userID, command.UserID, command.Now); err != nil {
			return project.ChatSummary{}, mapProjectWriteError("add initial chat member", err)
		}
	}
	if err = tx.QueryRow(ctx, `SELECT count(*)::int FROM chat_memberships WHERE chat_id=$1 AND removed_at IS NULL`, value.ID).Scan(&value.MemberCount); err != nil {
		return project.ChatSummary{}, fmt.Errorf("count chat members: %w", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('chat.created','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text,'name',$5::text),$6)`, command.RequestID, command.UserID, command.ProjectID, value.ID, command.Name, command.Now); err != nil {
		return project.ChatSummary{}, fmt.Errorf("audit chat creation: %w", err)
	}
	if err = enqueueChatMemberNotification(ctx, tx, command.ProjectID, value.ID, command.UserID, value.ID, command.Notification, command.Now); err != nil {
		return project.ChatSummary{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatSummary{}, mapProjectWriteError("commit chat creation", err)
	}
	value.CanManage = true
	return value, nil
}

func (repository *Postgres) UpdateProjectChat(ctx context.Context, command project.UpdateChatCommand) (project.ChatSummary, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatSummary{}, fmt.Errorf("begin chat update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := chatManagementAccess(ctx, tx, command.Actor, command.ProjectID, command.ChatID)
	if err != nil {
		return project.ChatSummary{}, err
	}
	if !canManage {
		return project.ChatSummary{}, project.ErrForbidden
	}
	tag, err := tx.Exec(ctx, `UPDATE chats SET name=$4,version=version+1 WHERE id=$1 AND project_id=$2 AND version=$3 AND archived_at IS NULL`, command.ChatID, command.ProjectID, command.Version, command.Name)
	if err != nil {
		return project.ChatSummary{}, mapProjectWriteError("rename chat", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ChatSummary{}, project.ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('chat.renamed','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, command.ChatID, command.Now); err != nil {
		return project.ChatSummary{}, fmt.Errorf("audit chat rename: %w", err)
	}
	if err = enqueueChatMemberNotification(ctx, tx, command.ProjectID, command.ChatID, command.UserID, command.RequestID, command.Notification, command.Now); err != nil {
		return project.ChatSummary{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatSummary{}, mapProjectWriteError("commit chat rename", err)
	}
	list, err := repository.ListProjectChats(ctx, command.Actor, command.ProjectID)
	if err != nil {
		return project.ChatSummary{}, err
	}
	for _, item := range list.Items {
		if item.ID == command.ChatID {
			return item, nil
		}
	}
	return project.ChatSummary{}, project.ErrNotFound
}

func (repository *Postgres) ListProjectChatMembers(ctx context.Context, actor project.Actor, projectID, chatID string) (project.ChatMemberList, error) {
	canManage, err := chatManagementAccess(ctx, repository.pool, actor, projectID, chatID)
	if err != nil {
		return project.ChatMemberList{}, err
	}
	rows, err := repository.pool.Query(ctx, `SELECT users.id::text,users.login,profile.first_name,profile.last_name,member.joined_at,(chat.created_by_user_id<>users.id) FROM chat_memberships member JOIN chats chat ON chat.id=member.chat_id AND chat.project_id=member.project_id JOIN users ON users.id=member.user_id JOIN profiles profile ON profile.user_id=users.id WHERE member.chat_id=$1 AND member.project_id=$2 AND member.removed_at IS NULL ORDER BY profile.first_name,users.login`, chatID, projectID)
	if err != nil {
		return project.ChatMemberList{}, fmt.Errorf("list chat members: %w", err)
	}
	defer rows.Close()
	items := []project.ChatMember{}
	for rows.Next() {
		var item project.ChatMember
		if err = rows.Scan(&item.UserID, &item.Login, &item.FirstName, &item.LastName, &item.JoinedAt, &item.Removable); err != nil {
			return project.ChatMemberList{}, fmt.Errorf("scan chat member: %w", err)
		}
		items = append(items, item)
	}
	return project.ChatMemberList{Items: items, CanManage: canManage}, rows.Err()
}

func (repository *Postgres) AddProjectChatMember(ctx context.Context, command project.ChatMemberCommand) (project.ChatMember, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatMember{}, fmt.Errorf("begin chat member addition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := chatManagementAccess(ctx, tx, command.Actor, command.ProjectID, command.ChatID)
	if err != nil {
		return project.ChatMember{}, err
	}
	if !canManage {
		return project.ChatMember{}, project.ErrForbidden
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_memberships WHERE project_id=$1 AND user_id=$2 AND revoked_at IS NULL)`, command.ProjectID, command.UserID).Scan(&allowed); err != nil {
		return project.ChatMember{}, fmt.Errorf("check project member: %w", err)
	}
	if !allowed {
		return project.ChatMember{}, project.ErrNotFound
	}
	tag, err := tx.Exec(ctx, `INSERT INTO chat_memberships(chat_id,project_id,user_id,added_by_user_id,joined_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING`, command.ChatID, command.ProjectID, command.UserID, command.Actor.UserID, command.Now)
	if err != nil {
		return project.ChatMember{}, mapProjectWriteError("add chat member", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ChatMember{}, project.ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('chat.member_added','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text,'userId',$5::text),$6)`, command.RequestID, command.Actor.UserID, command.ProjectID, command.ChatID, command.UserID, command.Now); err != nil {
		return project.ChatMember{}, fmt.Errorf("audit chat member addition: %w", err)
	}
	if err = enqueueChatMemberNotification(ctx, tx, command.ProjectID, command.ChatID, command.Actor.UserID, command.RequestID, command.Notification, command.Now); err != nil {
		return project.ChatMember{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatMember{}, mapProjectWriteError("commit chat member addition", err)
	}
	list, err := repository.ListProjectChatMembers(ctx, command.Actor, command.ProjectID, command.ChatID)
	if err != nil {
		return project.ChatMember{}, err
	}
	for _, item := range list.Items {
		if item.UserID == command.UserID {
			return item, nil
		}
	}
	return project.ChatMember{}, project.ErrNotFound
}

func (repository *Postgres) RemoveProjectChatMember(ctx context.Context, command project.ChatMemberCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chat member removal: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canManage, err := chatManagementAccess(ctx, tx, command.Actor, command.ProjectID, command.ChatID)
	if err != nil {
		return err
	}
	if !canManage {
		return project.ErrForbidden
	}
	var creator string
	if err = tx.QueryRow(ctx, `SELECT created_by_user_id::text FROM chats WHERE id=$1 AND project_id=$2 AND archived_at IS NULL`, command.ChatID, command.ProjectID).Scan(&creator); err != nil {
		return project.ErrNotFound
	}
	if creator == command.UserID {
		return project.ErrConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE chat_memberships SET removed_at=$4,removed_by_user_id=$3,version=version+1 WHERE chat_id=$1 AND user_id=$2 AND removed_at IS NULL`, command.ChatID, command.UserID, command.Actor.UserID, command.Now)
	if err != nil {
		return mapProjectWriteError("remove chat member", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('chat.member_removed','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text,'userId',$5::text),$6)`, command.RequestID, command.Actor.UserID, command.ProjectID, command.ChatID, command.UserID, command.Now); err != nil {
		return fmt.Errorf("audit chat member removal: %w", err)
	}
	if err = enqueueChatMemberNotification(ctx, tx, command.ProjectID, command.ChatID, command.Actor.UserID, command.RequestID, command.Notification, command.Now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *Postgres) MarkProjectChatRead(ctx context.Context, command project.MarkChatReadCommand) error {
	if _, _, err := authorizeContextChat(ctx, repository.pool, command.Actor, command.ProjectID, command.ChatID); err != nil {
		return err
	}
	tag, err := repository.pool.Exec(ctx, `
		UPDATE chat_memberships membership
		SET last_read_at=GREATEST(membership.last_read_at,message.created_at),version=membership.version+1
		FROM chat_messages message
		WHERE membership.chat_id=$1 AND membership.project_id=$2 AND membership.user_id=$3 AND membership.removed_at IS NULL
		  AND message.id=$4 AND message.chat_id=membership.chat_id AND message.project_id=membership.project_id
	`, command.ChatID, command.ProjectID, command.UserID, command.MessageID)
	if err != nil {
		return mapProjectWriteError("mark project chat read", err)
	}
	if tag.RowsAffected() == 0 {
		if command.SuperAdmin {
			return nil
		}
		return project.ErrNotFound
	}
	return nil
}

func (repository *Postgres) ListAccountNotifications(ctx context.Context, actor project.Actor, pageSize int) (project.AccountNotificationList, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text,project_id::text,event_type,title,body,href,created_at,read_at
		FROM account_notifications
		WHERE recipient_user_id=$1
		ORDER BY created_at DESC,id DESC
		LIMIT $2
	`, actor.UserID, pageSize)
	if err != nil {
		return project.AccountNotificationList{}, fmt.Errorf("list account notifications: %w", err)
	}
	defer rows.Close()
	result := project.AccountNotificationList{Items: []project.AccountNotification{}}
	for rows.Next() {
		var item project.AccountNotification
		if err = rows.Scan(&item.ID, &item.ProjectID, &item.EventType, &item.Title, &item.Body, &item.Href, &item.CreatedAt, &item.ReadAt); err != nil {
			return result, fmt.Errorf("scan account notification: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return result, fmt.Errorf("iterate account notifications: %w", err)
	}
	if err = repository.pool.QueryRow(ctx, `SELECT count(*)::int FROM account_notifications WHERE recipient_user_id=$1 AND read_at IS NULL`, actor.UserID).Scan(&result.UnreadCount); err != nil {
		return result, fmt.Errorf("count unread account notifications: %w", err)
	}
	return result, nil
}

func (repository *Postgres) MarkAccountNotificationRead(ctx context.Context, actor project.Actor, notificationID string, now time.Time) error {
	tag, err := repository.pool.Exec(ctx, `UPDATE account_notifications SET read_at=COALESCE(read_at,$3) WHERE id=$1 AND recipient_user_id=$2`, notificationID, actor.UserID, now)
	if err != nil {
		return mapProjectWriteError("mark account notification read", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ErrNotFound
	}
	return nil
}

func (repository *Postgres) MarkAllAccountNotificationsRead(ctx context.Context, actor project.Actor, now time.Time) error {
	if _, err := repository.pool.Exec(ctx, `UPDATE account_notifications SET read_at=$2 WHERE recipient_user_id=$1 AND read_at IS NULL`, actor.UserID, now); err != nil {
		return mapProjectWriteError("mark all account notifications read", err)
	}
	return nil
}

func chatManagementAccess(ctx context.Context, q projectAccessQuerier, actor project.Actor, projectID, chatID string) (bool, error) {
	var canManage bool
	err := q.QueryRow(ctx, `SELECT $3 OR chat.created_by_user_id=$2 OR EXISTS(SELECT 1 FROM project_memberships WHERE project_id=chat.project_id AND user_id=$2 AND project_role='project_admin' AND revoked_at IS NULL) FROM chats chat WHERE chat.id=$1 AND chat.project_id=$4 AND chat.archived_at IS NULL AND ($3 OR EXISTS(SELECT 1 FROM chat_memberships WHERE chat_id=chat.id AND user_id=$2 AND removed_at IS NULL))`, chatID, actor.UserID, actor.SuperAdmin, projectID).Scan(&canManage)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, project.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("authorize chat management: %w", err)
	}
	return canManage, nil
}

func (repository *Postgres) ListProjectDocuments(ctx context.Context, actor project.Actor, projectID string) (project.ProjectDocumentList, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return project.ProjectDocumentList{}, err
	}
	canManage, err := canManageProjectMembers(ctx, repository.pool, actor, projectID)
	if err != nil {
		return project.ProjectDocumentList{}, err
	}
	rows, err := repository.pool.Query(ctx, `SELECT id::text,project_id::text,name,media_type,size_bytes,uploaded_by_user_id::text,created_at,($3 OR uploaded_by_user_id=$2),version FROM project_documents WHERE project_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC,id DESC`, projectID, actor.UserID, canManage || actor.SuperAdmin)
	if err != nil {
		return project.ProjectDocumentList{}, fmt.Errorf("list project documents: %w", err)
	}
	defer rows.Close()
	items := []project.ProjectDocument{}
	for rows.Next() {
		var item project.ProjectDocument
		if err = rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.MediaType, &item.SizeBytes, &item.UploadedByUserID, &item.CreatedAt, &item.CanDelete, &item.Version); err != nil {
			return project.ProjectDocumentList{}, fmt.Errorf("scan project document: %w", err)
		}
		items = append(items, item)
	}
	return project.ProjectDocumentList{Items: items, CanUpload: true}, rows.Err()
}

func (repository *Postgres) UploadProjectDocument(ctx context.Context, command project.UploadProjectDocumentCommand) (project.ProjectDocument, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ProjectDocument{}, fmt.Errorf("begin project document upload: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, false); err != nil {
		return project.ProjectDocument{}, err
	}
	var item project.ProjectDocument
	err = tx.QueryRow(ctx, `INSERT INTO project_documents(project_id,name,media_type,size_bytes,content,uploaded_by_user_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text,project_id::text,name,media_type,size_bytes,uploaded_by_user_id::text,created_at,version`, command.ProjectID, command.Name, command.MediaType, len(command.Content), command.Content, command.UserID, command.Now).Scan(&item.ID, &item.ProjectID, &item.Name, &item.MediaType, &item.SizeBytes, &item.UploadedByUserID, &item.CreatedAt, &item.Version)
	if err != nil {
		return item, mapProjectWriteError("upload project document", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('project.document.uploaded','success',$1,$2,'project_document',$3,jsonb_build_object('documentId',$4::text,'name',$5::text),$6)`, command.RequestID, command.UserID, command.ProjectID, item.ID, command.Name, command.Now); err != nil {
		return item, fmt.Errorf("audit project document upload: %w", err)
	}
	if err = enqueueProjectMemberNotification(ctx, tx, command.ProjectID, command.UserID, item.ID, command.Notification, command.Now); err != nil {
		return item, err
	}
	if err = tx.Commit(ctx); err != nil {
		return item, mapProjectWriteError("commit project document upload", err)
	}
	item.CanDelete = true
	return item, nil
}

func (repository *Postgres) DownloadProjectDocument(ctx context.Context, actor project.Actor, projectID, documentID string) (project.ProjectDocumentContent, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return project.ProjectDocumentContent{}, err
	}
	var value project.ProjectDocumentContent
	err := repository.pool.QueryRow(ctx, `SELECT id::text,project_id::text,name,media_type,size_bytes,uploaded_by_user_id::text,created_at,version,content FROM project_documents WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, documentID, projectID).Scan(&value.ID, &value.ProjectID, &value.Name, &value.MediaType, &value.SizeBytes, &value.UploadedByUserID, &value.CreatedAt, &value.Version, &value.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, project.ErrNotFound
	}
	if err != nil {
		return value, fmt.Errorf("download project document: %w", err)
	}
	value.CanDelete = actor.SuperAdmin || value.UploadedByUserID == actor.UserID
	return value, nil
}

func (repository *Postgres) DeleteProjectDocument(ctx context.Context, command project.DeleteProjectDocumentCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin project document deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, false); err != nil {
		return err
	}
	canManage, err := canManageProjectMembers(ctx, tx, command.Actor, command.ProjectID)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE project_documents SET deleted_at=$4,deleted_by_user_id=$3,version=version+1 WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND ($5 OR uploaded_by_user_id=$3)`, command.DocumentID, command.ProjectID, command.UserID, command.Now, canManage || command.SuperAdmin)
	if err != nil {
		return mapProjectWriteError("delete project document", err)
	}
	if tag.RowsAffected() == 0 {
		return project.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES('project.document.deleted','success',$1,$2,'project_document',$3,jsonb_build_object('documentId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, command.DocumentID, command.Now); err != nil {
		return fmt.Errorf("audit project document deletion: %w", err)
	}
	if err = enqueueProjectMemberNotification(ctx, tx, command.ProjectID, command.UserID, command.DocumentID, command.Notification, command.Now); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return mapProjectWriteError("commit project document deletion", err)
	}
	return nil
}
