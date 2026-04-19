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
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"avikmukherjee.com/m/transaction-service/config"
	"avikmukherjee.com/m/transaction-service/internal/handler"
	"avikmukherjee.com/m/transaction-service/internal/kafka"
	"avikmukherjee.com/m/transaction-service/internal/middleware"
	"avikmukherjee.com/m/transaction-service/internal/repository"
	"avikmukherjee.com/m/transaction-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx := context.Background()

	// ── Database ─────────────────────────────────────────────────────
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer pool.Close()

	// ── Migrations ───────────────────────────────────────────────────
	repo := repository.NewTransactionRepository(pool)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	// ── Kafka Producer ───────────────────────────────────────────────
	producer := kafka.NewProducer(cfg.KafkaBroker)
	defer producer.Close()

	// ── Service & Handler ────────────────────────────────────────────
	svc := service.NewTransactionService(repo, producer, cfg.AccountServiceURL, cfg.InternalSecret)
	h := handler.NewTransactionHandler(svc)

	// ── Router ───────────────────────────────────────────────────────
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	// Public
	r.Get("/api/v1/transactions/health", h.Health)

	// Protected (JWT required)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.JWTSecret))
		r.Post("/api/v1/transactions", h.CreateTransaction)
		r.Get("/api/v1/transactions/{txID}", h.GetTransaction)
		r.Get("/api/v1/transactions", h.ListTransactions)
	})

	// ── HTTP Server ──────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[transaction-service] listening on :%s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[transaction-service] shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("[transaction-service] stopped")
}

// connectDB retries with exponential-ish backoff — needed because on first
// docker compose up, postgres may not be ready before the service starts.
func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	var (
		pool *pgxpool.Pool
		err  error
	)
	for i := range 10 {
		pool, err = pgxpool.New(ctx, cfg.DB.DSN())
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Println("[transaction-service] connected to postgres")
				return pool, nil
			}
		}
		wait := time.Duration(i+1) * time.Second
		log.Printf("[transaction-service] postgres not ready, retrying in %s...", wait)
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("could not connect to postgres after retries: %w", err)
}
