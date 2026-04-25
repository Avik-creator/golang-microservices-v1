package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	Env                  string
	KafkaBroker          string
	KafkaGroupID         string
	FraudLargeTxAmount   float64 // flag if single tx exceeds this
	FraudMaxTxPerMinute  int     // flag if account sends more than this per minute
	InternalSecret       string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	largeTx, err := strconv.ParseFloat(getEnv("FRAUD_LARGE_TX_THRESHOLD", "100000"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid FRAUD_LARGE_TX_THRESHOLD: %w", err)
	}

	maxTx, err := strconv.Atoi(getEnv("FRAUD_MAX_TX_PER_MINUTE", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid FRAUD_MAX_TX_PER_MINUTE: %w", err)
	}

	return &Config{
		Port:                getEnv("PORT", "3004"),
		Env:                 getEnv("ENV", "development"),
		KafkaBroker:         getEnv("KAFKA_BROKER", "kafka:9092"),
		KafkaGroupID:        getEnv("KAFKA_GROUP_ID", "fraud-service-group"),
		FraudLargeTxAmount:  largeTx,
		FraudMaxTxPerMinute: maxTx,
		InternalSecret:      getEnv("INTERNAL_SECRET", "internal-secret-change-me"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
