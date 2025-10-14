package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	LogLevel    string
	OKX         OKXConfig
}

type OKXConfig struct {
	Instruments []string // 要訂閱的交易對列表，例如: BTC-USDT,ETH-USDT
}

var AppConfig *Config

// Load loads configuration from environment variables and returns it
// Also sets the global AppConfig for backward compatibility
func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️  No .env file found")
	}

	instruments := getEnvOrDefault("OKX_INSTRUMENTS", "BTC-USDT,ETH-USDT")
	instList := parseInstruments(instruments)

	cfg := &Config{
		Port:        requireEnv("PORT"),
		Environment: requireEnv("ENVIRONMENT"),
		LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
		OKX: OKXConfig{
			Instruments: instList,
		},
	}

	AppConfig = cfg // Keep global for backward compatibility
	return cfg
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("❌ Environment variable %s is required but not set", key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func parseInstruments(instruments string) []string {
	if instruments == "" {
		return []string{}
	}

	result := []string{}
	for _, inst := range splitByComma(instruments) {
		trimmed := trimSpace(inst)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitByComma(s string) []string {
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
