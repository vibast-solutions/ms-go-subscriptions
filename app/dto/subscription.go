package dto

type SubscriptionTypeResponse struct {
	ID          uint64 `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	Status      int32  `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SubscriptionResponse struct {
	ID                 uint64  `json:"id"`
	SubscriptionTypeID uint64  `json:"subscription_type_id"`
	UserID             *string `json:"user_id,omitempty"`
	Email              *string `json:"email,omitempty"`
	Status             int32   `json:"status"`
	StartAt            *string `json:"start_at,omitempty"`
	EndAt              *string `json:"end_at,omitempty"`
	RenewAt            *string `json:"renew_at,omitempty"`
	AutoRenew          bool    `json:"auto_renew"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type ListSubscriptionTypesResponse struct {
	SubscriptionTypes []SubscriptionTypeResponse `json:"subscription_types"`
}

type CreateSubscriptionResponse struct {
	Subscription SubscriptionResponse `json:"subscription"`
	PaymentURL   string               `json:"payment_url,omitempty"`
}

type SubscriptionEnvelopeResponse struct {
	Subscription SubscriptionResponse `json:"subscription"`
}

type ListSubscriptionsResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
}

type MessageWithSubscriptionResponse struct {
	Message      string               `json:"message"`
	Subscription SubscriptionResponse `json:"subscription"`
}
