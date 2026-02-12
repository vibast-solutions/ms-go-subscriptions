package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/repository"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"github.com/vibast-solutions/ms-go-subscriptions/config"
)

type mockSubscriptionRepo struct {
	createFn                func(ctx context.Context, subscription *entity.Subscription) error
	updateFn                func(ctx context.Context, subscription *entity.Subscription) error
	findByIDFn              func(ctx context.Context, id uint64) (*entity.Subscription, error)
	findByTypeAndIdentityFn func(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error)
	listFn                  func(ctx context.Context, userID, email string) ([]*entity.Subscription, error)
	listDueAutoRenewFn      func(ctx context.Context, now time.Time) ([]*entity.Subscription, error)
	listPendingPaymentFn    func(ctx context.Context, cutoff time.Time) ([]*entity.Subscription, error)
	listExpiredActiveFn     func(ctx context.Context, now time.Time) ([]*entity.Subscription, error)
}

func (m *mockSubscriptionRepo) Create(ctx context.Context, subscription *entity.Subscription) error {
	if m.createFn != nil {
		return m.createFn(ctx, subscription)
	}
	return nil
}

func (m *mockSubscriptionRepo) Update(ctx context.Context, subscription *entity.Subscription) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, subscription)
	}
	return nil
}

func (m *mockSubscriptionRepo) FindByID(ctx context.Context, id uint64) (*entity.Subscription, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) FindByTypeAndIdentity(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error) {
	if m.findByTypeAndIdentityFn != nil {
		return m.findByTypeAndIdentityFn(ctx, subscriptionTypeID, userID, email)
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) List(ctx context.Context, userID, email string) ([]*entity.Subscription, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID, email)
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) ListDueAutoRenew(ctx context.Context, now time.Time) ([]*entity.Subscription, error) {
	if m.listDueAutoRenewFn != nil {
		return m.listDueAutoRenewFn(ctx, now)
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) ListPendingPaymentStale(ctx context.Context, cutoff time.Time) ([]*entity.Subscription, error) {
	if m.listPendingPaymentFn != nil {
		return m.listPendingPaymentFn(ctx, cutoff)
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) ListExpiredActive(ctx context.Context, now time.Time) ([]*entity.Subscription, error) {
	if m.listExpiredActiveFn != nil {
		return m.listExpiredActiveFn(ctx, now)
	}
	return nil, nil
}

type mockSubscriptionTypeRepo struct {
	listFn     func(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error)
	findByIDFn func(ctx context.Context, id uint64) (*entity.SubscriptionType, error)
}

func (m *mockSubscriptionTypeRepo) List(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error) {
	if m.listFn != nil {
		return m.listFn(ctx, typeFilter, hasStatus, status)
	}
	return nil, nil
}

func (m *mockSubscriptionTypeRepo) FindByID(ctx context.Context, id uint64) (*entity.SubscriptionType, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

type mockPlanTypeRepo struct {
	findBySubscriptionTypeIDFn func(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error)
}

func (m *mockPlanTypeRepo) FindBySubscriptionTypeID(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error) {
	if m.findBySubscriptionTypeIDFn != nil {
		return m.findBySubscriptionTypeIDFn(ctx, subscriptionTypeID)
	}
	return nil, nil
}

type fakePaymentService struct {
	result      payment.Result
	panicWith   string
	calledCount int
}

func (f *fakePaymentService) ProcessSubscriptionPayment(_ context.Context, _ uint64, _ uint64, _ *string, _ *string) payment.Result {
	f.calledCount++
	if f.panicWith != "" {
		panic(f.panicWith)
	}
	return f.result
}

func testConfig() config.SubscriptionConfig {
	return config.SubscriptionConfig{
		RenewBeforeEndMinutes:       2 * time.Hour,
		RenewalRetryIntervalMinutes: 30 * time.Minute,
		MaxRenewalRetryAgeMinutes:   2 * time.Hour,
		PendingPaymentTimeout:       10 * time.Minute,
	}
}

func copySubscription(src *entity.Subscription) *entity.Subscription {
	if src == nil {
		return nil
	}
	cp := *src
	if src.UserID != nil {
		v := *src.UserID
		cp.UserID = &v
	}
	if src.Email != nil {
		v := *src.Email
		cp.Email = &v
	}
	if src.StartAt != nil {
		v := *src.StartAt
		cp.StartAt = &v
	}
	if src.EndAt != nil {
		v := *src.EndAt
		cp.EndAt = &v
	}
	if src.RenewAt != nil {
		v := *src.RenewAt
		cp.RenewAt = &v
	}
	return &cp
}

func TestListSubscriptionTypesRejectsInvalidStatus(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.ListSubscriptionTypes(context.Background(), &types.ListSubscriptionTypesRequest{HasStatus: true, Status: 7})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestCreateSubscriptionRequiresIdentity(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 1})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestCreateSubscriptionTypeNotFound(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{},
		&mockSubscriptionTypeRepo{
			findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
				return nil, nil
			},
		},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u-1"})
	if !errors.Is(err, ErrSubscriptionTypeNotFound) {
		t.Fatalf("expected ErrSubscriptionTypeNotFound, got %v", err)
	}
}

func TestCreateEmailSubscriptionSuccess(t *testing.T) {
	createdCount := 0
	updatedCount := 0
	var created *entity.Subscription
	paymentSvc := &fakePaymentService{}

	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			findByTypeAndIdentityFn: func(_ context.Context, _ uint64, _ *string, _ *string) (*entity.Subscription, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, subscription *entity.Subscription) error {
				createdCount++
				subscription.ID = 42
				created = copySubscription(subscription)
				return nil
			},
			updateFn: func(_ context.Context, _ *entity.Subscription) error {
				updatedCount++
				return nil
			},
		},
		&mockSubscriptionTypeRepo{
			findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
				return &entity.SubscriptionType{ID: 1, Status: 10, Type: "email"}, nil
			},
		},
		&mockPlanTypeRepo{},
		paymentSvc,
		testConfig(),
	)

	res, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{
		SubscriptionTypeId: 1,
		UserId:             "u-1",
		Email:              "a@example.com",
		AutoRenew:          true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.PaymentURL != "" {
		t.Fatalf("expected no payment url, got %q", res.PaymentURL)
	}
	if res.Subscription == nil || res.Subscription.ID != 42 {
		t.Fatalf("unexpected subscription: %+v", res.Subscription)
	}
	if res.Subscription.Status != entity.SubscriptionStatusActive {
		t.Fatalf("expected active status, got %d", res.Subscription.Status)
	}
	if res.Subscription.AutoRenew {
		t.Fatalf("expected auto_renew disabled for email subscription")
	}
	if created == nil || created.StartAt != nil || created.EndAt != nil || created.RenewAt != nil {
		t.Fatalf("expected nil plan dates for email subscription, got %+v", created)
	}
	if createdCount != 1 || updatedCount != 0 {
		t.Fatalf("unexpected create/update count: %d/%d", createdCount, updatedCount)
	}
	if paymentSvc.calledCount != 0 {
		t.Fatalf("payment service should not be called for email subscriptions")
	}
}

func TestCreatePlanSubscriptionRequiresStartAt(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{},
		&mockSubscriptionTypeRepo{
			findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
				return &entity.SubscriptionType{ID: 2, Status: 10, Type: "plan"}, nil
			},
		},
		&mockPlanTypeRepo{
			findBySubscriptionTypeIDFn: func(_ context.Context, _ uint64) (*entity.PlanType, error) {
				return &entity.PlanType{ID: 10, SubscriptionTypeID: 2, DurationDays: 30}, nil
			},
		},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 2, UserId: "u-1"})
	if !errors.Is(err, ErrStartAtRequired) {
		t.Fatalf("expected ErrStartAtRequired, got %v", err)
	}
}

func TestCreatePlanSubscriptionPaymentRedirect(t *testing.T) {
	paymentSvc := &fakePaymentService{result: payment.Result{Type: payment.ResultTypeRedirect, PaymentURL: "https://pay.local/redirect"}}
	var updates []*entity.Subscription

	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			findByTypeAndIdentityFn: func(_ context.Context, _ uint64, _ *string, _ *string) (*entity.Subscription, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, subscription *entity.Subscription) error {
				subscription.ID = 101
				return nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				updates = append(updates, copySubscription(subscription))
				return nil
			},
		},
		&mockSubscriptionTypeRepo{findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
			return &entity.SubscriptionType{ID: 2, Status: 10, Type: "plan"}, nil
		}},
		&mockPlanTypeRepo{findBySubscriptionTypeIDFn: func(_ context.Context, _ uint64) (*entity.PlanType, error) {
			return &entity.PlanType{ID: 20, SubscriptionTypeID: 2, DurationDays: 30}, nil
		}},
		paymentSvc,
		testConfig(),
	)

	start := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	res, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{
		SubscriptionTypeId: 2,
		UserId:             "u-22",
		StartAt:            start,
		AutoRenew:          true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if paymentSvc.calledCount != 1 {
		t.Fatalf("expected payment to be called once, got %d", paymentSvc.calledCount)
	}
	if res.PaymentURL == "" {
		t.Fatalf("expected payment_url in response")
	}
	if res.Subscription.Status != entity.SubscriptionStatusPendingPayment {
		t.Fatalf("expected pending payment status, got %d", res.Subscription.Status)
	}
	if len(updates) != 1 {
		t.Fatalf("expected one update after payment result, got %d", len(updates))
	}
}

func TestCreateSubscriptionMapsDuplicateError(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			findByTypeAndIdentityFn: func(_ context.Context, _ uint64, _ *string, _ *string) (*entity.Subscription, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, _ *entity.Subscription) error {
				return repository.ErrSubscriptionAlreadyExists
			},
		},
		&mockSubscriptionTypeRepo{findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
			return &entity.SubscriptionType{ID: 1, Status: 10, Type: "email"}, nil
		}},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u-1"})
	if !errors.Is(err, ErrSubscriptionAlreadyExists) {
		t.Fatalf("expected ErrSubscriptionAlreadyExists, got %v", err)
	}
}

func TestCreateSubscriptionPaymentPanicIsHandled(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			findByTypeAndIdentityFn: func(_ context.Context, _ uint64, _ *string, _ *string) (*entity.Subscription, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, subscription *entity.Subscription) error {
				subscription.ID = 9
				return nil
			},
		},
		&mockSubscriptionTypeRepo{findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
			return &entity.SubscriptionType{ID: 2, Status: 10, Type: "plan"}, nil
		}},
		&mockPlanTypeRepo{findBySubscriptionTypeIDFn: func(_ context.Context, _ uint64) (*entity.PlanType, error) {
			return &entity.PlanType{ID: 4, SubscriptionTypeID: 2, DurationDays: 30}, nil
		}},
		&fakePaymentService{panicWith: "payments for renewals are not implemented"},
		testConfig(),
	)

	_, err := svc.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{
		SubscriptionTypeId: 2,
		UserId:             "u-1",
		StartAt:            time.Now().UTC().Format(time.RFC3339),
	})
	if err == nil || !strings.Contains(err.Error(), "payment processing failed") {
		t.Fatalf("expected wrapped payment panic error, got %v", err)
	}
}

func TestUpdateSubscriptionRejectsInvalidStatus(t *testing.T) {
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{findByIDFn: func(_ context.Context, _ uint64) (*entity.Subscription, error) {
			return &entity.Subscription{ID: 1, Status: entity.SubscriptionStatusActive}, nil
		}},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	_, err := svc.UpdateSubscription(context.Background(), &types.UpdateSubscriptionRequest{Id: 1, HasStatus: true, Status: 99})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestDeleteSubscriptionSoftDeletes(t *testing.T) {
	var updated *entity.Subscription
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			findByIDFn: func(_ context.Context, _ uint64) (*entity.Subscription, error) {
				renew := time.Now().UTC().Add(2 * time.Hour)
				return &entity.Subscription{ID: 3, Status: entity.SubscriptionStatusActive, AutoRenew: true, RenewAt: &renew}, nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				updated = copySubscription(subscription)
				return nil
			},
		},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	item, err := svc.DeleteSubscription(context.Background(), 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.Status != entity.SubscriptionStatusInactive || item.AutoRenew {
		t.Fatalf("unexpected returned state: %+v", item)
	}
	if updated == nil || updated.Status != entity.SubscriptionStatusInactive || updated.AutoRenew || updated.RenewAt != nil {
		t.Fatalf("unexpected persisted state: %+v", updated)
	}
}

func TestPaymentCallbackFailedSetsRetry(t *testing.T) {
	var updated *entity.Subscription
	repo := &mockSubscriptionRepo{
		findByIDFn: func(_ context.Context, _ uint64) (*entity.Subscription, error) {
			return &entity.Subscription{ID: 4, Status: entity.SubscriptionStatusPendingPayment}, nil
		},
		updateFn: func(_ context.Context, subscription *entity.Subscription) error {
			updated = copySubscription(subscription)
			return nil
		},
	}
	svc := NewPaymentCallbackService(repo, testConfig())

	err := svc.PaymentCallback(context.Background(), &types.PaymentCallbackRequest{SubscriptionId: 4, Status: "failed", TransactionId: "tx-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil || updated.Status != entity.SubscriptionStatusProcessing || updated.RenewAt == nil {
		t.Fatalf("unexpected updated state: %+v", updated)
	}
}

func TestRunAutoRenewalBatchSuccess(t *testing.T) {
	endAt := time.Now().UTC().Add(24 * time.Hour)
	renewAt := time.Now().UTC().Add(-2 * time.Minute)
	item := &entity.Subscription{
		ID:                 11,
		SubscriptionTypeID: 2,
		Status:             entity.SubscriptionStatusActive,
		AutoRenew:          true,
		EndAt:              &endAt,
		RenewAt:            &renewAt,
	}

	var updates []*entity.Subscription
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			listDueAutoRenewFn: func(_ context.Context, _ time.Time) ([]*entity.Subscription, error) {
				return []*entity.Subscription{item}, nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				updates = append(updates, copySubscription(subscription))
				return nil
			},
		},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{findBySubscriptionTypeIDFn: func(_ context.Context, _ uint64) (*entity.PlanType, error) {
			return &entity.PlanType{ID: 20, SubscriptionTypeID: 2, DurationDays: 30}, nil
		}},
		&fakePaymentService{result: payment.Result{Type: payment.ResultTypeSuccess}},
		testConfig(),
	)

	err := svc.RunAutoRenewalBatch(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(updates) < 2 {
		t.Fatalf("expected at least two updates, got %d", len(updates))
	}
	final := updates[len(updates)-1]
	if final.Status != entity.SubscriptionStatusActive {
		t.Fatalf("expected active status, got %d", final.Status)
	}
	if final.EndAt == nil || !final.EndAt.After(endAt) {
		t.Fatalf("expected extended end_at, got %v (old %v)", final.EndAt, endAt)
	}
	if final.RenewAt == nil {
		t.Fatalf("expected renew_at to be set")
	}
}

func TestRunAutoRenewalBatchDisablesExpiredRetries(t *testing.T) {
	endAt := time.Now().UTC().Add(-72 * time.Hour)
	renewAt := time.Now().UTC().Add(-10 * time.Minute)
	item := &entity.Subscription{
		ID:                 12,
		SubscriptionTypeID: 2,
		Status:             entity.SubscriptionStatusActive,
		AutoRenew:          true,
		EndAt:              &endAt,
		RenewAt:            &renewAt,
	}

	cfg := testConfig()
	cfg.MaxRenewalRetryAgeMinutes = 30 * time.Minute

	var final *entity.Subscription
	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			listDueAutoRenewFn: func(_ context.Context, _ time.Time) ([]*entity.Subscription, error) {
				return []*entity.Subscription{item}, nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				final = copySubscription(subscription)
				return nil
			},
		},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{findBySubscriptionTypeIDFn: func(_ context.Context, _ uint64) (*entity.PlanType, error) {
			return &entity.PlanType{ID: 20, SubscriptionTypeID: 2, DurationDays: 30}, nil
		}},
		&fakePaymentService{result: payment.Result{Type: payment.ResultTypeFailure}},
		cfg,
	)

	err := svc.RunAutoRenewalBatch(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if final == nil {
		t.Fatal("expected final update")
	}
	if final.Status != entity.SubscriptionStatusInactive || final.AutoRenew || final.RenewAt != nil {
		t.Fatalf("expected subscription to be inactivated after max retry age, got %+v", final)
	}
}

func TestRunPendingCleanupBatch(t *testing.T) {
	item := &entity.Subscription{ID: 22, Status: entity.SubscriptionStatusPendingPayment}
	var updated *entity.Subscription

	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			listPendingPaymentFn: func(_ context.Context, _ time.Time) ([]*entity.Subscription, error) {
				return []*entity.Subscription{item}, nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				updated = copySubscription(subscription)
				return nil
			},
		},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	if err := svc.RunPendingPaymentCleanupBatch(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil || updated.Status != entity.SubscriptionStatusProcessing || updated.RenewAt == nil {
		t.Fatalf("unexpected updated item: %+v", updated)
	}
}

func TestRunExpirationBatch(t *testing.T) {
	item := &entity.Subscription{ID: 30, Status: entity.SubscriptionStatusActive, AutoRenew: true}
	var updated *entity.Subscription

	svc := NewSubscriptionService(
		&mockSubscriptionRepo{
			listExpiredActiveFn: func(_ context.Context, _ time.Time) ([]*entity.Subscription, error) {
				return []*entity.Subscription{item}, nil
			},
			updateFn: func(_ context.Context, subscription *entity.Subscription) error {
				updated = copySubscription(subscription)
				return nil
			},
		},
		&mockSubscriptionTypeRepo{},
		&mockPlanTypeRepo{},
		&fakePaymentService{},
		testConfig(),
	)

	if err := svc.RunExpirationBatch(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil || updated.Status != entity.SubscriptionStatusInactive || updated.AutoRenew || updated.RenewAt != nil {
		t.Fatalf("unexpected updated item: %+v", updated)
	}
}

func TestParseStartAtValidation(t *testing.T) {
	if _, err := parseStartAt(""); !errors.Is(err, ErrStartAtRequired) {
		t.Fatalf("expected ErrStartAtRequired, got %v", err)
	}
	if _, err := parseStartAt("bad"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for invalid format, got %v", err)
	}
	if v, err := parseStartAt("2026-01-01T10:00:00Z"); err != nil || v.IsZero() {
		t.Fatalf("expected parsed time, got %v err=%v", v, err)
	}
}
