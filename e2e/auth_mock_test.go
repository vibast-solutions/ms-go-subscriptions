//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	authpb "github.com/vibast-solutions/ms-go-auth/app/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	defaultSubscriptionsCallerAPIKey   = "subscriptions-caller-key"
	defaultSubscriptionsNoAccessAPIKey = "subscriptions-no-access-key"
	defaultSubscriptionsAppAPIKey      = "subscriptions-app-api-key"
	subscriptionsAuthMockAddr          = "127.0.0.1:38083"
)

func subscriptionsCallerAPIKey() string {
	if value := strings.TrimSpace(os.Getenv("SUBSCRIPTIONS_CALLER_API_KEY")); value != "" {
		return value
	}
	return defaultSubscriptionsCallerAPIKey
}

func subscriptionsNoAccessAPIKey() string {
	if value := strings.TrimSpace(os.Getenv("SUBSCRIPTIONS_NO_ACCESS_API_KEY")); value != "" {
		return value
	}
	return defaultSubscriptionsNoAccessAPIKey
}

func subscriptionsAppAPIKey() string {
	if value := strings.TrimSpace(os.Getenv("SUBSCRIPTIONS_APP_API_KEY")); value != "" {
		return value
	}
	return defaultSubscriptionsAppAPIKey
}

type subscriptionsAuthGRPCServer struct {
	authpb.UnimplementedAuthServiceServer
}

func (s *subscriptionsAuthGRPCServer) ValidateInternalAccess(ctx context.Context, req *authpb.ValidateInternalAccessRequest) (*authpb.ValidateInternalAccessResponse, error) {
	if incomingSubscriptionsAPIKey(ctx) != subscriptionsAppAPIKey() {
		return nil, status.Error(codes.Unauthenticated, "unauthorized caller")
	}

	apiKey := strings.TrimSpace(req.GetApiKey())
	switch apiKey {
	case subscriptionsCallerAPIKey():
		return &authpb.ValidateInternalAccessResponse{
			ServiceName:   "subscriptions-gateway",
			AllowedAccess: []string{"subscriptions-service", "profile-service"},
		}, nil
	case subscriptionsNoAccessAPIKey():
		return &authpb.ValidateInternalAccessResponse{
			ServiceName:   "subscriptions-gateway",
			AllowedAccess: []string{"profile-service"},
		}, nil
	default:
		return nil, status.Error(codes.Unauthenticated, "invalid api key")
	}
}

func incomingSubscriptionsAPIKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("x-api-key")
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func TestMain(m *testing.M) {
	if os.Getenv("SUBSCRIPTIONS_CALLER_API_KEY") == "" {
		_ = os.Setenv("SUBSCRIPTIONS_CALLER_API_KEY", defaultSubscriptionsCallerAPIKey)
	}
	if os.Getenv("SUBSCRIPTIONS_NO_ACCESS_API_KEY") == "" {
		_ = os.Setenv("SUBSCRIPTIONS_NO_ACCESS_API_KEY", defaultSubscriptionsNoAccessAPIKey)
	}
	if os.Getenv("SUBSCRIPTIONS_APP_API_KEY") == "" {
		_ = os.Setenv("SUBSCRIPTIONS_APP_API_KEY", defaultSubscriptionsAppAPIKey)
	}

	listener, err := net.Listen("tcp", subscriptionsAuthMockAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start subscriptions auth grpc mock: %v\n", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, &subscriptionsAuthGRPCServer{})

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	exitCode := m.Run()

	grpcServer.GracefulStop()
	_ = listener.Close()

	os.Exit(exitCode)
}
