package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	Env            string
	DB             DBConfig
	JWTSecret      string
	InternalSecret string
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		d.Host, d.Port, d.Name, d.User, d.Password,
	)
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		Port:           getEnv("PORT", "3002"),
		Env:            getEnv("ENV", "development"),
		JWTSecret:      getEnv("JWT_SECRET", "default-secret"),
		InternalSecret: getEnv("INTERNAL_SECRET", "internal-secret-change-me"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			Name:     getEnv("DB_NAME", "accountdb"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "secret"),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
