package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/repository"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"github.com/vibast-solutions/ms-go-subscriptions/config"
)

type PaymentCallbackService struct {
	subscriptionRepo subscriptionRepository
	cfg              config.SubscriptionConfig
}

func NewPaymentCallbackService(subscriptionRepo subscriptionRepository, cfg config.SubscriptionConfig) *PaymentCallbackService {
	return &PaymentCallbackService{
		subscriptionRepo: subscriptionRepo,
		cfg:              cfg,
	}
}

func (s *PaymentCallbackService) PaymentCallback(ctx context.Context, req *types.PaymentCallbackRequest) error {
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
