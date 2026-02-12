package service

import "errors"

var (
	ErrSubscriptionNotFound      = errors.New("subscription not found")
	ErrSubscriptionTypeNotFound  = errors.New("subscription type not found")
	ErrSubscriptionAlreadyExists = errors.New("subscription already exists")
	ErrInvalidRequest            = errors.New("invalid request")
	ErrInvalidStatus             = errors.New("invalid status")
	ErrStartAtRequired           = errors.New("start_at is required for plan subscriptions")
	ErrNoFieldsToUpdate          = errors.New("no fields provided for update")
)
