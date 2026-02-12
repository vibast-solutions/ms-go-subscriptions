package entity

import "time"

type PlanType struct {
	ID                 uint64
	SubscriptionTypeID uint64
	PlanCode           string
	DisplayName        string
	Description        string
	PriceCents         int64
	Currency           string
	DurationDays       int32
	Features           string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
