package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ak/kws/internal/app"
	"github.com/ak/kws/internal/infrastructure/config"
	"github.com/ak/kws/internal/infrastructure/database"
	"github.com/ak/kws/internal/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kws",
		Short: "Kitchen Web Service - Cloud orchestrator for KOS instances",
		Long: `KWS (Kitchen Web Service) is a B2B cloud platform that manages
multiple KOS (Kitchen Operating System) instances across different tenants,
regions, and sites. It provides centralized recipe management, order routing,
and operational analytics.`,
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("KWS version %s (built %s)\n", version, buildTime)
		},
	})

	// Serve command
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the KWS server",
		RunE:  runServe,
	}
	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	logger.SetGlobal(log)
	defer log.Sync()

	log.Info("Starting KWS",
		zap.String("version", version),
		zap.String("environment", cfg.App.Env),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize MongoDB
	mongodb, err := database.NewMongoDB(cfg.MongoDB, log)
	if err != nil {
		return fmt.Errorf("failed to create MongoDB client: %w", err)
	}

	if err := mongodb.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := mongodb.Close(shutdownCtx); err != nil {
			log.Error("Failed to close MongoDB connection", zap.Error(err))
		}
	}()

	// Create application
	application, err := app.New(cfg, log, mongodb)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.GetAddress(),
		Handler:      application.Router(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", zap.String("address", cfg.GetAddress()))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		log.Info("Shutdown signal received", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	log.Info("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Info("Server shutdown complete")
	return nil
}
