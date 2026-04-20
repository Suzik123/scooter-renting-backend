package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	Version     string
	Environment string `env:"ENVIRONMENT" envDefault:"development"`

	DB     DBConfig
	Server ServerConfig
	JWT    JWTConfig
	Bcrypt BcryptConfig
}

type DBConfig struct {
	PostgresDSN     string        `env:"POSTGRES_DSN,required"`
	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS" envDefault:"5"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" envDefault:"30m"`
}

type ServerConfig struct {
	Address      string        `env:"SERVER_ADDRESS" envDefault:"0.0.0.0:8080"`
	ReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"15s"`
	CORSOrigins  []string      `env:"CORS_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
}

type JWTConfig struct {
	Secret string        `env:"JWT_SECRET,required"`
	Issuer string        `env:"JWT_ISSUER" envDefault:"uniscoot"`
	TTL    time.Duration `env:"JWT_TTL" envDefault:"24h"`
}

type BcryptConfig struct {
	Cost int `env:"BCRYPT_COST" envDefault:"12"`
}

// New loads .env if present, then parses environment variables into Config.
func New() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
