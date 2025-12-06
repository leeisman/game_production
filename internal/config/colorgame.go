package config

// ColorGameConfig (formerly GameConfig)
type ColorGameConfig struct {
	Server   ServerConfig
	Redis    RedisConfig
	Database DatabaseConfig
	Nacos    NacosConfig
	RepoType string
	Settings GameSettings
}

type GameSettings struct {
	MaxPlayersPerRoom int
}

// LoadColorGameConfig loads configuration for ColorGame Service
func LoadColorGameConfig() *ColorGameConfig {
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

	dbConfig := DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "casino_user"),
		Password: getEnv("DB_PASSWORD", "casino_pass"),
		Name:     getEnv("DB_NAME", "casino_db"),
	}

	return &ColorGameConfig{
		Server: ServerConfig{
			Port: getEnv("GAME_SERVER_PORT", "50052"),
			Name: "game-service",
		},
		Redis:    redisConfig,
		Database: dbConfig,
		Nacos:    nacosConfig,
		RepoType: getEnv("COLORGAME_REPO_TYPE", "memory"),
		Settings: GameSettings{
			MaxPlayersPerRoom: getEnvInt("GAME_MAX_PLAYERS", 100),
		},
	}
}
