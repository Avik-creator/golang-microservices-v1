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

	"avikmukherjee/m/notification-service/config"

	"avikmukherjee/m/notification-service/internal/consumer"
	"avikmukherjee/m/notification-service/internal/service"
)

func main() {
	cfg := config.Load()

	// ── Mailer ───────────────────────────────────────────────────────
	mailer := service.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	// ── Kafka Consumer ───────────────────────────────────────────────
	notifConsumer := consumer.NewNotificationConsumer(cfg.KafkaBroker, cfg.KafkaGroupID, mailer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consuming both topics in background
	go notifConsumer.Start(ctx)
	defer notifConsumer.Close()

	// ── HTTP Server (health only) ─────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/api/v1/notifications/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","service":"notification-service"}`)
	})

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
		log.Printf("[notification-service] listening on :%s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[notification-service] shutting down...")

	cancel() // stop Kafka consumers

	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	srv.Shutdown(shutdownCtx)

	log.Println("[notification-service] stopped")
}
