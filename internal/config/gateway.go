package config

import "time"

type GatewayConfig struct {
	Server    ServerConfig
	WebSocket WebSocketConfig
	Redis     RedisConfig
	Nacos     NacosConfig
}

type WebSocketConfig struct {
	PingInterval   time.Duration
	WriteWait      time.Duration
	PongWait       time.Duration
	MaxMessageSize int64
}

// LoadGatewayConfig loads configuration for Gateway Service
func LoadGatewayConfig() *GatewayConfig {
	redisConfig := RedisConfig{
		Host: getEnv("REDIS_HOST", "localhost"),
		Port: getEnv("REDIS_PORT", "6379"),
	}

	nacosConfig := NacosConfig{
		Host:        getEnv("NACOS_HOST", "localhost"),
		Port:        getEnv("NACOS_PORT", "8848"),
		NamespaceID: getEnv("NACOS_NAMESPACE", "public"),
		Group:       getEnv("NACOS_GROUP", "DEFAULT_GROUP"),
	}

	return &GatewayConfig{
		Server: ServerConfig{
			Port: getEnv("GATEWAY_SERVER_PORT", "8081"),
			Name: "gateway-service",
		},
		WebSocket: WebSocketConfig{
			PingInterval:   54 * time.Second,
			WriteWait:      10 * time.Second,
			PongWait:       60 * time.Second,
			MaxMessageSize: 512,
		},
		Redis: redisConfig,
		Nacos: nacosConfig,
	}
}
