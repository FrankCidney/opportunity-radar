package config

import "os"

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	} 
	return val
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(key + " must be set")
	}
	return val
}

func Load() Config {
	return Config{
		Env: getEnv("ENV", "development"),
		Port: getEnv("PORT", "8080"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
	}
}
