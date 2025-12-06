package config

import "time"

// UserConfig (formerly AuthConfig)
type UserConfig struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Nacos    NacosConfig
}

// LoadUserConfig loads configuration for User Service
func LoadUserConfig() *UserConfig {
	dbConfig := DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "casino_user"),
		Password: getEnv("DB_PASSWORD", "casino_pass"),
		Name:     getEnv("DB_NAME", "casino_db"),
	}

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

	return &UserConfig{
		Server: ServerConfig{
			Port:     getEnv("AUTH_SERVER_PORT", "50051"),
			HTTPPort: getEnv("AUTH_HTTP_PORT", "8082"),
			Name:     "user-service",
		},
		Database: dbConfig,
		Redis:    redisConfig,
		JWT: JWTConfig{
			Secret:   getEnv("JWT_SECRET", "dev-secret-key"),
			Duration: 24 * time.Hour,
		},
		Nacos: nacosConfig,
	}
}
