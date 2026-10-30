package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) GetProjectFinanceSummary(ctx context.Context, actor project.Actor, projectID string) (project.FinanceSummary, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return project.FinanceSummary{}, err
	}
	canCreate, err := canCreateProjectExpense(ctx, repository.pool, actor, projectID)
	if err != nil {
		return project.FinanceSummary{}, err
	}
	var summary project.FinanceSummary
	summary.CanCreateExpense = canCreate
	err = repository.pool.QueryRow(ctx, `
		SELECT project.currency_code,
		       COALESCE(sum(operation.amount * 100) FILTER (WHERE operation.direction='credit'),0)::bigint,
		       COALESCE(sum(operation.amount * 100) FILTER (WHERE operation.direction='debit'),0)::bigint,
		       COALESCE(sum(operation.amount * 100) FILTER (WHERE operation.direction='credit'),0)::bigint
		         - COALESCE(sum(operation.amount * 100) FILTER (WHERE operation.direction='debit'),0)::bigint,
		       COALESCE((SELECT sum(expense.amount * 100)::bigint FROM project_expense_requests expense
		                 WHERE expense.project_id=project.id AND expense.status IN ('pending_approval','auto_approved','approved','awaiting_payment')),0)::bigint
		FROM projects project
		LEFT JOIN project_ledger_operations operation ON operation.project_id=project.id
		WHERE project.id=$1 AND project.archived_at IS NULL
		GROUP BY project.id
	`, projectID).Scan(&summary.CurrencyCode, &summary.ConfirmedIncomeMinor, &summary.ConfirmedExpenseMinor, &summary.AvailableBalanceMinor, &summary.PendingExpenseMinor)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.FinanceSummary{}, project.ErrNotFound
	}
	if err != nil {
		return project.FinanceSummary{}, fmt.Errorf("get project finance summary: %w", err)
	}
	return summary, nil
}

func (repository *Postgres) ListProjectExpenses(ctx context.Context, actor project.Actor, projectID string) ([]project.Expense, bool, error) {
	if err := requireProjectAccess(ctx, repository.pool, actor, projectID, false); err != nil {
		return nil, false, err
	}
	canCreate, err := canCreateProjectExpense(ctx, repository.pool, actor, projectID)
	if err != nil {
		return nil, false, err
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT expense.id::text,expense.project_id::text,(expense.amount * 100)::bigint,expense.currency_code,expense.category,expense.description,
		       expense.vendor_name,expense.planned_payment_on,expense.status,expense.created_by_user_id::text,expense.created_at,expense.version,
		       (SELECT chat.id::text FROM chats chat WHERE chat.project_id=expense.project_id AND chat.kind='context' AND chat.context_type='expense' AND chat.context_id=expense.id AND chat.archived_at IS NULL)
		FROM project_expense_requests expense
		WHERE expense.project_id=$1
		ORDER BY expense.created_at DESC,expense.id DESC
		LIMIT 100
	`, projectID)
	if err != nil {
		return nil, false, fmt.Errorf("list project expenses: %w", err)
	}
	defer rows.Close()
	items := make([]project.Expense, 0)
	for rows.Next() {
		item, scanErr := scanProjectExpense(rows)
		if scanErr != nil {
			return nil, false, fmt.Errorf("scan project expense: %w", scanErr)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate project expenses: %w", err)
	}
	return items, canCreate, nil
}

func (repository *Postgres) CreateProjectExpense(ctx context.Context, command project.CreateExpenseCommand) (project.Expense, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.Expense{}, fmt.Errorf("begin project expense creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	currency, autoApprove, canCreate, err := projectExpenseAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return project.Expense{}, err
	}
	if !canCreate {
		return project.Expense{}, project.ErrForbidden
	}
	status := "pending_approval"
	if autoApprove {
		status = "auto_approved"
	}
	var item project.Expense
	err = tx.QueryRow(ctx, `
		INSERT INTO project_expense_requests
			(project_id,created_by_user_id,amount,currency_code,category,description,vendor_name,planned_payment_on,status,idempotency_key,request_hash,created_at,updated_at)
		VALUES ($1,$2,$3::numeric / 100,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
		ON CONFLICT (project_id,created_by_user_id,idempotency_key) DO NOTHING
		RETURNING id::text,project_id::text,(amount * 100)::bigint,currency_code,category,description,
		          vendor_name,planned_payment_on,status,created_by_user_id::text,created_at,version,NULL::text
	`, command.ProjectID, command.UserID, command.Request.AmountMinor, currency, command.Request.Category, command.Request.Description, command.Request.VendorName, command.Request.PlannedPaymentOn, status, command.Request.IdempotencyKey, command.RequestHash[:], command.Now).Scan(
		&item.ID, &item.ProjectID, &item.AmountMinor, &item.CurrencyCode, &item.Category, &item.Description, &item.VendorName, &item.PlannedPaymentOn, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.Version, &item.ContextChatID,
	)
	inserted := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		err = tx.QueryRow(ctx, `
			SELECT id::text,project_id::text,(amount * 100)::bigint,currency_code,category,description,
			       vendor_name,planned_payment_on,status,created_by_user_id::text,created_at,version,NULL::text,request_hash
			FROM project_expense_requests
			WHERE project_id=$1 AND created_by_user_id=$2 AND idempotency_key=$3
		`, command.ProjectID, command.UserID, command.Request.IdempotencyKey).Scan(
			&item.ID, &item.ProjectID, &item.AmountMinor, &item.CurrencyCode, &item.Category, &item.Description, &item.VendorName, &item.PlannedPaymentOn, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.Version, &item.ContextChatID, &storedHash,
		)
		if err == nil && !bytes.Equal(storedHash, command.RequestHash[:]) {
			return project.Expense{}, project.ErrConflict
		}
	}
	if err != nil {
		return project.Expense{}, mapProjectWriteError("insert project expense", err)
	}
	if inserted {
		if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('expense.created','success',$1,$2,'project_expense',$3,jsonb_build_object('expenseId',$4::text,'status',$5::text,'amountMinor',$6::bigint),$7)`, command.RequestID, command.UserID, command.ProjectID, item.ID, item.Status, item.AmountMinor, command.Now); err != nil {
			return project.Expense{}, fmt.Errorf("audit project expense creation: %w", err)
		}
		if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.UserID, item.ID, item.ID, "", notificationScopeFinancials, command.Notification, command.Now); err != nil {
			return project.Expense{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return project.Expense{}, mapProjectWriteError("commit project expense creation", err)
	}
	return item, nil
}

func canCreateProjectExpense(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID string) (bool, error) {
	_, _, allowed, err := projectExpenseAccess(ctx, querier, actor, projectID, false)
	return allowed, err
}

func projectExpenseAccess(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID string, lock bool) (string, bool, bool, error) {
	query := `
		SELECT project.currency_code,project.auto_approve_expenses,
		       $3 OR EXISTS (
			 SELECT 1 FROM project_memberships membership
			 WHERE membership.project_id=project.id AND membership.user_id=$2 AND membership.revoked_at IS NULL
			   AND (membership.project_role='project_admin' OR membership.privileges @> '{"financials.create":true}'::jsonb)
		       ),
		       $3 OR EXISTS (
			 SELECT 1 FROM project_memberships membership
			 WHERE membership.project_id=project.id AND membership.user_id=$2 AND membership.revoked_at IS NULL
		       )
		FROM projects project
		WHERE project.id=$1 AND project.archived_at IS NULL`
	if lock {
		query += " FOR UPDATE"
	}
	var currency string
	var autoApprove, canCreate, canView bool
	if err := querier.QueryRow(ctx, query, projectID, actor.UserID, actor.SuperAdmin).Scan(&currency, &autoApprove, &canCreate, &canView); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, false, project.ErrNotFound
		}
		return "", false, false, fmt.Errorf("authorize project expense access: %w", err)
	}
	if !canView {
		return "", false, false, project.ErrNotFound
	}
	return currency, autoApprove, canCreate, nil
}

type projectExpenseScanner interface {
	Scan(...any) error
}

func scanProjectExpense(row projectExpenseScanner) (project.Expense, error) {
	var item project.Expense
	err := row.Scan(&item.ID, &item.ProjectID, &item.AmountMinor, &item.CurrencyCode, &item.Category, &item.Description, &item.VendorName, &item.PlannedPaymentOn, &item.Status, &item.CreatedByUserID, &item.CreatedAt, &item.Version, &item.ContextChatID)
	return item, err
}
