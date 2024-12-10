package app

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type LoggerConfig struct {
	Level       string `yaml:"level" env:"LOGGER_LEVEL" env-default:"info"`
	HandlerType string `yaml:"handler_type" env:"LOGGER_HANDLER_TYPE" env-default:"text"`
}

type PgConfig struct {
	ConnectionURI string `yaml:"connection_uri" env:"PG_CONNECTION_URI" env-required:"true"`
	MigrationsURI string `yaml:"migrations_uri" env:"PG_MIGRATIONS_URI" env-default:"file://migrations"`
}

type ServerConfig struct {
	Address string `yaml:"address" env:"SERVER_ADDRESS" env-default:"0.0.0.0:8080"`
}

type AuthConfig struct {
	Secret string `yaml:"secret" env:"AUTH_SECRET" env-required:"true"`
}

type SmtpConfig struct {
	From     string `yaml:"from" env:"SMTP_FROM" env-required:"true"`
	Host     string `yaml:"host" env:"SMTP_HOST" env-required:"true"`
	Port     int    `yaml:"port" env:"SMTP_PORT" env-default:"587"`
	Username string `yaml:"username" env:"SMTP_USERNAME" env-required:"true"`
	Password string `yaml:"password" env:"SMTP_PASSWORD" env-required:"true"`
	TLS      bool   `yaml:"tls" env:"SMTP_TLS" env-default:"true"`
}

type Config struct {
	Logger   LoggerConfig
	Postgres PgConfig
	Server   ServerConfig
	Auth     AuthConfig
	Smtp     SmtpConfig
}

func mustLoadConfig(configPath string) *Config {
	cfg := &Config{}
	var cfgErr error
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfgErr = cleanenv.ReadEnv(cfg)
	} else if err == nil {
		cfgErr = cleanenv.ReadConfig(configPath, cfg)
	} else {
		cfgErr = err
	}
	if cfgErr != nil {
		log.Fatalf("cannot read config: %s", cfgErr)
	}
	return cfg
}
