package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/monitoring"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) SaveSample(ctx context.Context, sample monitoring.Sample) (bool, error) {
	metrics, err := json.Marshal(sample.Metrics)
	if err != nil {
		return false, fmt.Errorf("encode monitoring sample: %w", err)
	}
	result, err := repository.pool.Exec(ctx, `
		INSERT INTO monitoring_samples (sample_slot,collected_at,collector_status,release_version,metrics)
		VALUES ($1,$2,$3,NULLIF($4,''),$5)
		ON CONFLICT (sample_slot) DO NOTHING`, sample.Slot, sample.CollectedAt, sample.Status, sample.Release, metrics)
	if err != nil {
		return false, fmt.Errorf("save monitoring sample: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (repository *Postgres) BuildDailyAggregate(ctx context.Context, period monitoring.Period) (monitoring.DailyAggregate, error) {
	aggregate := monitoring.DailyAggregate{Period: period, Metrics: map[string]monitoring.Summary{}}
	aggregate.ExpectedSamples = int(period.End.Sub(period.Start) / (15 * time.Minute))
	var failed int
	err := repository.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM users WHERE created_at < $2),
			(SELECT count(*) FROM audit_events WHERE event_type='account.registration_accepted' AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT count(*) FROM audit_events WHERE event_type='auth.login' AND result='success' AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT count(DISTINCT actor_user_id) FROM audit_events WHERE event_type='auth.login' AND result='success' AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT count(*) FROM audit_events WHERE event_type='auth.login' AND result IN ('invalid_credentials','unverified','disabled','rate_limited') AND occurred_at >= $1 AND occurred_at < $2),
			(SELECT count(*) FROM sessions WHERE created_at < $2 AND (revoked_at IS NULL OR revoked_at >= $2) AND idle_expires_at > $2 AND absolute_expires_at > $2 AND replaced_by_session_id IS NULL),
			(SELECT count(*) FROM monitoring_samples WHERE sample_slot >= $1 AND sample_slot < $2),
			(SELECT count(*) FROM monitoring_samples WHERE sample_slot >= $1 AND sample_slot < $2 AND collector_status <> 'complete')
	`, period.Start, period.End).Scan(&aggregate.AccountsTotal, &aggregate.NewRegistrations, &aggregate.SuccessfulLogins,
		&aggregate.UniqueLoggedIn, &aggregate.FailedLogins, &aggregate.ActiveSessions, &aggregate.ReceivedSamples, &failed)
	if err != nil {
		return monitoring.DailyAggregate{}, fmt.Errorf("aggregate daily monitoring report: %w", err)
	}
	if failed > 0 {
		aggregate.FailedSubsystems = []string{"collector"}
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT metrics FROM monitoring_samples
		 WHERE sample_slot >= $1 AND sample_slot < $2
		 ORDER BY sample_slot`, period.Start, period.End)
	if err != nil {
		return monitoring.DailyAggregate{}, fmt.Errorf("read monitoring metrics: %w", err)
	}
	values := map[string][]float64{}
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			rows.Close()
			return monitoring.DailyAggregate{}, fmt.Errorf("scan monitoring metrics: %w", err)
		}
		var sample map[string]float64
		if err := json.Unmarshal(encoded, &sample); err != nil {
			rows.Close()
			return monitoring.DailyAggregate{}, fmt.Errorf("decode monitoring metrics: %w", err)
		}
		for _, key := range []string{"cpu_busy_percent", "ram_working_set_percent", "disk_used_percent", "io_wait_percent", "backup_age_hours"} {
			if value, ok := sample[key]; ok {
				values[key] = append(values[key], value)
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return monitoring.DailyAggregate{}, fmt.Errorf("iterate monitoring metrics: %w", err)
	}
	rows.Close()
	for key, samples := range values {
		aggregate.Metrics[key] = monitoring.Summarize(samples)
	}
	return aggregate, nil
}

func (repository *Postgres) EnqueueDailyReport(ctx context.Context, period monitoring.Period, aggregate monitoring.DailyAggregate, ciphertext []byte, keyVersion int, now time.Time) (int, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin daily report enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('arhdesign.monitoring.daily'))`); err != nil {
		return 0, fmt.Errorf("lock daily report scheduler: %w", err)
	}
	payload, err := json.Marshal(aggregate)
	if err != nil {
		return 0, fmt.Errorf("encode daily aggregate: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT users.id::text
		  FROM users
		  JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
		 WHERE users.status='active' AND users.global_role='super_admin'
		 ORDER BY users.id`)
	if err != nil {
		return 0, fmt.Errorf("resolve daily report recipients: %w", err)
	}
	var recipients []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		recipients = append(recipients, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	created := 0
	for _, recipient := range recipients {
		var reportID string
		err := tx.QueryRow(ctx, `
			INSERT INTO report_deliveries (report_type,period_start,period_end,recipient_id,aggregate_payload)
			VALUES ('daily_platform',$1,$2,$3,$4)
			ON CONFLICT (report_type,period_start,period_end,recipient_id) DO NOTHING
			RETURNING id::text`, period.Start, period.End, recipient, payload).Scan(&reportID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("insert daily report: %w", err)
		}
		var outboxID string
		key := fmt.Sprintf("monitoring.daily:%s:%s", period.Start.Format("2006-01-02"), recipient)
		if err := tx.QueryRow(ctx, `
			INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version,available_at)
			VALUES ($1,'telegram','monitoring.daily_report',$2,$3,$4,$5)
			RETURNING id::text`, recipient, key, ciphertext, keyVersion, now).Scan(&outboxID); err != nil {
			return 0, fmt.Errorf("insert daily report outbox: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE report_deliveries SET outbox_id=$2 WHERE id=$1`, reportID, outboxID); err != nil {
			return 0, fmt.Errorf("link daily report outbox: %w", err)
		}
		created++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit daily report enqueue: %w", err)
	}
	return created, nil
}

func (repository *Postgres) ApplyAlertObservation(ctx context.Context, observation monitoring.AlertObservation, ciphertext []byte, keyVersion int) (bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin monitoring incident: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id, state string
	var version int64
	emit := false
	err = tx.QueryRow(ctx, `
		SELECT id::text,state,version FROM monitoring_incidents
		 WHERE incident_type=$1 AND resource_key=$2 AND resolved_at IS NULL FOR UPDATE`, observation.Type, observation.Resource).Scan(&id, &state, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		if observation.State != "problem" {
			return false, nil
		}
		state = "pending"
		if observation.ImmediateActivation {
			state, emit = "active", true
		}
		peak, _ := json.Marshal(observation.Peak)
		err = tx.QueryRow(ctx, `
			INSERT INTO monitoring_incidents (incident_type,resource_key,state,first_seen_at,last_seen_at,activated_at,peak)
			VALUES ($1,$2,$3,$4::timestamptz,$4::timestamptz,CASE WHEN $3='active' THEN $4::timestamptz ELSE NULL END,$5)
			RETURNING id::text,version`, observation.Type, observation.Resource, state, observation.At, peak).Scan(&id, &version)
		if err != nil {
			return false, fmt.Errorf("insert monitoring incident: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("lock monitoring incident: %w", err)
	} else {
		peak, _ := json.Marshal(observation.Peak)
		switch {
		case observation.State == "problem" && state == "pending":
			state = "active"
			emit = true
			_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state='active',last_seen_at=$2,activated_at=$2,peak=$3,version=version+1 WHERE id=$1`, id, observation.At, peak)
		case observation.State == "problem":
			if state == "recovering" {
				state = "active"
			}
			_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state=$4,last_seen_at=$2,peak=peak || $3::jsonb,version=version+1 WHERE id=$1`, id, observation.At, peak, state)
		case observation.State == "recovered" && state == "pending":
			state = "resolved"
			_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state='resolved',last_seen_at=$2,resolved_at=$2,version=version+1 WHERE id=$1`, id, observation.At)
		case observation.State == "recovered" && state == "active":
			if observation.ImmediateRecovery {
				state, emit = "resolved", true
				_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state='resolved',last_seen_at=$2,resolved_at=$2,version=version+1 WHERE id=$1`, id, observation.At)
			} else {
				state = "recovering"
				_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state='recovering',last_seen_at=$2,version=version+1 WHERE id=$1`, id, observation.At)
			}
		case observation.State == "recovered" && state == "recovering":
			state, emit = "resolved", true
			_, err = tx.Exec(ctx, `UPDATE monitoring_incidents SET state='resolved',last_seen_at=$2,resolved_at=$2,version=version+1 WHERE id=$1`, id, observation.At)
		}
		if err != nil {
			return false, fmt.Errorf("update monitoring incident: %w", err)
		}
		version++
	}
	if emit {
		transition := state
		rows, err := tx.Query(ctx, `
			SELECT users.id::text FROM users
			JOIN telegram_account_bindings binding ON binding.user_id=users.id AND binding.revoked_at IS NULL
			WHERE users.status='active' AND users.global_role='super_admin' ORDER BY users.id`)
		if err != nil {
			return false, err
		}
		var recipients []string
		for rows.Next() {
			var recipient string
			if err := rows.Scan(&recipient); err != nil {
				rows.Close()
				return false, err
			}
			recipients = append(recipients, recipient)
		}
		rows.Close()
		for _, recipient := range recipients {
			key := fmt.Sprintf("monitoring.incident:%s:%s:%s", id, transition, recipient)
			if _, err := tx.Exec(ctx, `
				INSERT INTO notification_outbox (recipient_user_id,channel,message_type,idempotency_key,payload_ciphertext,payload_key_version,available_at)
				VALUES ($1,'telegram','monitoring.incident',$2,$3,$4,$5) ON CONFLICT (idempotency_key) DO NOTHING`,
				recipient, key, ciphertext, keyVersion, observation.At); err != nil {
				return false, fmt.Errorf("insert incident outbox: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit monitoring incident: %w", err)
	}
	return emit, nil
}
