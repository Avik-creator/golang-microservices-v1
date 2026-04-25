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

	"avikmukherjee.com/m/user-service/config"
	"avikmukherjee.com/m/user-service/internal/handler"
	"avikmukherjee.com/m/user-service/internal/middleware"
	"avikmukherjee.com/m/user-service/internal/repository"
	"avikmukherjee.com/m/user-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// ── Database ─────────────────────────────────────────────────────
	ctx := context.Background()
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		log.Fatalf("db connection error: %v", err)
	}
	defer pool.Close()

// ── Layers ───────────────────────────────────────────────────────
	repo := repository.NewUserRepository(pool)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	svc := service.NewUserService(repo, cfg.JWTSecret, cfg.JWTExpiryHours)
	h := handler.NewUserHandler(svc)

	// ── Router ───────────────────────────────────────────────────────

// ── Router ───────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	// Public routes
	r.Get("/api/v1/users/health", h.Health)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
	})

	// Protected routes
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Use(middleware.Authenticate(svc))
		r.Get("/me", h.GetProfile)
	})

// ── Server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
log.Printf("[user-service] listening on :%s (env: %s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("[user-service] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("[user-service] stopped")

}

// connectDB retries the DB connection with backoff (useful on first docker-compose up).
func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for i := range 10 {
		pool, err = pgxpool.New(ctx, cfg.DB.DSN())
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Println("[user-service] connected to postgres")
				return pool, nil
			}
		}
		wait := time.Duration(i+1) * time.Second
		log.Printf("[user-service] postgres not ready, retrying in %s...", wait)
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("could not connect to postgres after retries: %w", err)
}
