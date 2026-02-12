package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/config"
)

type controllerSubRepo struct {
	createFn                func(ctx context.Context, subscription *entity.Subscription) error
	updateFn                func(ctx context.Context, subscription *entity.Subscription) error
	findByIDFn              func(ctx context.Context, id uint64) (*entity.Subscription, error)
	findByTypeAndIdentityFn func(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error)
	listFn                  func(ctx context.Context, userID, email string) ([]*entity.Subscription, error)
}

func (r *controllerSubRepo) Create(ctx context.Context, subscription *entity.Subscription) error {
	if r.createFn != nil {
		return r.createFn(ctx, subscription)
	}
	return nil
}

func (r *controllerSubRepo) Update(ctx context.Context, subscription *entity.Subscription) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, subscription)
	}
	return nil
}

func (r *controllerSubRepo) FindByID(ctx context.Context, id uint64) (*entity.Subscription, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (r *controllerSubRepo) FindByTypeAndIdentity(ctx context.Context, subscriptionTypeID uint64, userID, email *string) (*entity.Subscription, error) {
	if r.findByTypeAndIdentityFn != nil {
		return r.findByTypeAndIdentityFn(ctx, subscriptionTypeID, userID, email)
	}
	return nil, nil
}

func (r *controllerSubRepo) List(ctx context.Context, userID, email string) ([]*entity.Subscription, error) {
	if r.listFn != nil {
		return r.listFn(ctx, userID, email)
	}
	return nil, nil
}

func (r *controllerSubRepo) ListDueAutoRenew(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

func (r *controllerSubRepo) ListPendingPaymentStale(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

func (r *controllerSubRepo) ListExpiredActive(context.Context, time.Time) ([]*entity.Subscription, error) {
	return nil, nil
}

type controllerSubTypeRepo struct {
	listFn     func(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error)
	findByIDFn func(ctx context.Context, id uint64) (*entity.SubscriptionType, error)
}

func (r *controllerSubTypeRepo) List(ctx context.Context, typeFilter string, hasStatus bool, status int32) ([]*entity.SubscriptionType, error) {
	if r.listFn != nil {
		return r.listFn(ctx, typeFilter, hasStatus, status)
	}
	return nil, nil
}

func (r *controllerSubTypeRepo) FindByID(ctx context.Context, id uint64) (*entity.SubscriptionType, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, nil
}

type controllerPlanTypeRepo struct {
	findFn func(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error)
}

func (r *controllerPlanTypeRepo) FindBySubscriptionTypeID(ctx context.Context, subscriptionTypeID uint64) (*entity.PlanType, error) {
	if r.findFn != nil {
		return r.findFn(ctx, subscriptionTypeID)
	}
	return nil, nil
}

type controllerPaymentService struct {
	result payment.Result
}

func (s *controllerPaymentService) ProcessSubscriptionPayment(context.Context, uint64, uint64, *string, *string) payment.Result {
	return s.result
}

func newControllerForTest(repo *controllerSubRepo, stRepo *controllerSubTypeRepo, planRepo *controllerPlanTypeRepo, paySvc *controllerPaymentService) *SubscriptionController {
	cfg := config.SubscriptionConfig{
		RenewBeforeEndMinutes:       time.Hour,
		RenewalRetryIntervalMinutes: time.Minute,
		MaxRenewalRetryAgeMinutes:   2 * time.Hour,
		PendingPaymentTimeout:       5 * time.Minute,
	}
	subscriptionSvc := service.NewSubscriptionService(repo, stRepo, planRepo, paySvc, cfg)
	paymentCallbackSvc := service.NewPaymentCallbackService(repo, cfg)
	return NewSubscriptionController(subscriptionSvc, paymentCallbackSvc)
}

func TestCreateSubscriptionBadBody(t *testing.T) {
	ctrl := newControllerForTest(&controllerSubRepo{}, &controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/subscriptions", bytes.NewBufferString("{bad"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := ctrl.CreateSubscription(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateSubscriptionTypeNotFound(t *testing.T) {
	ctrl := newControllerForTest(
		&controllerSubRepo{},
		&controllerSubTypeRepo{findByIDFn: func(context.Context, uint64) (*entity.SubscriptionType, error) { return nil, nil }},
		&controllerPlanTypeRepo{},
		&controllerPaymentService{},
	)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/subscriptions", bytes.NewBufferString(`{"subscription_type_id":1,"user_id":"u1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_ = ctrl.CreateSubscription(ctx)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCreateSubscriptionSuccess(t *testing.T) {
	ctrl := newControllerForTest(
		&controllerSubRepo{
			createFn: func(_ context.Context, s *entity.Subscription) error {
				s.ID = 77
				return nil
			},
		},
		&controllerSubTypeRepo{findByIDFn: func(_ context.Context, _ uint64) (*entity.SubscriptionType, error) {
			return &entity.SubscriptionType{ID: 1, Status: 10, Type: "email"}, nil
		}},
		&controllerPlanTypeRepo{},
		&controllerPaymentService{},
	)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/subscriptions", bytes.NewBufferString(`{"subscription_type_id":1,"user_id":"u1","email":"u1@example.com"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_ = ctrl.CreateSubscription(ctx)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Subscription struct {
			ID uint64 `json:"id"`
		} `json:"subscription"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if payload.Subscription.ID == 0 {
		t.Fatalf("expected subscription payload, got %s", rec.Body.String())
	}
}

func TestGetSubscriptionNotFound(t *testing.T) {
	ctrl := newControllerForTest(
		&controllerSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, nil }},
		&controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{},
	)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/subscriptions/9", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("9")

	_ = ctrl.GetSubscription(ctx)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateSubscriptionValidationError(t *testing.T) {
	ctrl := newControllerForTest(&controllerSubRepo{}, &controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/subscriptions/3", bytes.NewBufferString(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("3")

	_ = ctrl.UpdateSubscription(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteSubscriptionNotFound(t *testing.T) {
	ctrl := newControllerForTest(
		&controllerSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, nil }},
		&controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{},
	)
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/subscriptions/3", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("3")

	_ = ctrl.DeleteSubscription(ctx)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListSubscriptionTypesInvalidStatus(t *testing.T) {
	ctrl := newControllerForTest(&controllerSubRepo{}, &controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/subscription-types?status=5", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_ = ctrl.ListSubscriptionTypes(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPaymentCallbackNotFound(t *testing.T) {
	ctrl := newControllerForTest(
		&controllerSubRepo{findByIDFn: func(context.Context, uint64) (*entity.Subscription, error) { return nil, nil }},
		&controllerSubTypeRepo{}, &controllerPlanTypeRepo{}, &controllerPaymentService{},
	)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/payment-callback", bytes.NewBufferString(`{"subscription_id":1,"status":"success","transaction_id":"tx1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_ = ctrl.PaymentCallback(ctx)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
