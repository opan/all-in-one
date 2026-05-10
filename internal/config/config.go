package config

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Logging   LoggingConfig   `mapstructure:"log"`
	Http      HTTPConfig      `mapstructure:"http"`
	Auth      Auth            `mapstructure:"auth"`
	Shortener ShortenerConfig `mapstructure:"shortener"`
}

type ShortenerConfig struct {
	CodeLength          int                 `mapstructure:"code_length"`
	MaxCreateRetries    int                 `mapstructure:"max_create_retries"`
	PublicCreateEnabled bool                `mapstructure:"public_create_enabled"`
	RateLimit           ShortenerRateLimit  `mapstructure:"rate_limit"`
	URL                 ShortenerURLConfig  `mapstructure:"url"`
}

type ShortenerRateLimit struct {
	CreatesPerWindow       int `mapstructure:"creates_per_window"`
	WindowMinutes          int `mapstructure:"window_minutes"`
	PublicCreatesPerWindow int `mapstructure:"public_creates_per_window"`
}

type ShortenerURLConfig struct {
	MaxLength      int      `mapstructure:"max_length"`
	AllowedSchemes []string `mapstructure:"allowed_schemes"`
	BlockedHosts   []string `mapstructure:"blocked_hosts"`
}

type ServerConfig struct {
	Port           string   `mapstructure:"port"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type StorageConfig struct {
	Type   string       `mapstructure:"type"`   // "memory" or "sqlite"
	Memory MemoryConfig `mapstructure:"memory"` // used for memory storage
	SQLite SQLiteConfig `mapstructure:"sqlite"` // used for sqlite storage
}

type MemoryConfig struct{}

type SQLiteConfig struct {
	DBPath string `mapstructure:"db_path"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"` // e.g., "info", "debug"
}

type HTTPConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type Auth struct {
	JWTSecret         string `mapstructure:"jwt_secret"`
	DirectAuthEnabled bool   `mapstructure:"direct_auth_enabled"`
	SecureCookie      bool   `mapstructure:"secure_cookie"`
	TOTPEncryptionKey string `mapstructure:"totp_encryption_key"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set default values
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("server.allowed_origins", []string{"*"})
	viper.SetDefault("storage.type", "memory")
	viper.SetDefault("log.level", "debug")
	viper.SetDefault("http.timeout", 30)
	viper.SetDefault("shortener.code_length", 7)
	viper.SetDefault("shortener.max_create_retries", 5)
	viper.SetDefault("shortener.public_create_enabled", false)
	viper.SetDefault("shortener.rate_limit.creates_per_window", 100)
	viper.SetDefault("shortener.rate_limit.window_minutes", 15)
	viper.SetDefault("shortener.rate_limit.public_creates_per_window", 20)
	viper.SetDefault("shortener.url.max_length", 2048)
	viper.SetDefault("shortener.url.allowed_schemes", []string{"http", "https"})
	viper.SetDefault("shortener.url.blocked_hosts", []string{})

	// Enable environment variable support
	// Viper maps nested keys to env vars by replacing dots with underscores and
	// uppercasing, then prepends the prefix. e.g. auth.jwt_secret → ALLINONE_AUTH_JWT_SECRET.
	viper.SetEnvPrefix("ALLINONE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Explicitly bind env vars for keys that contain underscores — viper's AutomaticEnv
	// cannot reliably reverse-map env vars to nested config keys when both dots and
	// underscores are present (ambiguous after the dot→underscore replacement).
	viper.BindEnv("auth.jwt_secret", "ALLINONE_AUTH_JWT_SECRET")
	viper.BindEnv("auth.totp_encryption_key", "ALLINONE_AUTH_TOTP_ENCRYPTION_KEY")
	viper.BindEnv("auth.direct_auth_enabled", "ALLINONE_AUTH_DIRECT_AUTH_ENABLED")
	viper.BindEnv("auth.secure_cookie", "ALLINONE_AUTH_SECURE_COOKIE")
	viper.BindEnv("storage.sqlite.db_path", "ALLINONE_STORAGE_SQLITE_DB_PATH")

	// Try to read config file (it's okay if it doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No config file found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// make sure jwt_secret is set
	if !viper.IsSet("auth.jwt_secret") {
		return nil, fmt.Errorf("missing required configuration: auth.jwt_secret")
	}

	if !viper.IsSet("auth.totp_encryption_key") {
		return nil, fmt.Errorf("missing required configuration: auth.totp_encryption_key (32-byte hex-encoded key for 2FA secret encryption)")
	}

	totpKey := viper.GetString("auth.totp_encryption_key")
	keyBytes, err := hex.DecodeString(totpKey)
	if err != nil || len(keyBytes) != 32 {
		return nil, fmt.Errorf("auth.totp_encryption_key must be a valid 64-character hex string (32 bytes)")
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
