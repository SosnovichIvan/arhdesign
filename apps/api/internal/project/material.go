package project

import (
	"context"
	"strings"
	"time"
)

var allowedMaterialCategories = map[string]struct{}{
	"materials": {}, "furniture": {}, "delivery": {}, "installation": {}, "other": {},
}

var allowedMaterialPaymentStatuses = map[string]struct{}{"unpaid": {}, "partial": {}, "paid": {}}
var allowedMaterialDeliveryStatuses = map[string]struct{}{"expected": {}, "partially_delivered": {}, "delivered": {}, "installed": {}, "cancelled": {}}

type Material struct {
	ID, ProjectID, Category, Name, SupplierName, CurrencyCode, PaymentStatus, DeliveryStatus, CreatedByUserID string
	ContactInfo, ContractReference, LinkedExpenseID, LinkedExpenseDescription, Notes, ContextChatID           *string
	PlannedDeliveryOn, ActualDeliveryOn, InstallationOn                                                       *time.Time
	ContractAmountMinor, PaidAmountMinor, RemainingAmountMinor                                                int64
	CreatedAt, UpdatedAt                                                                                      time.Time
	Version                                                                                                   int64
	CanEdit, CanDelete                                                                                        bool
}

type MaterialInput struct {
	Category, Name, SupplierName, PaymentStatus, DeliveryStatus string
	ContactInfo, ContractReference, LinkedExpenseID, Notes      *string
	PlannedDeliveryOn, ActualDeliveryOn, InstallationOn         *time.Time
	ContractAmountMinor                                         int64
}

type CreateMaterialCommand struct {
	Actor
	ProjectID, RequestID string
	Input                MaterialInput
	Now                  time.Time
	Notification         *EventNotification
}

type UpdateMaterialCommand struct {
	Actor
	ProjectID, MaterialID, RequestID string
	ExpectedVersion                  int64
	Input                            MaterialInput
	Now                              time.Time
	Notification                     *EventNotification
}

type DeleteMaterialCommand struct {
	Actor
	ProjectID, MaterialID, RequestID, Reason string
	ExpectedVersion                          int64
	Now                                      time.Time
	Notification                             *EventNotification
}

type MaterialList struct {
	Items                                  []Material
	CanCreate, CanViewFinancialInformation bool
}

func (service *Service) ListMaterials(ctx context.Context, actor Actor, projectID string) (MaterialList, error) {
	if !validUUID(actor.UserID) {
		return MaterialList{}, ErrInvalidInput
	}
	if !validUUID(projectID) {
		return MaterialList{}, ErrNotFound
	}
	items, canCreate, canViewFinancials, err := service.store.ListProjectMaterials(ctx, actor, projectID)
	return MaterialList{Items: items, CanCreate: canCreate, CanViewFinancialInformation: canViewFinancials}, err
}

func (service *Service) CreateMaterial(ctx context.Context, actor Actor, projectID, requestID string, input MaterialInput) (Material, error) {
	input = normalizeMaterialInput(input)
	if !validUUID(actor.UserID) || !validUUID(projectID) || len(requestID) < 8 || validateMaterialInput(input) != nil {
		return Material{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.material.created", "В проект добавлен материал\nМатериал: "+input.Name+"\nПоставщик: "+input.SupplierName)
	if err != nil {
		return Material{}, err
	}
	return service.store.CreateProjectMaterial(ctx, CreateMaterialCommand{Actor: actor, ProjectID: projectID, RequestID: requestID, Input: input, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) UpdateMaterial(ctx context.Context, actor Actor, projectID, materialID, requestID string, expectedVersion int64, input MaterialInput) (Material, error) {
	input = normalizeMaterialInput(input)
	if !validUUID(actor.UserID) || !validUUID(projectID) || !validUUID(materialID) || len(requestID) < 8 || expectedVersion < 1 || validateMaterialInput(input) != nil {
		return Material{}, ErrInvalidInput
	}
	notification, err := service.eventNotification("project.material.updated", "Материал проекта изменён\nМатериал: "+input.Name+"\nПоставщик: "+input.SupplierName)
	if err != nil {
		return Material{}, err
	}
	return service.store.UpdateProjectMaterial(ctx, UpdateMaterialCommand{Actor: actor, ProjectID: projectID, MaterialID: materialID, RequestID: requestID, ExpectedVersion: expectedVersion, Input: input, Now: service.now().UTC(), Notification: notification})
}

func (service *Service) DeleteMaterial(ctx context.Context, actor Actor, projectID, materialID, requestID string, expectedVersion int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if !validUUID(actor.UserID) || !validUUID(projectID) || !validUUID(materialID) || len(requestID) < 8 || expectedVersion < 1 || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 {
		return ErrInvalidInput
	}
	notification, notifyErr := service.eventNotification("project.material.deleted", "Материал удалён из проекта\nПричина: "+reason)
	if notifyErr != nil {
		return notifyErr
	}
	return service.store.DeleteProjectMaterial(ctx, DeleteMaterialCommand{Actor: actor, ProjectID: projectID, MaterialID: materialID, RequestID: requestID, Reason: reason, ExpectedVersion: expectedVersion, Now: service.now().UTC(), Notification: notification})
}

func normalizeMaterialInput(input MaterialInput) MaterialInput {
	input.Category = strings.TrimSpace(input.Category)
	input.Name = strings.TrimSpace(input.Name)
	input.SupplierName = strings.TrimSpace(input.SupplierName)
	input.PaymentStatus = strings.TrimSpace(input.PaymentStatus)
	input.DeliveryStatus = strings.TrimSpace(input.DeliveryStatus)
	input.ContactInfo = trimOptional(input.ContactInfo)
	input.ContractReference = trimOptional(input.ContractReference)
	input.LinkedExpenseID = trimOptional(input.LinkedExpenseID)
	input.Notes = trimOptional(input.Notes)
	return input
}

func validateMaterialInput(input MaterialInput) error {
	if _, ok := allowedMaterialCategories[input.Category]; !ok {
		return ErrInvalidInput
	}
	if _, ok := allowedMaterialPaymentStatuses[input.PaymentStatus]; !ok {
		return ErrInvalidInput
	}
	if _, ok := allowedMaterialDeliveryStatuses[input.DeliveryStatus]; !ok {
		return ErrInvalidInput
	}
	if len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 300 || len([]rune(input.SupplierName)) < 2 || len([]rune(input.SupplierName)) > 200 || input.ContractAmountMinor < 1 || input.ContractAmountMinor > maxExpenseMinor {
		return ErrInvalidInput
	}
	if input.ContactInfo != nil && len([]rune(*input.ContactInfo)) > 500 {
		return ErrInvalidInput
	}
	if input.ContractReference != nil && len([]rune(*input.ContractReference)) > 200 {
		return ErrInvalidInput
	}
	if input.Notes != nil && len([]rune(*input.Notes)) > 5000 {
		return ErrInvalidInput
	}
	if input.LinkedExpenseID != nil && !validUUID(*input.LinkedExpenseID) {
		return ErrInvalidInput
	}
	if input.ActualDeliveryOn != nil && input.DeliveryStatus != "delivered" && input.DeliveryStatus != "installed" {
		return ErrInvalidInput
	}
	return nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
