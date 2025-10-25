package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	LogLevel    string
	Strategy    StrategyConfig
	Redis       RedisConfig
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Instruments []string   // 要監控的交易對列表，例如: BTC-USDT,ETH-USDT
	Type        string     // 策略類型: grid, dca, etc.
	Grid        GridConfig // 網格策略參數
}

// GridConfig 網格策略配置
type GridConfig struct {
	TakeProfitMin float64 // 最小停利百分比
	TakeProfitMax float64 // 最大停利百分比
	MaxPositions  int     // 最大持倉數量
	MaxNotional   float64 // 最大持倉名義價值（美元）
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

	instruments := getEnvOrDefault("STRATEGY_INSTRUMENTS", "BTC-USDT,ETH-USDT")
	instList := parseInstruments(instruments)

	cfg := &Config{
		Port:        requireEnv("PORT"),
		Environment: requireEnv("ENVIRONMENT"),
		LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
		Strategy: StrategyConfig{
			Instruments: instList,
			Type:        getEnvOrDefault("STRATEGY_TYPE", "grid"),
			Grid: GridConfig{
				TakeProfitMin: getEnvFloatOrDefault("GRID_TP_MIN", 0.001), // 0.1%
				TakeProfitMax: getEnvFloatOrDefault("GRID_TP_MAX", 0.003), // 0.3%
				MaxPositions:  getEnvIntOrDefault("GRID_MAX_POSITIONS", 30),
				MaxNotional:   getEnvFloatOrDefault("GRID_MAX_NOTIONAL", 3000.0),
			},
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

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Printf("⚠️  Invalid float value for %s, using default: %f", key, defaultValue)
		return defaultValue
	}
	return floatValue
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
