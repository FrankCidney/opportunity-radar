package config

import (
	"os"
	"time"
)

type Config struct {
	Env                 string
	Port                string
	DatabaseURL         string
	SchedulerEnabled    bool
	SchedulerInterval   time.Duration
	SchedulerRunOnStart bool
	SchedulerRunTimeout time.Duration
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

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	switch val {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes", "on", "ON", "On":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "No", "off", "OFF", "Off":
		return false
	default:
		panic(key + " must be a boolean value")
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	duration, err := time.ParseDuration(val)
	if err != nil {
		panic(key + " must be a valid duration")
	}

	return duration
}

func Load() Config {
	return Config{
		Env:                 getEnv("ENV", "development"),
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         mustGetEnv("DATABASE_URL"),
		SchedulerEnabled:    getEnvBool("SCHEDULER_ENABLED", true),
		SchedulerInterval:   getEnvDuration("SCHEDULER_INTERVAL", 24*time.Hour),
		SchedulerRunOnStart: getEnvBool("SCHEDULER_RUN_ON_START", true),
		SchedulerRunTimeout: getEnvDuration("SCHEDULER_RUN_TIMEOUT", 30*time.Minute),
	}
}
