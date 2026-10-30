//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/monitoring"
)

func TestMonitoringDailyReportIsIdempotentAcrossConcurrentSchedulersAndInactiveRecipient(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_monitoring")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	var recipient string
	if err := pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at)
			VALUES ('daily.admin','daily.admin','daily.admin@example.com','daily.admin@example.com','active','super_admin',$1)
			RETURNING id
		), profile AS (
			INSERT INTO profiles (user_id,professional_role_id,first_name)
			SELECT created.id,role.id,'Admin' FROM created CROSS JOIN professional_roles role WHERE role.code='designer'
		), binding AS (
			INSERT INTO telegram_account_bindings (user_id,chat_id,verified_at) SELECT id,7001,$1 FROM created
		)
		SELECT id::text FROM created`, now).Scan(&recipient); err != nil {
		t.Fatal(err)
	}
	period := monitoring.Period{Start: time.Date(2026, 10, 1, 21, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 2, 21, 0, 0, 0, time.UTC)}
	for index, cpu := range []float64{10, 30} {
		if _, err := store.SaveSample(ctx, monitoring.Sample{Slot: period.Start.Add(time.Duration(index) * 15 * time.Minute), CollectedAt: period.Start, Status: "complete", Metrics: map[string]float64{"cpu_busy_percent": cpu, "ram_working_set_percent": 20 + cpu}}); err != nil {
			t.Fatal(err)
		}
	}
	aggregate, err := store.BuildDailyAggregate(ctx, period)
	if err != nil || aggregate.ReceivedSamples != 2 || aggregate.ExpectedSamples != 96 || aggregate.Metrics["cpu_busy_percent"].P95 != 30 {
		t.Fatalf("aggregate=%#v error=%v", aggregate, err)
	}
	ciphertext := []byte("encrypted-daily-report")
	var wait sync.WaitGroup
	errorsChannel := make(chan error, 8)
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := store.EnqueueDailyReport(ctx, period, aggregate, ciphertext, 1, now)
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	var reports, outbox int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM report_deliveries WHERE report_type='daily_platform'`).Scan(&reports); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_outbox WHERE message_type='monitoring.daily_report'`).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if reports != 1 || outbox != 1 {
		t.Fatalf("reports=%d outbox=%d", reports, outbox)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status='disabled',security_version=security_version+1,version=version+1 WHERE id=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	terminalized, err := store.TerminalizeInactiveTelegramAccountOutbox(ctx, now.Add(time.Minute))
	if err != nil || terminalized != 1 {
		t.Fatalf("terminalized=%d error=%v", terminalized, err)
	}
	var reportState, outboxState string
	if err := pool.QueryRow(ctx, `SELECT report.state,outbox.state FROM report_deliveries report JOIN notification_outbox outbox ON outbox.id=report.outbox_id`).Scan(&reportState, &outboxState); err != nil {
		t.Fatal(err)
	}
	if reportState != "recipient_inactive" || outboxState != "terminal" {
		t.Fatalf("report=%s outbox=%s", reportState, outboxState)
	}
}

func TestMonitoringAlertStateMachineEmitsOnlyTransitions(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_monitoring_alert")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `
		WITH created AS (
			INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at)
			VALUES ('alert.admin','alert.admin','alert.admin@example.com','alert.admin@example.com','active','super_admin',$1) RETURNING id
		), profile AS (
			INSERT INTO profiles (user_id,professional_role_id,first_name)
			SELECT created.id,role.id,'Admin' FROM created CROSS JOIN professional_roles role WHERE role.code='designer'
		)
		INSERT INTO telegram_account_bindings (user_id,chat_id,verified_at) SELECT id,7002,$1 FROM created`, now); err != nil {
		t.Fatal(err)
	}
	problem := monitoring.AlertObservation{Type: "cpu.high", Resource: "application", State: "problem", At: now, Value: 90, Threshold: 85, Peak: map[string]float64{"value": 90}}
	if emitted, err := store.ApplyAlertObservation(ctx, problem, []byte("encrypted-alert"), 1); err != nil || emitted {
		t.Fatalf("first emitted=%v error=%v", emitted, err)
	}
	problem.At = now.Add(15 * time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, problem, []byte("encrypted-alert"), 1); err != nil || !emitted {
		t.Fatalf("second emitted=%v error=%v", emitted, err)
	}
	problem.At = now.Add(30 * time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, problem, []byte("encrypted-alert"), 1); err != nil || emitted {
		t.Fatalf("repeat emitted=%v error=%v", emitted, err)
	}
	recovered := problem
	recovered.State, recovered.Value, recovered.At = "recovered", 70, now.Add(45*time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, recovered, []byte("encrypted-recovery"), 1); err != nil || emitted {
		t.Fatalf("first recovery emitted=%v error=%v", emitted, err)
	}
	recovered.At = now.Add(60 * time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, recovered, []byte("encrypted-recovery"), 1); err != nil || !emitted {
		t.Fatalf("second recovery emitted=%v error=%v", emitted, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_outbox WHERE message_type='monitoring.incident'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("outbox=%d error=%v", count, err)
	}
}

func TestMonitoringAlertImmediateAndPendingRecoveryTransitions(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_monitoring_alert_variants")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `
		WITH created AS (
			INSERT INTO users (login,login_normalized,email,email_normalized,status,global_role,email_verified_at)
			VALUES ('variant.admin','variant.admin','variant.admin@example.com','variant.admin@example.com','active','super_admin',$1) RETURNING id
		), profile AS (
			INSERT INTO profiles (user_id,professional_role_id,first_name)
			SELECT created.id,role.id,'Admin' FROM created CROSS JOIN professional_roles role WHERE role.code='designer'
		)
		INSERT INTO telegram_account_bindings (user_id,chat_id,verified_at) SELECT id,7003,$1 FROM created`, now); err != nil {
		t.Fatal(err)
	}

	pending := monitoring.AlertObservation{Type: "disk.pending", Resource: "application", State: "problem", At: now, Value: 91, Threshold: 85, Peak: map[string]float64{"value": 91}}
	if emitted, err := store.ApplyAlertObservation(ctx, pending, []byte("pending"), 1); err != nil || emitted {
		t.Fatalf("pending incident emitted=%v error=%v", emitted, err)
	}
	pending.State, pending.At = "recovered", now.Add(time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, pending, []byte("pending-recovery"), 1); err != nil || emitted {
		t.Fatalf("pending recovery emitted=%v error=%v", emitted, err)
	}

	immediate := monitoring.AlertObservation{Type: "backup.missing", Resource: "backup", State: "problem", At: now, Value: 100, Threshold: 1, Peak: map[string]float64{"value": 100}, ImmediateActivation: true}
	if emitted, err := store.ApplyAlertObservation(ctx, immediate, []byte("immediate"), 1); err != nil || !emitted {
		t.Fatalf("immediate incident emitted=%v error=%v", emitted, err)
	}
	immediate.State, immediate.At, immediate.ImmediateRecovery = "recovered", now.Add(time.Minute), true
	if emitted, err := store.ApplyAlertObservation(ctx, immediate, []byte("immediate-recovery"), 1); err != nil || !emitted {
		t.Fatalf("immediate recovery emitted=%v error=%v", emitted, err)
	}

	recovering := monitoring.AlertObservation{Type: "memory.flapping", Resource: "application", State: "problem", At: now, Value: 90, Threshold: 85, Peak: map[string]float64{"value": 90}, ImmediateActivation: true}
	if _, err := store.ApplyAlertObservation(ctx, recovering, []byte("flapping-active"), 1); err != nil {
		t.Fatal(err)
	}
	recovering.State, recovering.At, recovering.ImmediateActivation = "recovered", now.Add(time.Minute), false
	if emitted, err := store.ApplyAlertObservation(ctx, recovering, []byte("flapping-recovering"), 1); err != nil || emitted {
		t.Fatalf("recovering incident emitted=%v error=%v", emitted, err)
	}
	recovering.State, recovering.At = "problem", now.Add(2*time.Minute)
	if emitted, err := store.ApplyAlertObservation(ctx, recovering, []byte("flapping-problem"), 1); err != nil || emitted {
		t.Fatalf("flapping incident emitted=%v error=%v", emitted, err)
	}

	sampleSlot := now.Truncate(15 * time.Minute)
	if created, err := store.SaveSample(ctx, monitoring.Sample{Slot: sampleSlot, CollectedAt: now, Status: "partial", Metrics: map[string]float64{"disk_used_percent": 90}}); err != nil || !created {
		t.Fatalf("partial sample created=%v error=%v", created, err)
	}
	if created, err := store.SaveSample(ctx, monitoring.Sample{Slot: sampleSlot, CollectedAt: now, Status: "complete", Metrics: map[string]float64{"disk_used_percent": 10}}); err != nil || created {
		t.Fatalf("duplicate sample created=%v error=%v", created, err)
	}
	aggregate, err := store.BuildDailyAggregate(ctx, monitoring.Period{Start: sampleSlot, End: sampleSlot.Add(15 * time.Minute)})
	if err != nil || len(aggregate.FailedSubsystems) != 1 || aggregate.FailedSubsystems[0] != "collector" || aggregate.Metrics["disk_used_percent"].Maximum != 90 {
		t.Fatalf("partial aggregate=%#v error=%v", aggregate, err)
	}
}
