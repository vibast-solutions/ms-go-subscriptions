package types

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func NewListSubscriptionTypesRequestFromContext(ctx echo.Context) (*ListSubscriptionTypesRequest, error) {
	statusRaw := strings.TrimSpace(ctx.QueryParam("status"))
	req := &ListSubscriptionTypesRequest{Type: strings.TrimSpace(ctx.QueryParam("type"))}
	if statusRaw != "" {
		statusValue, err := strconv.ParseInt(statusRaw, 10, 32)
		if err != nil {
			return nil, err
		}
		req.HasStatus = true
		req.Status = int32(statusValue)
	}

	return req, nil
}

func (r *ListSubscriptionTypesRequest) Validate() error {
	if r.GetHasStatus() && r.GetStatus() != 0 && r.GetStatus() != 10 {
		return errors.New("status must be 0 or 10")
	}
	return nil
}

func NewCreateSubscriptionRequestFromContext(ctx echo.Context) (*CreateSubscriptionRequest, error) {
	var body CreateSubscriptionRequest
	if err := ctx.Bind(&body); err != nil {
		return nil, err
	}
	body.UserId = strings.TrimSpace(body.UserId)
	body.Email = strings.TrimSpace(body.Email)
	body.StartAt = strings.TrimSpace(body.StartAt)
	return &body, nil
}

func (r *CreateSubscriptionRequest) Validate() error {
	if r.GetSubscriptionTypeId() == 0 {
		return errors.New("subscription_type_id is required")
	}
	if strings.TrimSpace(r.GetUserId()) == "" && strings.TrimSpace(r.GetEmail()) == "" {
		return errors.New("at least one of user_id or email is required")
	}
	if r.GetStartAt() != "" {
		if _, err := time.Parse(time.RFC3339, r.GetStartAt()); err != nil {
			return errors.New("start_at must be RFC3339")
		}
	}

	return nil
}

func NewGetSubscriptionRequestFromContext(ctx echo.Context) (*GetSubscriptionRequest, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, err
	}
	return &GetSubscriptionRequest{Id: id}, nil
}

func (r *GetSubscriptionRequest) Validate() error {
	if r.GetId() == 0 {
		return errors.New("invalid subscription id")
	}
	return nil
}

func NewListSubscriptionsRequestFromContext(ctx echo.Context) (*ListSubscriptionsRequest, error) {
	return &ListSubscriptionsRequest{
		UserId: strings.TrimSpace(ctx.QueryParam("user_id")),
		Email:  strings.TrimSpace(ctx.QueryParam("email")),
	}, nil
}

func (r *ListSubscriptionsRequest) Validate() error {
	return nil
}

func NewUpdateSubscriptionRequestFromContext(ctx echo.Context) (*UpdateSubscriptionRequest, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, err
	}

	var body struct {
		AutoRenew *bool  `json:"auto_renew"`
		Status    *int32 `json:"status"`
	}
	if err := ctx.Bind(&body); err != nil {
		return nil, err
	}

	req := &UpdateSubscriptionRequest{Id: id}
	if body.AutoRenew != nil {
		req.HasAutoRenew = true
		req.AutoRenew = *body.AutoRenew
	}
	if body.Status != nil {
		req.HasStatus = true
		req.Status = *body.Status
	}

	return req, nil
}

func (r *UpdateSubscriptionRequest) Validate() error {
	if r.GetId() == 0 {
		return errors.New("invalid subscription id")
	}
	if !r.GetHasAutoRenew() && !r.GetHasStatus() {
		return errors.New("at least one of auto_renew or status is required")
	}
	if r.GetHasStatus() {
		switch r.GetStatus() {
		case 0, 1, 2, 10:
		default:
			return errors.New("status must be one of 0, 1, 2, 10")
		}
	}
	return nil
}

func NewDeleteSubscriptionRequestFromContext(ctx echo.Context) (*DeleteSubscriptionRequest, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, err
	}
	return &DeleteSubscriptionRequest{Id: id}, nil
}

func (r *DeleteSubscriptionRequest) Validate() error {
	if r.GetId() == 0 {
		return errors.New("invalid subscription id")
	}
	return nil
}

func NewCancelSubscriptionRequestFromContext(ctx echo.Context) (*CancelSubscriptionRequest, error) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, err
	}
	return &CancelSubscriptionRequest{Id: id}, nil
}

func (r *CancelSubscriptionRequest) Validate() error {
	if r.GetId() == 0 {
		return errors.New("invalid subscription id")
	}
	return nil
}

func NewPaymentCallbackRequestFromContext(ctx echo.Context) (*PaymentCallbackRequest, error) {
	var body PaymentCallbackRequest
	if err := ctx.Bind(&body); err != nil {
		return nil, err
	}
	body.Status = strings.TrimSpace(strings.ToLower(body.Status))
	body.TransactionId = strings.TrimSpace(body.TransactionId)
	return &body, nil
}

func (r *PaymentCallbackRequest) Validate() error {
	if r.GetSubscriptionId() == 0 {
		return errors.New("subscription_id is required")
	}
	if r.GetStatus() != "success" && r.GetStatus() != "failed" {
		return errors.New("status must be success or failed")
	}
	return nil
}
