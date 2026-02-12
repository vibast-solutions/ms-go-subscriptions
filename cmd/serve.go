package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	authclient "github.com/vibast-solutions/lib-go-auth/client"
	authmiddleware "github.com/vibast-solutions/lib-go-auth/middleware"
	authlibservice "github.com/vibast-solutions/lib-go-auth/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/controller"
	grpcserver "github.com/vibast-solutions/ms-go-subscriptions/app/grpc"
	"github.com/vibast-solutions/ms-go-subscriptions/app/payment"
	"github.com/vibast-solutions/ms-go-subscriptions/app/repository"
	"github.com/vibast-solutions/ms-go-subscriptions/app/service"
	"github.com/vibast-solutions/ms-go-subscriptions/app/types"
	"github.com/vibast-solutions/ms-go-subscriptions/config"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP and gRPC servers",
	Long:  "Start both HTTP (Echo) and gRPC servers for the subscriptions service.",
	Run:   runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) {
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
	paymentService := payment.NewStubService()
	subscriptionService := service.NewSubscriptionService(subscriptionRepo, subscriptionTypeRepo, planTypeRepo, paymentService, cfg.Subscriptions)
	paymentCallbackService := service.NewPaymentCallbackService(subscriptionRepo, cfg.Subscriptions)
	grpcSubscriptionServer := grpcserver.NewServer(subscriptionService, paymentCallbackService)
	subscriptionController := controller.NewSubscriptionController(subscriptionService, paymentCallbackService)

	authGRPCClient, err := authclient.NewGRPCClientFromAddr(context.Background(), cfg.InternalEndpoints.AuthGRPCAddr)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize auth gRPC client")
	}
	defer authGRPCClient.Close()
	internalAuthService := authlibservice.NewInternalAuthService(authGRPCClient)
	echoInternalAuthMiddleware := authmiddleware.NewEchoInternalAuthMiddleware(internalAuthService)
	grpcInternalAuthMiddleware := authmiddleware.NewGRPCInternalAuthMiddleware(internalAuthService)

	e := setupHTTPServer(subscriptionController, echoInternalAuthMiddleware, cfg.App.ServiceName)
	grpcSrv, lis := setupGRPCServer(cfg, grpcSubscriptionServer, grpcInternalAuthMiddleware, cfg.App.ServiceName)

	go func() {
		httpAddr := net.JoinHostPort(cfg.HTTP.Host, cfg.HTTP.Port)
		logrus.WithField("addr", httpAddr).Info("Starting HTTP server")
		if err := e.Start(httpAddr); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("HTTP server error")
		}
	}()

	go func() {
		logrus.WithField("addr", lis.Addr().String()).Info("Starting gRPC server")
		if err := grpcSrv.Serve(lis); err != nil {
			logrus.WithError(err).Fatal("gRPC server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Warn("HTTP shutdown error")
	}
	grpcSrv.GracefulStop()

	logrus.Info("Server stopped")
}

func setupHTTPServer(
	subscriptionController *controller.SubscriptionController,
	internalAuthMiddleware *authmiddleware.EchoInternalAuthMiddleware,
	appServiceName string,
) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.Use(echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogURI:       true,
		LogStatus:    true,
		LogMethod:    true,
		LogRemoteIP:  true,
		LogLatency:   true,
		LogUserAgent: true,
		LogError:     true,
		HandleError:  true,
		LogRequestID: true,
		LogValuesFunc: func(_ echo.Context, v echomiddleware.RequestLoggerValues) error {
			fields := logrus.Fields{
				"remote_ip":  v.RemoteIP,
				"host":       v.Host,
				"method":     v.Method,
				"uri":        v.URI,
				"status":     v.Status,
				"latency":    v.Latency.String(),
				"latency_ns": v.Latency.Nanoseconds(),
				"user_agent": v.UserAgent,
			}
			entry := logrus.WithFields(fields)
			if v.Error != nil {
				entry = entry.WithError(v.Error)
			}
			entry.Info("http_request")
			return nil
		},
	}))
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		Generator: func() string {
			return fmt.Sprintf("rest-%s", uuid.New().String())
		},
	}))
	e.Use(internalAuthMiddleware.RequireInternalAccess(appServiceName))

	e.GET("/health", subscriptionController.Health)

	e.GET("/subscription-types", subscriptionController.ListSubscriptionTypes)

	subscriptions := e.Group("/subscriptions")
	subscriptions.POST("", subscriptionController.CreateSubscription)
	subscriptions.GET("", subscriptionController.ListSubscriptions)
	subscriptions.GET("/:id", subscriptionController.GetSubscription)
	subscriptions.PATCH("/:id", subscriptionController.UpdateSubscription)
	subscriptions.DELETE("/:id", subscriptionController.DeleteSubscription)
	subscriptions.POST("/:id/cancel", subscriptionController.CancelSubscription)

	webhooks := e.Group("/webhooks")
	webhooks.POST("/payment-callback", subscriptionController.PaymentCallback)

	return e
}

func setupGRPCServer(
	cfg *config.Config,
	subscriptionServer *grpcserver.Server,
	internalAuthMiddleware *authmiddleware.GRPCInternalAuthMiddleware,
	appServiceName string,
) (*grpc.Server, net.Listener) {
	grpcAddr := net.JoinHostPort(cfg.GRPC.Host, cfg.GRPC.Port)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcserver.RecoveryInterceptor(),
			grpcserver.RequestIDInterceptor(),
			grpcserver.LoggingInterceptor(),
			internalAuthMiddleware.UnaryRequireInternalAccess(appServiceName),
		),
	)
	types.RegisterSubscriptionsServiceServer(grpcSrv, subscriptionServer)

	return grpcSrv, lis
}
