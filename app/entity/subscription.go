package entity

import "time"

const (
	SubscriptionStatusInactive       int32 = 0
	SubscriptionStatusProcessing     int32 = 1
	SubscriptionStatusPendingPayment int32 = 2
	SubscriptionStatusActive         int32 = 10
)

type Subscription struct {
	ID                 uint64
	SubscriptionTypeID uint64
	UserID             *string
	Email              *string
	Status             int32
	StartAt            *time.Time
	EndAt              *time.Time
	RenewAt            *time.Time
	AutoRenew          bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
