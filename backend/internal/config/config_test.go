package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
)

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()

	// Секреты без значения должны ронять старт, а не подставлять умолчание:
	// иначе небезопасная конфигурация уезжает в деплой незаметно (SEC5).
	require.Error(t, err)
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "local-password")
	t.Setenv("JWT_SECRET", "local-development-secret")

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, 8080, cfg.HTTP.Port)
	require.Equal(t, 80, cfg.Scoring.Thresholds.Resilient)
	require.Equal(t, 60, cfg.Scoring.Thresholds.Attentive)
	require.False(t, cfg.Production())
}

func TestDSNEscapesPassword(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "p@ss:word/with?special")
	t.Setenv("JWT_SECRET", "local-development-secret")

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Contains(t, cfg.Postgres.DSN(), "p%40ss%3Aword%2Fwith%3Fspecial")
}
