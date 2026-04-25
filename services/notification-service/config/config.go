package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	Env          string
	KafkaBroker  string
	KafkaGroupID string
	SMTPHost     string
	SMTPPort     string
	SMTPFrom     string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:         getEnv("PORT", "3005"),
		Env:          getEnv("ENV", "development"),
		KafkaBroker:  getEnv("KAFKA_BROKER", "kafka:9092"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "notification-service-group"),
		SMTPHost:     getEnv("SMTP_HOST", "mailhog"),
		SMTPPort:     getEnv("SMTP_PORT", "1025"),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@fintech.local"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
