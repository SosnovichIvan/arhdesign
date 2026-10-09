package project

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const maxExpenseMinor int64 = 99_999_999_999_999

var allowedExpenseCategories = map[string]struct{}{
	"materials": {}, "furniture": {}, "contractor": {}, "delivery": {},
	"installation": {}, "design": {}, "other": {},
}

type FinanceSummary struct {
	CurrencyCode                                string
	ConfirmedIncomeMinor, ConfirmedExpenseMinor int64
	AvailableBalanceMinor, PendingExpenseMinor  int64
	CanCreateExpense                            bool
}

type Expense struct {
	ID, ProjectID, CurrencyCode, Category, Description, Status, CreatedByUserID string
	VendorName, ContextChatID                                                   *string
	PlannedPaymentOn                                                            *time.Time
	AmountMinor                                                                 int64
	CreatedAt                                                                   time.Time
	Version                                                                     int64
}

type CreateExpenseRequest struct {
	AmountMinor      int64
	Category         string
	Description      string
	VendorName       *string
	PlannedPaymentOn *time.Time
	IdempotencyKey   string
}

type CreateExpenseCommand struct {
	Actor
	ProjectID, RequestID string
	Request              CreateExpenseRequest
	RequestHash          [32]byte
	Now                  time.Time
	Notification         *EventNotification
}

type ExpenseList struct {
	Items            []Expense
	CanCreateExpense bool
}

func (service *Service) FinanceSummary(ctx context.Context, actor Actor, projectID string) (FinanceSummary, error) {
	if !validUUID(actor.UserID) {
		return FinanceSummary{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return FinanceSummary{}, ErrNotFound
	}
	return service.store.GetProjectFinanceSummary(ctx, actor, projectID)
}

func (service *Service) ListExpenses(ctx context.Context, actor Actor, projectID string) (ExpenseList, error) {
	if !validUUID(actor.UserID) {
		return ExpenseList{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return ExpenseList{}, ErrNotFound
	}
	items, canCreate, err := service.store.ListProjectExpenses(ctx, actor, projectID)
	return ExpenseList{Items: items, CanCreateExpense: canCreate}, err
}

func (service *Service) CreateExpense(ctx context.Context, actor Actor, projectID, requestID string, request CreateExpenseRequest) (Expense, error) {
	request.Category = strings.TrimSpace(request.Category)
	request.Description = strings.TrimSpace(request.Description)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.VendorName != nil {
		value := strings.TrimSpace(*request.VendorName)
		if value == "" {
			request.VendorName = nil
		} else {
			request.VendorName = &value
		}
	}
	if !validUUID(actor.UserID) || len(requestID) < 8 || request.AmountMinor < 1 || request.AmountMinor > maxExpenseMinor || len([]rune(request.Description)) < 2 || len([]rune(request.Description)) > 2000 || len(request.IdempotencyKey) < 8 || len(request.IdempotencyKey) > 128 {
		return Expense{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return Expense{}, ErrNotFound
	}
	if _, ok := allowedExpenseCategories[request.Category]; !ok {
		return Expense{}, ErrInvalidInput
	}
	if request.VendorName != nil && len([]rune(*request.VendorName)) > 200 {
		return Expense{}, ErrInvalidInput
	}
	hash := expenseFingerprint(projectID, actor.UserID, request)
	notification, err := service.eventNotification("project.expense.created", "Новый расход по проекту\nОписание: "+request.Description+"\nСумма: "+formatMinorAmount(request.AmountMinor))
	if err != nil {
		return Expense{}, err
	}
	return service.store.CreateProjectExpense(ctx, CreateExpenseCommand{Actor: actor, ProjectID: projectID, RequestID: requestID, Request: request, RequestHash: hash, Now: service.now().UTC(), Notification: notification})
}

func formatMinorAmount(value int64) string {
	return fmt.Sprintf("%d.%02d", value/100, value%100)
}

func expenseFingerprint(projectID, userID string, request CreateExpenseRequest) [32]byte {
	buffer := make([]byte, 0, 512)
	appendPart := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		buffer = append(buffer, size[:]...)
		buffer = append(buffer, value...)
	}
	appendPart(projectID)
	appendPart(userID)
	appendPart(request.Category)
	appendPart(request.Description)
	if request.VendorName != nil {
		appendPart(*request.VendorName)
	} else {
		appendPart("")
	}
	if request.PlannedPaymentOn != nil {
		appendPart(request.PlannedPaymentOn.UTC().Format("2006-01-02"))
	} else {
		appendPart("")
	}
	var amount [8]byte
	binary.BigEndian.PutUint64(amount[:], uint64(request.AmountMinor))
	buffer = append(buffer, amount[:]...)
	return sha256.Sum256(buffer)
}
