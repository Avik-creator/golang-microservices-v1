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

	"avikmukherjee/m/account-service/config"
	"avikmukherjee/m/account-service/internal/handler"
	"avikmukherjee/m/account-service/internal/middleware"
	"avikmukherjee/m/account-service/internal/repository"
	"avikmukherjee/m/account-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx := context.Background()
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer pool.Close()

	repo := repository.NewAccountRepository(pool)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	svc := service.NewAccountService(repo)
	h := handler.NewAccountHandler(svc, cfg.InternalSecret)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Internal-Secret"},
	}))

	// Public health
	r.Get("/api/v1/accounts/health", h.Health)

	// Protected routes (JWT required)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.JWTSecret))
		r.Post("/api/v1/accounts", h.CreateAccount)
		r.Get("/api/v1/accounts", h.ListAccounts)
		r.Get("/api/v1/accounts/{accountID}", h.GetAccount)
	})

	// Internal routes — only reachable inside Docker network, not via Nginx
	r.Post("/internal/accounts/balance", h.UpdateBalance)

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
		log.Printf("[account-service] listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[account-service] shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for i := range 10 {
		pool, err = pgxpool.New(ctx, cfg.DB.DSN())
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Println("[account-service] connected to postgres")
				return pool, nil
			}
		}
		wait := time.Duration(i+1) * time.Second
		log.Printf("[account-service] postgres not ready, retrying in %s...", wait)
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("could not connect: %w", err)
}
