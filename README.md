# Subscriptions Microservice

`github.com/vibast-solutions/ms-go-subscriptions`

Subscriptions microservice for managing email and plan subscriptions via HTTP and gRPC.

This service follows the same architecture and API-key security model as the other services in this repository:
- HTTP and gRPC in the same service process
- Internal API key verification via Auth service (`lib-go-auth` middleware)
- Layered architecture (`controller/grpc -> service -> repository -> entity`)

## Features

- List subscription types
- Create subscription (email or plan)
- Get subscription by ID
- List subscriptions by `user_id` and/or `email`
- Update subscription (`auto_renew`, `status`)
- Soft-delete subscription
- Cancel subscription (disable renewals)
- Payment callback endpoint
- Background jobs for:
  - auto-renewal
  - stale pending-payment cleanup
  - expiration cleanup

## Requirements

- Go 1.25+
- MySQL 8+
- Auth service reachable via gRPC (`AUTH_SERVICE_GRPC_ADDR`)

## Build

```bash
# Build native binary
make build

# Build all targets
make build-all
```

## Run

```bash
# Start HTTP + gRPC API
./build/subscriptions-service serve

# Start background jobs process
./build/subscriptions-service jobs
```

Or directly:

```bash
go run main.go serve
go run main.go jobs
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_SERVICE_NAME` | `subscriptions-service` | Service name used for internal auth access checks |
| `APP_API_KEY` | (empty) | Caller key used by auth-lib when validating internal access |
| `AUTH_SERVICE_GRPC_ADDR` | `localhost:9090` | Auth service gRPC endpoint |
| `HTTP_HOST` | `0.0.0.0` | HTTP bind host |
| `HTTP_PORT` | `8080` | HTTP bind port |
| `GRPC_HOST` | `0.0.0.0` | gRPC bind host |
| `GRPC_PORT` | `9090` | gRPC bind port |
| `MYSQL_DSN` | (required) | MySQL DSN |
| `MYSQL_MAX_OPEN_CONNS` | `10` | DB pool max open conns |
| `MYSQL_MAX_IDLE_CONNS` | `5` | DB pool max idle conns |
| `MYSQL_CONN_MAX_LIFETIME_MINUTES` | `30` | DB conn max lifetime (minutes) |
| `LOG_LEVEL` | `info` | Log level |
| `RENEW_BEFORE_END_MINUTES` | `1440` | Renew attempt lead time before `end_at` |
| `RENEWAL_RETRY_INTERVAL_MINUTES` | `60` | Retry delay after failed payment |
| `MAX_RENEWAL_RETRY_AGE_MINUTES` | `10080` | Max retry window past `end_at` before inactivation |
| `PENDING_PAYMENT_TIMEOUT_MINUTES` | `30` | Timeout for stale pending-payment records |
| `AUTO_RENEW_INTERVAL_MINUTES` | `1` | Auto-renew job interval |
| `PENDING_CLEANUP_INTERVAL_MINUTES` | `10` | Pending cleanup job interval |
| `EXPIRATION_CHECK_INTERVAL_MINUTES` | `60` | Expiration job interval |

## HTTP API

- `GET /subscription-types?status=10&type=plan`
- `POST /subscriptions`
- `GET /subscriptions/:id`
- `GET /subscriptions?user_id=u1&email=a@b.com`
- `PATCH /subscriptions/:id`
- `DELETE /subscriptions/:id`
- `POST /subscriptions/:id/cancel`
- `POST /webhooks/payment-callback`
- `GET /health`

All routes are protected by internal API key access middleware, matching the current repository security approach.

## gRPC API

Service: `subscriptions.SubscriptionsService`

Methods:
- `ListSubscriptionTypes`
- `CreateSubscription`
- `GetSubscription`
- `ListSubscriptions`
- `UpdateSubscription`
- `DeleteSubscription`
- `CancelSubscription`
- `PaymentCallback`

Generate gRPC files:

```bash
PATH="$HOME/go/bin:$PATH" ./scripts/gen_proto.sh
```

## Database

See:
- `deployment.md`
- `schema.sql`
