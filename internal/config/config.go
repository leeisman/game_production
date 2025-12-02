package config

import (
	"os"
	"strconv"
)

// MonolithConfig holds all configuration for monolith mode
type MonolithConfig struct {
	User      UserConfig
	ColorGame ColorGameConfig
	Gateway   GatewayConfig
}

// LoadMonolithConfig loads all configurations for monolith mode
func LoadMonolithConfig() *MonolithConfig {
	userCfg := LoadUserConfig()
	colorGameCfg := LoadColorGameConfig()
	gatewayCfg := LoadGatewayConfig()

	return &MonolithConfig{
		User:      *userCfg,
		ColorGame: *colorGameCfg,
		Gateway:   *gatewayCfg,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}
