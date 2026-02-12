E2E Tests (Docker Compose)

Prereqs:
- Docker Desktop (or Docker Engine) running.

Run:
1. cd subscriptions/e2e
2. docker compose up -d --build
3. cd ..
4. SUBSCRIPTIONS_HTTP_URL=http://localhost:38080 SUBSCRIPTIONS_GRPC_ADDR=localhost:39090 go test ./e2e -v -tags e2e

Shortcut:
- subscriptions/e2e/run.sh

Coverage:
- HTTP and gRPC API-key auth behavior (401/403 and gRPC unauth/forbidden)
- Subscription type listing
- Email-subscription create/get/list/cancel/delete flow
- Payment callback status transition
- Plan create behavior with phase-1 payment stub

Teardown:
- cd subscriptions/e2e
- docker compose down -v
