package config

import (
	"fmt"
	"os"
	"strconv"
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
	DigestEnabled       bool
	DigestToEmail       string
	DigestTopN          int
	DigestLookback      time.Duration
	ResendAPIKey        string
	ResendFromEmail     string
	ResendFromName      string
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func mustGetEnv(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("%s must be set", key)
	}
	return val, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	val := os.Getenv(key)
	if val == "" {
		return fallback, nil
	}

	switch val {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes", "on", "ON", "On":
		return true, nil
	case "0", "false", "FALSE", "False", "no", "NO", "No", "off", "OFF", "Off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean value", key)
	}
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	val := os.Getenv(key)
	if val == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", key)
	}

	return duration, nil
}

func getEnvInt(key string, fallback int) (int, error) {
	val := os.Getenv(key)
	if val == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", key)
	}

	return parsed, nil
}

func Load() (Config, error) {
	databaseURL, err := mustGetEnv("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	schedulerEnabled, err := getEnvBool("SCHEDULER_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	schedulerInterval, err := getEnvDuration("SCHEDULER_INTERVAL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	schedulerRunOnStart, err := getEnvBool("SCHEDULER_RUN_ON_START", true)
	if err != nil {
		return Config{}, err
	}

	schedulerRunTimeout, err := getEnvDuration("SCHEDULER_RUN_TIMEOUT", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	digestEnabled, err := getEnvBool("DIGEST_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	digestTopN, err := getEnvInt("DIGEST_TOP_N", 10)
	if err != nil {
		return Config{}, err
	}

	digestLookback, err := getEnvDuration("DIGEST_LOOKBACK", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Env:                 getEnv("ENV", "development"),
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         databaseURL,
		SchedulerEnabled:    schedulerEnabled,
		SchedulerInterval:   schedulerInterval,
		SchedulerRunOnStart: schedulerRunOnStart,
		SchedulerRunTimeout: schedulerRunTimeout,
		DigestEnabled:       digestEnabled,
		DigestToEmail:       getEnv("DIGEST_TO_EMAIL", ""),
		DigestTopN:          digestTopN,
		DigestLookback:      digestLookback,
		ResendAPIKey:        getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:     getEnv("RESEND_FROM_EMAIL", ""),
		ResendFromName:      getEnv("RESEND_FROM_NAME", ""),
	}, nil
}
