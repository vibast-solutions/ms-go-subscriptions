package grpc

import (
	"context"
	"errors"

	"github.com/vibast-solutions/ms-go-subscriptions/app/mapper"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentCallbackService interface {
	PaymentCallback(ctx context.Context, req *types.PaymentCallbackRequest) error
}

type Server struct {
	types.UnimplementedSubscriptionsServiceServer
	subscriptionService    *service.SubscriptionService
	paymentCallbackService paymentCallbackService
}

func NewServer(subscriptionService *service.SubscriptionService, paymentCallbackService paymentCallbackService) *Server {
	return &Server{
		subscriptionService:    subscriptionService,
		paymentCallbackService: paymentCallbackService,
	}
}

func (s *Server) Health(_ context.Context, _ *types.HealthRequest) (*types.HealthResponse, error) {
	return &types.HealthResponse{Status: "ok"}, nil
}

func (s *Server) ListSubscriptionTypes(ctx context.Context, req *types.ListSubscriptionTypesRequest) (*types.ListSubscriptionTypesResponse, error) {
	l := loggerWithContext(ctx)
	if err := req.Validate(); err != nil {
		l.WithError(err).Debug("List subscription types validation failed")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	items, err := s.subscriptionService.ListSubscriptionTypes(ctx, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatus) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		l.WithError(err).Error("List subscription types failed")
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &types.ListSubscriptionTypesResponse{
		SubscriptionTypes: mapper.SubscriptionTypesToProto(items),
	}, nil
}

func (s *Server) CreateSubscription(ctx context.Context, req *types.CreateSubscriptionRequest) (*types.CreateSubscriptionResponse, error) {
	l := loggerWithContext(ctx)
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	result, err := s.subscriptionService.CreateSubscription(ctx, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest), errors.Is(err, service.ErrStartAtRequired):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrSubscriptionTypeNotFound):
			return nil, status.Error(codes.NotFound, "subscription type not found")
		case errors.Is(err, service.ErrSubscriptionAlreadyExists):
			return nil, status.Error(codes.AlreadyExists, "subscription already exists")
		default:
			l.WithError(err).Error("Create subscription failed")
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &types.CreateSubscriptionResponse{
		Subscription: mapper.SubscriptionToProto(result.Subscription),
		PaymentUrl:   result.PaymentURL,
	}, nil
}

func (s *Server) GetSubscription(ctx context.Context, req *types.GetSubscriptionRequest) (*types.SubscriptionEnvelopeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	item, err := s.subscriptionService.GetSubscription(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return nil, status.Error(codes.NotFound, "subscription not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &types.SubscriptionEnvelopeResponse{Subscription: mapper.SubscriptionToProto(item)}, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, req *types.ListSubscriptionsRequest) (*types.ListSubscriptionsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	items, err := s.subscriptionService.ListSubscriptions(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &types.ListSubscriptionsResponse{
		Subscriptions: mapper.SubscriptionsToProto(items),
	}, nil
}

func (s *Server) UpdateSubscription(ctx context.Context, req *types.UpdateSubscriptionRequest) (*types.SubscriptionEnvelopeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	item, err := s.subscriptionService.UpdateSubscription(ctx, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStatus), errors.Is(err, service.ErrNoFieldsToUpdate):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return nil, status.Error(codes.NotFound, "subscription not found")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &types.SubscriptionEnvelopeResponse{Subscription: mapper.SubscriptionToProto(item)}, nil
}

func (s *Server) DeleteSubscription(ctx context.Context, req *types.DeleteSubscriptionRequest) (*types.MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	item, err := s.subscriptionService.DeleteSubscription(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return nil, status.Error(codes.NotFound, "subscription not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &types.MessageResponse{Message: "Subscription deleted successfully", Subscription: mapper.SubscriptionToProto(item)}, nil
}

func (s *Server) CancelSubscription(ctx context.Context, req *types.CancelSubscriptionRequest) (*types.MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	item, err := s.subscriptionService.CancelSubscription(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return nil, status.Error(codes.NotFound, "subscription not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &types.MessageResponse{Message: "Subscription cancelled successfully", Subscription: mapper.SubscriptionToProto(item)}, nil
}

func (s *Server) PaymentCallback(ctx context.Context, req *types.PaymentCallbackRequest) (*types.MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.paymentCallbackService.PaymentCallback(ctx, req); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrSubscriptionNotFound):
			return nil, status.Error(codes.NotFound, "subscription not found")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &types.MessageResponse{Message: "Payment processed successfully"}, nil
}
