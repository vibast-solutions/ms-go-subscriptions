package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	types.UnimplementedSubscriptionsServiceServer
	subscriptionService *service.SubscriptionService
}

func NewServer(subscriptionService *service.SubscriptionService) *Server {
	return &Server{subscriptionService: subscriptionService}
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

	resp := &types.ListSubscriptionTypesResponse{SubscriptionTypes: make([]*types.SubscriptionType, 0, len(items))}
	for _, item := range items {
		resp.SubscriptionTypes = append(resp.SubscriptionTypes, &types.SubscriptionType{
			Id:          item.ID,
			Type:        item.Type,
			DisplayName: item.DisplayName,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return resp, nil
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
		Subscription: toGRPCSubscription(result.Subscription),
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

	return &types.SubscriptionEnvelopeResponse{Subscription: toGRPCSubscription(item)}, nil
}

func (s *Server) ListSubscriptions(ctx context.Context, req *types.ListSubscriptionsRequest) (*types.ListSubscriptionsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	items, err := s.subscriptionService.ListSubscriptions(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	resp := &types.ListSubscriptionsResponse{Subscriptions: make([]*types.Subscription, 0, len(items))}
	for _, item := range items {
		resp.Subscriptions = append(resp.Subscriptions, toGRPCSubscription(item))
	}
	return resp, nil
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

	return &types.SubscriptionEnvelopeResponse{Subscription: toGRPCSubscription(item)}, nil
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

	return &types.MessageResponse{Message: "Subscription deleted successfully", Subscription: toGRPCSubscription(item)}, nil
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

	return &types.MessageResponse{Message: "Subscription cancelled successfully", Subscription: toGRPCSubscription(item)}, nil
}

func (s *Server) PaymentCallback(ctx context.Context, req *types.PaymentCallbackRequest) (*types.MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.subscriptionService.PaymentCallback(ctx, req); err != nil {
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

func toGRPCSubscription(item *entity.Subscription) *types.Subscription {
	return &types.Subscription{
		Id:                 item.ID,
		SubscriptionTypeId: item.SubscriptionTypeID,
		UserId:             derefString(item.UserID),
		Email:              derefString(item.Email),
		Status:             item.Status,
		StartAt:            formatTime(item.StartAt),
		EndAt:              formatTime(item.EndAt),
		RenewAt:            formatTime(item.RenewAt),
		AutoRenew:          item.AutoRenew,
		CreatedAt:          item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func formatTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
