package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"github.com/vibast-solutions/ms-go-subscriptions/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcSubRepo struct {
	createFn                func(ctx context.Context, subscription *entity.Subscription) error
	updateFn                func(ctx context.Context, subscription *entity.Subscription) error
	findByIDFn              func(ctx context.Context, id uint64) (*entity.Subscription, error)
	findByTypeAndIdentityFn func(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error)
	listFn                  func(ctx context.Context, userID, email string) ([]*entity.Subscription, error)
}

func (r *grpcSubRepo) Create(ctx context.Context, subscription *entity.Subscription) error {
	if r.createFn != nil {
		return r.createFn(ctx, subscription)
	}
	return nil
}

func (r *grpcSubRepo) Update(ctx context.Context, subscription *entity.Subscription) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, subscription)
	}
	return nil
}

func (r *grpcSubRepo) FindByID(ctx context.Context, id uint64) (*entity.Subscription, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (r *grpcSubRepo) FindByTypeAndIdentity(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error) {
	if r.findByTypeAndIdentityFn != nil {
		return r.findByTypeAndIdentityFn(ctx, subscriptionTypeID, userID, email)
	}
	return nil, nil
}

func (r *grpcSubRepo) List(ctx context.Context, userID, email string) ([]*entity.Subscription, error) {
	if r.listFn != nil {
		return r.listFn(ctx, userID, email)
	}
	return nil, nil
}

func (r *grpcSubRepo) ListDueAutoRenew(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

func (r *grpcSubRepo) ListPendingPaymentStale(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

func (r *grpcSubRepo) ListExpiredActive(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

type grpcSubTypeRepo struct {
	listFn     func(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error)
	findByIDFn func(ctx context.Context, id uint64) (*entity.SubscriptionType, error)
}

func (r *grpcSubTypeRepo) List(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error) {
	if r.listFn != nil {
		return r.listFn(ctx, typeFilter, hasStatus, status)
	}
	return nil, nil
}

func (r *grpcSubTypeRepo) FindByID(ctx context.Context, id uint64) (*entity.SubscriptionType, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, nil
}

type grpcPlanRepo struct {
	findFn func(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error)
}

func (r *grpcPlanRepo) FindBySubscriptionTypeID(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error) {
	if r.findFn != nil {
		return r.findFn(ctx, subscriptionTypeID)
	}
	return nil, nil
}

type grpcPayment struct {
	result payment.Result
}

func (p *grpcPayment) ProcessSubscriptionPayment(context.Context, uint64, uint64, *string, *string) payment.Result {
	return p.result
}

func newGRPCServerForTest(repo *grpcSubRepo, stRepo *grpcSubTypeRepo, planRepo *grpcPlanRepo, pay *grpcPayment) *Server {
	cfg := config.SubscriptionConfig{
		RenewBeforeEndMinutes:       time.Hour,
		RenewalRetryIntervalMinutes: time.Minute,
		MaxRenewalRetryAgeMinutes:   2 * time.Hour,
		PendingPaymentTimeout:       5 * time.Minute,
	}
	svc := service.NewSubscriptionService(repo, stRepo, planRepo, pay, cfg)
	paymentCallbackSvc := service.NewPaymentCallbackService(repo, cfg)
	return NewServer(svc, paymentCallbackSvc)
}

func TestCreateSubscriptionInvalidArgument(t *testing.T) {
	srv := newGRPCServerForTest(&grpcSubRepo{}, &grpcSubTypeRepo{}, &grpcPlanRepo{}, &grpcPayment{})

	_, err := srv.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateSubscriptionNotFound(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{},
		&grpcSubTypeRepo{findByIDFn: func(context.Context, uint64) (*entity.SubscriptionType, error) { return nil, nil }},
		&grpcPlanRepo{},
		&grpcPayment{},
	)

	_, err := srv.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u1"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestCreateSubscriptionSuccess(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{createFn: func(_ context.Context, s *entity.Subscription) error { s.ID = 123; return nil }},
		&grpcSubTypeRepo{findByIDFn: func(context.Context, uint64) (*entity.SubscriptionType, error) {
			return &entity.SubscriptionType{ID: 1, Status: 10, Type: "email"}, nil
		}},
		&grpcPlanRepo{},
		&grpcPayment{},
	)

	resp, err := srv.CreateSubscription(context.Background(), &types.CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.GetSubscription().GetId() != 123 {
		t.Fatalf("expected id=123, got %+v", resp.GetSubscription())
	}
}

func TestGetSubscriptionNotFound(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, nil }},
		&grpcSubTypeRepo{}, &grpcPlanRepo{}, &grpcPayment{},
	)

	_, err := srv.GetSubscription(context.Background(), &types.GetSubscriptionRequest{Id: 9})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestUpdateSubscriptionInvalidStatus(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) {
			return &entity.Subscription{ID: 1, Status: entity.SubscriptionStatusActive}, nil
		}},
		&grpcSubTypeRepo{}, &grpcPlanRepo{}, &grpcPayment{},
	)

	_, err := srv.UpdateSubscription(context.Background(), &types.UpdateSubscriptionRequest{Id: 1, HasStatus: true, Status: 99})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestDeleteSubscriptionNotFound(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, nil }},
		&grpcSubTypeRepo{}, &grpcPlanRepo{}, &grpcPayment{},
	)

	_, err := srv.DeleteSubscription(context.Background(), &types.DeleteSubscriptionRequest{Id: 2})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestPaymentCallbackInternalError(t *testing.T) {
	srv := newGRPCServerForTest(
		&grpcSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, errors.New("db down") }},
		&grpcSubTypeRepo{}, &grpcPlanRepo{}, &grpcPayment{},
	)

	_, err := srv.PaymentCallback(context.Background(), &types.PaymentCallbackRequest{SubscriptionId: 1, Status: "success"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}
