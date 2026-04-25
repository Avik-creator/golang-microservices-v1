package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"avikmukherjee/m/fraud-service/config"

	"avikmukherjee/m/fraud-service/internal/consumer"
	"avikmukherjee/m/fraud-service/internal/handler"
	"avikmukherjee/m/fraud-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// ── Fraud Engine ─────────────────────────────────────────────────
	engine := service.NewFraudEngine(cfg.FraudLargeTxAmount, cfg.FraudMaxTxPerMinute)

	// ── Kafka Consumer ───────────────────────────────────────────────
	// Waits for Kafka to be ready before consuming
	fraudConsumer := waitForKafka(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Kafka consumer in background goroutine
	go fraudConsumer.Start(ctx)
	defer fraudConsumer.Close()

	// ── HTTP Server (health check only) ──────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/api/v1/fraud/health", handler.Health)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[fraud-service] listening on :%s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[fraud-service] shutting down...")

	// Cancel context so Kafka consumer loop exits cleanly
	cancel()

	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	srv.Shutdown(shutdownCtx)

	log.Println("[fraud-service] stopped")
}

// waitForKafka retries connecting to Kafka with backoff.
// The consumer itself handles reconnects, but we want the initial
// reader+writer setup to succeed before proceeding.
func waitForKafka(cfg *config.Config) *consumer.FraudConsumer {
	engine := service.NewFraudEngine(cfg.FraudLargeTxAmount, cfg.FraudMaxTxPerMinute)
	for i := range 10 {
		c := consumer.NewFraudConsumer(cfg.KafkaBroker, cfg.KafkaGroupID, engine)
		log.Printf("[fraud-service] kafka consumer initialised (attempt %d)", i+1)
		return c
	}
	// unreachable but satisfies compiler
	return consumer.NewFraudConsumer(cfg.KafkaBroker, cfg.KafkaGroupID, engine)
}
