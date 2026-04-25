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
	Minio        MinioConfig
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:         getEnv("PORT", "3006"),
		Env:          getEnv("ENV", "development"),
		KafkaBroker:  getEnv("KAFKA_BROKER", "kafka:9092"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "audit-service-group"),
		Minio: MinioConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("MINIO_BUCKET", "audit-logs"),
			UseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
