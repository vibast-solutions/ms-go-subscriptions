//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	defaultHTTPBase = "http://localhost:38080"
	defaultGRPCAddr = "localhost:39090"
)

type httpClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient(baseURL string) *httpClient {
	return &httpClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *httpClient) doJSON(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	return c.doJSONWithAPIKey(t, method, path, body, subscriptionsCallerAPIKey())
}

func (c *httpClient) doJSONWithAPIKey(t *testing.T, method, path string, body any, apiKey string) (*http.Response, []byte) {
	t.Helper()

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json marshal failed: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	return resp, bodyBytes
}

func waitForHTTP(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/health", nil)
		req.Header.Set("X-API-Key", subscriptionsCallerAPIKey())
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("http service not ready at %s", baseURL)
}

func waitForGRPC(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("grpc service not ready at %s", addr)
}

func withGRPCAPIKey() grpc.DialOption {
	return grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", subscriptionsCallerAPIKey())
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}

func dialSubscriptionsGRPC(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), withGRPCAPIKey())
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	return conn
}

func dialSubscriptionsGRPCRaw(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	return conn
}

func grpcContextWithAPIKey(apiKey string) context.Context {
	if apiKey == "" {
		return context.Background()
	}
	return metadata.AppendToOutgoingContext(context.Background(), "x-api-key", apiKey)
}

func TestSubscriptionsE2E(t *testing.T) {
	httpBase := os.Getenv("SUBSCRIPTIONS_HTTP_URL")
	if httpBase == "" {
		httpBase = defaultHTTPBase
	}
	grpcAddr := os.Getenv("SUBSCRIPTIONS_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = defaultGRPCAddr
	}

	if err := waitForHTTP(httpBase, 30*time.Second); err != nil {
		t.Fatalf("http not ready: %v", err)
	}
	if err := waitForGRPC(grpcAddr, 30*time.Second); err != nil {
		t.Fatalf("grpc not ready: %v", err)
	}

	client := newHTTPClient(httpBase)

	conn := dialSubscriptionsGRPC(t, grpcAddr)
	defer conn.Close()
	grpcClient := types.NewSubscriptionsServiceClient(conn)

	rawConn := dialSubscriptionsGRPCRaw(t, grpcAddr)
	defer rawConn.Close()
	rawGRPCClient := types.NewSubscriptionsServiceClient(rawConn)

	state := struct {
		subscriptionID uint64
		userID         string
		email          string
	}{
		userID: fmt.Sprintf("sub-e2e-user-%d", time.Now().UnixNano()),
		email:  fmt.Sprintf("sub-e2e-%d@example.com", time.Now().UnixNano()),
	}

	t.Run("HTTPUnauthorizedMissingAPIKey", func(t *testing.T) {
		resp, _ := client.doJSONWithAPIKey(t, http.MethodGet, "/health", nil, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("HTTPForbiddenInsufficientAccess", func(t *testing.T) {
		resp, _ := client.doJSONWithAPIKey(t, http.MethodGet, "/health", nil, subscriptionsNoAccessAPIKey())
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("GRPCUnauthorizedMissingAPIKey", func(t *testing.T) {
		_, err := rawGRPCClient.GetSubscription(context.Background(), &types.GetSubscriptionRequest{Id: 1})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("GRPCForbiddenInsufficientAccess", func(t *testing.T) {
		_, err := rawGRPCClient.GetSubscription(grpcContextWithAPIKey(subscriptionsNoAccessAPIKey()), &types.GetSubscriptionRequest{Id: 1})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("HTTPListSubscriptionTypes", func(t *testing.T) {
		resp, body := client.doJSON(t, http.MethodGet, "/subscription-types?status=10", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json unmarshal failed: %v", err)
		}
		if payload["subscription_types"] == nil {
			t.Fatalf("missing subscription_types payload")
		}
	})

	t.Run("HTTPCreateEmailSubscription", func(t *testing.T) {
		resp, body := client.doJSON(t, http.MethodPost, "/subscriptions", map[string]any{
			"subscription_type_id": 1,
			"user_id":              state.userID,
			"email":                state.email,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, string(body))
		}

		var payload struct {
			Subscription struct {
				ID     uint64 `json:"id"`
				Status int32  `json:"status"`
			} `json:"subscription"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json unmarshal failed: %v", err)
		}
		if payload.Subscription.ID == 0 {
			t.Fatalf("expected generated id, got body=%s", string(body))
		}
		if payload.Subscription.Status != 10 {
			t.Fatalf("expected active status, got %d", payload.Subscription.Status)
		}
		state.subscriptionID = payload.Subscription.ID
	})

	t.Run("GRPCGetCreatedSubscription", func(t *testing.T) {
		res, err := grpcClient.GetSubscription(context.Background(), &types.GetSubscriptionRequest{Id: state.subscriptionID})
		if err != nil {
			t.Fatalf("grpc get failed: %v", err)
		}
		if res.GetSubscription().GetId() != state.subscriptionID {
			t.Fatalf("unexpected grpc subscription id: %d", res.GetSubscription().GetId())
		}
	})

	t.Run("HTTPListSubscriptionsWithFilter", func(t *testing.T) {
		resp, body := client.doJSON(t, http.MethodGet, "/subscriptions?user_id="+state.userID, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json unmarshal failed: %v", err)
		}
		if payload["subscriptions"] == nil {
			t.Fatalf("expected subscriptions list in payload")
		}
	})

	t.Run("HTTPPaymentCallbackFailed", func(t *testing.T) {
		resp, body := client.doJSON(t, http.MethodPost, "/webhooks/payment-callback", map[string]any{
			"subscription_id": state.subscriptionID,
			"status":          "failed",
			"transaction_id":  fmt.Sprintf("txn-%d", time.Now().UnixNano()),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
		}

		resp, body = client.doJSON(t, http.MethodGet, "/subscriptions/"+strconv.FormatUint(state.subscriptionID, 10), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
		}

		var payload struct {
			Subscription struct {
				Status int32 `json:"status"`
			} `json:"subscription"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json unmarshal failed: %v", err)
		}
		if payload.Subscription.Status != 1 {
			t.Fatalf("expected processing status=1 after failed callback, got %d", payload.Subscription.Status)
		}
	})

	t.Run("HTTPCancelAndDelete", func(t *testing.T) {
		resp, body := client.doJSON(t, http.MethodPost, "/subscriptions/"+strconv.FormatUint(state.subscriptionID, 10)+"/cancel", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for cancel, got %d body=%s", resp.StatusCode, string(body))
		}

		resp, body = client.doJSON(t, http.MethodDelete, "/subscriptions/"+strconv.FormatUint(state.subscriptionID, 10), nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for delete, got %d body=%s", resp.StatusCode, string(body))
		}

		var payload struct {
			Subscription struct {
				Status int32 `json:"status"`
			} `json:"subscription"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("json unmarshal failed: %v", err)
		}
		if payload.Subscription.Status != 0 {
			t.Fatalf("expected status=0 after delete, got %d", payload.Subscription.Status)
		}
	})

	t.Run("HTTPPlanCreateFailsUntilPaymentsImplemented", func(t *testing.T) {
		resp, _ := client.doJSON(t, http.MethodPost, "/subscriptions", map[string]any{
			"subscription_type_id": 2,
			"user_id":              state.userID + "-plan",
			"email":                "plan-" + state.email,
			"start_at":             time.Now().UTC().Format(time.RFC3339),
			"auto_renew":           true,
		})
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 500 for plan creation with payment stub, got %d", resp.StatusCode)
		}
	})
}
