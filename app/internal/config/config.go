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

	DB       DBConfig
	Server   ServerConfig
	JWT      JWTConfig
	Bcrypt   BcryptConfig
	Stripe   StripeConfig
	Google   GoogleConfig
	Redis    RedisConfig
	Rabbit   RabbitConfig
	SMTP     SMTPConfig
	Auth     AuthConfig
	Frontend FrontendConfig
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

type StripeConfig struct {
	SecretKey     string `env:"STRIPE_SECRET_KEY,required"`
	WebhookSecret string `env:"STRIPE_WEBHOOK_SECRET,required"`
	Currency      string `env:"STRIPE_CURRENCY" envDefault:"usd"`
}

type GoogleConfig struct {
	ClientID     string `env:"GOOGLE_CLIENT_ID,required"`
	HostedDomain string `env:"GOOGLE_HOSTED_DOMAIN"`
}

type RedisConfig struct {
	URL string `env:"REDIS_URL" envDefault:"redis://redis:6379/0"`
}

type RabbitConfig struct {
	URL            string        `env:"RABBIT_URL" envDefault:"amqp://guest:guest@rabbitmq:5672/"`
	PublishTimeout time.Duration `env:"RABBIT_PUBLISH_TIMEOUT" envDefault:"5s"`
}

// AuthConfig groups auth-flow knobs that are not strictly JWT-related.
type AuthConfig struct {
	PasswordResetTTL time.Duration `env:"PASSWORD_RESET_TTL" envDefault:"30m"`
}

// FrontendConfig captures the public URL of the SPA, used to build links in
// transactional emails (currently only the password-reset confirm link).
type FrontendConfig struct {
	BaseURL string `env:"FRONTEND_BASE_URL" envDefault:"http://localhost:5173"`
}

type SMTPConfig struct {
	Host     string `env:"SMTP_HOST" envDefault:"mailhog"`
	Port     int    `env:"SMTP_PORT" envDefault:"1025"`
	From     string `env:"SMTP_FROM" envDefault:"noreply@uniscoot.local"`
	User     string `env:"SMTP_USER" envDefault:""`
	Password string `env:"SMTP_PASSWORD" envDefault:""`
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
