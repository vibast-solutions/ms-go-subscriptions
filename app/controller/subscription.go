package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/vibast-solutions/ms-go-subscriptions/app/factory"
	"github.com/vibast-solutions/ms-go-subscriptions/app/mapper"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
)

type paymentCallbackService interface {
	PaymentCallback(ctx context.Context, req *types.PaymentCallbackRequest) error
}

type SubscriptionController struct {
	subscriptionService    *service.SubscriptionService
	paymentCallbackService paymentCallbackService
	logger                 logrus.FieldLogger
}

func NewSubscriptionController(
	subscriptionService *service.SubscriptionService,
	paymentCallbackService paymentCallbackService,
) *SubscriptionController {
	return &SubscriptionController{
		subscriptionService:    subscriptionService,
		paymentCallbackService: paymentCallbackService,
		logger:                 factory.NewModuleLogger("subscriptions-controller"),
	}
}

func (c *SubscriptionController) Health(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, &types.HealthResponse{Status: "ok"})
}

func (c *SubscriptionController) ListSubscriptionTypes(ctx echo.Context) error {
	req, err := types.NewListSubscriptionTypesRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid query params")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	items, err := c.subscriptionService.ListSubscriptionTypes(ctx.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatus) {
			return c.writeError(ctx, http.StatusBadRequest, err.Error())
		}
		c.logger.WithError(err).Error("List subscription types failed")
		return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.JSON(http.StatusOK, &types.ListSubscriptionTypesResponse{
		SubscriptionTypes: mapper.SubscriptionTypesToProto(items),
	})
}

func (c *SubscriptionController) CreateSubscription(ctx echo.Context) error {
	req, err := types.NewCreateSubscriptionRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	result, err := c.subscriptionService.CreateSubscription(ctx.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest), errors.Is(err, service.ErrStartAtRequired):
			return c.writeError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSubscriptionTypeNotFound):
			return c.writeError(ctx, http.StatusNotFound, "subscription type not found")
		case errors.Is(err, service.ErrSubscriptionAlreadyExists):
			return c.writeError(ctx, http.StatusConflict, "subscription already exists")
		default:
			c.logger.WithError(err).Error("Create subscription failed")
			return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
		}
	}

	return ctx.JSON(http.StatusCreated, &types.CreateSubscriptionResponse{
		Subscription: mapper.SubscriptionToProto(result.Subscription),
		PaymentUrl:   result.PaymentURL,
	})
}

func (c *SubscriptionController) GetSubscription(ctx echo.Context) error {
	req, err := types.NewGetSubscriptionRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	item, err := c.subscriptionService.GetSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return c.writeError(ctx, http.StatusNotFound, "subscription not found")
		}
		c.logger.WithError(err).Error("Get subscription failed")
		return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.JSON(http.StatusOK, &types.SubscriptionEnvelopeResponse{
		Subscription: mapper.SubscriptionToProto(item),
	})
}

func (c *SubscriptionController) ListSubscriptions(ctx echo.Context) error {
	req, err := types.NewListSubscriptionsRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	items, err := c.subscriptionService.ListSubscriptions(ctx.Request().Context(), req)
	if err != nil {
		c.logger.WithError(err).Error("List subscriptions failed")
		return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.JSON(http.StatusOK, &types.ListSubscriptionsResponse{
		Subscriptions: mapper.SubscriptionsToProto(items),
	})
}

func (c *SubscriptionController) UpdateSubscription(ctx echo.Context) error {
	req, err := types.NewUpdateSubscriptionRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	item, err := c.subscriptionService.UpdateSubscription(ctx.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStatus), errors.Is(err, service.ErrNoFieldsToUpdate):
			return c.writeError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return c.writeError(ctx, http.StatusNotFound, "subscription not found")
		default:
			c.logger.WithError(err).Error("Update subscription failed")
			return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
		}
	}

	return ctx.JSON(http.StatusOK, &types.SubscriptionEnvelopeResponse{
		Subscription: mapper.SubscriptionToProto(item),
	})
}

func (c *SubscriptionController) DeleteSubscription(ctx echo.Context) error {
	req, err := types.NewDeleteSubscriptionRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	item, err := c.subscriptionService.DeleteSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return c.writeError(ctx, http.StatusNotFound, "subscription not found")
		}
		c.logger.WithError(err).Error("Delete subscription failed")
		return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.JSON(http.StatusOK, &types.MessageResponse{
		Message:      "Subscription deleted successfully",
		Subscription: mapper.SubscriptionToProto(item),
	})
}

func (c *SubscriptionController) CancelSubscription(ctx echo.Context) error {
	req, err := types.NewCancelSubscriptionRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	item, err := c.subscriptionService.CancelSubscription(ctx.Request().Context(), req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return c.writeError(ctx, http.StatusNotFound, "subscription not found")
		}
		c.logger.WithError(err).Error("Cancel subscription failed")
		return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.JSON(http.StatusOK, &types.MessageResponse{
		Message:      "Subscription cancelled successfully",
		Subscription: mapper.SubscriptionToProto(item),
	})
}

func (c *SubscriptionController) PaymentCallback(ctx echo.Context) error {
	req, err := types.NewPaymentCallbackRequestFromContext(ctx)
	if err != nil {
		return c.writeError(ctx, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return c.writeError(ctx, http.StatusBadRequest, err.Error())
	}

	if err := c.paymentCallbackService.PaymentCallback(ctx.Request().Context(), req); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			return c.writeError(ctx, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return c.writeError(ctx, http.StatusNotFound, "subscription not found")
		default:
			c.logger.WithError(err).Error("Payment callback failed")
			return c.writeError(ctx, http.StatusInternalServerError, "internal server error")
		}
	}

	return ctx.JSON(http.StatusOK, &types.MessageResponse{Message: "Payment processed successfully"})
}

func (c *SubscriptionController) writeError(ctx echo.Context, statusCode int, message string) error {
	return ctx.JSON(statusCode, &types.ErrorResponse{Error: message})
}
