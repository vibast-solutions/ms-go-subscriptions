# Subscriptions Service Deployment Guide

## Runtime Topology

Processes:
- API process: `subscriptions-service serve`
- Renewal process: `subscriptions-service renew` (or `subscriptions-service --worker renew`)
- Pending payment cancellation process: `subscriptions-service cancel pending-payment` (or `subscriptions-service --worker cancel pending-payment`)
- Expired subscription cancellation process: `subscriptions-service cancel expired` (or `subscriptions-service --worker cancel expired`)

Protocols:
- HTTP + gRPC (API process)
- In-process cron-like loops (worker mode via global `--worker` flag)

Default ports:
- HTTP: `8080`
- gRPC: `9090`

Dependencies:
- MySQL: required
- Auth service gRPC: required for internal API-key auth checks

## Required Environment Variables

- `MYSQL_DSN`

## Optional Environment Variables

- `APP_SERVICE_NAME` (default `subscriptions-service`)
- `APP_API_KEY` (used by auth-lib)
- `AUTH_SERVICE_GRPC_ADDR` (default `localhost:9090`)
- `HTTP_HOST` / `HTTP_PORT`
- `GRPC_HOST` / `GRPC_PORT`
- `MYSQL_MAX_OPEN_CONNS`
- `MYSQL_MAX_IDLE_CONNS`
- `MYSQL_CONN_MAX_LIFETIME_MINUTES`
- `LOG_LEVEL`
- `RENEW_BEFORE_END_MINUTES`
- `RENEWAL_RETRY_INTERVAL_MINUTES`
- `MAX_RENEWAL_RETRY_AGE_MINUTES`
- `PENDING_PAYMENT_TIMEOUT_MINUTES`
- `AUTO_RENEW_INTERVAL_MINUTES`
- `PENDING_CLEANUP_INTERVAL_MINUTES`
- `EXPIRATION_CHECK_INTERVAL_MINUTES`

## MySQL Schema

```sql
CREATE DATABASE IF NOT EXISTS subscriptions;
USE subscriptions;

CREATE TABLE subscription_types (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_subscription_types_type (type),
    INDEX idx_subscription_types_status (status)
);

CREATE TABLE plan_types (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    subscription_type_id BIGINT UNSIGNED NOT NULL,
    plan_code VARCHAR(50) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    price_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    duration_days INT NOT NULL,
    features JSON NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_plan_types_subscription_type_id FOREIGN KEY (subscription_type_id) REFERENCES subscription_types(id),
    UNIQUE INDEX idx_plan_types_subscription_type_id (subscription_type_id),
    UNIQUE INDEX idx_plan_types_plan_code (plan_code)
);

CREATE TABLE subscriptions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    subscription_type_id BIGINT UNSIGNED NOT NULL,
    user_id VARCHAR(255) NULL,
    email VARCHAR(255) NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    start_at DATETIME NULL,
    end_at DATETIME NULL,
    renew_at DATETIME NULL,
    auto_renew TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_subscriptions_subscription_type_id FOREIGN KEY (subscription_type_id) REFERENCES subscription_types(id),
    INDEX idx_subscriptions_user_id (user_id),
    INDEX idx_subscriptions_email (email),
    INDEX idx_subscriptions_status (status),
    INDEX idx_subscriptions_renew_at (renew_at),
    INDEX idx_subscriptions_end_at (end_at),
    UNIQUE INDEX idx_subscriptions_type_user_email (subscription_type_id, user_id, email)
);
```

## Operational Notes

- Keep API and command workers as separate deploy units for independent scaling.
- Service expects callers to provide identity context (`user_id` and/or `email`).
- Payment service is currently a stub that panics with:
  - `payments for renewals are not implemented`
- For plan subscriptions, this should be treated as a known phase-1 limitation.
