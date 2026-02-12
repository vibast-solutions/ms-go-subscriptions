package types

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNewListSubscriptionTypesRequestFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("GET", "/subscription-types?status=10&type=plan", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	parsed, err := NewListSubscriptionTypesRequestFromContext(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !parsed.GetHasStatus() || parsed.GetStatus() != 10 || parsed.GetType() != "plan" {
		t.Fatalf("unexpected parsed request: %+v", parsed)
	}
}

func TestListSubscriptionTypesValidate(t *testing.T) {
	req := &ListSubscriptionTypesRequest{HasStatus: true, Status: 5}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCreateSubscriptionValidate(t *testing.T) {
	req := &CreateSubscriptionRequest{SubscriptionTypeId: 1}
	if err := req.Validate(); err == nil {
		t.Fatal("expected user/email validation error")
	}

	req = &CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u1", StartAt: "not-rfc3339"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected start_at validation error")
	}

	req = &CreateSubscriptionRequest{SubscriptionTypeId: 1, UserId: "u1", StartAt: "2026-01-01T10:00:00Z"}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestNewUpdateSubscriptionRequestFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("PATCH", "/subscriptions/12", bytes.NewBufferString(`{"auto_renew":true,"status":10}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues("12")

	parsed, err := NewUpdateSubscriptionRequestFromContext(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.GetId() != 12 || !parsed.GetHasAutoRenew() || !parsed.GetAutoRenew() || !parsed.GetHasStatus() || parsed.GetStatus() != 10 {
		t.Fatalf("unexpected parsed request: %+v", parsed)
	}
}

func TestUpdateSubscriptionValidate(t *testing.T) {
	req := &UpdateSubscriptionRequest{Id: 1}
	if err := req.Validate(); err == nil {
		t.Fatal("expected missing fields validation error")
	}

	req = &UpdateSubscriptionRequest{Id: 1, HasStatus: true, Status: 99}
	if err := req.Validate(); err == nil {
		t.Fatal("expected invalid status validation error")
	}
}

func TestNewPaymentCallbackRequestFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/webhooks/payment-callback", bytes.NewBufferString(`{"subscription_id":11,"status":" SUCCESS ","transaction_id":" tx-1 "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	parsed, err := NewPaymentCallbackRequestFromContext(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.GetStatus() != "success" || parsed.GetTransactionId() != "tx-1" {
		t.Fatalf("unexpected parsed callback: %+v", parsed)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatalf("expected valid callback, got %v", err)
	}
}

func TestDeleteAndCancelValidate(t *testing.T) {
	if err := (&DeleteSubscriptionRequest{}).Validate(); err == nil {
		t.Fatal("expected invalid delete request")
	}
	if err := (&CancelSubscriptionRequest{}).Validate(); err == nil {
		t.Fatal("expected invalid cancel request")
	}
}
