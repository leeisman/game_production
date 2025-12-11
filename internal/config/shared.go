package config

import "time"

// --- Shared Configs ---

type ServerConfig struct {
	Port     string // gRPC port
	HTTPPort string // HTTP port for independent service
	Name     string // Service Name for Nacos
	LogLevel string // debug, info, warn, error
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Host string
	Port string
}

type NacosConfig struct {
	Host        string
	Port        string
	NamespaceID string
	Group       string
}

type JWTConfig struct {
	Secret   string
	Duration time.Duration
}
