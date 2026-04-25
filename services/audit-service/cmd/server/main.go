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

	"avikmukherjee/m/audit-service/config"
	"avikmukherjee/m/audit-service/internal/consumer"
	"avikmukherjee/m/audit-service/internal/handler"
	"avikmukherjee/m/audit-service/internal/service"
)

func main() {
	cfg := config.Load()

	// ── MinIO Audit Store ─────────────────────────────────────────────
	store, err := service.NewAuditStore(
		cfg.Minio.Endpoint,
		cfg.Minio.AccessKey,
		cfg.Minio.SecretKey,
		cfg.Minio.Bucket,
		cfg.Minio.UseSSL,
	)
	if err != nil {
		log.Fatalf("[audit-service] failed to connect to MinIO: %v", err)
	}

	// ── Kafka Consumer ────────────────────────────────────────────────
	auditConsumer := consumer.NewAuditConsumer(cfg.KafkaBroker, cfg.KafkaGroupID, store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go auditConsumer.Start(ctx)
	defer auditConsumer.Close()

	// ── HTTP Server ───────────────────────────────────────────────────
	h := handler.NewAuditHandler(store)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/api/v1/audit/health", h.Health)
	r.Get("/api/v1/audit/logs", h.ListLogs)

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
		log.Printf("[audit-service] listening on :%s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[audit-service] shutting down...")

	cancel() // stop Kafka consumers

	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	srv.Shutdown(shutdownCtx)

	log.Println("[audit-service] stopped")
}
