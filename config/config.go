package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port, DatabaseURL, JWTSecret, LLMAPIKey, LLMBaseURL, LLMModel, StoragePath, AppBaseURL string
}

func Load() Config {
	_ = godotenv.Load()
	return Config{Port: value("APP_PORT", "8080"), DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: value("JWT_SECRET", "development-secret-change-me"), LLMAPIKey: os.Getenv("LLM_API_KEY"), LLMBaseURL: strings.TrimRight(value("LLM_BASE_URL", "https://api.openai.com/v1"), "/"), LLMModel: value("LLM_MODEL", "gpt-4.1-mini"), StoragePath: value("DOCUMENT_STORAGE_PATH", "./storage"), AppBaseURL: strings.TrimRight(value("APP_BASE_URL", "http://localhost:8080"), "/")}
}
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
