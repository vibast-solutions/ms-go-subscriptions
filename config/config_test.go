package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s failed: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		}
	})
}

func TestLoadRequiresMySQLDSN(t *testing.T) {
	unsetEnv(t, "MYSQL_DSN")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing MYSQL_DSN")
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	setEnv(t, "MYSQL_DSN", "root:root@tcp(localhost:3306)/subscriptions?parseTime=true")
	setEnv(t, "APP_SERVICE_NAME", "subs-test")
	setEnv(t, "HTTP_PORT", "8181")
	setEnv(t, "GRPC_PORT", "9191")
	setEnv(t, "MYSQL_MAX_OPEN_CONNS", "20")
	setEnv(t, "MYSQL_MAX_IDLE_CONNS", "8")
	setEnv(t, "MYSQL_CONN_MAX_LIFETIME_MINUTES", "40")
	setEnv(t, "RENEW_BEFORE_END_MINUTES", "30")
	setEnv(t, "RENEWAL_RETRY_INTERVAL_MINUTES", "15")
	setEnv(t, "MAX_RENEWAL_RETRY_AGE_MINUTES", "120")
	setEnv(t, "PENDING_PAYMENT_TIMEOUT_MINUTES", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.App.ServiceName != "subs-test" {
		t.Fatalf("unexpected app service name: %s", cfg.App.ServiceName)
	}
	if cfg.HTTP.Port != "8181" || cfg.GRPC.Port != "9191" {
		t.Fatalf("unexpected ports: http=%s grpc=%s", cfg.HTTP.Port, cfg.GRPC.Port)
	}
	if cfg.MySQL.MaxOpenConns != 20 || cfg.MySQL.MaxIdleConns != 8 {
		t.Fatalf("unexpected mysql pool config: %+v", cfg.MySQL)
	}
	if cfg.MySQL.ConnMaxLifetime != 40*time.Minute {
		t.Fatalf("unexpected mysql lifetime: %v", cfg.MySQL.ConnMaxLifetime)
	}
	if cfg.Subscriptions.RenewBeforeEndMinutes != 30*time.Minute {
		t.Fatalf("unexpected renew-before value: %v", cfg.Subscriptions.RenewBeforeEndMinutes)
	}
	if cfg.Subscriptions.RenewalRetryIntervalMinutes != 15*time.Minute {
		t.Fatalf("unexpected retry interval: %v", cfg.Subscriptions.RenewalRetryIntervalMinutes)
	}
	if cfg.Subscriptions.MaxRenewalRetryAgeMinutes != 120*time.Minute {
		t.Fatalf("unexpected max retry age: %v", cfg.Subscriptions.MaxRenewalRetryAgeMinutes)
	}
	if cfg.Subscriptions.PendingPaymentTimeout != 5*time.Minute {
		t.Fatalf("unexpected pending timeout: %v", cfg.Subscriptions.PendingPaymentTimeout)
	}
}
