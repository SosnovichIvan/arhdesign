package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *Postgres) ListGlobalChats(ctx context.Context, query globalchat.ListQuery) ([]globalchat.Summary, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT chat.id::text,chat.kind,chat.name,chat.last_activity_at,chat.version,
		       CASE WHEN membership.user_id IS NULL THEN 0 ELSE (
		         SELECT count(*)::integer FROM global_chat_messages unread
		         WHERE unread.chat_id=chat.id AND unread.author_user_id<>$1::uuid
		           AND unread.created_at>COALESCE(membership.last_read_at,membership.joined_at)
		       ) END AS unread_count
		FROM global_chats chat
		LEFT JOIN global_chat_members membership ON membership.chat_id=chat.id AND membership.user_id=$1::uuid AND membership.left_at IS NULL
		WHERE chat.archived_at IS NULL
		  AND ($2::boolean OR membership.user_id IS NOT NULL)
		  AND ($3::timestamptz IS NULL OR (chat.last_activity_at,chat.id)<($3::timestamptz,$4::uuid))
		ORDER BY chat.last_activity_at DESC,chat.id DESC
		LIMIT $5
	`, query.UserID, query.Privileged, query.BeforeLastActivityAt, query.BeforeID, query.PageSize)
	if err != nil {
		return nil, fmt.Errorf("list global chats: %w", err)
	}
	defer rows.Close()
	items := make([]globalchat.Summary, 0, query.PageSize)
	ids := make([]string, 0, query.PageSize)
	for rows.Next() {
		var item globalchat.Summary
		if err = rows.Scan(&item.ID, &item.Kind, &item.Name, &item.LastActivityAt, &item.Version, &item.UnreadCount); err != nil {
			return nil, fmt.Errorf("scan global chat: %w", err)
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global chats: %w", err)
	}
	if err = repository.hydrateGlobalChatSummaries(ctx, items, ids, query.UserID); err != nil {
		return nil, err
	}
	return items, nil
}

func (repository *Postgres) SearchGlobalChatCandidates(ctx context.Context, userID, query string) ([]globalchat.Candidate, error) {
	pattern := escapeGlobalChatLike(query) + "%"
	rows, err := repository.pool.Query(ctx, `
		SELECT users.id::text,users.login,users.email,profile.first_name,profile.last_name
		FROM users JOIN profiles profile ON profile.user_id=users.id
		WHERE users.status='active' AND users.id<>$1::uuid
		  AND (users.login_normalized LIKE $2 ESCAPE '\' OR users.email_normalized LIKE $2 ESCAPE '\')
		ORDER BY CASE WHEN users.login_normalized=$3 THEN 0 ELSE 1 END,users.login_normalized,users.id
		LIMIT 20
	`, userID, pattern, query)
	if err != nil {
		return nil, fmt.Errorf("search global chat candidates: %w", err)
	}
	defer rows.Close()
	items := make([]globalchat.Candidate, 0, 20)
	for rows.Next() {
		var item globalchat.Candidate
		if err = rows.Scan(&item.UserID, &item.Login, &item.Email, &item.FirstName, &item.LastName); err != nil {
			return nil, fmt.Errorf("scan global chat candidate: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global chat candidates: %w", err)
	}
	return items, nil
}

func (repository *Postgres) CreateGlobalChat(ctx context.Context, command globalchat.CreateCommand) (globalchat.Summary, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return globalchat.Summary{}, fmt.Errorf("begin global chat creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	memberIDs := append([]string{command.UserID}, command.MemberUserIDs...)
	var activeCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status='active' AND id=ANY($1::uuid[])`, memberIDs).Scan(&activeCount); err != nil {
		return globalchat.Summary{}, fmt.Errorf("validate global chat members: %w", err)
	}
	if activeCount != len(memberIDs) {
		return globalchat.Summary{}, globalchat.ErrNotFound
	}

	var chatID string
	if command.Kind == "direct" {
		pair := append([]string(nil), memberIDs...)
		sort.Strings(pair)
		directKey := strings.Join(pair, ":")
		err = tx.QueryRow(ctx, `
			INSERT INTO global_chats (kind,direct_key,created_by_user_id,created_at,last_activity_at)
			VALUES ('direct',$1,$2,$3,$3)
			ON CONFLICT (direct_key) WHERE kind='direct' AND archived_at IS NULL
			DO UPDATE SET last_activity_at=global_chats.last_activity_at
			RETURNING id::text
		`, directKey, command.UserID, command.Now).Scan(&chatID)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO global_chats (kind,name,created_by_user_id,created_at,last_activity_at)
			VALUES ('group',$1,$2,$3,$3) RETURNING id::text
		`, command.Name, command.UserID, command.Now).Scan(&chatID)
	}
	if err != nil {
		return globalchat.Summary{}, mapGlobalChatWriteError("insert global chat", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO global_chat_members (chat_id,user_id,added_by_user_id,joined_at,last_read_at)
		SELECT $1::uuid,value::uuid,$2::uuid,$3::timestamptz,$3::timestamptz FROM unnest($4::text[]) value
		ON CONFLICT (chat_id,user_id) DO UPDATE SET left_at=NULL,last_read_at=COALESCE(global_chat_members.last_read_at,EXCLUDED.last_read_at)
	`, chatID, command.UserID, command.Now, memberIDs); err != nil {
		return globalchat.Summary{}, mapGlobalChatWriteError("insert global chat members", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,metadata,occurred_at)
		VALUES ('global_chat.created','success',$1,$2,'global_chat',jsonb_build_object('chatId',$3::text,'kind',$4::text),$5)
	`, command.RequestID, command.UserID, chatID, command.Kind, command.Now); err != nil {
		return globalchat.Summary{}, fmt.Errorf("audit global chat creation: %w", err)
	}
	var createdSummary globalchat.Summary
	if err = tx.QueryRow(ctx, `SELECT id::text,kind,name,last_activity_at,version FROM global_chats WHERE id=$1`, chatID).Scan(&createdSummary.ID, &createdSummary.Kind, &createdSummary.Name, &createdSummary.LastActivityAt, &createdSummary.Version); err != nil {
		return globalchat.Summary{}, fmt.Errorf("load created global chat: %w", err)
	}
	items := []globalchat.Summary{createdSummary}
	if err = repository.hydrateGlobalChatSummariesWith(ctx, tx, items, []string{chatID}, command.UserID); err != nil {
		return globalchat.Summary{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return globalchat.Summary{}, mapGlobalChatWriteError("commit global chat creation", err)
	}
	return items[0], nil
}

func (repository *Postgres) GetGlobalChat(ctx context.Context, query globalchat.ConversationQuery) (globalchat.Conversation, error) {
	var summary globalchat.Summary
	var member bool
	err := repository.pool.QueryRow(ctx, `
		SELECT chat.id::text,chat.kind,chat.name,chat.last_activity_at,chat.version,
		       membership.user_id IS NOT NULL,
		       CASE WHEN membership.user_id IS NULL THEN 0 ELSE (
		         SELECT count(*)::integer FROM global_chat_messages unread
		         WHERE unread.chat_id=chat.id AND unread.author_user_id<>$2::uuid
		           AND unread.created_at>COALESCE(membership.last_read_at,membership.joined_at)
		       ) END
		FROM global_chats chat
		LEFT JOIN global_chat_members membership ON membership.chat_id=chat.id AND membership.user_id=$2::uuid AND membership.left_at IS NULL
		WHERE chat.id=$1::uuid AND chat.archived_at IS NULL AND ($3::boolean OR membership.user_id IS NOT NULL)
	`, query.ChatID, query.UserID, query.Privileged).Scan(&summary.ID, &summary.Kind, &summary.Name, &summary.LastActivityAt, &summary.Version, &member, &summary.UnreadCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return globalchat.Conversation{}, globalchat.ErrNotFound
	}
	if err != nil {
		return globalchat.Conversation{}, fmt.Errorf("authorize global chat: %w", err)
	}
	items := []globalchat.Summary{summary}
	if err = repository.hydrateGlobalChatSummaries(ctx, items, []string{summary.ID}, query.UserID); err != nil {
		return globalchat.Conversation{}, err
	}
	summary = items[0]

	rows, err := repository.pool.Query(ctx, `
		SELECT message.id::text,message.chat_id::text,message.body,message.created_at,message.edited_at,message.deleted_at,message.version,
		       users.id::text,users.login,profile.first_name,profile.last_name
		FROM global_chat_messages message
		JOIN users ON users.id=message.author_user_id JOIN profiles profile ON profile.user_id=users.id
		WHERE message.chat_id=$1::uuid
		  AND ($2::uuid IS NULL OR (message.created_at,message.id)<(SELECT anchor.created_at,anchor.id FROM global_chat_messages anchor WHERE anchor.chat_id=$1::uuid AND anchor.id=$2::uuid))
		ORDER BY message.created_at DESC,message.id DESC LIMIT $3
	`, query.ChatID, query.BeforeID, query.PageSize+1)
	if err != nil {
		return globalchat.Conversation{}, fmt.Errorf("list global chat messages: %w", err)
	}
	defer rows.Close()
	messages := make([]globalchat.Message, 0, query.PageSize+1)
	for rows.Next() {
		var message globalchat.Message
		if err = rows.Scan(&message.ID, &message.ChatID, &message.Body, &message.CreatedAt, &message.EditedAt, &message.DeletedAt, &message.Version,
			&message.Author.UserID, &message.Author.Login, &message.Author.FirstName, &message.Author.LastName); err != nil {
			return globalchat.Conversation{}, fmt.Errorf("scan global chat message: %w", err)
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		return globalchat.Conversation{}, fmt.Errorf("iterate global chat messages: %w", err)
	}
	hasMore := len(messages) > query.PageSize
	if hasMore {
		messages = messages[:query.PageSize]
	}
	var next *string
	if hasMore && len(messages) > 0 {
		value := messages[len(messages)-1].ID
		next = &value
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return globalchat.Conversation{Chat: summary, CurrentUserID: query.UserID, Messages: messages, NextCursor: next, HasMore: hasMore, CanSend: member || query.Privileged}, nil
}

func (repository *Postgres) CreateGlobalChatMessage(ctx context.Context, command globalchat.SendCommand) (globalchat.Message, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return globalchat.Message{}, fmt.Errorf("begin global chat message: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	if err = tx.QueryRow(ctx, `
		SELECT $3::boolean OR EXISTS (SELECT 1 FROM global_chat_members member WHERE member.chat_id=chat.id AND member.user_id=$2::uuid AND member.left_at IS NULL)
		FROM global_chats chat WHERE chat.id=$1::uuid AND chat.archived_at IS NULL FOR UPDATE
	`, command.ChatID, command.UserID, command.Privileged).Scan(&allowed); errors.Is(err, pgx.ErrNoRows) || err == nil && !allowed {
		return globalchat.Message{}, globalchat.ErrNotFound
	} else if err != nil {
		return globalchat.Message{}, fmt.Errorf("authorize global chat message: %w", err)
	}
	result, err := tx.Exec(ctx, `
		INSERT INTO global_chat_messages (chat_id,author_user_id,client_message_id,body,created_at)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (chat_id,client_message_id) DO NOTHING
	`, command.ChatID, command.UserID, command.ClientMessageID, command.Body, command.Now)
	if err != nil {
		return globalchat.Message{}, mapGlobalChatWriteError("insert global chat message", err)
	}
	if result.RowsAffected() == 0 {
		var authorID, body string
		if err = tx.QueryRow(ctx, `SELECT author_user_id::text,body FROM global_chat_messages WHERE chat_id=$1 AND client_message_id=$2`, command.ChatID, command.ClientMessageID).Scan(&authorID, &body); err != nil {
			return globalchat.Message{}, fmt.Errorf("load repeated global chat message: %w", err)
		}
		if authorID != command.UserID || body != command.Body {
			return globalchat.Message{}, globalchat.ErrConflict
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE global_chats SET last_activity_at=$2,version=version+1 WHERE id=$1`, command.ChatID, command.Now); err != nil {
			return globalchat.Message{}, fmt.Errorf("update global chat activity: %w", err)
		}
		if _, err = tx.Exec(ctx, `UPDATE global_chat_members SET last_read_at=$3 WHERE chat_id=$1 AND user_id=$2 AND left_at IS NULL`, command.ChatID, command.UserID, command.Now); err != nil {
			return globalchat.Message{}, fmt.Errorf("advance author read marker: %w", err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,metadata,occurred_at) VALUES ('global_chat.message_created','success',$1,$2,'global_chat_message',jsonb_build_object('chatId',$3::text,'clientMessageId',$4::text),$5)`, command.RequestID, command.UserID, command.ChatID, command.ClientMessageID, command.Now); err != nil {
			return globalchat.Message{}, fmt.Errorf("audit global chat message: %w", err)
		}
		if command.Notification != nil {
			if _, err = tx.Exec(ctx, `
				INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version,metadata,created_at,updated_at)
				SELECT member.user_id,'telegram',$4,$4 || ':' || $3::text || ':' || member.user_id,$5,$6,jsonb_build_object('chatId',$1::text,'messageId',$3::text),$7,$7
				FROM global_chat_members member
				JOIN users ON users.id=member.user_id AND users.status='active'
				JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
				JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
				LEFT JOIN account_notification_preferences preferences ON preferences.user_id=users.id
				WHERE member.chat_id=$1::uuid AND member.left_at IS NULL AND member.user_id<>$2::uuid AND COALESCE(preferences.telegram_events_enabled,TRUE)
				ON CONFLICT (idempotency_key) DO NOTHING
			`, command.ChatID, command.UserID, command.ClientMessageID, command.Notification.MessageType, command.Notification.PayloadCiphertext, command.Notification.PayloadKeyVersion, command.Now); err != nil {
				return globalchat.Message{}, fmt.Errorf("enqueue global chat notification: %w", err)
			}
		}
	}
	message, err := queryGlobalChatMessage(ctx, tx, command.ChatID, command.ClientMessageID)
	if err != nil {
		return globalchat.Message{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return globalchat.Message{}, mapGlobalChatWriteError("commit global chat message", err)
	}
	return message, nil
}

func (repository *Postgres) MarkGlobalChatRead(ctx context.Context, actor globalchat.Actor, chatID string, now time.Time) error {
	result, err := repository.pool.Exec(ctx, `
		UPDATE global_chat_members member SET last_read_at=GREATEST(COALESCE(last_read_at,joined_at),$3)
		FROM global_chats chat
		WHERE member.chat_id=$1::uuid AND member.user_id=$2::uuid AND member.left_at IS NULL
		  AND chat.id=member.chat_id AND chat.archived_at IS NULL
	`, chatID, actor.UserID, now)
	if err != nil {
		return fmt.Errorf("mark global chat read: %w", err)
	}
	if result.RowsAffected() == 0 && !actor.Privileged {
		return globalchat.ErrNotFound
	}
	return nil
}

type globalChatQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (repository *Postgres) hydrateGlobalChatSummaries(ctx context.Context, items []globalchat.Summary, ids []string, viewerID string) error {
	return repository.hydrateGlobalChatSummariesWith(ctx, repository.pool, items, ids, viewerID)
}

func (repository *Postgres) hydrateGlobalChatSummariesWith(ctx context.Context, querier globalChatQuerier, items []globalchat.Summary, ids []string, viewerID string) error {
	if len(ids) == 0 {
		return nil
	}
	index := make(map[string]int, len(items))
	for position := range items {
		index[items[position].ID] = position
	}
	rows, err := querier.Query(ctx, `
		SELECT member.chat_id::text,users.id::text,users.login,users.email,profile.first_name,profile.last_name,member.joined_at
		FROM global_chat_members member JOIN users ON users.id=member.user_id JOIN profiles profile ON profile.user_id=users.id
		WHERE member.chat_id=ANY($1::uuid[]) AND member.left_at IS NULL
		ORDER BY member.chat_id,member.joined_at,member.user_id
	`, ids)
	if err != nil {
		return fmt.Errorf("load global chat members: %w", err)
	}
	for rows.Next() {
		var chatID string
		var member globalchat.Member
		if err = rows.Scan(&chatID, &member.UserID, &member.Login, &member.Email, &member.FirstName, &member.LastName, &member.JoinedAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan global chat member: %w", err)
		}
		position, exists := index[chatID]
		if exists {
			items[position].Members = append(items[position].Members, member)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate global chat members: %w", err)
	}
	rows.Close()

	rows, err = querier.Query(ctx, `
		SELECT DISTINCT ON (message.chat_id) message.chat_id::text,message.id::text,message.body,message.created_at,message.edited_at,message.deleted_at,message.version,
		       users.id::text,users.login,profile.first_name,profile.last_name
		FROM global_chat_messages message JOIN users ON users.id=message.author_user_id JOIN profiles profile ON profile.user_id=users.id
		WHERE message.chat_id=ANY($1::uuid[])
		ORDER BY message.chat_id,message.created_at DESC,message.id DESC
	`, ids)
	if err != nil {
		return fmt.Errorf("load global chat last messages: %w", err)
	}
	for rows.Next() {
		var message globalchat.Message
		if err = rows.Scan(&message.ChatID, &message.ID, &message.Body, &message.CreatedAt, &message.EditedAt, &message.DeletedAt, &message.Version,
			&message.Author.UserID, &message.Author.Login, &message.Author.FirstName, &message.Author.LastName); err != nil {
			rows.Close()
			return fmt.Errorf("scan global chat last message: %w", err)
		}
		position, exists := index[message.ChatID]
		if exists {
			items[position].LastMessage = &message
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate global chat last messages: %w", err)
	}
	rows.Close()
	for position := range items {
		if items[position].Kind == "group" && items[position].Name != nil {
			items[position].DisplayName = *items[position].Name
			continue
		}
		items[position].DisplayName = "Личный чат"
		for _, member := range items[position].Members {
			if member.UserID != viewerID {
				items[position].DisplayName = strings.TrimSpace(member.FirstName + " " + valueOrEmpty(member.LastName))
				if items[position].DisplayName == "" {
					items[position].DisplayName = "@" + member.Login
				}
				break
			}
		}
	}
	return nil
}

func queryGlobalChatMessage(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, chatID, clientMessageID string) (globalchat.Message, error) {
	var message globalchat.Message
	err := querier.QueryRow(ctx, `
		SELECT message.id::text,message.chat_id::text,message.body,message.created_at,message.edited_at,message.deleted_at,message.version,
		       users.id::text,users.login,profile.first_name,profile.last_name
		FROM global_chat_messages message JOIN users ON users.id=message.author_user_id JOIN profiles profile ON profile.user_id=users.id
		WHERE message.chat_id=$1::uuid AND message.client_message_id=$2::uuid
	`, chatID, clientMessageID).Scan(&message.ID, &message.ChatID, &message.Body, &message.CreatedAt, &message.EditedAt, &message.DeletedAt, &message.Version,
		&message.Author.UserID, &message.Author.Login, &message.Author.FirstName, &message.Author.LastName)
	if err != nil {
		return globalchat.Message{}, fmt.Errorf("load global chat message: %w", err)
	}
	return message, nil
}

func mapGlobalChatWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return globalchat.ErrConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func escapeGlobalChatLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
