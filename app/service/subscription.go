package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/repository"
	"github.com/vibast-solutions/ms-go-subscriptions/config"
)

type listSubscriptionTypesRequest interface {
	GetType() string
	GetHasStatus() bool
	GetStatus() int32
}

type createSubscriptionRequest interface {
	GetSubscriptionTypeId() uint64
	GetUserId() string
	GetEmail() string
	GetStartAt() string
	GetAutoRenew() bool
}

type updateSubscriptionRequest interface {
	GetId() uint64
	GetHasAutoRenew() bool
	GetAutoRenew() bool
	GetHasStatus() bool
	GetStatus() int32
}

type listSubscriptionsRequest interface {
	GetUserId() string
	GetEmail() string
}

type paymentCallbackRequest interface {
	GetSubscriptionId() uint64
	GetStatus() string
	GetTransactionId() string
}

type CreateResult struct {
	Subscription *entity.Subscription
	PaymentURL   string
}

type SubscriptionService struct {
	subscriptionRepo     subscriptionRepository
	subscriptionTypeRepo subscriptionTypeRepository
	planTypeRepo         planTypeRepository
	paymentService       payment.Service
	cfg                  config.SubscriptionConfig
}

type subscriptionRepository interface {
	Create(ctx context.Context, subscription *entity.Subscription) error
	Update(ctx context.Context, subscription *entity.Subscription) error
	FindByID(ctx context.Context, id uint64) (*entity.Subscription, error)
	FindByTypeAndIdentity(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error)
	List(ctx context.Context, userID, email string) ([]*entity.Subscription, error)
	ListDueAutoRenew(ctx context.Context, now time.Time) ([]*entity.Subscription, error)
	ListPendingPaymentStale(ctx context.Context, cutoff time.Time) ([]*entity.Subscription, error)
	ListExpiredActive(ctx context.Context, now time.Time) ([]*entity.Subscription, error)
}

type subscriptionTypeRepository interface {
	List(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error)
	FindByID(ctx context.Context, id uint64) (*entity.SubscriptionType, error)
}

type planTypeRepository interface {
	FindBySubscriptionTypeID(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error)
}

func NewSubscriptionService(
	subscriptionRepo subscriptionRepository,
	subscriptionTypeRepo subscriptionTypeRepository,
	planTypeRepo planTypeRepository,
	paymentService payment.Service,
	cfg config.SubscriptionConfig,
) *SubscriptionService {
	return &SubscriptionService{
		subscriptionRepo:     subscriptionRepo,
		subscriptionTypeRepo: subscriptionTypeRepo,
		planTypeRepo:         planTypeRepo,
		paymentService:       paymentService,
		cfg:                  cfg,
	}
}

func (s *SubscriptionService) ListSubscriptionTypes(ctx context.Context, req listSubscriptionTypesRequest) ([]*entity.SubscriptionType, error) {
	if req.GetHasStatus() && !isSubscriptionTypeStatusAllowed(req.GetStatus()) {
		return nil, ErrInvalidStatus
	}

	items, err := s.subscriptionTypeRepo.List(ctx, strings.TrimSpace(req.GetType()), req.GetHasStatus(), req.GetStatus())
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SubscriptionService) CreateSubscription(ctx context.Context, req createSubscriptionRequest) (*CreateResult, error) {
	userID := normalizeOptionalString(req.GetUserId())
	email := normalizeOptionalString(req.GetEmail())
	if userID == nil && email == nil {
		return nil, fmt.Errorf("%w: at least one of user_id or email is required", ErrInvalidRequest)
	}

	subscriptionType, err := s.subscriptionTypeRepo.FindByID(ctx, req.GetSubscriptionTypeId())
	if err != nil {
		return nil, err
	}
	if subscriptionType == nil || subscriptionType.Status != 10 {
		return nil, ErrSubscriptionTypeNotFound
	}

	planType, err := s.planTypeRepo.FindBySubscriptionTypeID(ctx, req.GetSubscriptionTypeId())
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	subscription, err := s.subscriptionRepo.FindByTypeAndIdentity(ctx, req.GetSubscriptionTypeId(), userID, email)
	if err != nil {
		return nil, err
	}

	isNew := subscription == nil
	if isNew {
		subscription = &entity.Subscription{
			SubscriptionTypeID: req.GetSubscriptionTypeId(),
			UserID:             userID,
			Email:              email,
			CreatedAt:          now,
		}
	}

	subscription.SubscriptionTypeID = req.GetSubscriptionTypeId()
	subscription.UserID = userID
	subscription.Email = email
	subscription.AutoRenew = req.GetAutoRenew()

	if planType != nil {
		startAt, err := parseStartAt(req.GetStartAt())
		if err != nil {
			return nil, err
		}
		subscription.StartAt = &startAt
		endAt := startAt.Add(time.Duration(planType.DurationDays) * 24 * time.Hour)
		subscription.EndAt = &endAt
		if subscription.AutoRenew {
			renewAt := endAt.Add(-s.cfg.RenewBeforeEndMinutes)
			subscription.RenewAt = &renewAt
		} else {
			subscription.RenewAt = nil
		}
		subscription.Status = entity.SubscriptionStatusProcessing
	} else {
		subscription.StartAt = nil
		subscription.EndAt = nil
		subscription.RenewAt = nil
		subscription.AutoRenew = false
		subscription.Status = entity.SubscriptionStatusActive
	}
	subscription.UpdatedAt = now

	if isNew {
		if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
			if errors.Is(err, repository.ErrSubscriptionAlreadyExists) {
				return nil, ErrSubscriptionAlreadyExists
			}
			return nil, err
		}
	} else {
		if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
			if errors.Is(err, repository.ErrSubscriptionNotFound) {
				return nil, ErrSubscriptionNotFound
			}
			return nil, err
		}
	}

	result := &CreateResult{Subscription: subscription}
	if planType == nil {
		return result, nil
	}

	payResult, err := s.processPaymentSafely(ctx, subscription.ID, planType.ID, subscription.UserID, subscription.Email)
	if err != nil {
		return nil, err
	}

	now = time.Now().UTC()
	switch payResult.Type {
	case payment.ResultTypeSuccess:
		subscription.Status = entity.SubscriptionStatusActive
	case payment.ResultTypeRedirect:
		subscription.Status = entity.SubscriptionStatusPendingPayment
		result.PaymentURL = payResult.PaymentURL
	case payment.ResultTypeFailure:
		subscription.Status = entity.SubscriptionStatusProcessing
		renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
		subscription.RenewAt = &renewAt
	default:
		subscription.Status = entity.SubscriptionStatusProcessing
		renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
		subscription.RenewAt = &renewAt
	}
	subscription.UpdatedAt = now

	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return result, nil
}

func (s *SubscriptionService) GetSubscription(ctx context.Context, id uint64) (*entity.Subscription, error) {
	subscription, err := s.subscriptionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}
	return subscription, nil
}

func (s *SubscriptionService) ListSubscriptions(ctx context.Context, req listSubscriptionsRequest) ([]*entity.Subscription, error) {
	items, err := s.subscriptionRepo.List(ctx, strings.TrimSpace(req.GetUserId()), strings.TrimSpace(req.GetEmail()))
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SubscriptionService) UpdateSubscription(ctx context.Context, req updateSubscriptionRequest) (*entity.Subscription, error) {
	subscription, err := s.subscriptionRepo.FindByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}
	if !req.GetHasAutoRenew() && !req.GetHasStatus() {
		return nil, ErrNoFieldsToUpdate
	}

	if req.GetHasStatus() {
		if !isSubscriptionStatusAllowed(req.GetStatus()) {
			return nil, ErrInvalidStatus
		}
		subscription.Status = req.GetStatus()
	}
	if req.GetHasAutoRenew() {
		subscription.AutoRenew = req.GetAutoRenew()
		if !subscription.AutoRenew {
			subscription.RenewAt = nil
		} else if subscription.EndAt != nil {
			renewAt := subscription.EndAt.Add(-s.cfg.RenewBeforeEndMinutes)
			subscription.RenewAt = &renewAt
		}
	}
	if subscription.Status == entity.SubscriptionStatusInactive {
		subscription.AutoRenew = false
		subscription.RenewAt = nil
	}

	subscription.UpdatedAt = time.Now().UTC()
	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return subscription, nil
}

func (s *SubscriptionService) DeleteSubscription(ctx context.Context, id uint64) (*entity.Subscription, error) {
	subscription, err := s.subscriptionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}

	subscription.Status = entity.SubscriptionStatusInactive
	subscription.AutoRenew = false
	subscription.RenewAt = nil
	subscription.UpdatedAt = time.Now().UTC()

	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return subscription, nil
}

func (s *SubscriptionService) CancelSubscription(ctx context.Context, id uint64) (*entity.Subscription, error) {
	subscription, err := s.subscriptionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, ErrSubscriptionNotFound
	}

	subscription.AutoRenew = false
	subscription.RenewAt = nil
	subscription.UpdatedAt = time.Now().UTC()

	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return subscription, nil
}

func (s *SubscriptionService) PaymentCallback(ctx context.Context, req paymentCallbackRequest) error {
	subscription, err := s.subscriptionRepo.FindByID(ctx, req.GetSubscriptionId())
	if err != nil {
		return err
	}
	if subscription == nil {
		return ErrSubscriptionNotFound
	}

	now := time.Now().UTC()
	switch strings.ToLower(strings.TrimSpace(req.GetStatus())) {
	case "success":
		subscription.Status = entity.SubscriptionStatusActive
	case "failed":
		subscription.Status = entity.SubscriptionStatusProcessing
		renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
		subscription.RenewAt = &renewAt
	default:
		return fmt.Errorf("%w: invalid callback status", ErrInvalidRequest)
	}
	subscription.UpdatedAt = now

	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		if errors.Is(err, repository.ErrSubscriptionNotFound) {
			return ErrSubscriptionNotFound
		}
		return err
	}

	return nil
}

func (s *SubscriptionService) RunAutoRenewalBatch(ctx context.Context) error {
	now := time.Now().UTC()
	items, err := s.subscriptionRepo.ListDueAutoRenew(ctx, now)
	if err != nil {
		return err
	}

	for _, item := range items {
		item.Status = entity.SubscriptionStatusProcessing
		item.UpdatedAt = now
		if err := s.subscriptionRepo.Update(ctx, item); err != nil {
			continue
		}

		planType, err := s.planTypeRepo.FindBySubscriptionTypeID(ctx, item.SubscriptionTypeID)
		if err != nil || planType == nil {
			item.Status = entity.SubscriptionStatusInactive
			item.AutoRenew = false
			item.RenewAt = nil
			item.UpdatedAt = time.Now().UTC()
			_ = s.subscriptionRepo.Update(ctx, item)
			continue
		}

		payResult, err := s.processPaymentSafely(ctx, item.ID, planType.ID, item.UserID, item.Email)
		now = time.Now().UTC()
		if err != nil {
			renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
			item.RenewAt = &renewAt
			item.Status = entity.SubscriptionStatusProcessing
			item.UpdatedAt = now
			if shouldDeactivateForRetryAge(item, s.cfg.MaxRenewalRetryAgeMinutes) {
				item.Status = entity.SubscriptionStatusInactive
				item.AutoRenew = false
				item.RenewAt = nil
			}
			_ = s.subscriptionRepo.Update(ctx, item)
			continue
		}

		switch payResult.Type {
		case payment.ResultTypeSuccess:
			item.Status = entity.SubscriptionStatusActive
			base := now
			if item.EndAt != nil {
				base = *item.EndAt
			}
			newEnd := base.Add(time.Duration(planType.DurationDays) * 24 * time.Hour)
			item.EndAt = &newEnd
			if item.AutoRenew {
				renewAt := newEnd.Add(-s.cfg.RenewBeforeEndMinutes)
				item.RenewAt = &renewAt
			}
		case payment.ResultTypeRedirect:
			item.Status = entity.SubscriptionStatusPendingPayment
			renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
			item.RenewAt = &renewAt
		case payment.ResultTypeFailure:
			item.Status = entity.SubscriptionStatusProcessing
			renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
			item.RenewAt = &renewAt
		}

		if shouldDeactivateForRetryAge(item, s.cfg.MaxRenewalRetryAgeMinutes) {
			item.Status = entity.SubscriptionStatusInactive
			item.AutoRenew = false
			item.RenewAt = nil
		}

		item.UpdatedAt = now
		_ = s.subscriptionRepo.Update(ctx, item)
	}

	return nil
}

func (s *SubscriptionService) RunPendingPaymentCleanupBatch(ctx context.Context) error {
	now := time.Now().UTC()
	cutoff := now.Add(-s.cfg.PendingPaymentTimeout)
	items, err := s.subscriptionRepo.ListPendingPaymentStale(ctx, cutoff)
	if err != nil {
		return err
	}

	for _, item := range items {
		item.Status = entity.SubscriptionStatusProcessing
		if item.RenewAt == nil || item.RenewAt.Before(now) {
			renewAt := now.Add(s.cfg.RenewalRetryIntervalMinutes)
			item.RenewAt = &renewAt
		}
		item.UpdatedAt = now
		_ = s.subscriptionRepo.Update(ctx, item)
	}

	return nil
}

func (s *SubscriptionService) RunExpirationBatch(ctx context.Context) error {
	now := time.Now().UTC()
	items, err := s.subscriptionRepo.ListExpiredActive(ctx, now)
	if err != nil {
		return err
	}

	for _, item := range items {
		item.Status = entity.SubscriptionStatusInactive
		item.AutoRenew = false
		item.RenewAt = nil
		item.UpdatedAt = now
		_ = s.subscriptionRepo.Update(ctx, item)
	}

	return nil
}

func parseStartAt(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, ErrStartAtRequired
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid start_at format", ErrInvalidRequest)
	}
	return t.UTC(), nil
}

func normalizeOptionalString(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isSubscriptionStatusAllowed(status int32) bool {
	switch status {
	case entity.SubscriptionStatusInactive,
		entity.SubscriptionStatusProcessing,
		entity.SubscriptionStatusPendingPayment,
		entity.SubscriptionStatusActive:
		return true
	default:
		return false
	}
}

func isSubscriptionTypeStatusAllowed(status int32) bool {
	return status == 0 || status == 10
}

func (s *SubscriptionService) processPaymentSafely(ctx context.Context, subscriptionID, planTypeID uint64, userID, email *string) (_ payment.Result, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("payment processing failed: %v", rec)
		}
	}()

	return s.paymentService.ProcessSubscriptionPayment(ctx, subscriptionID, planTypeID, userID, email), nil
}

func shouldDeactivateForRetryAge(item *entity.Subscription, maxRetryAge time.Duration) bool {
	if item.EndAt == nil || item.RenewAt == nil {
		return false
	}
	return item.RenewAt.Sub(*item.EndAt) > maxRetryAge
}
