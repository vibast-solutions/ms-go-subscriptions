package payment

import "context"

type ResultType string

const (
	ResultTypeSuccess  ResultType = "success"
	ResultTypeRedirect ResultType = "redirect"
	ResultTypeFailure  ResultType = "failure"
)

type Result struct {
	Type          ResultType
	TransactionID string
	PaymentURL    string
	Error         string
}

type Service interface {
	ProcessSubscriptionPayment(ctx context.Context, subscriptionID uint64, planTypeID uint64, userID *string, email *string) Result
}
