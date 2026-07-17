package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                    string
	Port                   string
	DatabaseURL            string
	SchedulerEnabled       bool
	SchedulerInterval      time.Duration
	SchedulerRunOnStart    bool
	SchedulerRunTimeout    time.Duration
	ResendAPIKey           string
	ResendFromEmail        string
	ResendFromName         string
	PublicBaseURL          string
	RegistrationEnabled    bool
	TrustProxyHeaders      bool
	AuthCSRFKey            string
	AuthSessionTTL         time.Duration
	AuthVerificationTTL    time.Duration
	AuthPasswordResetTTL   time.Duration
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
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

	registrationEnabled, err := getEnvBool("REGISTRATION_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	trustProxyHeaders, err := getEnvBool("TRUST_PROXY_HEADERS", false)
	if err != nil {
		return Config{}, err
	}
	authSessionTTL, err := getEnvDuration("AUTH_SESSION_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	authVerificationTTL, err := getEnvDuration("AUTH_VERIFICATION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	authPasswordResetTTL, err := getEnvDuration("AUTH_PASSWORD_RESET_TTL", time.Hour)
	if err != nil {
		return Config{}, err
	}

	env := getEnv("ENV", "development")
	port := getEnv("PORT", "8080")
	publicBaseURL := strings.TrimRight(getEnv("PUBLIC_BASE_URL", ""), "/")
	authCSRFKey := getEnv("AUTH_CSRF_KEY", "")
	resendAPIKey := getEnv("RESEND_API_KEY", "")
	resendFromEmail := getEnv("RESEND_FROM_EMAIL", "")
	bootstrapEmail := getEnv("BOOTSTRAP_ADMIN_EMAIL", "")
	bootstrapPassword := getEnv("BOOTSTRAP_ADMIN_PASSWORD", "")

	isProduction := strings.EqualFold(env, "production")
	if isProduction {
		if len(authCSRFKey) < 32 {
			return Config{}, fmt.Errorf("AUTH_CSRF_KEY must contain at least 32 bytes in production")
		}
		if err := validateProductionBaseURL(publicBaseURL); err != nil {
			return Config{}, err
		}
		if resendAPIKey == "" || resendFromEmail == "" {
			return Config{}, fmt.Errorf("RESEND_API_KEY and RESEND_FROM_EMAIL are required for account email in production")
		}
	} else {
		if authCSRFKey == "" {
			authCSRFKey = "development-only-csrf-key-change-me"
		}
		if publicBaseURL == "" {
			publicBaseURL = "http://localhost:" + port
		}
	}
	if (bootstrapEmail == "") != (bootstrapPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}

	return Config{
		Env:                    env,
		Port:                   port,
		DatabaseURL:            databaseURL,
		SchedulerEnabled:       schedulerEnabled,
		SchedulerInterval:      schedulerInterval,
		SchedulerRunOnStart:    schedulerRunOnStart,
		SchedulerRunTimeout:    schedulerRunTimeout,
		ResendAPIKey:           resendAPIKey,
		ResendFromEmail:        resendFromEmail,
		ResendFromName:         getEnv("RESEND_FROM_NAME", ""),
		PublicBaseURL:          publicBaseURL,
		RegistrationEnabled:    registrationEnabled,
		TrustProxyHeaders:      trustProxyHeaders,
		AuthCSRFKey:            authCSRFKey,
		AuthSessionTTL:         authSessionTTL,
		AuthVerificationTTL:    authVerificationTTL,
		AuthPasswordResetTTL:   authPasswordResetTTL,
		BootstrapAdminEmail:    bootstrapEmail,
		BootstrapAdminPassword: bootstrapPassword,
	}, nil
}

func validateProductionBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be an HTTPS origin without credentials, query, or fragment in production")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("PUBLIC_BASE_URL must not contain a path in production")
	}
	return nil
}
