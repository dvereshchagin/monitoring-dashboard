package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/releaseanalyzer"
	"github.com/dreschagin/monitoring-dashboard/pkg/config"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"

	_ "github.com/lib/pq"
)

func main() {
	baseCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load base config: %v\n", err)
		os.Exit(1)
	}

	analyzerCfg, err := releaseanalyzer.LoadConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load analyzer config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(os.Getenv("LOG_LEVEL"))
	log.Info(
		"Starting release analyzer",
		"interval", analyzerCfg.Interval.String(),
		"port", analyzerCfg.Port,
	)

	db, err := sql.Open("postgres", baseCfg.Database.DSN())
	if err != nil {
		log.Error("Failed to connect to database", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Error("Failed to ping database", err)
		os.Exit(1)
	}

	service := releaseanalyzer.NewService(db)
	runner := releaseanalyzer.NewRunner(service, log, analyzerCfg.Interval)
	handler := releaseanalyzer.NewHandler(runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := runner.RunOnce(ctx); err != nil {
		log.Error("Initial analyzer cycle failed", err)
	}

	go runner.Start(ctx)

	server := &http.Server{
		Addr:         ":" + analyzerCfg.Port,
		Handler:      handler.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Info("Release analyzer HTTP server started", "port", analyzerCfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Release analyzer HTTP server failed", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Info("Shutdown signal received")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Release analyzer HTTP server shutdown failed", err)
	}

	log.Info("Release analyzer stopped")
}
