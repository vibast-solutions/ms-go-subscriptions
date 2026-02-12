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

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Run subscription background jobs",
	Long:  "Run auto-renewal, pending cleanup, and expiration jobs.",
	Run:   runJobs,
}

func init() {
	rootCmd.AddCommand(jobsCmd)
}

func runJobs(_ *cobra.Command, _ []string) {
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
	defer db.Close()

	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
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

	autoRenewTicker := time.NewTicker(cfg.Jobs.AutoRenewInterval)
	defer autoRenewTicker.Stop()
	pendingTicker := time.NewTicker(cfg.Jobs.PendingCleanupInterval)
	defer pendingTicker.Stop()
	expirationTicker := time.NewTicker(cfg.Jobs.ExpirationCheckInterval)
	defer expirationTicker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runJob("auto_renewal", func() error {
		return subscriptionService.RunAutoRenewalBatch(ctx)
	})
	runJob("pending_cleanup", func() error {
		return subscriptionService.RunPendingPaymentCleanupBatch(ctx)
	})
	runJob("expiration", func() error {
		return subscriptionService.RunExpirationBatch(ctx)
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-quit:
			logrus.Info("Jobs shutdown requested")
			return
		case <-autoRenewTicker.C:
			runJob("auto_renewal", func() error {
				return subscriptionService.RunAutoRenewalBatch(ctx)
			})
		case <-pendingTicker.C:
			runJob("pending_cleanup", func() error {
				return subscriptionService.RunPendingPaymentCleanupBatch(ctx)
			})
		case <-expirationTicker.C:
			runJob("expiration", func() error {
				return subscriptionService.RunExpirationBatch(ctx)
			})
		}
	}
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
