package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	httpdto "github.com/vibast-solutions/ms-go-subscriptions/app/dto"
	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/factory"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
)

type SubscriptionController struct {
	subscriptionService *service.SubscriptionService
	logger              logrus.FieldLogger
}

func NewSubscriptionController(subscriptionService *service.SubscriptionService) *SubscriptionController {
	return &SubscriptionController{
		subscriptionService: subscriptionService,
		logger:              factory.NewModuleLogger("subscriptions-controller"),
	}
}

func (c *SubscriptionController) ListSubscriptionTypes(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewListSubscriptionTypesRequestFromContext(ctx)
	if err != nil {
		l.WithError(err).Debug("Failed to create list subscription types request")
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid query params"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	items, err := c.subscriptionService.ListSubscriptionTypes(ctx.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatus) {
			return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
		}
		l.WithError(err).Error("List subscription types failed")
		return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
	}

	resp := httpdto.ListSubscriptionTypesResponse{SubscriptionTypes: make([]httpdto.SubscriptionTypeResponse, 0, len(items))}
	for _, item := range items {
		resp.SubscriptionTypes = append(resp.SubscriptionTypes, toSubscriptionTypeResponse(item))
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *SubscriptionController) CreateSubscription(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewCreateSubscriptionRequestFromContext(ctx)
	if err != nil {
		l.WithError(err).Debug("Failed to parse create subscription request")
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request body"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	result, err := c.subscriptionService.CreateSubscription(ctx.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest), errors.Is(err, service.ErrStartAtRequired):
			return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrSubscriptionTypeNotFound):
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription type not found"})
		case errors.Is(err, service.ErrSubscriptionAlreadyExists):
			return ctx.JSON(http.StatusConflict, httpdto.ErrorResponse{Error: "subscription already exists"})
		default:
			l.WithError(err).Error("Create subscription failed")
			return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
		}
	}

	resp := httpdto.CreateSubscriptionResponse{Subscription: toSubscriptionResponse(result.Subscription)}
	if result.PaymentURL != "" {
		resp.PaymentURL = result.PaymentURL
	}

	return ctx.JSON(http.StatusCreated, resp)
}

func (c *SubscriptionController) GetSubscription(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewGetSubscriptionRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	subscription, err := c.subscriptionService.GetSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription not found"})
		}
		l.WithError(err).Error("Get subscription failed")
		return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
	}

	return ctx.JSON(http.StatusOK, httpdto.SubscriptionEnvelopeResponse{Subscription: toSubscriptionResponse(subscription)})
}

func (c *SubscriptionController) ListSubscriptions(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewListSubscriptionsRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	items, err := c.subscriptionService.ListSubscriptions(ctx.Request().Context(), req)
	if err != nil {
		l.WithError(err).Error("List subscriptions failed")
		return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
	}

	resp := httpdto.ListSubscriptionsResponse{Subscriptions: make([]httpdto.SubscriptionResponse, 0, len(items))}
	for _, item := range items {
		resp.Subscriptions = append(resp.Subscriptions, toSubscriptionResponse(item))
	}
	return ctx.JSON(http.StatusOK, resp)
}

func (c *SubscriptionController) UpdateSubscription(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewUpdateSubscriptionRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	subscription, err := c.subscriptionService.UpdateSubscription(ctx.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStatus), errors.Is(err, service.ErrNoFieldsToUpdate):
			return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription not found"})
		default:
			l.WithError(err).Error("Update subscription failed")
			return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
		}
	}

	return ctx.JSON(http.StatusOK, httpdto.SubscriptionEnvelopeResponse{Subscription: toSubscriptionResponse(subscription)})
}

func (c *SubscriptionController) DeleteSubscription(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewDeleteSubscriptionRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	subscription, err := c.subscriptionService.DeleteSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription not found"})
		}
		l.WithError(err).Error("Delete subscription failed")
		return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
	}

	return ctx.JSON(http.StatusOK, httpdto.MessageWithSubscriptionResponse{
		Message:      "Subscription deleted successfully",
		Subscription: toSubscriptionResponse(subscription),
	})
}

func (c *SubscriptionController) CancelSubscription(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewCancelSubscriptionRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	subscription, err := c.subscriptionService.CancelSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription not found"})
		}
		l.WithError(err).Error("Cancel subscription failed")
		return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
	}

	return ctx.JSON(http.StatusOK, httpdto.MessageWithSubscriptionResponse{
		Message:      "Subscription cancelled successfully",
		Subscription: toSubscriptionResponse(subscription),
	})
}

func (c *SubscriptionController) PaymentCallback(ctx echo.Context) error {
	l := c.logger
	req, err := types.NewPaymentCallbackRequestFromContext(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: "invalid request body"})
	}
	if err := req.Validate(); err != nil {
		return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
	}

	if err := c.subscriptionService.PaymentCallback(ctx.Request().Context(), req); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			return ctx.JSON(http.StatusBadRequest, httpdto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return ctx.JSON(http.StatusNotFound, httpdto.ErrorResponse{Error: "subscription not found"})
		default:
			l.WithError(err).Error("Payment callback failed")
			return ctx.JSON(http.StatusInternalServerError, httpdto.ErrorResponse{Error: "internal server error"})
		}
	}

	return ctx.JSON(http.StatusOK, httpdto.MessageResponse{Message: "Payment processed successfully"})
}

func toSubscriptionTypeResponse(item *entity.SubscriptionType) httpdto.SubscriptionTypeResponse {
	return httpdto.SubscriptionTypeResponse{
		ID:          item.ID,
		Type:        item.Type,
		DisplayName: item.DisplayName,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSubscriptionResponse(item *entity.Subscription) httpdto.SubscriptionResponse {
	return httpdto.SubscriptionResponse{
		ID:                 item.ID,
		SubscriptionTypeID: item.SubscriptionTypeID,
		UserID:             item.UserID,
		Email:              item.Email,
		Status:             item.Status,
		StartAt:            formatTimePtr(item.StartAt),
		EndAt:              formatTimePtr(item.EndAt),
		RenewAt:            formatTimePtr(item.RenewAt),
		AutoRenew:          item.AutoRenew,
		CreatedAt:          item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}
