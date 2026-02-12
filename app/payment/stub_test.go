package payment

import (
	"context"
	"strings"
	"testing"
)

func TestStubServicePanics(t *testing.T) {
	svc := NewStubService()

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic")
		}
		if msg, ok := rec.(string); !ok || !strings.Contains(msg, "payments for renewals are not implemented") {
			t.Fatalf("unexpected panic: %v", rec)
		}
	}()

	_ = svc.ProcessSubscriptionPayment(context.Background(), 1, 2, nil, nil)
}
