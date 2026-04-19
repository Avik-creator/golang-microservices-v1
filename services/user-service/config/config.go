package config

import (
	"fmt"
	"strconv"
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	Env            string
	DB             DBConfig
	JWTSecret      string
	JWTExpiryHours int
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

// DSN returns a pgx-compatible connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		d.Host, d.Port, d.Name, d.User, d.Password,
	)
}

func Load() (*Config, error) {
	// Load .env if present (ignored in prod containers where env vars are injected)
	_ = godotenv.Load()

	jwtExpiry, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS: %w", err)
	}

	return &Config{
		Port: getEnv("PORT", "3001"),
		Env:  getEnv("ENV", "development"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			Name:     getEnv("DB_NAME", "userdb"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "secret"),
		},
		JWTSecret:      getEnv("JWT_SECRET", "default-secret"),
		JWTExpiryHours: jwtExpiry,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
