package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	Cache     CacheConfig     `yaml:"cache"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Email     EmailConfig     `yaml:"email"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	Expiration time.Duration `yaml:"expiration"`
}

type CacheConfig struct {
	TaskTTL time.Duration `yaml:"task_ttl"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type EmailConfig struct {
	ServiceURL   string        `yaml:"service_url"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxFailures  int           `yaml:"max_failures"`
	ResetTimeout time.Duration `yaml:"reset_timeout"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.Server.Port = 8080
	cfg.Server.ReadTimeout = 15 * time.Second
	cfg.Server.WriteTimeout = 15 * time.Second
	cfg.Server.IdleTimeout = 60 * time.Second
	cfg.Database.MaxOpenConns = 25
	cfg.Database.MaxIdleConns = 10
	cfg.Database.ConnMaxLifetime = 5 * time.Minute
	cfg.JWT.Expiration = 24 * time.Hour
	cfg.Cache.TaskTTL = 5 * time.Minute
	cfg.RateLimit.RequestsPerMinute = 100
	cfg.Email.MaxFailures = 5
	cfg.Email.ResetTimeout = 60 * time.Second
	cfg.Email.Timeout = 5 * time.Second

	if path := os.Getenv("CONFIG_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		cfg.Database.DSN = dsn
	}
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		cfg.Redis.Addr = redisAddr
	}
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.JWT.Secret = secret
	}

	return cfg, nil
}
