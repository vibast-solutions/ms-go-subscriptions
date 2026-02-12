package entity

import "time"

type SubscriptionType struct {
	ID          uint64
	Type        string
	DisplayName string
	Status      int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
