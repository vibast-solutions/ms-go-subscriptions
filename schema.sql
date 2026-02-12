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
