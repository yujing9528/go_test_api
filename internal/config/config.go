package config

import (
	"os"
	"time"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func Load(defaultAddr string) Config {
	// 从环境变量读取配置，未设置时使用默认值
	return Config{
		Addr:         getEnv("ADDR", defaultAddr),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/go_api?sslmode=disable"),
		ReadTimeout:  getEnvDuration("READ_TIMEOUT", 5*time.Second),
		WriteTimeout: getEnvDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:  getEnvDuration("IDLE_TIMEOUT", 120*time.Second),
	}
}

func getEnv(key, fallback string) string {
	// 读取字符串环境变量
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	// 读取时间长度环境变量（如 5s/1m）
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
