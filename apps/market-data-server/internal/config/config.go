package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	LogLevel    string
	OKX         OKXConfig
	Redis       RedisConfig
}

type OKXConfig struct {
	Instruments  []string              // 要訂閱的交易對列表，例如: BTC-USDT,ETH-USDT
	Subscription SubscriptionSelection // 訂閱選擇器
}

// SubscriptionSelection 訂閱選擇器，控制要啟用哪些頻道
type SubscriptionSelection struct {
	Ticker  bool            // 是否訂閱 Ticker（即時價格）
	Candles map[string]bool // K線訂閱，key 為週期（例如: "1m", "5m", "1H"）
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
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

	// 解析訂閱選擇器
	subscription := parseSubscriptionSelection()

	cfg := &Config{
		Environment: requireEnv("ENVIRONMENT"),
		LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
		OKX: OKXConfig{
			Instruments:  instList,
			Subscription: subscription,
		},
		Redis: RedisConfig{
			Addr:     requireEnv("REDIS_ADDR"),
			Password: getEnvOrDefault("REDIS_PASSWORD", ""),
			DB:       getEnvIntOrDefault("REDIS_DB", 0),
			PoolSize: getEnvIntOrDefault("REDIS_POOL_SIZE", 10),
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

func getEnvIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("⚠️  Invalid integer value for %s, using default: %d", key, defaultValue)
		return defaultValue
	}
	return intValue
}

func parseSubscriptionSelection() SubscriptionSelection {
	// 解析 Ticker 訂閱
	enableTicker := getEnvOrDefault("OKX_SUBSCRIBE_TICKER", "false") == "true"

	// 解析 Candle 訂閱
	// 格式: OKX_SUBSCRIBE_CANDLES=1m,5m,1H
	candlesStr := getEnvOrDefault("OKX_SUBSCRIBE_CANDLES", "")
	candles := make(map[string]bool)

	if candlesStr != "" {
		candleList := splitByComma(candlesStr)
		for _, candle := range candleList {
			trimmed := trimSpace(candle)
			if trimmed != "" {
				candles[trimmed] = true
			}
		}
	}

	return SubscriptionSelection{
		Ticker:  enableTicker,
		Candles: candles,
	}
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
