# Subscriptions Microservice - Claude Context

## Overview
Subscriptions microservice with HTTP + gRPC APIs and background jobs.

It manages:
- subscription types
- plan metadata
- user subscriptions
- plan lifecycle operations (renewal, pending cleanup, expiration)

## Technology Stack
- Echo (HTTP)
- gRPC
- Cobra
- MySQL (`database/sql`)
- Logrus
- Internal API-key auth via `lib-go-auth`

## Module
- `github.com/vibast-solutions/ms-go-subscriptions`

## Directory Structure
```
subscriptions/
├── main.go
├── Makefile
├── cmd/
│   ├── root.go
│   ├── serve.go
│   ├── jobs.go
│   ├── logging.go
│   └── version.go
├── config/
│   └── config.go
├── proto/
│   └── subscriptions.proto
├── app/
│   ├── controller/
│   │   ├── proto.go
│   │   └── subscription.go
│   ├── grpc/
│   │   ├── interceptor.go
│   │   └── server.go
│   ├── service/
│   │   ├── errors.go
│   │   ├── payment.go
│   │   └── subscription.go
│   ├── mapper/
│   │   └── subscriptions.go
│   ├── repository/
│   │   ├── common.go
│   │   ├── subscription_type.go
│   │   ├── plan_type.go
│   │   └── subscription.go
│   ├── entity/
│   │   ├── subscription_type.go
│   │   ├── plan_type.go
│   │   └── subscription.go
│   ├── types/
│   │   ├── subscriptions.go
│   │   ├── subscriptions.pb.go
│   │   └── subscriptions_grpc.pb.go
│   ├── payment/
│   │   ├── payment.go
│   │   └── stub.go
│   └── factory/
│       └── logger.go
└── scripts/
    └── gen_proto.sh
```

## Security
- HTTP middleware: `EchoInternalAuthMiddleware.RequireInternalAccess(APP_SERVICE_NAME)`
- gRPC middleware: `GRPCInternalAuthMiddleware.UnaryRequireInternalAccess(APP_SERVICE_NAME)`

## Commands
- `subscriptions serve`
- `subscriptions renew`
- `subscriptions --worker renew`
- `subscriptions cancel pending-payment`
- `subscriptions --worker cancel pending-payment`
- `subscriptions cancel expired`
- `subscriptions --worker cancel expired`
- `subscriptions version`
