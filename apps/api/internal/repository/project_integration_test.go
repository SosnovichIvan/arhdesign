//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectCreationVisibilityAndLateCustomerAssignmentIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_projects")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := project.NewService(store, []byte("project-integration-cursor-key-32"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{9}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}
	user1 := insertProjectTestUser(t, pool, "customer.one", "customer.one@example.com")
	user2 := insertProjectTestUser(t, pool, "customer.two", "customer.two@example.com")
	user3 := insertProjectTestUser(t, pool, "designer.one", "designer.one@example.com")
	notificationAdmin := insertProjectTestUser(t, pool, "notification.admin", "notification.admin@example.com")
	if _, err = pool.Exec(context.Background(), `UPDATE users SET global_role='super_admin' WHERE id=$1`, notificationAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO telegram_subscribers (chat_id,username,user_id,verified_at) VALUES (9901,'notification_admin',$1,$2)`, notificationAdmin, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO telegram_account_bindings (user_id,chat_id,chat_username,verified_at) VALUES ($1,9901,'notification_admin',$2)`, notificationAdmin, now); err != nil {
		t.Fatal(err)
	}
	accessHash := []byte("project-access-hash")
	if _, err = pool.Exec(context.Background(), `
		INSERT INTO sessions (user_id,family_id,access_token_hash,refresh_token_hash,csrf_token_hash,hash_key_version,captured_security_version,created_at,last_seen_at,idle_expires_at,absolute_expires_at,remember_me)
		VALUES ($1,gen_random_uuid(),$2,$3,$4,1,1,$5,$5,$6,$6,false)
	`, user1, accessHash, []byte("project-refresh-hash"), []byte("project-csrf-hash"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	resolved, found, err := store.ResolveAccessSession(context.Background(), accessHash, now)
	if err != nil || !found || resolved.ID != user1 || resolved.Login != "customer.one" {
		t.Fatalf("resolved=%#v found=%v error=%v", resolved, found, err)
	}
	if _, found, err = store.ResolveAccessSession(context.Background(), []byte("missing"), now); err != nil || found {
		t.Fatalf("missing session found=%v error=%v", found, err)
	}
	if _, found, err = store.ResolveAccessSession(context.Background(), accessHash, now.Add(2*time.Hour)); err != nil || found {
		t.Fatalf("expired session found=%v error=%v", found, err)
	}

	lastName := "Один"
	ordinary, err := service.Create(context.Background(), project.Actor{UserID: user1, Login: "customer.one", FirstName: "Заказчик", LastName: &lastName}, project.CreateRequest{Name: "Полянка", Type: "interior_design", CurrencyCode: "RUB", RequestID: "request-ordinary", AutoApproveExpenses: true})
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.CustomerUserID == nil || *ordinary.CustomerUserID != user1 || len(ordinary.AdminUserIDs) != 1 || ordinary.AdminUserIDs[0] != user1 {
		t.Fatalf("ordinary project=%#v", ordinary)
	}
	var notificationCiphertext []byte
	var notificationKeyVersion int
	if err = pool.QueryRow(context.Background(), `SELECT payload_ciphertext,payload_key_version FROM notification_outbox WHERE recipient_user_id=$1 AND message_type='project.created'`, notificationAdmin).Scan(&notificationCiphertext, &notificationKeyVersion); err != nil {
		t.Fatal(err)
	}
	notificationPayload, err := cipher.Decrypt(notificationCiphertext, "project.created", notificationKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	var telegramMessage struct {
		Text string `json:"text"`
	}
	if err = json.Unmarshal(notificationPayload, &telegramMessage); err != nil || !strings.Contains(telegramMessage.Text, "Заказчик Один (@customer.one)") || !strings.Contains(telegramMessage.Text, "Полянка") {
		t.Fatalf("telegram notification=%q error=%v", telegramMessage.Text, err)
	}
	var customerRoles, adminRoles int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE project_role='customer'), count(*) FILTER (WHERE project_role='project_admin') FROM project_memberships WHERE project_id=$1 AND revoked_at IS NULL`, ordinary.ID).Scan(&customerRoles, &adminRoles); err != nil || customerRoles != 1 || adminRoles != 1 {
		t.Fatalf("customer roles=%d admin roles=%d error=%v", customerRoles, adminRoles, err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO project_memberships (project_id,user_id,project_role) VALUES ($1,$2,'unknown_role')`, ordinary.ID, user3); err == nil {
		t.Fatal("unknown project role was accepted")
	}
	visible, err := service.List(context.Background(), project.ListRequest{Actor: project.Actor{UserID: user1}})
	if err != nil || len(visible.Items) != 1 {
		t.Fatalf("creator page=%#v error=%v", visible, err)
	}
	hidden, err := service.List(context.Background(), project.ListRequest{Actor: project.Actor{UserID: user2}})
	if err != nil || len(hidden.Items) != 0 {
		t.Fatalf("foreign page=%#v error=%v", hidden, err)
	}
	if _, err = service.Get(context.Background(), project.Actor{UserID: user2}, ordinary.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign get error=%v", err)
	}
	updatedName := "Полянка — обновлено"
	updatedStatus := "active"
	updated, err := service.Update(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-update", ordinary.Version, project.UpdateRequest{Name: &updatedName, Status: &updatedStatus})
	if err != nil || updated.Name != updatedName || updated.Status != updatedStatus || updated.Version != ordinary.Version+1 {
		t.Fatalf("updated project=%#v error=%v", updated, err)
	}
	updatedType, updatedAddress, updatedDescription, autoApprove := "architecture", "Москва", "Полное описание", false
	start, finish := now.AddDate(0, 0, 1), now.AddDate(0, 1, 0)
	updated, err = service.Update(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-full-update", updated.Version, project.UpdateRequest{
		Type: &updatedType, Address: project.Optional[*string]{Set: true, Value: &updatedAddress}, Description: project.Optional[*string]{Set: true, Value: &updatedDescription},
		PlannedStartOn: project.Optional[*time.Time]{Set: true, Value: &start}, PlannedFinishOn: project.Optional[*time.Time]{Set: true, Value: &finish}, AutoApproveExpenses: &autoApprove,
	})
	if err != nil || updated.Type != updatedType || updated.Address == nil || *updated.Address != updatedAddress || updated.Description == nil || updated.PlannedStartOn == nil || updated.PlannedStartOn.Format(time.DateOnly) != start.Format(time.DateOnly) || updated.AutoApproveExpenses {
		t.Fatalf("full update=%#v error=%v", updated, err)
	}
	earlyFinish := start.AddDate(0, 0, -1)
	if _, err = service.Update(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-invalid-merged-date", updated.Version, project.UpdateRequest{PlannedFinishOn: project.Optional[*time.Time]{Set: true, Value: &earlyFinish}}); !errors.Is(err, project.ErrInvalidInput) {
		t.Fatalf("merged date validation error=%v", err)
	}
	if _, err = service.Update(context.Background(), project.Actor{UserID: user2}, ordinary.ID, "request-foreign-update", updated.Version, project.UpdateRequest{Name: &updatedName}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign update error=%v", err)
	}
	if _, err = service.Update(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-stale-update", ordinary.Version, project.UpdateRequest{Name: &updatedName}); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale update error=%v", err)
	}

	members, err := service.ListMembers(context.Background(), project.Actor{UserID: user1}, ordinary.ID)
	if err != nil || !members.CanManage || len(members.Items) != 1 || len(members.Items[0].ProjectRoles) != 2 || members.Items[0].Removable {
		t.Fatalf("initial members=%#v error=%v", members, err)
	}
	candidates, err := service.SearchMemberCandidates(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "designer")
	if err != nil || len(candidates) != 1 || candidates[0].UserID != user3 {
		t.Fatalf("member candidates=%#v error=%v", candidates, err)
	}
	addedMember, err := service.AddMember(context.Background(), project.Actor{UserID: user1}, ordinary.ID, user3, "request-member-add")
	if err != nil || addedMember.UserID != user3 || len(addedMember.ProjectRoles) != 1 || addedMember.ProjectRoles[0] != "executor" || !addedMember.Removable {
		t.Fatalf("added member=%#v error=%v", addedMember, err)
	}
	if _, err = service.AddMember(context.Background(), project.Actor{UserID: user1}, ordinary.ID, user3, "request-member-repeat"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("duplicate member error=%v", err)
	}
	if _, err = service.SearchMemberCandidates(context.Background(), project.Actor{UserID: user3}, ordinary.ID, "customer"); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("executor candidate search error=%v", err)
	}
	if _, err = service.Get(context.Background(), project.Actor{UserID: user3}, ordinary.ID); err != nil {
		t.Fatalf("added member project visibility error=%v", err)
	}
	if err = service.RemoveMember(context.Background(), project.Actor{UserID: user1}, ordinary.ID, user1, "request-member-protected"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("customer removal error=%v", err)
	}
	if err = service.RemoveMember(context.Background(), project.Actor{UserID: user1}, ordinary.ID, user3, "request-member-remove"); err != nil {
		t.Fatalf("remove member error=%v", err)
	}
	if _, err = service.Get(context.Background(), project.Actor{UserID: user3}, ordinary.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked member visibility error=%v", err)
	}

	finance, err := service.FinanceSummary(context.Background(), project.Actor{UserID: user1}, ordinary.ID)
	if err != nil || !finance.CanCreateExpense || finance.CurrencyCode != "RUB" || finance.PendingExpenseMinor != 0 {
		t.Fatalf("initial finance=%#v error=%v", finance, err)
	}
	vendor := "Свет и форма"
	plannedPayment := now.AddDate(0, 0, 5)
	expenseRequest := project.CreateExpenseRequest{AmountMinor: 125_050, Category: "materials", Description: "Светильники", VendorName: &vendor, PlannedPaymentOn: &plannedPayment, IdempotencyKey: "expense-key-ordinary"}
	expense, err := service.CreateExpense(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-expense-create", expenseRequest)
	if err != nil || expense.AmountMinor != 125_050 || expense.Status != "pending_approval" || expense.CurrencyCode != "RUB" {
		t.Fatalf("expense=%#v error=%v", expense, err)
	}
	replayed, err := service.CreateExpense(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-expense-replay", expenseRequest)
	if err != nil || replayed.ID != expense.ID {
		t.Fatalf("replayed expense=%#v error=%v", replayed, err)
	}
	expenseRequest.AmountMinor++
	if _, err = service.CreateExpense(context.Background(), project.Actor{UserID: user1}, ordinary.ID, "request-expense-conflict", expenseRequest); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("expense idempotency conflict error=%v", err)
	}
	if _, err = service.FinanceSummary(context.Background(), project.Actor{UserID: user2}, ordinary.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign finance error=%v", err)
	}
	finance, err = service.FinanceSummary(context.Background(), project.Actor{UserID: user1}, ordinary.ID)
	if err != nil || finance.PendingExpenseMinor != 125_050 || finance.ConfirmedExpenseMinor != 0 {
		t.Fatalf("pending finance=%#v error=%v", finance, err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE project_expense_requests SET status='paid' WHERE id=$1`, expense.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `
		INSERT INTO project_ledger_operations (project_id,source_expense_request_id,created_by_user_id,operation_type,direction,amount,currency_code,description,occurred_at)
		VALUES ($1,$2,$3,'expense','debit',1250.50,'RUB','Оплата светильников',$4),
		       ($1,NULL,$3,'income','credit',2000.00,'RUB','Поступление заказчика',$4)
	`, ordinary.ID, expense.ID, user1, now); err != nil {
		t.Fatal(err)
	}
	finance, err = service.FinanceSummary(context.Background(), project.Actor{UserID: user1}, ordinary.ID)
	if err != nil || finance.ConfirmedIncomeMinor != 200_000 || finance.ConfirmedExpenseMinor != 125_050 || finance.AvailableBalanceMinor != 74_950 || finance.PendingExpenseMinor != 0 {
		t.Fatalf("ledger finance=%#v error=%v", finance, err)
	}
	if _, err = pool.Exec(context.Background(), `UPDATE project_ledger_operations SET amount=1 WHERE project_id=$1`, ordinary.ID); err == nil {
		t.Fatal("immutable ledger accepted an update")
	}
	expenses, err := service.ListExpenses(context.Background(), project.Actor{UserID: user1}, ordinary.ID)
	if err != nil || len(expenses.Items) != 1 || expenses.Items[0].ID != expense.ID || !expenses.CanCreateExpense {
		t.Fatalf("expenses=%#v error=%v", expenses, err)
	}

	super := project.Actor{UserID: user1, SuperAdmin: true}
	withoutCustomer, err := service.Create(context.Background(), super, project.CreateRequest{Name: "Дом 26", Type: "architecture", CurrencyCode: "RUB", RequestID: "request-superadmin", AdminUserIDs: []string{user3}})
	if err != nil || withoutCustomer.CustomerUserID != nil {
		t.Fatalf("customerless project=%#v error=%v", withoutCustomer, err)
	}
	var projectNotificationCount int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE message_type='project.created'`).Scan(&projectNotificationCount); err != nil || projectNotificationCount != 1 {
		t.Fatalf("project notification count=%d error=%v", projectNotificationCount, err)
	}
	assigned, err := service.AssignCustomer(context.Background(), super, withoutCustomer.ID, user2, "request-assign", withoutCustomer.Version)
	if err != nil || assigned.CustomerUserID == nil || *assigned.CustomerUserID != user2 || assigned.Version != 2 {
		t.Fatalf("assigned project=%#v error=%v", assigned, err)
	}
	repeated, err := service.AssignCustomer(context.Background(), super, withoutCustomer.ID, user2, "request-repeat", assigned.Version)
	if err != nil || repeated.Version != assigned.Version {
		t.Fatalf("repeated assignment=%#v error=%v", repeated, err)
	}
	if _, err = service.AssignCustomer(context.Background(), super, withoutCustomer.ID, user3, "request-replace", assigned.Version); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("replacement error=%v", err)
	}
	if _, err = service.Create(context.Background(), super, project.CreateRequest{Name: "Rollback", Type: "architecture", CurrencyCode: "RUB", RequestID: "request-rollback", AdminUserIDs: []string{"00000000-0000-0000-0000-000000000099"}}); !errors.Is(err, project.ErrAccountUnavailable) {
		t.Fatalf("missing account error=%v", err)
	}
	var rolledBack int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM projects WHERE name='Rollback'`).Scan(&rolledBack); err != nil || rolledBack != 0 {
		t.Fatalf("rolled back projects=%d error=%v", rolledBack, err)
	}
	var auditCount int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE event_type IN ('project.created','project.customer_assigned')`).Scan(&auditCount); err != nil || auditCount != 3 {
		t.Fatalf("audit count=%d error=%v", auditCount, err)
	}
	if err = service.Archive(context.Background(), project.Actor{UserID: user2}, withoutCustomer.ID, "request-ordinary-archive", assigned.Version); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("ordinary archive error=%v", err)
	}
	if err = service.Archive(context.Background(), super, withoutCustomer.ID, "request-stale-archive", assigned.Version+1); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale archive error=%v", err)
	}
	if err = service.Archive(context.Background(), super, withoutCustomer.ID, "request-super-archive", assigned.Version); err != nil {
		t.Fatalf("super archive error=%v", err)
	}
	if _, err = service.Get(context.Background(), super, withoutCustomer.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("archived project error=%v", err)
	}
	if err = service.Archive(context.Background(), super, withoutCustomer.ID, "request-repeat-archive", assigned.Version); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("repeated archive error=%v", err)
	}
}

func TestConcurrentLateCustomerAssignmentKeepsSingleCustomer(t *testing.T) {
	pool := newMigrationSchemaPool(t, "r1_project_customer_race")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, _ := project.NewService(store, []byte("project-integration-cursor-key-32"), func() time.Time { return now })
	admin := insertProjectTestUser(t, pool, "super.admin", "super@example.com")
	first := insertProjectTestUser(t, pool, "customer.first", "first@example.com")
	second := insertProjectTestUser(t, pool, "customer.second", "second@example.com")
	actor := project.Actor{UserID: admin, SuperAdmin: true}
	created, err := service.Create(context.Background(), actor, project.CreateRequest{Name: "Race", Type: "architecture", CurrencyCode: "RUB", RequestID: "request-race-create"})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index, customer := range []string{first, second} {
		wait.Add(1)
		go func(index int, customer string) {
			defer wait.Done()
			_, assignErr := service.AssignCustomer(context.Background(), actor, created.ID, customer, "request-race-"+string(rune('a'+index)), created.Version)
			results <- assignErr
		}(index, customer)
	}
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for result := range results {
		if result == nil {
			successes++
		} else if errors.Is(result, project.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected assignment error=%v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	var activeCustomers int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM project_memberships WHERE project_id=$1 AND project_role='customer' AND revoked_at IS NULL`, created.ID).Scan(&activeCustomers); err != nil || activeCustomers != 1 {
		t.Fatalf("active customers=%d error=%v", activeCustomers, err)
	}
}

func TestProjectUpcomingTasksAndMeetingsRespectScopeAndRange(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_upcoming")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := project.NewService(store, []byte("project-upcoming-cursor-key-32--"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	owner := insertProjectTestUser(t, pool, "upcoming.owner", "upcoming.owner@example.com")
	foreign := insertProjectTestUser(t, pool, "upcoming.foreign", "upcoming.foreign@example.com")
	executor := insertProjectTestUser(t, pool, "upcoming.executor", "upcoming.executor@example.com")
	actor := project.Actor{UserID: owner}
	created, err := service.Create(context.Background(), actor, project.CreateRequest{Name: "Будущие события", Type: "interior_design", CurrencyCode: "RUB", RequestID: "request-upcoming-project"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), actor, created.ID, project.CreateTaskRequest{Title: "Подготовить планы", Description: "Первый этаж", DueAt: now.Add(24 * time.Hour), RequestID: "request-upcoming-task"})
	if err != nil || task.Kind != project.UpcomingTask || task.Status == nil || *task.Status != "new" {
		t.Fatalf("task=%#v error=%v", task, err)
	}
	if _, err = service.AddMember(context.Background(), actor, created.ID, executor, "request-add-executor"); err != nil {
		t.Fatal(err)
	}
	assignedTask, err := service.CreateTask(context.Background(), actor, created.ID, project.CreateTaskRequest{Title: "Собрать ведомость", DueAt: now.Add(36 * time.Hour), AssigneeUserIDs: []string{executor}, RequestID: "request-assigned-task"})
	if err != nil {
		t.Fatal(err)
	}
	ownerTasks, err := service.ListTasks(context.Background(), actor, created.ID)
	if err != nil || !ownerTasks.CanCreate || len(ownerTasks.Items) != 2 {
		t.Fatalf("owner tasks=%#v error=%v", ownerTasks, err)
	}
	executorActor := project.Actor{UserID: executor}
	executorTasks, err := service.ListTasks(context.Background(), executorActor, created.ID)
	if err != nil || executorTasks.CanCreate || len(executorTasks.Items) != 1 || executorTasks.Items[0].ID != assignedTask.ID || !executorTasks.Items[0].CanChangeStatus {
		t.Fatalf("executor tasks=%#v error=%v", executorTasks, err)
	}
	updatedTask, err := service.UpdateTask(context.Background(), executorActor, created.ID, assignedTask.ID, project.UpdateTaskRequest{Status: project.TaskInProgress, ExpectedVersion: executorTasks.Items[0].Version, RequestID: "request-start-task"})
	if err != nil || updatedTask.Status != project.TaskInProgress || updatedTask.StartedAt == nil {
		t.Fatalf("updated task=%#v error=%v", updatedTask, err)
	}
	if _, err = service.UpdateTask(context.Background(), executorActor, created.ID, assignedTask.ID, project.UpdateTaskRequest{Status: project.TaskReview, ExpectedVersion: executorTasks.Items[0].Version, RequestID: "request-stale-task"}); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale task update error=%v", err)
	}
	meeting, err := service.CreateMeeting(context.Background(), actor, created.ID, project.CreateMeetingRequest{Title: "Встреча на объекте", Location: "Москва", StartsAt: now.Add(72 * time.Hour), EndsAt: now.Add(73 * time.Hour), RequestID: "request-upcoming-meeting"})
	if err != nil || meeting.Kind != project.UpcomingMeeting || meeting.Location == nil || *meeting.Location != "Москва" {
		t.Fatalf("meeting=%#v error=%v", meeting, err)
	}
	short, err := service.ListUpcoming(context.Background(), actor, created.ID, 2)
	if err != nil || len(short.Items) != 2 || short.Items[0].ID != task.ID || short.Items[1].ID != assignedTask.ID {
		t.Fatalf("short feed=%#v error=%v", short, err)
	}
	long, err := service.ListUpcoming(context.Background(), actor, created.ID, 4)
	if err != nil || len(long.Items) != 3 || long.Items[0].ID != task.ID || long.Items[1].ID != assignedTask.ID || long.Items[2].ID != meeting.ID {
		t.Fatalf("long feed=%#v error=%v", long, err)
	}
	calendar, err := service.ListCalendar(context.Background(), actor, created.ID, now, now.Add(4*24*time.Hour))
	if err != nil || len(calendar.Items) != 3 || calendar.Items[0].ID != task.ID || calendar.Items[2].ID != meeting.ID {
		t.Fatalf("owner calendar=%#v error=%v", calendar, err)
	}
	executorCalendar, err := service.ListCalendar(context.Background(), executorActor, created.ID, now, now.Add(4*24*time.Hour))
	if err != nil || len(executorCalendar.Items) != 2 || executorCalendar.Items[0].ID != assignedTask.ID || executorCalendar.Items[1].ID != meeting.ID {
		t.Fatalf("executor calendar=%#v error=%v", executorCalendar, err)
	}
	globalCalendar, err := service.ListGlobalCalendar(context.Background(), executorActor, now, now.Add(4*24*time.Hour), nil)
	if err != nil || len(globalCalendar.Projects) != 1 || globalCalendar.Projects[0].ID != created.ID || len(globalCalendar.Projects[0].Items) != 2 || globalCalendar.Projects[0].Items[0].ID != assignedTask.ID {
		t.Fatalf("executor global calendar=%#v error=%v", globalCalendar, err)
	}
	foreignActor := project.Actor{UserID: foreign}
	foreignCalendar, err := service.ListGlobalCalendar(context.Background(), foreignActor, now, now.Add(4*24*time.Hour), nil)
	if err != nil || len(foreignCalendar.Projects) != 0 {
		t.Fatalf("foreign global calendar=%#v error=%v", foreignCalendar, err)
	}
	if _, err = service.ListUpcoming(context.Background(), foreignActor, created.ID, 7); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign list error=%v", err)
	}
	if _, err = service.CreateTask(context.Background(), foreignActor, created.ID, project.CreateTaskRequest{Title: "Чужая задача", DueAt: now.Add(time.Hour), RequestID: "request-foreign-task"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign task error=%v", err)
	}
	var auditCount int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE project_id=$1 AND event_type IN ('task.created','task.status_changed','meeting.created')`, created.ID).Scan(&auditCount); err != nil || auditCount != 4 {
		t.Fatalf("audit count=%d error=%v", auditCount, err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO project_meetings (project_id,created_by_user_id,title,starts_at,ends_at) VALUES ($1,$2,'Ошибка',$3,$3)`, created.ID, owner, now.Add(96*time.Hour)); err == nil {
		t.Fatal("meeting with equal start and end was accepted")
	}
}

func TestProjectChatAuthorizationHistoryAndIdempotencyIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_chat")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := project.NewService(store, []byte("project-chat-cursor-key-32------"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	owner := insertProjectTestUser(t, pool, "chat.owner", "chat.owner@example.com")
	participant := insertProjectTestUser(t, pool, "chat.participant", "chat.participant@example.com")
	foreign := insertProjectTestUser(t, pool, "chat.foreign", "chat.foreign@example.com")
	ownerActor := project.Actor{UserID: owner}
	created, err := service.Create(context.Background(), ownerActor, project.CreateRequest{Name: "Чат проекта", Type: "interior_design", CurrencyCode: "RUB", RequestID: "request-chat-project"})
	if err != nil {
		t.Fatal(err)
	}

	clientID := "00000000-0000-0000-0000-000000000101"
	message, err := service.SendChatMessage(context.Background(), ownerActor, created.ID, project.CreateChatMessageRequest{Body: "  Первый комментарий  ", ClientMessageID: clientID, RequestID: "request-chat-message"})
	if err != nil || message.Body != "Первый комментарий" || message.Author.UserID != owner {
		t.Fatalf("message=%#v error=%v", message, err)
	}
	repeated, err := service.SendChatMessage(context.Background(), ownerActor, created.ID, project.CreateChatMessageRequest{Body: "Первый комментарий", ClientMessageID: clientID, RequestID: "request-chat-repeated"})
	if err != nil || repeated.ID != message.ID {
		t.Fatalf("repeated=%#v error=%v", repeated, err)
	}
	if _, err = service.SendChatMessage(context.Background(), ownerActor, created.ID, project.CreateChatMessageRequest{Body: "Другой текст", ClientMessageID: clientID, RequestID: "request-chat-conflict"}); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("idempotency conflict error=%v", err)
	}

	if _, err = service.AddMember(context.Background(), ownerActor, created.ID, participant, "request-chat-add-member"); err != nil {
		t.Fatal(err)
	}
	participantActor := project.Actor{UserID: participant}
	page, err := service.ListChat(context.Background(), participantActor, created.ID, 50, "")
	if err != nil || page.CurrentUserID != participant || len(page.Messages) != 1 || page.Messages[0].ID != message.ID || !page.CanSend {
		t.Fatalf("participant page=%#v error=%v", page, err)
	}
	now = now.Add(time.Second)
	second, err := service.SendChatMessage(context.Background(), participantActor, created.ID, project.CreateChatMessageRequest{Body: "Ответ участника", ClientMessageID: "00000000-0000-0000-0000-000000000102", RequestID: "request-chat-response"})
	if err != nil || second.Author.UserID != participant {
		t.Fatalf("second=%#v error=%v", second, err)
	}

	latest, err := service.ListChat(context.Background(), ownerActor, created.ID, 1, "")
	if err != nil || len(latest.Messages) != 1 || latest.Messages[0].ID != second.ID || !latest.HasMore || latest.NextCursor == nil {
		t.Fatalf("latest=%#v error=%v", latest, err)
	}
	older, err := service.ListChat(context.Background(), ownerActor, created.ID, 1, *latest.NextCursor)
	if err != nil || len(older.Messages) != 1 || older.Messages[0].ID != message.ID || older.HasMore {
		t.Fatalf("older=%#v error=%v", older, err)
	}

	foreignActor := project.Actor{UserID: foreign}
	if _, err = service.ListChat(context.Background(), foreignActor, created.ID, 50, ""); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign list error=%v", err)
	}
	if _, err = service.SendChatMessage(context.Background(), foreignActor, created.ID, project.CreateChatMessageRequest{Body: "Чужое сообщение", ClientMessageID: "00000000-0000-0000-0000-000000000103", RequestID: "request-chat-foreign"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign send error=%v", err)
	}
	if err = service.RemoveMember(context.Background(), ownerActor, created.ID, participant, "request-chat-remove-member"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ListChat(context.Background(), participantActor, created.ID, 50, ""); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked list error=%v", err)
	}
	var messages, auditEvents int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_messages WHERE project_id=$1`, created.ID).Scan(&messages); err != nil || messages != 2 {
		t.Fatalf("messages=%d error=%v", messages, err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE project_id=$1 AND event_type='chat.message_created'`, created.ID).Scan(&auditEvents); err != nil || auditEvents != 2 {
		t.Fatalf("audit events=%d error=%v", auditEvents, err)
	}
}

func TestProjectContextChatsRespectEntityAccessAndStayUniqueIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_context_chat")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := project.NewService(store, []byte("context-chat-cursor-key-32-------"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	owner := insertProjectTestUser(t, pool, "context.owner", "context.owner@example.com")
	executor := insertProjectTestUser(t, pool, "context.executor", "context.executor@example.com")
	foreign := insertProjectTestUser(t, pool, "context.foreign", "context.foreign@example.com")
	ownerActor := project.Actor{UserID: owner}
	created, err := service.Create(context.Background(), ownerActor, project.CreateRequest{Name: "Контекстные чаты", Type: "interior_design", CurrencyCode: "RUB", RequestID: "request-context-project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddMember(context.Background(), ownerActor, created.ID, executor, "request-context-member"); err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), ownerActor, created.ID, project.CreateTaskRequest{Title: "Рабочие чертежи", DueAt: now.Add(24 * time.Hour), AssigneeUserIDs: []string{executor}, RequestID: "request-context-task"})
	if err != nil {
		t.Fatal(err)
	}
	vendor := "Фабрика"
	expense, err := service.CreateExpense(context.Background(), ownerActor, created.ID, "request-context-expense", project.CreateExpenseRequest{AmountMinor: 250_000, Category: "furniture", Description: "Диван в гостиную", VendorName: &vendor, IdempotencyKey: "context-expense-key"})
	if err != nil {
		t.Fatal(err)
	}
	material, err := service.CreateMaterial(context.Background(), ownerActor, created.ID, "request-context-material", project.MaterialInput{Category: "furniture", Name: "Диван", SupplierName: vendor, ContractAmountMinor: 250_000, LinkedExpenseID: &expense.ID, PaymentStatus: "unpaid", DeliveryStatus: "expected"})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{ContextType: "task", ContextID: task.ID, Name: "Обсуждение чертежей", RequestID: "request-open-task-chat"})
	if err != nil || opened.ContextTitle != "Рабочие чертежи" {
		t.Fatalf("opened=%#v err=%v", opened, err)
	}
	executorActor := project.Actor{UserID: executor}
	replayed, err := service.OpenContextChat(context.Background(), executorActor, created.ID, project.OpenContextChatRequest{ContextType: "task", ContextID: task.ID, Name: "Другое имя", RequestID: "request-reopen-task-chat"})
	if err != nil || replayed.ChatID != opened.ChatID || replayed.Name != "Обсуждение чертежей" {
		t.Fatalf("replayed=%#v err=%v", replayed, err)
	}
	contextMessageRequest := project.CreateChatMessageRequest{Body: "Чертежи загружены", ClientMessageID: "00000000-0000-0000-0000-000000000201", RequestID: "request-context-message"}
	message, err := service.SendContextChatMessage(context.Background(), executorActor, created.ID, opened.ChatID, contextMessageRequest)
	if err != nil || message.Author.UserID != executor {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	repeatedMessage, err := service.SendContextChatMessage(context.Background(), executorActor, created.ID, opened.ChatID, contextMessageRequest)
	if err != nil || repeatedMessage.ID != message.ID {
		t.Fatalf("repeated context message=%#v err=%v", repeatedMessage, err)
	}
	contextMessageRequest.Body = "Другой комментарий"
	contextMessageRequest.RequestID = "request-context-conflict"
	if _, err = service.SendContextChatMessage(context.Background(), executorActor, created.ID, opened.ChatID, contextMessageRequest); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("context message conflict error=%v", err)
	}
	page, err := service.ListContextChat(context.Background(), ownerActor, created.ID, opened.ChatID, 50, "")
	if err != nil || page.Context == nil || page.Context.ContextType != "task" || len(page.Messages) != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	now = now.Add(time.Second)
	secondMessage, err := service.SendContextChatMessage(context.Background(), ownerActor, created.ID, opened.ChatID, project.CreateChatMessageRequest{
		Body: "Комментарий заказчика", ClientMessageID: "00000000-0000-0000-0000-000000000202", RequestID: "request-context-second-message",
	})
	if err != nil {
		t.Fatal(err)
	}
	latestPage, err := service.ListContextChat(context.Background(), ownerActor, created.ID, opened.ChatID, 1, "")
	if err != nil || len(latestPage.Messages) != 1 || latestPage.Messages[0].ID != secondMessage.ID || !latestPage.HasMore || latestPage.NextCursor == nil {
		t.Fatalf("latest page=%#v err=%v", latestPage, err)
	}
	olderPage, err := service.ListContextChat(context.Background(), ownerActor, created.ID, opened.ChatID, 1, *latestPage.NextCursor)
	if err != nil || len(olderPage.Messages) != 1 || olderPage.Messages[0].ID != message.ID || olderPage.HasMore {
		t.Fatalf("older page=%#v err=%v", olderPage, err)
	}

	if _, err = service.OpenContextChat(context.Background(), executorActor, created.ID, project.OpenContextChatRequest{ContextType: "material", ContextID: material.ID, Name: "Обсуждение дивана", RequestID: "request-denied-material"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("executor material access err=%v", err)
	}
	missingContextID := "00000000-0000-0000-0000-000000000997"
	if _, err = service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{ContextType: "material", ContextID: missingContextID, Name: "Неизвестный материал", RequestID: "request-missing-material"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing material context error=%v", err)
	}
	if _, err = service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{ContextType: "expense", ContextID: missingContextID, Name: "Неизвестный расход", RequestID: "request-missing-expense"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing expense context error=%v", err)
	}
	materialChat, err := service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{ContextType: "material", ContextID: material.ID, Name: "Обсуждение дивана", RequestID: "request-open-material"})
	if err != nil || materialChat.ContextTitle != "Диван" {
		t.Fatalf("material chat=%#v err=%v", materialChat, err)
	}

	results := make(chan project.ChatContext, 2)
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			value, openErr := service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{ContextType: "expense", ContextID: expense.ID, Name: "Обсуждение расхода", RequestID: fmt.Sprintf("request-open-expense-%d", index)})
			results <- value
			errorsChannel <- openErr
		}(index)
	}
	wait.Wait()
	close(results)
	close(errorsChannel)
	for openErr := range errorsChannel {
		if openErr != nil {
			t.Fatalf("concurrent open err=%v", openErr)
		}
	}
	var expenseChatID string
	for value := range results {
		if expenseChatID == "" {
			expenseChatID = value.ChatID
		} else if value.ChatID != expenseChatID {
			t.Fatalf("context chat ids differ: %s != %s", value.ChatID, expenseChatID)
		}
	}

	foreignActor := project.Actor{UserID: foreign}
	if _, err = service.ListContextChat(context.Background(), foreignActor, created.ID, opened.ChatID, 50, ""); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("foreign list err=%v", err)
	}
	if err = service.RemoveMember(context.Background(), ownerActor, created.ID, executor, "request-context-remove"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ListContextChat(context.Background(), executorActor, created.ID, opened.ChatID, 50, ""); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked list err=%v", err)
	}

	var taskChats, expenseChats, contextAudits int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM chats WHERE project_id=$1 AND kind='context' AND context_type='task' AND context_id=$2`, created.ID, task.ID).Scan(&taskChats); err != nil || taskChats != 1 {
		t.Fatalf("task chats=%d err=%v", taskChats, err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM chats WHERE project_id=$1 AND kind='context' AND context_type='expense' AND context_id=$2`, created.ID, expense.ID).Scan(&expenseChats); err != nil || expenseChats != 1 {
		t.Fatalf("expense chats=%d err=%v", expenseChats, err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE project_id=$1 AND event_type='chat.context_created'`, created.ID).Scan(&contextAudits); err != nil || contextAudits != 3 {
		t.Fatalf("context audits=%d err=%v", contextAudits, err)
	}
}

func TestProjectCollaborationNotificationsResolveEligibleRecipientsIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_collaboration_notifications")
	applyMigrationPaths(t, pool, migrationPaths(t))
	store := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	service, err := project.NewService(store, []byte("project-event-notification-key-32"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := account.NewPayloadCipher(bytes.Repeat([]byte{7}, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ConfigureNotifications(cipher); err != nil {
		t.Fatal(err)
	}

	owner := insertProjectTestUser(t, pool, "notify.owner", "notify.owner@example.com")
	assignee := insertProjectTestUser(t, pool, "notify.assignee", "notify.assignee@example.com")
	unrelatedMember := insertProjectTestUser(t, pool, "notify.member", "notify.member@example.com")
	mutedMember := insertProjectTestUser(t, pool, "notify.muted", "notify.muted@example.com")
	outsider := insertProjectTestUser(t, pool, "notify.outsider", "notify.outsider@example.com")
	projectAdmin := insertProjectTestUser(t, pool, "notify.admin", "notify.admin@example.com")
	bindProjectTelegram(t, pool, owner, 99201, true, now)
	bindProjectTelegram(t, pool, assignee, 99202, true, now)
	bindProjectTelegram(t, pool, unrelatedMember, 99203, true, now)
	bindProjectTelegram(t, pool, mutedMember, 99204, false, now)
	bindProjectTelegram(t, pool, outsider, 99205, true, now)
	bindProjectTelegram(t, pool, projectAdmin, 99206, true, now)

	ownerActor := project.Actor{UserID: owner, Login: "notify.owner", FirstName: "Светлана"}
	created, err := service.Create(context.Background(), ownerActor, project.CreateRequest{Name: "Совместный проект", Type: "interior_design", CurrencyCode: "RUB", RequestID: "request-notify-project"})
	if err != nil {
		t.Fatal(err)
	}
	for index, userID := range []string{assignee, unrelatedMember, mutedMember, projectAdmin} {
		if _, err = service.AddMember(context.Background(), ownerActor, created.ID, userID, fmt.Sprintf("request-notify-member-%d", index)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(context.Background(), `
		UPDATE project_memberships
		SET project_role='project_admin'
		WHERE project_id=$1 AND user_id=$2 AND project_role='executor'
	`, created.ID, projectAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `
		UPDATE project_memberships
		SET privileges='{"financials.view":true,"materials.view":true,"tasks.view_all":true}'::jsonb
		WHERE project_id=$1 AND user_id=$2 AND project_role='executor'
	`, created.ID, unrelatedMember); err != nil {
		t.Fatal(err)
	}

	messageRequest := project.CreateChatMessageRequest{
		Body:            "Проверьте обновлённый план проекта",
		ClientMessageID: "00000000-0000-0000-0000-000000000301",
		RequestID:       "request-notify-chat-message",
	}
	message, err := service.SendChatMessage(context.Background(), ownerActor, created.ID, messageRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SendChatMessage(context.Background(), ownerActor, created.ID, messageRequest); err != nil {
		t.Fatalf("idempotent chat replay: %v", err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.chat.message_created", message.ID, []string{assignee, unrelatedMember, projectAdmin}, "Проверьте обновлённый план")

	task, err := service.CreateTask(context.Background(), ownerActor, created.ID, project.CreateTaskRequest{
		Title:           "Подготовить рабочие чертежи",
		DueAt:           now.Add(48 * time.Hour),
		AssigneeUserIDs: []string{assignee},
		RequestID:       "request-notify-task",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.task.assigned", task.ID, []string{assignee}, "Подготовить рабочие чертежи")
	updatedTask, err := service.UpdateTask(context.Background(), ownerActor, created.ID, task.ID, project.UpdateTaskRequest{
		Status:          project.TaskInProgress,
		ExpectedVersion: task.Version,
		RequestID:       "request-notify-task-status",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.task.status_changed", fmt.Sprintf("%s:%d", task.ID, updatedTask.Version), []string{assignee, unrelatedMember, projectAdmin}, "В работе")
	if _, err = service.UpdateTask(context.Background(), ownerActor, created.ID, task.ID, project.UpdateTaskRequest{Status: project.TaskReview, ExpectedVersion: task.Version, RequestID: "request-notify-task-stale"}); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale task update error=%v", err)
	}
	if _, err = service.UpdateTask(context.Background(), project.Actor{UserID: assignee}, created.ID, task.ID, project.UpdateTaskRequest{Status: project.TaskAccepted, ExpectedVersion: updatedTask.Version, RequestID: "request-notify-task-forbidden"}); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden task update error=%v", err)
	}
	if _, err = service.UpdateTask(context.Background(), ownerActor, created.ID, "00000000-0000-0000-0000-000000000998", project.UpdateTaskRequest{Status: project.TaskInProgress, ExpectedVersion: 1, RequestID: "request-notify-task-missing"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing task update error=%v", err)
	}

	contextChat, err := service.OpenContextChat(context.Background(), ownerActor, created.ID, project.OpenContextChatRequest{
		ContextType: "task",
		ContextID:   task.ID,
		Name:        "Рабочие чертежи",
		RequestID:   "request-notify-task-chat",
	})
	if err != nil {
		t.Fatal(err)
	}
	contextMessage, err := service.SendContextChatMessage(context.Background(), ownerActor, created.ID, contextChat.ChatID, project.CreateChatMessageRequest{
		Body:            "Добавила комментарии к чертежам",
		ClientMessageID: "00000000-0000-0000-0000-000000000302",
		RequestID:       "request-notify-context-message",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.context_chat.message_created", contextMessage.ID, []string{assignee, unrelatedMember, projectAdmin}, "комментарии к чертежам")

	meeting, err := service.CreateMeeting(context.Background(), ownerActor, created.ID, project.CreateMeetingRequest{
		Title:     "Встреча на объекте",
		StartsAt:  now.Add(72 * time.Hour),
		EndsAt:    now.Add(73 * time.Hour),
		RequestID: "request-notify-meeting",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.meeting.created", meeting.ID, []string{assignee, unrelatedMember, projectAdmin}, "Встреча на объекте")

	expenseRequest := project.CreateExpenseRequest{
		AmountMinor: 125050, Category: "materials", Description: "Керамогранит для санузла",
		IdempotencyKey: "notify-expense-idempotency",
	}
	expense, err := service.CreateExpense(context.Background(), ownerActor, created.ID, "request-notify-expense", expenseRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateExpense(context.Background(), ownerActor, created.ID, "request-notify-expense-replay", expenseRequest); err != nil {
		t.Fatalf("idempotent expense replay: %v", err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.expense.created", expense.ID, []string{unrelatedMember, projectAdmin}, "1250.50")
	conflictingExpense := expenseRequest
	conflictingExpense.AmountMinor++
	if _, err = service.CreateExpense(context.Background(), ownerActor, created.ID, "request-notify-expense-conflict", conflictingExpense); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("conflicting expense replay error=%v", err)
	}
	if _, err = service.CreateExpense(context.Background(), project.Actor{UserID: assignee}, created.ID, "request-notify-expense-forbidden", project.CreateExpenseRequest{AmountMinor: 100, Category: "other", Description: "Тестовый расход", IdempotencyKey: "notify-expense-forbidden"}); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden expense creation error=%v", err)
	}
	assigneeExpenses, err := service.ListExpenses(context.Background(), project.Actor{UserID: assignee}, created.ID)
	if err != nil || assigneeExpenses.CanCreateExpense || len(assigneeExpenses.Items) != 1 {
		t.Fatalf("assignee expenses=%#v error=%v", assigneeExpenses, err)
	}

	material, err := service.CreateMaterial(context.Background(), ownerActor, created.ID, "request-notify-material", project.MaterialInput{
		Category: "materials", Name: "Керамогранит", SupplierName: "Поставщик", PaymentStatus: "unpaid",
		DeliveryStatus: "expected", ContractAmountMinor: 125050,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.material.created", material.ID, []string{unrelatedMember, projectAdmin}, "Керамогранит")
	materialList, err := service.ListMaterials(context.Background(), ownerActor, created.ID)
	if err != nil || !materialList.CanCreate || !materialList.CanViewFinancialInformation || len(materialList.Items) != 1 || materialList.Items[0].ID != material.ID {
		t.Fatalf("material list=%#v error=%v", materialList, err)
	}
	material.Name = "Керамогранит серый"
	updatedMaterial, err := service.UpdateMaterial(context.Background(), ownerActor, created.ID, material.ID, "request-notify-material-update", material.Version, project.MaterialInput{
		Category: material.Category, Name: material.Name, SupplierName: material.SupplierName, PaymentStatus: material.PaymentStatus,
		DeliveryStatus: material.DeliveryStatus, ContractAmountMinor: material.ContractAmountMinor,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.material.updated", fmt.Sprintf("%s:%d", material.ID, updatedMaterial.Version), []string{unrelatedMember, projectAdmin}, "Керамогранит серый")
	if err = service.DeleteMaterial(context.Background(), ownerActor, created.ID, material.ID, "request-notify-material-delete", updatedMaterial.Version, "Выбран другой поставщик"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.material.deleted", fmt.Sprintf("%s:%d", material.ID, updatedMaterial.Version+1), []string{unrelatedMember, projectAdmin}, "Выбран другой поставщик")
	materialInput := project.MaterialInput{Category: "materials", Name: "Керамогранит", SupplierName: "Поставщик", PaymentStatus: "unpaid", DeliveryStatus: "expected", ContractAmountMinor: 125050}
	if _, err = service.UpdateMaterial(context.Background(), ownerActor, created.ID, material.ID, "request-notify-material-stale", updatedMaterial.Version, materialInput); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale material update error=%v", err)
	}
	missingMaterialID := "00000000-0000-0000-0000-000000000999"
	if _, err = service.UpdateMaterial(context.Background(), ownerActor, created.ID, missingMaterialID, "request-notify-material-missing", 1, materialInput); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing material update error=%v", err)
	}
	assigneeActor := project.Actor{UserID: assignee, Login: "notify.assignee"}
	if _, err = service.CreateMaterial(context.Background(), assigneeActor, created.ID, "request-notify-material-forbidden", materialInput); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden material creation error=%v", err)
	}
	if err = service.DeleteMaterial(context.Background(), assigneeActor, created.ID, material.ID, "request-notify-material-delete-forbidden", updatedMaterial.Version+1, "Нет доступа"); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden material deletion error=%v", err)
	}
	if err = service.DeleteMaterial(context.Background(), ownerActor, created.ID, missingMaterialID, "request-notify-material-delete-missing", 1, "Не найден"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing material deletion error=%v", err)
	}
	assigneeMembers, canAssigneeManage, err := store.ListProjectMembers(context.Background(), assigneeActor, created.ID)
	if err != nil || canAssigneeManage || len(assigneeMembers) != 5 {
		t.Fatalf("assignee members=%d canManage=%v error=%v", len(assigneeMembers), canAssigneeManage, err)
	}

	projectChat, err := service.CreateChat(context.Background(), ownerActor, created.ID, project.CreateChatRequest{
		Name: "Согласование кухни", MemberUserIDs: []string{assignee}, RequestID: "request-project-chat-create",
	})
	if err != nil || projectChat.Name != "Согласование кухни" || projectChat.MemberCount != 2 || !projectChat.CanManage {
		t.Fatalf("created project chat=%#v error=%v", projectChat, err)
	}
	chats, err := service.ListChats(context.Background(), ownerActor, created.ID)
	if err != nil || !chats.CanCreate || len(chats.Items) < 2 {
		t.Fatalf("project chats=%#v error=%v", chats, err)
	}
	projectChat, err = service.UpdateChat(context.Background(), ownerActor, created.ID, projectChat.ID, "Согласование кухни и гостиной", "request-project-chat-update", projectChat.Version)
	if err != nil || projectChat.Name != "Согласование кухни и гостиной" || projectChat.Version != 2 {
		t.Fatalf("updated project chat=%#v error=%v", projectChat, err)
	}
	if _, err = service.UpdateChat(context.Background(), ownerActor, created.ID, projectChat.ID, "Устаревшее название", "request-project-chat-update-stale", 1); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale project chat update error=%v", err)
	}
	if _, err = service.UpdateChat(context.Background(), assigneeActor, created.ID, projectChat.ID, "Чужое название", "request-project-chat-update-forbidden", projectChat.Version); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden project chat update error=%v", err)
	}
	chatMembers, err := service.ListChatMembers(context.Background(), ownerActor, created.ID, projectChat.ID)
	if err != nil || !chatMembers.CanManage || len(chatMembers.Items) != 2 {
		t.Fatalf("initial chat members=%#v error=%v", chatMembers, err)
	}
	addedChatMember, err := service.AddChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, unrelatedMember, "request-project-chat-member-add")
	if err != nil || addedChatMember.UserID != unrelatedMember {
		t.Fatalf("added chat member=%#v error=%v", addedChatMember, err)
	}
	if _, err = service.AddChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, unrelatedMember, "request-project-chat-member-add-repeat"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("duplicate chat member error=%v", err)
	}
	if _, err = service.AddChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, outsider, "request-project-chat-member-add-outsider"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("non-project chat member error=%v", err)
	}
	if _, err = service.AddChatMember(context.Background(), assigneeActor, created.ID, projectChat.ID, unrelatedMember, "request-project-chat-member-add-forbidden"); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden chat member addition error=%v", err)
	}
	chatMessage, err := service.SendContextChatMessage(context.Background(), ownerActor, created.ID, projectChat.ID, project.CreateChatMessageRequest{
		Body: "Проверьте ведомость", ClientMessageID: "00000000-0000-0000-0000-000000000399", RequestID: "request-project-chat-message",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.MarkChatRead(context.Background(), assigneeActor, created.ID, projectChat.ID, chatMessage.ID); err != nil {
		t.Fatalf("mark chat read: %v", err)
	}
	missingID := "00000000-0000-0000-0000-000000000998"
	if err = service.MarkChatRead(context.Background(), assigneeActor, created.ID, projectChat.ID, missingID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("mark missing chat message read error=%v", err)
	}
	notifications, err := service.ListNotifications(context.Background(), assigneeActor, 50)
	if err != nil || notifications.UnreadCount == 0 || len(notifications.Items) == 0 {
		t.Fatalf("account notifications=%#v error=%v", notifications, err)
	}
	if err = service.MarkNotificationRead(context.Background(), assigneeActor, notifications.Items[0].ID); err != nil {
		t.Fatalf("mark notification read: %v", err)
	}
	if err = service.MarkNotificationRead(context.Background(), assigneeActor, missingID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("mark missing notification read error=%v", err)
	}
	if err = service.MarkAllNotificationsRead(context.Background(), assigneeActor); err != nil {
		t.Fatalf("mark all notifications read: %v", err)
	}
	if err = service.RemoveChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, unrelatedMember, "request-project-chat-member-remove"); err != nil {
		t.Fatalf("remove chat member: %v", err)
	}
	if err = service.RemoveChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, unrelatedMember, "request-project-chat-member-remove-missing"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("remove missing chat member error=%v", err)
	}
	if err = service.RemoveChatMember(context.Background(), ownerActor, created.ID, projectChat.ID, owner, "request-project-chat-member-remove-creator"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("remove chat creator error=%v", err)
	}
	if err = service.RemoveChatMember(context.Background(), assigneeActor, created.ID, projectChat.ID, owner, "request-project-chat-member-remove-forbidden"); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden chat member removal error=%v", err)
	}

	documentBody := []byte("project plan")
	document, err := service.UploadDocument(context.Background(), ownerActor, created.ID, "plan.pdf", "application/pdf", "request-project-document-upload", bytes.NewReader(documentBody), int64(len(documentBody)))
	if err != nil || !document.CanDelete || document.Name != "plan.pdf" {
		t.Fatalf("uploaded document=%#v error=%v", document, err)
	}
	documents, err := service.ListDocuments(context.Background(), ownerActor, created.ID)
	if err != nil || !documents.CanUpload || len(documents.Items) != 1 || documents.Items[0].ID != document.ID {
		t.Fatalf("project documents=%#v error=%v", documents, err)
	}
	downloaded, err := service.DownloadDocument(context.Background(), assigneeActor, created.ID, document.ID)
	if err != nil || !bytes.Equal(downloaded.Content, documentBody) || downloaded.CanDelete {
		t.Fatalf("downloaded document=%#v error=%v", downloaded, err)
	}
	if err = service.DeleteDocument(context.Background(), ownerActor, created.ID, document.ID, "request-project-document-delete"); err != nil {
		t.Fatalf("delete document: %v", err)
	}
	if _, err = service.DownloadDocument(context.Background(), ownerActor, created.ID, document.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("deleted document download error=%v", err)
	}
	if err = service.DeleteDocument(context.Background(), ownerActor, created.ID, document.ID, "request-project-document-delete-missing"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("delete missing document error=%v", err)
	}
	if _, err = service.CreateChat(context.Background(), ownerActor, created.ID, project.CreateChatRequest{Name: "Недопустимый участник", MemberUserIDs: []string{outsider}, RequestID: "request-project-chat-invalid-member"}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("chat with non-project member error=%v", err)
	}
	if err = service.DeleteChat(context.Background(), assigneeActor, created.ID, projectChat.ID, "request-project-chat-delete-forbidden", projectChat.Version); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden project chat deletion error=%v", err)
	}
	if err = service.DeleteChat(context.Background(), ownerActor, created.ID, projectChat.ID, "request-project-chat-delete-stale", projectChat.Version-1); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("stale project chat deletion error=%v", err)
	}
	if err = service.DeleteChat(context.Background(), ownerActor, created.ID, projectChat.ID, "request-project-chat-delete", projectChat.Version); err != nil {
		t.Fatalf("delete project chat: %v", err)
	}
	if _, err = service.ListChatMembers(context.Background(), ownerActor, created.ID, projectChat.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("deleted project chat members error=%v", err)
	}

	if _, err = service.AddMember(context.Background(), ownerActor, created.ID, outsider, "request-notify-add-outsider"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.member.added", "request-notify-add-outsider", []string{outsider, projectAdmin}, "новый участник")
	if _, err = service.AddMember(context.Background(), ownerActor, created.ID, outsider, "request-notify-add-duplicate"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("duplicate member addition error=%v", err)
	}
	if _, err = service.AddMember(context.Background(), project.Actor{UserID: assignee}, created.ID, outsider, "request-notify-add-forbidden"); !errors.Is(err, project.ErrForbidden) {
		t.Fatalf("forbidden member addition error=%v", err)
	}
	if err = service.RemoveMember(context.Background(), ownerActor, created.ID, owner, "request-notify-remove-customer"); !errors.Is(err, project.ErrConflict) {
		t.Fatalf("protected customer removal error=%v", err)
	}
	if err = service.RemoveMember(context.Background(), ownerActor, created.ID, outsider, "request-notify-remove-outsider"); err != nil {
		t.Fatal(err)
	}
	assertProjectNotificationRecipients(t, pool, cipher, "project.member.removed", "request-notify-remove-outsider", []string{projectAdmin}, "удалён из проекта")
	if err = service.RemoveMember(context.Background(), ownerActor, created.ID, outsider, "request-notify-remove-missing"); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing member removal error=%v", err)
	}
	if _, err = service.ListMaterials(context.Background(), project.Actor{UserID: outsider}, created.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked material list error=%v", err)
	}
	if _, err = service.ListExpenses(context.Background(), project.Actor{UserID: outsider}, created.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked expense list error=%v", err)
	}
	if _, err = service.ListMembers(context.Background(), project.Actor{UserID: outsider}, created.ID); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("revoked member list error=%v", err)
	}

	var outsiderNotifications, mutedNotifications, actorNotifications int
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE recipient_user_id=$1 AND message_type LIKE 'project.%' AND message_type<>'project.member.added'`, outsider).Scan(&outsiderNotifications); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE recipient_user_id=$1 AND message_type LIKE 'project.%'`, mutedMember).Scan(&mutedNotifications); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), `SELECT count(*) FROM notification_outbox WHERE recipient_user_id=$1 AND message_type LIKE 'project.%'`, owner).Scan(&actorNotifications); err != nil {
		t.Fatal(err)
	}
	if outsiderNotifications != 0 || mutedNotifications != 0 || actorNotifications != 0 {
		t.Fatalf("unexpected notifications outsider=%d muted=%d actor=%d", outsiderNotifications, mutedNotifications, actorNotifications)
	}
}

func TestProjectRepositoryReportsDatabaseOutageIntegration(t *testing.T) {
	pool := newMigrationSchemaPool(t, "project_database_outage")
	store := NewPostgres(pool)
	pool.Close()
	ctx := context.Background()
	actor := project.Actor{UserID: "00000000-0000-0000-0000-000000000001"}
	projectID := "00000000-0000-0000-0000-000000000010"
	entityID := "00000000-0000-0000-0000-000000000020"
	now := time.Now().UTC()

	tests := []struct {
		name string
		call func() error
	}{
		{"create project", func() error { _, err := store.CreateProject(ctx, project.CreateCommand{Actor: actor}); return err }},
		{"list projects", func() error { _, err := store.ListProjects(ctx, project.ListQuery{Actor: actor}); return err }},
		{"get project", func() error { _, err := store.GetProject(ctx, actor, projectID); return err }},
		{"update project", func() error { _, err := store.UpdateProject(ctx, project.UpdateCommand{Actor: actor}); return err }},
		{"archive project", func() error { return store.ArchiveProject(ctx, project.ArchiveCommand{Actor: actor}) }},
		{"assign customer", func() error {
			_, err := store.AssignProjectCustomer(ctx, project.AssignCustomerCommand{Actor: actor})
			return err
		}},
		{"members list", func() error { _, _, err := store.ListProjectMembers(ctx, actor, projectID); return err }},
		{"members search", func() error {
			_, err := store.SearchProjectMemberCandidates(ctx, project.MemberCandidateQuery{Actor: actor, ProjectID: projectID, Query: "user", Limit: 10})
			return err
		}},
		{"member add", func() error { _, err := store.AddProjectMember(ctx, project.MemberCommand{Actor: actor}); return err }},
		{"member remove", func() error { return store.RemoveProjectMember(ctx, project.MemberCommand{Actor: actor}) }},
		{"finance summary", func() error { _, err := store.GetProjectFinanceSummary(ctx, actor, projectID); return err }},
		{"expenses list", func() error { _, _, err := store.ListProjectExpenses(ctx, actor, projectID); return err }},
		{"expense create", func() error {
			_, err := store.CreateProjectExpense(ctx, project.CreateExpenseCommand{Actor: actor})
			return err
		}},
		{"materials list", func() error { _, _, _, err := store.ListProjectMaterials(ctx, actor, projectID); return err }},
		{"material create", func() error {
			_, err := store.CreateProjectMaterial(ctx, project.CreateMaterialCommand{Actor: actor})
			return err
		}},
		{"material update", func() error {
			_, err := store.UpdateProjectMaterial(ctx, project.UpdateMaterialCommand{Actor: actor})
			return err
		}},
		{"material delete", func() error { return store.DeleteProjectMaterial(ctx, project.DeleteMaterialCommand{Actor: actor}) }},
		{"tasks list", func() error {
			_, _, err := store.ListProjectTasks(ctx, project.TaskQuery{Actor: actor, ProjectID: projectID})
			return err
		}},
		{"task update", func() error {
			_, err := store.UpdateProjectTask(ctx, project.UpdateTaskCommand{Actor: actor})
			return err
		}},
		{"upcoming list", func() error {
			_, err := store.ListProjectUpcoming(ctx, project.UpcomingQuery{Actor: actor, ProjectID: projectID, RangeStart: now, End: now.Add(time.Hour)})
			return err
		}},
		{"task create", func() error {
			_, err := store.CreateProjectTask(ctx, project.CreateTaskCommand{Actor: actor})
			return err
		}},
		{"meeting create", func() error {
			_, err := store.CreateProjectMeeting(ctx, project.CreateMeetingCommand{Actor: actor})
			return err
		}},
		{"chat list", func() error {
			_, err := store.ListProjectChat(ctx, project.ChatQuery{Actor: actor, ProjectID: projectID})
			return err
		}},
		{"chat message", func() error {
			_, err := store.CreateProjectChatMessage(ctx, project.CreateChatMessageCommand{Actor: actor})
			return err
		}},
		{"context open", func() error {
			_, err := store.OpenProjectContextChat(ctx, project.OpenContextChatCommand{Actor: actor})
			return err
		}},
		{"context list", func() error {
			_, err := store.ListProjectContextChat(ctx, project.ChatQuery{Actor: actor, ProjectID: projectID, ChatID: entityID})
			return err
		}},
		{"context message", func() error {
			_, err := store.CreateProjectContextChatMessage(ctx, project.CreateChatMessageCommand{Actor: actor})
			return err
		}},
		{"account lookup", func() error { _, _, err := store.FindLoginAccount(ctx, "user"); return err }},
		{"session resolve", func() error { _, _, err := store.ResolveAccessSession(ctx, []byte("hash"), now); return err }},
		{"session create", func() error { return store.CreateSession(ctx, account.NewSession{}) }},
		{"admin bootstrap", func() error {
			_, _, err := store.BootstrapSuperAdminSession(ctx, account.BootstrapSession{})
			return err
		}},
		{"technical admin bootstrap", func() error {
			_, _, err := store.EnsureTechnicalAdmin(ctx, account.TechnicalAdminBootstrap{})
			return err
		}},
		{"session rotate", func() error { _, err := store.RotateSession(ctx, account.SessionRotation{}); return err }},
		{"session revoke", func() error { return store.RevokeSessionFamily(ctx, account.SessionRevocation{}) }},
		{"login failure", func() error { return store.RecordLoginFailure(ctx, "rejected", "request-outage") }},
		{"rate limit", func() error { _, err := store.ConsumeRateLimit(ctx, account.RateLimitRequest{}); return err }},
		{"pending account", func() error { return store.CreatePendingAccount(ctx, account.PendingAccount{}) }},
		{"verification channel", func() error {
			_, err := store.SetVerificationChannel(ctx, account.VerificationChannelSelection{})
			return err
		}},
		{"telegram auth begin", func() error { return store.BeginTelegramAuth(ctx, account.TelegramAuthStart{}) }},
		{"telegram auth advance", func() error { return store.AdvanceTelegramAuth(ctx, account.TelegramAuthAdvance{}) }},
		{"telegram auth step", func() error { _, err := store.TelegramAuthStep(ctx, 1, now); return err }},
		{"telegram auth account", func() error { _, _, err := store.TelegramAuthAccount(ctx, 1, now); return err }},
		{"telegram auth complete", func() error { return store.CompleteTelegramAuth(ctx, account.TelegramAuthCompletion{}) }},
		{"telegram auth reject", func() error { return store.RejectTelegramAuth(ctx, account.TelegramAuthRejection{}) }},
		{"verification consume", func() error { return store.ConsumeVerificationToken(ctx, []byte("hash"), 1, now, "request-outage") }},
		{"pending email", func() error { _, _, err := store.PendingAccountEmail(ctx, "user"); return err }},
		{"verification replace", func() error {
			_, err := store.ReplaceVerificationToken(ctx, account.ReplacementVerification{})
			return err
		}},
		{"active email", func() error { _, _, err := store.ActiveAccountEmail(ctx, "user"); return err }},
		{"password reset create", func() error { _, err := store.CreatePasswordReset(ctx, account.PasswordResetIssue{}); return err }},
		{"telegram password reset", func() error {
			_, err := store.CreateTelegramPasswordReset(ctx, account.PasswordResetIssue{})
			return err
		}},
		{"password reset context", func() error {
			_, err := store.PasswordResetContext(ctx, []byte("hash"), 1, now, "request-outage")
			return err
		}},
		{"password reset complete", func() error { return store.CompletePasswordReset(ctx, account.PasswordResetCompletion{}) }},
		{"password reset audit", func() error { return store.RecordPasswordResetRequest(ctx, "rejected", "request-outage") }},
		{"password reset failure", func() error { return store.RecordPasswordResetFailure(ctx, "request-outage") }},
		{"claim email outbox", func() error { _, _, err := store.ClaimEmailOutbox(ctx, now); return err }},
		{"recover email outbox", func() error { _, err := store.RecoverStaleEmailOutbox(ctx, now, now); return err }},
		{"deliver email outbox", func() error { return store.MarkEmailOutboxDelivered(ctx, entityID, now) }},
		{"fail email outbox", func() error { return store.MarkEmailOutboxFailed(ctx, entityID, 1, 5, now, now, "outage") }},
		{"recover telegram account outbox", func() error { _, err := store.RecoverStaleTelegramAccountOutbox(ctx, now, now); return err }},
		{"terminalize telegram account outbox", func() error { _, err := store.TerminalizeInactiveTelegramAccountOutbox(ctx, now); return err }},
		{"claim telegram account outbox", func() error { _, _, err := store.ClaimTelegramAccountOutbox(ctx, now); return err }},
		{"deliver telegram account outbox", func() error { return store.MarkTelegramAccountOutboxDelivered(ctx, entityID, now) }},
		{"fail telegram account outbox", func() error { return store.MarkTelegramAccountOutboxFailed(ctx, entityID, 1, 5, now, now, "outage") }},
		{"project chats", func() error { _, err := store.ListProjectChats(ctx, actor, projectID); return err }},
		{"project chat create", func() error {
			_, err := store.CreateProjectChat(ctx, project.CreateChatCommand{Actor: actor})
			return err
		}},
		{"project chat update", func() error {
			_, err := store.UpdateProjectChat(ctx, project.UpdateChatCommand{Actor: actor})
			return err
		}},
		{"project chat delete", func() error { return store.DeleteProjectChat(ctx, project.DeleteChatCommand{Actor: actor}) }},
		{"project chat members", func() error { _, err := store.ListProjectChatMembers(ctx, actor, projectID, entityID); return err }},
		{"project chat member add", func() error {
			_, err := store.AddProjectChatMember(ctx, project.ChatMemberCommand{Actor: actor})
			return err
		}},
		{"project chat member remove", func() error { return store.RemoveProjectChatMember(ctx, project.ChatMemberCommand{Actor: actor}) }},
		{"project chat read", func() error {
			return store.MarkProjectChatRead(ctx, project.MarkChatReadCommand{Actor: actor, ProjectID: projectID, ChatID: entityID, MessageID: entityID})
		}},
		{"account notifications", func() error { _, err := store.ListAccountNotifications(ctx, actor, 20); return err }},
		{"account notification read", func() error { return store.MarkAccountNotificationRead(ctx, actor, entityID, now) }},
		{"account notifications read all", func() error { return store.MarkAllAccountNotificationsRead(ctx, actor, now) }},
		{"project documents", func() error { _, err := store.ListProjectDocuments(ctx, actor, projectID); return err }},
		{"project document upload", func() error {
			_, err := store.UploadProjectDocument(ctx, project.UploadProjectDocumentCommand{Actor: actor})
			return err
		}},
		{"project document download", func() error { _, err := store.DownloadProjectDocument(ctx, actor, projectID, entityID); return err }},
		{"project document delete", func() error {
			return store.DeleteProjectDocument(ctx, project.DeleteProjectDocumentCommand{Actor: actor})
		}},
		{"global chats", func() error {
			_, err := store.ListGlobalChats(ctx, globalchat.ListQuery{Actor: globalchat.Actor{UserID: actor.UserID}, PageSize: 10})
			return err
		}},
		{"global chat candidates", func() error { _, err := store.SearchGlobalChatCandidates(ctx, actor.UserID, "user"); return err }},
		{"global chat create", func() error {
			_, err := store.CreateGlobalChat(ctx, globalchat.CreateCommand{Actor: globalchat.Actor{UserID: actor.UserID}})
			return err
		}},
		{"global chat get", func() error {
			_, err := store.GetGlobalChat(ctx, globalchat.ConversationQuery{Actor: globalchat.Actor{UserID: actor.UserID}, ChatID: entityID})
			return err
		}},
		{"global chat message", func() error {
			_, err := store.CreateGlobalChatMessage(ctx, globalchat.SendCommand{Actor: globalchat.Actor{UserID: actor.UserID}})
			return err
		}},
		{"global chat read", func() error {
			return store.MarkGlobalChatRead(ctx, globalchat.Actor{UserID: actor.UserID}, entityID, now)
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); err == nil {
				t.Fatal("database outage must be returned")
			}
		})
	}
}

func bindProjectTelegram(t *testing.T, pool *pgxpool.Pool, userID string, chatID int64, enabled bool, now time.Time) {
	t.Helper()
	username := fmt.Sprintf("project_notify_%d", chatID)
	if _, err := pool.Exec(context.Background(), `INSERT INTO telegram_subscribers (chat_id,username,user_id,verified_at) VALUES ($1,$2,$3,$4)`, chatID, username, userID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO telegram_account_bindings (user_id,chat_id,chat_username,verified_at) VALUES ($1,$2,$3,$4)`, userID, chatID, username, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO account_notification_preferences (user_id,telegram_events_enabled,updated_at) VALUES ($1,$2,$3)`, userID, enabled, now); err != nil {
		t.Fatal(err)
	}
}

func assertProjectNotificationRecipients(t *testing.T, pool *pgxpool.Pool, cipher account.PayloadCipher, messageType, eventID string, expected []string, textFragment string) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT recipient_user_id::text,payload_ciphertext,payload_key_version
		FROM notification_outbox
		WHERE message_type=$1 AND (metadata->>'eventId'=$2 OR metadata->>'taskId'=$2 OR metadata->>'messageId'=$2)
		ORDER BY recipient_user_id
	`, messageType, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	actual := make([]string, 0, len(expected))
	for rows.Next() {
		var userID string
		var ciphertext []byte
		var keyVersion int
		if err = rows.Scan(&userID, &ciphertext, &keyVersion); err != nil {
			t.Fatal(err)
		}
		plaintext, decryptErr := cipher.Decrypt(ciphertext, messageType, keyVersion)
		if decryptErr != nil {
			t.Fatal(decryptErr)
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err = json.Unmarshal(plaintext, &payload); err != nil || !strings.Contains(payload.Text, textFragment) {
			t.Fatalf("notification payload=%q error=%v", payload.Text, err)
		}
		actual = append(actual, userID)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(actual)
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		t.Fatalf("%s recipients=%v expected=%v", messageType, actual, expected)
	}
}

func insertProjectTestUser(t *testing.T, pool *pgxpool.Pool, login, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO users (login,login_normalized,email,email_normalized,status) VALUES ($1,$1,$2,$2,'active') RETURNING id::text`, login, email).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO profiles (user_id,professional_role_id,first_name) SELECT $1,id,$2 FROM professional_roles WHERE code='customer'`, id, login); err != nil {
		t.Fatal(err)
	}
	return id
}
