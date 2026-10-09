package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectChat(ctx context.Context, query project.ChatQuery) (project.ProjectChatPage, error) {
	var page project.ProjectChatPage
	err := repository.pool.QueryRow(ctx, `
		SELECT chat.id::text,chat.project_id::text
		FROM chats chat
		JOIN projects project ON project.id=chat.project_id AND project.archived_at IS NULL
		WHERE chat.project_id=$1 AND chat.kind='project' AND chat.archived_at IS NULL
		  AND ($3::boolean OR (
		    EXISTS (SELECT 1 FROM project_memberships membership WHERE membership.project_id=chat.project_id AND membership.user_id=$2 AND membership.revoked_at IS NULL)
		    AND EXISTS (SELECT 1 FROM chat_memberships membership WHERE membership.chat_id=chat.id AND membership.user_id=$2 AND membership.removed_at IS NULL)
		  ))
		ORDER BY chat.created_at,chat.id LIMIT 1
	`, query.ProjectID, query.UserID, query.SuperAdmin).Scan(&page.ChatID, &page.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.ProjectChatPage{}, project.ErrNotFound
	}
	if err != nil {
		return project.ProjectChatPage{}, fmt.Errorf("authorize project chat: %w", err)
	}

	rows, err := repository.pool.Query(ctx, `
		SELECT message.id::text,message.chat_id::text,message.project_id::text,
		       CASE WHEN message.deleted_at IS NULL THEN message.body ELSE 'Сообщение удалено' END,
		       message.author_user_id::text,users.login,profile.first_name,profile.last_name,
		       message.created_at,message.edited_at,message.deleted_at,message.version
		FROM chat_messages message
		JOIN chats chat ON chat.id=message.chat_id AND chat.project_id=message.project_id AND chat.archived_at IS NULL
		JOIN projects project ON project.id=chat.project_id AND project.archived_at IS NULL
		JOIN users ON users.id=message.author_user_id
		JOIN profiles profile ON profile.user_id=users.id
		WHERE chat.id=$1 AND chat.project_id=$2
		  AND ($4::boolean OR EXISTS (
		    SELECT 1 FROM project_memberships membership
		    WHERE membership.project_id=chat.project_id AND membership.user_id=$3 AND membership.revoked_at IS NULL
		  ))
		  AND ($5::uuid IS NULL OR (message.created_at,message.id) < (
		    SELECT anchor.created_at,anchor.id FROM chat_messages anchor
		    WHERE anchor.chat_id=chat.id AND anchor.id=$5::uuid
		  ))
		ORDER BY message.created_at DESC,message.id DESC
		LIMIT $6
	`, page.ChatID, page.ProjectID, query.UserID, query.SuperAdmin, query.BeforeID, query.PageSize+1)
	if err != nil {
		return project.ProjectChatPage{}, fmt.Errorf("list project chat messages: %w", err)
	}
	defer rows.Close()
	messages := make([]project.ChatMessage, 0, query.PageSize+1)
	for rows.Next() {
		message, scanErr := scanProjectChatMessage(rows)
		if scanErr != nil {
			return project.ProjectChatPage{}, fmt.Errorf("scan project chat message: %w", scanErr)
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		return project.ProjectChatPage{}, fmt.Errorf("iterate project chat messages: %w", err)
	}
	if len(messages) > query.PageSize {
		page.HasMore = true
		messages = messages[:query.PageSize]
	}
	if page.HasMore && len(messages) > 0 {
		cursor := messages[len(messages)-1].ID
		page.NextCursor = &cursor
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	page.Messages = messages
	page.CurrentUserID = query.UserID
	page.CanSend = true
	return page, nil
}

func (repository *Postgres) CreateProjectChatMessage(ctx context.Context, command project.CreateChatMessageCommand) (project.ChatMessage, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatMessage{}, fmt.Errorf("begin project chat message creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = requireProjectAccess(ctx, tx, command.Actor, command.ProjectID, false); err != nil {
		return project.ChatMessage{}, err
	}
	var chatID string
	if err = tx.QueryRow(ctx, `SELECT chat.id::text FROM chats chat WHERE chat.project_id=$1 AND chat.kind='project' AND chat.archived_at IS NULL AND ($3 OR EXISTS (SELECT 1 FROM chat_memberships member WHERE member.chat_id=chat.id AND member.user_id=$2 AND member.removed_at IS NULL)) ORDER BY chat.created_at,chat.id LIMIT 1`, command.ProjectID, command.UserID, command.SuperAdmin).Scan(&chatID); errors.Is(err, pgx.ErrNoRows) {
		return project.ChatMessage{}, project.ErrNotFound
	} else if err != nil {
		return project.ChatMessage{}, fmt.Errorf("find project chat: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO chat_messages (chat_id,project_id,author_user_id,client_message_id,body,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (chat_id,client_message_id) DO NOTHING
	`, chatID, command.ProjectID, command.UserID, command.ClientMessageID, command.Body, command.Now)
	if err != nil {
		return project.ChatMessage{}, mapProjectWriteError("insert project chat message", err)
	}
	inserted := tag.RowsAffected() > 0
	if !inserted {
		var existingAuthor, existingBody string
		if err = tx.QueryRow(ctx, `SELECT author_user_id::text,body FROM chat_messages WHERE chat_id=$1 AND client_message_id=$2`, chatID, command.ClientMessageID).Scan(&existingAuthor, &existingBody); err != nil {
			return project.ChatMessage{}, fmt.Errorf("load repeated project chat message: %w", err)
		}
		if existingAuthor != command.UserID || existingBody != command.Body {
			return project.ChatMessage{}, project.ErrConflict
		}
	} else if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at)
		VALUES ('chat.message_created','success',$1,$2,'chat_message',$3,jsonb_build_object('chatId',$4::text,'clientMessageId',$5::text),$6)
	`, command.RequestID, command.UserID, command.ProjectID, chatID, command.ClientMessageID, command.Now); err != nil {
		return project.ChatMessage{}, fmt.Errorf("audit project chat message creation: %w", err)
	}
	message, err := queryProjectChatMessage(ctx, tx, chatID, command.ClientMessageID)
	if err != nil {
		return project.ChatMessage{}, err
	}
	if inserted {
		if err = enqueueProjectMemberNotification(ctx, tx, command.ProjectID, command.UserID, message.ID, command.Notification, command.Now); err != nil {
			return project.ChatMessage{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatMessage{}, mapProjectWriteError("commit project chat message creation", err)
	}
	return message, nil
}

func (repository *Postgres) OpenProjectContextChat(ctx context.Context, command project.OpenContextChatCommand) (project.ChatContext, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatContext{}, fmt.Errorf("begin context chat creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	contextTitle, err := contextEntityTitle(ctx, tx, command.Actor, command.ProjectID, command.ContextType, command.ContextID)
	if err != nil {
		return project.ChatContext{}, err
	}
	var chatID, storedName string
	inserted := true
	err = tx.QueryRow(ctx, `
		INSERT INTO chats (project_id,kind,context_type,context_id,name,created_by_user_id,created_at)
		VALUES ($1,'context',$2,$3,$4,$5,$6)
		ON CONFLICT (project_id,context_type,context_id) WHERE kind='context' AND archived_at IS NULL DO NOTHING
		RETURNING id::text,name
	`, command.ProjectID, command.ContextType, command.ContextID, command.Name, command.UserID, command.Now).Scan(&chatID, &storedName)
	if errors.Is(err, pgx.ErrNoRows) {
		inserted = false
		err = tx.QueryRow(ctx, `SELECT id::text,name FROM chats WHERE project_id=$1 AND kind='context' AND context_type=$2 AND context_id=$3 AND archived_at IS NULL`, command.ProjectID, command.ContextType, command.ContextID).Scan(&chatID, &storedName)
	}
	if err != nil {
		return project.ChatContext{}, mapProjectWriteError("open context chat", err)
	}
	if inserted {
		if _, err = tx.Exec(ctx, `
			INSERT INTO chat_memberships (chat_id,project_id,user_id,added_by_user_id,joined_at)
			SELECT $1,$2,membership.user_id,$3,$4
			FROM project_memberships membership
			WHERE membership.project_id=$2 AND membership.revoked_at IS NULL
			ON CONFLICT (chat_id,user_id) WHERE removed_at IS NULL DO NOTHING
		`, chatID, command.ProjectID, command.UserID, command.Now); err != nil {
			return project.ChatContext{}, mapProjectWriteError("seed context chat members", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at)
			VALUES ('chat.context_created','success',$1,$2,'chat',$3,jsonb_build_object('chatId',$4::text,'contextType',$5::text,'contextId',$6::text),$7)
		`, command.RequestID, command.UserID, command.ProjectID, chatID, command.ContextType, command.ContextID, command.Now); err != nil {
			return project.ChatContext{}, fmt.Errorf("audit context chat creation: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatContext{}, mapProjectWriteError("commit context chat creation", err)
	}
	return project.ChatContext{ChatID: chatID, ProjectID: command.ProjectID, ContextType: command.ContextType, ContextID: command.ContextID, ContextTitle: contextTitle, Name: storedName}, nil
}

func (repository *Postgres) ListProjectContextChat(ctx context.Context, query project.ChatQuery) (project.ProjectChatPage, error) {
	chatContext, kind, err := authorizeContextChat(ctx, repository.pool, query.Actor, query.ProjectID, query.ChatID)
	if err != nil {
		return project.ProjectChatPage{}, err
	}
	messages, hasMore, nextCursor, err := listContextChatMessages(ctx, repository.pool, chatContext.ChatID, chatContext.ProjectID, query.BeforeID, query.PageSize)
	if err != nil {
		return project.ProjectChatPage{}, err
	}
	page := project.ProjectChatPage{
		ChatID: chatContext.ChatID, ProjectID: chatContext.ProjectID, CurrentUserID: query.UserID,
		Messages: messages, HasMore: hasMore, NextCursor: nextCursor, CanSend: true,
	}
	if kind == "context" {
		page.Context = &chatContext
	}
	return page, nil
}

func (repository *Postgres) CreateProjectContextChatMessage(ctx context.Context, command project.CreateChatMessageCommand) (project.ChatMessage, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.ChatMessage{}, fmt.Errorf("begin context chat message creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, _, err = authorizeContextChat(ctx, tx, command.Actor, command.ProjectID, command.ChatID); err != nil {
		return project.ChatMessage{}, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO chat_messages (chat_id,project_id,author_user_id,client_message_id,body,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (chat_id,client_message_id) DO NOTHING
	`, command.ChatID, command.ProjectID, command.UserID, command.ClientMessageID, command.Body, command.Now)
	if err != nil {
		return project.ChatMessage{}, mapProjectWriteError("insert context chat message", err)
	}
	inserted := tag.RowsAffected() > 0
	if !inserted {
		var existingAuthor, existingBody string
		if err = tx.QueryRow(ctx, `SELECT author_user_id::text,body FROM chat_messages WHERE chat_id=$1 AND client_message_id=$2`, command.ChatID, command.ClientMessageID).Scan(&existingAuthor, &existingBody); err != nil {
			return project.ChatMessage{}, fmt.Errorf("load repeated context chat message: %w", err)
		}
		if existingAuthor != command.UserID || existingBody != command.Body {
			return project.ChatMessage{}, project.ErrConflict
		}
	} else if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at)
		VALUES ('chat.message_created','success',$1,$2,'chat_message',$3,jsonb_build_object('chatId',$4::text,'clientMessageId',$5::text),$6)
	`, command.RequestID, command.UserID, command.ProjectID, command.ChatID, command.ClientMessageID, command.Now); err != nil {
		return project.ChatMessage{}, fmt.Errorf("audit context chat message creation: %w", err)
	}
	message, err := queryProjectChatMessage(ctx, tx, command.ChatID, command.ClientMessageID)
	if err != nil {
		return project.ChatMessage{}, err
	}
	if inserted {
		if err = enqueueContextChatNotification(ctx, tx, command.ProjectID, command.ChatID, message.ID, command.UserID, command.Notification, command.Now); err != nil {
			return project.ChatMessage{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return project.ChatMessage{}, mapProjectWriteError("commit context chat message creation", err)
	}
	return message, nil
}

func authorizeContextChat(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID, chatID string) (project.ChatContext, string, error) {
	var value project.ChatContext
	var kind string
	var contextType, contextID *string
	err := querier.QueryRow(ctx, `
		SELECT id::text,project_id::text,kind,context_type,context_id::text,name
		FROM chats
		WHERE id=$1 AND project_id=$2 AND archived_at IS NULL
		  AND ($4 OR (
		    EXISTS (SELECT 1 FROM project_memberships WHERE project_id=chats.project_id AND user_id=$3 AND revoked_at IS NULL)
		    AND EXISTS (SELECT 1 FROM chat_memberships WHERE chat_id=chats.id AND user_id=$3 AND removed_at IS NULL)
		  ))
	`, chatID, projectID, actor.UserID, actor.SuperAdmin).Scan(&value.ChatID, &value.ProjectID, &kind, &contextType, &contextID, &value.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, "", project.ErrNotFound
	}
	if err != nil {
		return value, "", fmt.Errorf("load context chat: %w", err)
	}
	if kind == "context" && contextType != nil && contextID != nil {
		value.ContextType, value.ContextID = *contextType, *contextID
		value.ContextTitle, err = contextEntityTitle(ctx, querier, actor, projectID, value.ContextType, value.ContextID)
	}
	return value, kind, err
}

func contextEntityTitle(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID, contextType, contextID string) (string, error) {
	var title string
	switch contextType {
	case "task":
		if _, _, _, err := taskMutationAccess(ctx, querier, actor, projectID, contextID); err != nil {
			return "", err
		}
		if err := querier.QueryRow(ctx, `SELECT title FROM project_tasks WHERE project_id=$1 AND id=$2`, projectID, contextID).Scan(&title); err != nil {
			return "", mapContextEntityReadError("task", err)
		}
	case "material":
		access, err := materialAccess(ctx, querier, actor, projectID, false)
		if err != nil {
			return "", err
		}
		if !access.view {
			return "", project.ErrNotFound
		}
		if err = querier.QueryRow(ctx, `SELECT name FROM project_materials WHERE project_id=$1 AND id=$2 AND deleted_at IS NULL`, projectID, contextID).Scan(&title); err != nil {
			return "", mapContextEntityReadError("material", err)
		}
	case "expense":
		if _, _, _, err := projectExpenseAccess(ctx, querier, actor, projectID, false); err != nil {
			return "", err
		}
		if err := querier.QueryRow(ctx, `SELECT description FROM project_expense_requests WHERE project_id=$1 AND id=$2`, projectID, contextID).Scan(&title); err != nil {
			return "", mapContextEntityReadError("expense", err)
		}
	default:
		return "", project.ErrNotFound
	}
	return title, nil
}

func mapContextEntityReadError(entity string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return project.ErrNotFound
	}
	return fmt.Errorf("load context %s: %w", entity, err)
}

func listContextChatMessages(ctx context.Context, querier projectAccessQuerier, chatID, projectID string, beforeID *string, pageSize int) ([]project.ChatMessage, bool, *string, error) {
	rows, err := querier.Query(ctx, `
		SELECT message.id::text,message.chat_id::text,message.project_id::text,
		       CASE WHEN message.deleted_at IS NULL THEN message.body ELSE 'Сообщение удалено' END,
		       message.author_user_id::text,users.login,profile.first_name,profile.last_name,
		       message.created_at,message.edited_at,message.deleted_at,message.version
		FROM chat_messages message
		JOIN users ON users.id=message.author_user_id
		JOIN profiles profile ON profile.user_id=users.id
		WHERE message.chat_id=$1 AND message.project_id=$2
		  AND ($3::uuid IS NULL OR (message.created_at,message.id) < (
		    SELECT anchor.created_at,anchor.id FROM chat_messages anchor
		    WHERE anchor.chat_id=$1 AND anchor.id=$3::uuid
		  ))
		ORDER BY message.created_at DESC,message.id DESC
		LIMIT $4
	`, chatID, projectID, beforeID, pageSize+1)
	if err != nil {
		return nil, false, nil, fmt.Errorf("list context chat messages: %w", err)
	}
	defer rows.Close()
	messages := make([]project.ChatMessage, 0, pageSize+1)
	for rows.Next() {
		message, scanErr := scanProjectChatMessage(rows)
		if scanErr != nil {
			return nil, false, nil, fmt.Errorf("scan context chat message: %w", scanErr)
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		return nil, false, nil, fmt.Errorf("iterate context chat messages: %w", err)
	}
	hasMore := len(messages) > pageSize
	if hasMore {
		messages = messages[:pageSize]
	}
	var nextCursor *string
	if hasMore && len(messages) > 0 {
		cursor := messages[len(messages)-1].ID
		nextCursor = &cursor
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, hasMore, nextCursor, nil
}

func queryProjectChatMessage(ctx context.Context, query projectAccessQuerier, chatID, clientMessageID string) (project.ChatMessage, error) {
	message, err := scanProjectChatMessage(query.QueryRow(ctx, `
		SELECT message.id::text,message.chat_id::text,message.project_id::text,message.body,
		       message.author_user_id::text,users.login,profile.first_name,profile.last_name,
		       message.created_at,message.edited_at,message.deleted_at,message.version
		FROM chat_messages message
		JOIN users ON users.id=message.author_user_id
		JOIN profiles profile ON profile.user_id=users.id
		WHERE message.chat_id=$1 AND message.client_message_id=$2
	`, chatID, clientMessageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return project.ChatMessage{}, project.ErrNotFound
	}
	if err != nil {
		return project.ChatMessage{}, fmt.Errorf("load project chat message: %w", err)
	}
	return message, nil
}

type projectChatMessageScanner interface {
	Scan(...any) error
}

func scanProjectChatMessage(row projectChatMessageScanner) (project.ChatMessage, error) {
	var message project.ChatMessage
	err := row.Scan(
		&message.ID, &message.ChatID, &message.ProjectID, &message.Body,
		&message.Author.UserID, &message.Author.Login, &message.Author.FirstName, &message.Author.LastName,
		&message.CreatedAt, &message.EditedAt, &message.DeletedAt, &message.Version,
	)
	return message, err
}
