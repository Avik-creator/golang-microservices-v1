package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	Env               string
	DB                DBConfig
	JWTSecret         string
	KafkaBroker       string
	AccountServiceURL string
	InternalSecret    string
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
		Port:              getEnv("PORT", "3003"),
		Env:               getEnv("ENV", "development"),
		JWTSecret:         getEnv("JWT_SECRET", "default-secret"),
		KafkaBroker:       getEnv("KAFKA_BROKER", "kafka:9092"),
		AccountServiceURL: getEnv("ACCOUNT_SERVICE_URL", "http://account-service:3002"),
		InternalSecret:    getEnv("INTERNAL_SECRET", "internal-secret-change-me"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			Name:     getEnv("DB_NAME", "transactiondb"),
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
