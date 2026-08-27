package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config 应用配置，全部来自环境变量，默认值适合本地开发。
type Config struct {
	ProjectName string
	Port        string
	SecretKey   string
	DatabaseURL string
	DatabasePass string // 数据库主密码，空表示明文模式（仅开发环境）

	AdminUsername string
	AdminPassword string
	AdminNickname string
}

// Load 读取 .env 与环境变量，构造配置。
func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		ProjectName:   getEnv("PROJECT_NAME", "拾财 PennyPick"),
		Port:          getEnv("PORT", "8003"),
		SecretKey:     getEnv("SECRET_KEY", "please-change-me-to-a-random-secret"),
		DatabaseURL:   getEnv("DATABASE_URL", "sqlite:///./pennypick.db"),
		DatabasePass:  os.Getenv("PENNYPICK_DB_PASS"),
		AdminUsername: getEnv("INIT_ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("INIT_ADMIN_PASSWORD", "admin123"),
		AdminNickname: getEnv("INIT_ADMIN_NICKNAME", "主人"),
	}

	if cfg.SecretKey == "please-change-me-to-a-random-secret" {
		log.Println("[warn] 请修改 SECRET_KEY 为强随机值")
	}
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
