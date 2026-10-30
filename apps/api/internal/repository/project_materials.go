package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
)

func (repository *Postgres) ListProjectMaterials(ctx context.Context, actor project.Actor, projectID string) ([]project.Material, bool, bool, error) {
	access, err := materialAccess(ctx, repository.pool, actor, projectID, false)
	if err != nil {
		return nil, false, false, err
	}
	if !access.view {
		return nil, false, false, project.ErrNotFound
	}
	rows, err := repository.pool.Query(ctx, `
		SELECT material.id::text,material.project_id::text,material.category,material.name,material.supplier_name,
		       material.contact_info,material.contract_reference,(material.contract_amount*100)::bigint,material.currency_code,
		       material.linked_expense_request_id::text,expense.description,
		       CASE WHEN COALESCE(ledger.amount,0) >= material.contract_amount THEN 'paid'
		            WHEN COALESCE(ledger.amount,0) > 0 THEN 'partial' ELSE 'unpaid' END,
		       material.delivery_status,material.planned_delivery_on,material.actual_delivery_on,material.installation_on,material.notes,
		       CASE WHEN $2::boolean THEN (COALESCE(ledger.amount,0)*100)::bigint ELSE 0 END,
		       CASE WHEN $2::boolean THEN (GREATEST(material.contract_amount-COALESCE(ledger.amount,0),0)*100)::bigint ELSE 0 END,
		       material.created_by_user_id::text,material.created_at,material.updated_at,material.version,$3::boolean,$4::boolean,
		       (SELECT chat.id::text FROM chats chat WHERE chat.project_id=material.project_id AND chat.kind='context' AND chat.context_type='material' AND chat.context_id=material.id AND chat.archived_at IS NULL)
		FROM project_materials material
		LEFT JOIN project_expense_requests expense ON expense.id=material.linked_expense_request_id
		LEFT JOIN project_ledger_operations ledger ON ledger.source_expense_request_id=material.linked_expense_request_id AND ledger.direction='debit'
		WHERE material.project_id=$1 AND material.deleted_at IS NULL
		ORDER BY material.created_at DESC,material.id DESC
		LIMIT 200`, projectID, access.financials, access.update, access.delete)
	if err != nil {
		return nil, false, false, fmt.Errorf("list project materials: %w", err)
	}
	defer rows.Close()
	items := make([]project.Material, 0)
	for rows.Next() {
		item, scanErr := scanProjectMaterial(rows)
		if scanErr != nil {
			return nil, false, false, fmt.Errorf("scan project material: %w", scanErr)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, false, false, fmt.Errorf("iterate project materials: %w", err)
	}
	return items, access.create, access.financials, nil
}

func (repository *Postgres) CreateProjectMaterial(ctx context.Context, command project.CreateMaterialCommand) (project.Material, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.Material{}, fmt.Errorf("begin material creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	access, err := materialAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return project.Material{}, err
	}
	if !access.create {
		return project.Material{}, project.ErrForbidden
	}
	var item project.Material
	err = tx.QueryRow(ctx, `
		INSERT INTO project_materials (project_id,created_by_user_id,category,name,supplier_name,contact_info,contract_reference,
		 contract_amount,currency_code,linked_expense_request_id,payment_status,delivery_status,planned_delivery_on,actual_delivery_on,installation_on,notes,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::numeric/100,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17)
		RETURNING id::text,project_id::text,category,name,supplier_name,contact_info,contract_reference,(contract_amount*100)::bigint,currency_code,
		 linked_expense_request_id::text,NULL::text,payment_status,delivery_status,planned_delivery_on,actual_delivery_on,installation_on,notes,
		 0::bigint,(contract_amount*100)::bigint,created_by_user_id::text,created_at,updated_at,version,true,true,NULL::text`,
		command.ProjectID, command.UserID, command.Input.Category, command.Input.Name, command.Input.SupplierName, command.Input.ContactInfo,
		command.Input.ContractReference, command.Input.ContractAmountMinor, access.currency, command.Input.LinkedExpenseID, command.Input.PaymentStatus,
		command.Input.DeliveryStatus, command.Input.PlannedDeliveryOn, command.Input.ActualDeliveryOn, command.Input.InstallationOn, command.Input.Notes, command.Now,
	).Scan(materialScanTargets(&item)...)
	if err != nil {
		return project.Material{}, mapProjectWriteError("insert project material", err)
	}
	if err = recordMaterialVersion(ctx, tx, item.ID, command.ProjectID, item.Version, command.UserID, "created", command.Now); err != nil {
		return project.Material{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('material.created','success',$1,$2,'project_material',$3,jsonb_build_object('materialId',$4::text),$5)`, command.RequestID, command.UserID, command.ProjectID, item.ID, command.Now); err != nil {
		return project.Material{}, fmt.Errorf("audit material creation: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.UserID, item.ID, item.ID, "", notificationScopeMaterials, command.Notification, command.Now); err != nil {
		return project.Material{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.Material{}, mapProjectWriteError("commit material creation", err)
	}
	return item, nil
}

func (repository *Postgres) UpdateProjectMaterial(ctx context.Context, command project.UpdateMaterialCommand) (project.Material, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return project.Material{}, fmt.Errorf("begin material update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	access, err := materialAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return project.Material{}, err
	}
	if !access.update {
		return project.Material{}, project.ErrForbidden
	}
	var item project.Material
	err = tx.QueryRow(ctx, `
		UPDATE project_materials SET category=$4,name=$5,supplier_name=$6,contact_info=$7,contract_reference=$8,
		 contract_amount=$9::numeric/100,linked_expense_request_id=$10,payment_status=$11,delivery_status=$12,
		 planned_delivery_on=$13,actual_delivery_on=$14,installation_on=$15,notes=$16,updated_at=$17,version=version+1
		WHERE project_id=$1 AND id=$2 AND version=$3 AND deleted_at IS NULL
		RETURNING id::text,project_id::text,category,name,supplier_name,contact_info,contract_reference,(contract_amount*100)::bigint,currency_code,
		 linked_expense_request_id::text,NULL::text,payment_status,delivery_status,planned_delivery_on,actual_delivery_on,installation_on,notes,
		 0::bigint,(contract_amount*100)::bigint,created_by_user_id::text,created_at,updated_at,version,true,true,NULL::text`,
		command.ProjectID, command.MaterialID, command.ExpectedVersion, command.Input.Category, command.Input.Name, command.Input.SupplierName,
		command.Input.ContactInfo, command.Input.ContractReference, command.Input.ContractAmountMinor, command.Input.LinkedExpenseID,
		command.Input.PaymentStatus, command.Input.DeliveryStatus, command.Input.PlannedDeliveryOn, command.Input.ActualDeliveryOn,
		command.Input.InstallationOn, command.Input.Notes, command.Now,
	).Scan(materialScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return project.Material{}, materialConflictOrNotFound(ctx, tx, command.ProjectID, command.MaterialID)
	}
	if err != nil {
		return project.Material{}, mapProjectWriteError("update project material", err)
	}
	if err = recordMaterialVersion(ctx, tx, item.ID, command.ProjectID, item.Version, command.UserID, "updated", command.Now); err != nil {
		return project.Material{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('material.updated','success',$1,$2,'project_material',$3,jsonb_build_object('materialId',$4::text,'version',$5::bigint),$6)`, command.RequestID, command.UserID, command.ProjectID, item.ID, item.Version, command.Now); err != nil {
		return project.Material{}, fmt.Errorf("audit material update: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.UserID, fmt.Sprintf("%s:%d", item.ID, item.Version), item.ID, "", notificationScopeMaterials, command.Notification, command.Now); err != nil {
		return project.Material{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return project.Material{}, mapProjectWriteError("commit material update", err)
	}
	return item, nil
}

func (repository *Postgres) DeleteProjectMaterial(ctx context.Context, command project.DeleteMaterialCommand) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin material deletion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	access, err := materialAccess(ctx, tx, command.Actor, command.ProjectID, true)
	if err != nil {
		return err
	}
	if !access.delete {
		return project.ErrForbidden
	}
	var nextVersion int64
	err = tx.QueryRow(ctx, `UPDATE project_materials SET deleted_at=$5,deleted_by_user_id=$4,delete_reason=$6,updated_at=$5,version=version+1 WHERE project_id=$1 AND id=$2 AND version=$3 AND deleted_at IS NULL RETURNING version`, command.ProjectID, command.MaterialID, command.ExpectedVersion, command.UserID, command.Now, command.Reason).Scan(&nextVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return materialConflictOrNotFound(ctx, tx, command.ProjectID, command.MaterialID)
	}
	if err != nil {
		return mapProjectWriteError("delete project material", err)
	}
	if err = recordMaterialVersion(ctx, tx, command.MaterialID, command.ProjectID, nextVersion, command.UserID, "deleted", command.Now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events (event_type,result,request_id,actor_user_id,subject_type,project_id,metadata,occurred_at) VALUES ('material.deleted','success',$1,$2,'project_material',$3,jsonb_build_object('materialId',$4::text,'reason',$5::text),$6)`, command.RequestID, command.UserID, command.ProjectID, command.MaterialID, command.Reason, command.Now); err != nil {
		return fmt.Errorf("audit material deletion: %w", err)
	}
	if err = enqueueScopedProjectNotification(ctx, tx, command.ProjectID, command.UserID, fmt.Sprintf("%s:%d", command.MaterialID, nextVersion), command.MaterialID, "", notificationScopeMaterials, command.Notification, command.Now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type materialPermissions struct {
	currency                                 string
	view, create, update, delete, financials bool
}

func materialAccess(ctx context.Context, querier projectAccessQuerier, actor project.Actor, projectID string, lock bool) (materialPermissions, error) {
	query := `SELECT project.currency_code,
	 $3::boolean OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=project.id AND m.user_id=$2::uuid AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges ?| ARRAY['materials.view','materials.create','materials.update','materials.delete'])),
	 $3::boolean OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=project.id AND m.user_id=$2::uuid AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges @> '{"materials.create":true}'::jsonb)),
	 $3::boolean OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=project.id AND m.user_id=$2::uuid AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges @> '{"materials.update":true}'::jsonb)),
	 $3::boolean OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=project.id AND m.user_id=$2::uuid AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges @> '{"materials.delete":true}'::jsonb)),
	 $3::boolean OR EXISTS (SELECT 1 FROM project_memberships m WHERE m.project_id=project.id AND m.user_id=$2::uuid AND m.revoked_at IS NULL AND (m.project_role IN ('customer','project_admin') OR m.privileges @> '{"financials.view":true}'::jsonb))
	 FROM projects project WHERE project.id=$1 AND project.archived_at IS NULL`
	if lock {
		query += " FOR UPDATE"
	}
	var value materialPermissions
	if err := querier.QueryRow(ctx, query, projectID, actor.UserID, actor.SuperAdmin).Scan(&value.currency, &value.view, &value.create, &value.update, &value.delete, &value.financials); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return value, project.ErrNotFound
		}
		return value, fmt.Errorf("authorize material access: %w", err)
	}
	return value, nil
}

func recordMaterialVersion(ctx context.Context, tx pgx.Tx, materialID, projectID string, version int64, userID, changeType string, now any) error {
	_, err := tx.Exec(ctx, `INSERT INTO project_material_versions (material_id,project_id,version,changed_by_user_id,change_type,snapshot,changed_at) SELECT id,project_id,version,$4,$5,to_jsonb(project_materials),$6 FROM project_materials WHERE id=$1 AND project_id=$2 AND version=$3`, materialID, projectID, version, userID, changeType, now)
	if err != nil {
		return fmt.Errorf("record material version: %w", err)
	}
	return nil
}

func materialConflictOrNotFound(ctx context.Context, querier projectAccessQuerier, projectID, materialID string) error {
	var exists bool
	if err := querier.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_materials WHERE project_id=$1 AND id=$2)`, projectID, materialID).Scan(&exists); err != nil {
		return fmt.Errorf("check material conflict: %w", err)
	}
	if exists {
		return project.ErrConflict
	}
	return project.ErrNotFound
}

type projectMaterialScanner interface{ Scan(...any) error }

func scanProjectMaterial(row projectMaterialScanner) (project.Material, error) {
	var item project.Material
	err := row.Scan(materialScanTargets(&item)...)
	return item, err
}

func materialScanTargets(item *project.Material) []any {
	return []any{&item.ID, &item.ProjectID, &item.Category, &item.Name, &item.SupplierName, &item.ContactInfo, &item.ContractReference, &item.ContractAmountMinor, &item.CurrencyCode, &item.LinkedExpenseID, &item.LinkedExpenseDescription, &item.PaymentStatus, &item.DeliveryStatus, &item.PlannedDeliveryOn, &item.ActualDeliveryOn, &item.InstallationOn, &item.Notes, &item.PaidAmountMinor, &item.RemainingAmountMinor, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.Version, &item.CanEdit, &item.CanDelete, &item.ContextChatID}
}
