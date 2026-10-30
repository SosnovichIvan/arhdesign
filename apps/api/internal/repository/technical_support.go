package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) CreateTechnicalReport(ctx context.Context, report technicalsupport.Report) (bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin technical report: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if report.Kind != "feedback" {
		var existingID string
		err = tx.QueryRow(ctx, `
			SELECT id::text FROM technical_reports
			WHERE kind=$1 AND fingerprint=$2 AND last_seen_at>$3
			ORDER BY last_seen_at DESC,id DESC FOR UPDATE LIMIT 1
		`, report.Kind, report.Fingerprint, report.Now.Add(-15*time.Minute)).Scan(&existingID)
		if err == nil {
			if _, err = tx.Exec(ctx, `UPDATE technical_reports SET occurrence_count=occurrence_count+1,last_seen_at=$2 WHERE id=$1`, existingID, report.Now); err != nil {
				return false, fmt.Errorf("aggregate technical report: %w", err)
			}
			if err = tx.Commit(ctx); err != nil {
				return false, fmt.Errorf("commit technical report aggregation: %w", err)
			}
			return false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("find recent technical report: %w", err)
		}
	}

	var reportID string
	err = tx.QueryRow(ctx, `
		INSERT INTO technical_reports (
			kind,category,actor_user_id,request_id,method,route,status_code,error_code,
			fingerprint,summary,user_message,first_seen_at,last_seen_at,retention_until
		) VALUES ($1,NULLIF($2,''),NULLIF($3,'')::uuid,$4,NULLIF($5,''),$6,NULLIF($7,0),NULLIF($8,''),$9,$10,NULLIF($11,''),$12,$12,$13)
		RETURNING id::text
	`, report.Kind, report.Category, report.ActorUserID, report.RequestID, report.Method, report.Route, report.StatusCode, report.ErrorCode,
		report.Fingerprint, report.Summary, report.UserMessage, report.Now, report.Now.Add(90*24*time.Hour)).Scan(&reportID)
	if err != nil {
		return false, fmt.Errorf("insert technical report: %w", err)
	}
	messageType := "technical." + report.Kind
	if _, err = tx.Exec(ctx, `
		INSERT INTO notification_outbox (
			recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version,metadata,created_at,updated_at
		)
		SELECT users.id,'telegram',$2,$2 || ':' || $1::text || ':' || users.id,$3,$4,
		       jsonb_build_object('reportId',$1::text,'kind',$5::text),$6,$6
		FROM users
		JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		JOIN telegram_subscribers subscriber ON subscriber.user_id=users.id AND subscriber.chat_id=binding.chat_id AND subscriber.unsubscribed_at IS NULL
		WHERE users.status='active' AND users.global_role='technical_admin'
		ON CONFLICT (idempotency_key) DO NOTHING
	`, reportID, messageType, report.PayloadCiphertext, report.PayloadKeyVersion, report.Kind, report.Now); err != nil {
		return false, fmt.Errorf("enqueue technical report: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,metadata,occurred_at)
		VALUES ('technical_report.created','success',$1,NULLIF($2,'')::uuid,'technical_report',jsonb_build_object('reportId',$3::text,'kind',$4::text),$5)
	`, report.RequestID, report.ActorUserID, reportID, report.Kind, report.Now); err != nil {
		return false, fmt.Errorf("audit technical report: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit technical report: %w", err)
	}
	return true, nil
}
