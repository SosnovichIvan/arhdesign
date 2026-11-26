package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/chatimage"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) beginChatImageAccess(ctx context.Context, access chatimage.Access) (pgx.Tx, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin chat image access: %w", err)
	}
	var managesChat bool
	if access.ProjectID == "" {
		err = tx.QueryRow(ctx, `
			SELECT chat.created_by_user_id=$2
			FROM global_chats chat
			JOIN global_chat_members member ON member.chat_id=chat.id AND member.user_id=$2 AND member.left_at IS NULL
			WHERE chat.id=$1 AND chat.archived_at IS NULL FOR SHARE OF chat,member
		`, access.ChatID, access.UserID).Scan(&managesChat)
	} else {
		err = tx.QueryRow(ctx, `
			SELECT $4 OR chat.created_by_user_id=$2 OR EXISTS (
				SELECT 1 FROM project_memberships pm
				WHERE pm.project_id=$3 AND pm.user_id=$2 AND pm.project_role='project_admin' AND pm.revoked_at IS NULL
			)
			FROM chats chat
			JOIN projects ON projects.id=chat.project_id AND projects.archived_at IS NULL
			WHERE chat.id=$1 AND chat.project_id=$3 AND chat.archived_at IS NULL
			  AND ($4 OR (
				EXISTS (SELECT 1 FROM project_memberships pm WHERE pm.project_id=$3 AND pm.user_id=$2 AND pm.revoked_at IS NULL)
				AND EXISTS (SELECT 1 FROM chat_memberships member WHERE member.chat_id=chat.id AND member.user_id=$2 AND member.removed_at IS NULL FOR SHARE)
			  ))
			FOR SHARE OF projects,chat
		`, access.ChatID, access.UserID, access.ProjectID, access.SuperAdmin).Scan(&managesChat)
		if err == nil {
			_, _, err = authorizeContextChat(ctx, tx, project.Actor{UserID: access.UserID, SuperAdmin: access.SuperAdmin}, access.ProjectID, access.ChatID)
		}
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, project.ErrNotFound) || errors.Is(err, project.ErrForbidden) {
			return nil, false, chatimage.ErrNotFound
		}
		return nil, false, fmt.Errorf("authorize chat image access: %w", err)
	}
	return tx, managesChat, nil
}

func (repository *Postgres) ListChatImages(ctx context.Context, access chatimage.Access, cursor *chatimage.Cursor) ([]chatimage.Image, error) {
	tx, managesChat, err := repository.beginChatImageAccess(ctx, access)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before *time.Time
	var id *string
	var pinned bool
	if cursor != nil {
		before, id, pinned = &cursor.CreatedAt, &cursor.ID, cursor.Pinned
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text,name,media_type,size_bytes,width,height,created_at,pinned,($4 OR uploader_user_id=$3)
		FROM chat_images
		WHERE scope=$1 AND chat_id=$2 AND deleted_at IS NULL
		  AND ($5::timestamptz IS NULL OR (pinned,created_at,id)<($6,$5,$7::uuid))
		ORDER BY pinned DESC,created_at DESC,id DESC LIMIT $8
	`, access.Scope(), access.ChatID, access.UserID, managesChat, before, pinned, id, chatimage.PageSize+1)
	if err != nil {
		return nil, fmt.Errorf("list chat images: %w", err)
	}
	defer rows.Close()
	items := make([]chatimage.Image, 0, chatimage.PageSize+1)
	for rows.Next() {
		item, err := scanChatImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chat image: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat images: %w", err)
	}
	return items, nil
}

func (repository *Postgres) StoreChatImage(ctx context.Context, access chatimage.Access, upload chatimage.Upload) (chatimage.Image, error) {
	tx, _, err := repository.beginChatImageAccess(ctx, access)
	if err != nil {
		return chatimage.Image{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	value, err := scanChatImage(tx.QueryRow(ctx, `
		INSERT INTO chat_images(id,scope,chat_id,uploader_user_id,name,media_type,size_bytes,width,height,content_hash,content)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO NOTHING
		RETURNING id::text,name,media_type,size_bytes,width,height,created_at,pinned,true
	`, upload.ID, access.Scope(), access.ChatID, access.UserID, upload.Name, upload.MediaType, upload.SizeBytes,
		upload.Width, upload.Height, upload.Hash[:], upload.Content))
	if errors.Is(err, pgx.ErrNoRows) {
		value, err = scanChatImage(tx.QueryRow(ctx, `
			SELECT id::text,name,media_type,size_bytes,width,height,created_at,pinned,true
			FROM chat_images
			WHERE id=$1 AND scope=$2 AND chat_id=$3 AND uploader_user_id=$4
			  AND name=$5 AND content_hash=$6 AND deleted_at IS NULL FOR SHARE
		`, upload.ID, access.Scope(), access.ChatID, access.UserID, upload.Name, upload.Hash[:]))
		if errors.Is(err, pgx.ErrNoRows) {
			return chatimage.Image{}, chatimage.ErrConflict
		}
	}
	if err != nil {
		return chatimage.Image{}, fmt.Errorf("store chat image: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return chatimage.Image{}, fmt.Errorf("commit chat image upload: %w", err)
	}
	return value, nil
}

func (repository *Postgres) ViewChatImage(ctx context.Context, access chatimage.Access, id string) (chatimage.Content, error) {
	tx, _, err := repository.beginChatImageAccess(ctx, access)
	if err != nil {
		return chatimage.Content{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var content chatimage.Content
	err = tx.QueryRow(ctx, `
		SELECT media_type,content FROM chat_images
		WHERE id=$1 AND scope=$2 AND chat_id=$3 AND deleted_at IS NULL
	`, id, access.Scope(), access.ChatID).Scan(&content.MediaType, &content.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return content, chatimage.ErrNotFound
	}
	if err != nil {
		return content, fmt.Errorf("load chat image bytes: %w", err)
	}
	return content, nil
}

func (repository *Postgres) SetChatImagePin(ctx context.Context, access chatimage.Access, id string, pinned bool) error {
	tx, _, err := repository.beginChatImageAccess(ctx, access)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE chat_images SET pinned=$4 WHERE id=$1 AND scope=$2 AND chat_id=$3 AND deleted_at IS NULL
	`, id, access.Scope(), access.ChatID, pinned)
	if err != nil {
		return fmt.Errorf("set chat image pin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return chatimage.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (repository *Postgres) DeleteChatImage(ctx context.Context, access chatimage.Access, id string) error {
	tx, managesChat, err := repository.beginChatImageAccess(ctx, access)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var allowed bool
	err = tx.QueryRow(ctx, `
		SELECT $5 OR uploader_user_id=$4 FROM chat_images
		WHERE id=$1 AND scope=$2 AND chat_id=$3 AND deleted_at IS NULL FOR UPDATE
	`, id, access.Scope(), access.ChatID, access.UserID, managesChat).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return chatimage.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("authorize chat image deletion: %w", err)
	}
	if !allowed {
		return chatimage.ErrForbidden
	}
	if _, err = tx.Exec(ctx, `UPDATE chat_images SET deleted_at=now(),content=NULL,pinned=false WHERE id=$1`, id); err != nil {
		return fmt.Errorf("delete chat image bytes: %w", err)
	}
	return tx.Commit(ctx)
}

func scanChatImage(row pgx.Row) (chatimage.Image, error) {
	var value chatimage.Image
	err := row.Scan(&value.ID, &value.Name, &value.MediaType, &value.SizeBytes, &value.Width, &value.Height, &value.CreatedAt, &value.Pinned, &value.CanDelete)
	return value, err
}
