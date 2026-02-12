package cmd

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/repository"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/config"

	_ "github.com/go-sql-driver/mysql"
)

var (
	renewWorker         bool
	cancelPendingWorker bool
	cancelExpiredWorker bool
)

var renewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Run auto-renewal processing",
	Run: func(_ *cobra.Command, _ []string) {
		runCommand(
			"renew",
			renewWorker,
			func(cfg *config.Config) time.Duration { return cfg.Jobs.AutoRenewInterval },
			func(s *service.SubscriptionService, ctx context.Context) error {
				return s.RunAutoRenewalBatch(ctx)
			},
		)
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Run cancellation-related processing commands",
}

var cancelPendingPaymentCmd = &cobra.Command{
	Use:   "pending-payment",
	Short: "Reset stale pending-payment subscriptions back to processing",
	Run: func(_ *cobra.Command, _ []string) {
		runCommand(
			"cancel_pending_payment",
			cancelPendingWorker,
			func(cfg *config.Config) time.Duration { return cfg.Jobs.PendingCleanupInterval },
			func(s *service.SubscriptionService, ctx context.Context) error {
				return s.RunPendingPaymentCleanupBatch(ctx)
			},
		)
	},
}

var cancelExpiredCmd = &cobra.Command{
	Use:   "expired",
	Short: "Mark expired active subscriptions as inactive",
	Run: func(_ *cobra.Command, _ []string) {
		runCommand(
			"cancel_expired",
			cancelExpiredWorker,
			func(cfg *config.Config) time.Duration { return cfg.Jobs.ExpirationCheckInterval },
			func(s *service.SubscriptionService, ctx context.Context) error {
				return s.RunExpirationBatch(ctx)
			},
		)
	},
}

func init() {
	rootCmd.AddCommand(renewCmd)
	rootCmd.AddCommand(cancelCmd)
	cancelCmd.AddCommand(cancelPendingPaymentCmd)
	cancelCmd.AddCommand(cancelExpiredCmd)

	renewCmd.Flags().BoolVar(&renewWorker, "worker", false, "Run continuously using configured interval")
	cancelPendingPaymentCmd.Flags().BoolVar(&cancelPendingWorker, "worker", false, "Run continuously using configured interval")
	cancelExpiredCmd.Flags().BoolVar(&cancelExpiredWorker, "worker", false, "Run continuously using configured interval")
}

func runCommand(
	name string,
	worker bool,
	intervalResolver func(cfg *config.Config) time.Duration,
	fn func(s *service.SubscriptionService, ctx context.Context) error,
) {
	cfg, subscriptionService, cleanup := mustCreateSubscriptionService()
	defer cleanup()

	if worker {
		runWorker(name, intervalResolver(cfg), subscriptionService, fn)
		return
	}

	ctx := context.Background()
	runJob(name, func() error { return fn(subscriptionService, ctx) })
}

func runWorker(
	name string,
	interval time.Duration,
	subscriptionService *service.SubscriptionService,
	fn func(s *service.SubscriptionService, ctx context.Context) error,
) {
	if interval <= 0 {
		logrus.WithField("job", name).Fatal("invalid worker interval")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runJob(name, func() error { return fn(subscriptionService, ctx) })

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-quit:
			logrus.WithField("job", name).Info("Worker shutdown requested")
			return
		case <-ticker.C:
			runJob(name, func() error { return fn(subscriptionService, ctx) })
		}
	}
}

func mustCreateSubscriptionService() (*config.Config, *service.SubscriptionService, func()) {
	cfg, err := config.Load()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}
	if err := configureLogging(cfg); err != nil {
		logrus.WithError(err).Fatal("Failed to configure logging")
	}

	db, err := sql.Open("mysql", cfg.MySQL.DSN)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}

	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		logrus.WithError(err).Fatal("Failed to ping database")
	}

	subscriptionRepo := repository.NewSubscriptionRepository(db)
	subscriptionTypeRepo := repository.NewSubscriptionTypeRepository(db)
	planTypeRepo := repository.NewPlanTypeRepository(db)
	subscriptionService := service.NewSubscriptionService(
		subscriptionRepo,
		subscriptionTypeRepo,
		planTypeRepo,
		payment.NewStubService(),
		cfg.Subscriptions,
	)

	cleanup := func() {
		if err := db.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close database")
		}
	}

	return cfg, subscriptionService, cleanup
}

func runJob(name string, fn func() error) {
	start := time.Now()
	err := fn()
	latency := time.Since(start)
	if err != nil {
		logrus.WithError(err).WithField("job", name).WithField("latency", latency.String()).Error("job_failed")
		return
	}
	logrus.WithField("job", name).WithField("latency", latency.String()).Info("job_completed")
}
