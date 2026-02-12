package mapper

import (
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/entity"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
)

func SubscriptionTypeToProto(item *entity.SubscriptionType) *types.SubscriptionType {
	if item == nil {
		return nil
	}

	return &types.SubscriptionType{
		Id:          item.ID,
		Type:        item.Type,
		DisplayName: item.DisplayName,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func SubscriptionTypesToProto(items []*entity.SubscriptionType) []*types.SubscriptionType {
	result := make([]*types.SubscriptionType, 0, len(items))
	for _, item := range items {
		result = append(result, SubscriptionTypeToProto(item))
	}
	return result
}

func SubscriptionToProto(item *entity.Subscription) *types.Subscription {
	if item == nil {
		return nil
	}

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

func SubscriptionsToProto(items []*entity.Subscription) []*types.Subscription {
	result := make([]*types.Subscription, 0, len(items))
	for _, item := range items {
		result = append(result, SubscriptionToProto(item))
	}
	return result
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
