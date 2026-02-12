package payment

import "context"

type StubService struct{}

func NewStubService() *StubService {
	return &StubService{}
}

func (s *StubService) ProcessSubscriptionPayment(_ context.Context, _ uint64, _ uint64, _ *string, _ *string) Result {
	panic("payments for renewals are not implemented")
}
