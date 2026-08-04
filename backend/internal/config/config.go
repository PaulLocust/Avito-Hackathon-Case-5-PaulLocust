// Package config читает конфигурацию из переменных окружения (SEC5).
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
)

type Config struct {
	Env      string
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Auth     AuthConfig
	Log      LogConfig
	Scoring  ScoringConfig
	Content  ContentConfig
}

type HTTPConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
}

type PostgresConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Database       string
	SSLMode        string
	MaxConns       int32
	ConnectTimeout time.Duration
	// ConnectRetries — повторы при старте, пока поднимается контейнер БД.
	ConnectRetries int
}

type AuthConfig struct {
	JWTSecret  string
	TokenTTL   time.Duration
	BcryptCost int
}

type LogConfig struct {
	Level  string // debug | info | warn | error
	Format string // json | text
}

// ScoringConfig — пороги уровней задаются конфигурацией, а не константами
// в коде (FR21).
type ScoringConfig struct {
	Thresholds domain.Thresholds
}

type ContentConfig struct {
	SeedDir string
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(p.User), url.QueryEscape(p.Password),
		p.Host, p.Port, p.Database, p.SSLMode,
	)
}

func Load() (Config, error) {
	cfg := Config{
		Env: env("APP_ENV", "development"),
		HTTP: HTTPConfig{
			Port:            envInt("HTTP_PORT", 8080),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
			AllowedOrigins:  envList("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000"}),
		},
		Postgres: PostgresConfig{
			Host:           env("POSTGRES_HOST", "localhost"),
			Port:           envInt("POSTGRES_PORT", 5432),
			User:           env("POSTGRES_USER", "antiscam"),
			Password:       env("POSTGRES_PASSWORD", ""),
			Database:       env("POSTGRES_DB", "antiscam"),
			SSLMode:        env("POSTGRES_SSLMODE", "disable"),
			MaxConns:       int32(envInt("POSTGRES_MAX_CONNS", 10)),
			ConnectTimeout: envDuration("POSTGRES_CONNECT_TIMEOUT", 5*time.Second),
			ConnectRetries: envInt("POSTGRES_CONNECT_RETRIES", 10),
		},
		Auth: AuthConfig{
			JWTSecret:  env("JWT_SECRET", ""),
			TokenTTL:   envDuration("JWT_TTL", 24*time.Hour),
			BcryptCost: envInt("BCRYPT_COST", 10),
		},
		Log: LogConfig{
			Level:  env("LOG_LEVEL", "info"),
			Format: env("LOG_FORMAT", "json"),
		},
		Scoring: ScoringConfig{
			Thresholds: domain.Thresholds{
				Resilient: envInt("SCORING_THRESHOLD_RESILIENT", 80),
				Attentive: envInt("SCORING_THRESHOLD_ATTENTIVE", 60),
			},
		},
		Content: ContentConfig{
			SeedDir: env("SEED_DIR", "./seed"),
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Production() bool { return c.Env == "production" }

// validate: отсутствующий секрет роняет старт, а не подставляет умолчание.
func (c Config) validate() error {
	var problems []string

	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		problems = append(problems, "HTTP_PORT: ожидается число от 1 до 65535")
	}

	if c.Postgres.Password == "" {
		problems = append(problems, "POSTGRES_PASSWORD: обязателен, задайте его в .env")
	}

	if c.Auth.JWTSecret == "" {
		problems = append(problems, "JWT_SECRET: обязателен, задайте его в .env")
	}

	if c.Production() && len(c.Auth.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET: в production требуется не менее 32 символов")
	}

	thresholds := c.Scoring.Thresholds
	if thresholds.Attentive <= 0 || thresholds.Resilient <= thresholds.Attentive || thresholds.Resilient > 100 {
		problems = append(problems, "SCORING_THRESHOLD_*: ожидается 0 < attentive < resilient <= 100")
	}

	if len(problems) > 0 {
		return fmt.Errorf("некорректная конфигурация:\n  - %s", strings.Join(problems, "\n  - "))
	}

	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envList(key string, fallback []string) []string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
