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
	RBAC      RBACConfig      `mapstructure:"rbac"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

type TelemetryConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	ServiceName    string        `mapstructure:"service_name"`
	ServiceVersion string        `mapstructure:"service_version"`
	Environment    string        `mapstructure:"environment"`
	OTLPEndpoint   string        `mapstructure:"otlp_endpoint"`
	OTLPInsecure   bool          `mapstructure:"otlp_insecure"`
	SampleRatio    float64       `mapstructure:"sample_ratio"`
	MetricInterval time.Duration `mapstructure:"metric_interval"`
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
	ResolvePerWindow       int `mapstructure:"resolve_per_window"`
	ResolveWindowMinutes   int `mapstructure:"resolve_window_minutes"`
}

type ShortenerURLConfig struct {
	MaxLength      int      `mapstructure:"max_length"`
	AllowedSchemes []string `mapstructure:"allowed_schemes"`
	BlockedHosts   []string `mapstructure:"blocked_hosts"`
}

type ServerConfig struct {
	Port           string   `mapstructure:"port"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	SwaggerEnabled bool     `mapstructure:"swagger_enabled"`
}

type StorageConfig struct {
	Type     string         `mapstructure:"type"`     // "memory" | "sqlite" | "postgres"
	Memory   MemoryConfig   `mapstructure:"memory"`   // used for memory storage
	SQLite   SQLiteConfig   `mapstructure:"sqlite"`   // used for sqlite storage
	Postgres PostgresConfig `mapstructure:"postgres"` // used for postgres storage
}

type MemoryConfig struct{}

type SQLiteConfig struct {
	DBPath string `mapstructure:"db_path"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"` // disable | require | verify-full
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
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

type RBACConfig struct {
	// AdminUsername identifies the user Bootstrap assigns to the admin group
	// whenever the admin group is empty (fresh install, or recovery from a
	// hypothetical lockout). See docs/adr/ACCESS_MANAGEMENT_ADR.md ADR-004.
	AdminUsername string `mapstructure:"admin_username"`
	// DirectAuthIsAdmin controls the RBAC treatment of the x-direct-auth-username
	// dev bypass (config.Auth.DirectAuthEnabled): that path already forgoes
	// authentication entirely, so by default it is also treated as a superuser.
	DirectAuthIsAdmin bool `mapstructure:"direct_auth_is_admin"`
}

// RateLimitConfig holds the internal/ratelimit app-feature's boot-time
// operational knobs. Per-target limits live in the DB (ADR-001 of
// docs/adr/RATE_LIMITING_ADR.md) — this is only the master switch and the
// settings that apply platform-wide.
type RateLimitConfig struct {
	// Enabled is the master kill switch; read once at boot (ADR-008).
	Enabled bool `mapstructure:"enabled"`
	// CacheRefreshInterval is how often the in-memory rule cache is
	// refreshed from the DB as a backstop (writes also trigger an
	// immediate reload — ADR-008).
	CacheRefreshInterval time.Duration `mapstructure:"cache_refresh_interval"`
	// CleanupInterval is how often expired daily-quota counter rows are
	// pruned (see CounterRetentionDays).
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	// CounterRetentionDays is how many days of daily-quota counter rows to
	// keep before the cleanup ticker prunes them.
	CounterRetentionDays int `mapstructure:"counter_retention_days"`
	// Timezone is the timezone daily-quota calendar-day buckets are
	// computed in (ADR-006). Defaults to UTC.
	Timezone string `mapstructure:"timezone"`
	// TrustProxyHeaders controls whether client IP is resolved from
	// X-Forwarded-For/X-Real-IP (true) or r.RemoteAddr (false). Defaults to
	// false since trusting these headers without a sanitizing edge proxy is
	// spoofable (ADR-009).
	TrustProxyHeaders bool `mapstructure:"trust_proxy_headers"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set default values
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("server.allowed_origins", []string{"*"})
	viper.SetDefault("server.swagger_enabled", false)
	viper.SetDefault("storage.type", "memory")
	viper.SetDefault("storage.postgres.host", "localhost")
	viper.SetDefault("storage.postgres.port", 5432)
	viper.SetDefault("storage.postgres.sslmode", "disable")
	viper.SetDefault("log.level", "debug")
	viper.SetDefault("http.timeout", 30)
	viper.SetDefault("shortener.code_length", 7)
	viper.SetDefault("shortener.max_create_retries", 5)
	viper.SetDefault("shortener.public_create_enabled", false)
	viper.SetDefault("shortener.rate_limit.creates_per_window", 100)
	viper.SetDefault("shortener.rate_limit.window_minutes", 15)
	viper.SetDefault("shortener.rate_limit.public_creates_per_window", 20)
	viper.SetDefault("shortener.rate_limit.resolve_per_window", 300)
	viper.SetDefault("shortener.rate_limit.resolve_window_minutes", 1)
	viper.SetDefault("shortener.url.max_length", 2048)
	viper.SetDefault("shortener.url.allowed_schemes", []string{"http", "https"})
	viper.SetDefault("shortener.url.blocked_hosts", []string{})
	viper.SetDefault("rbac.admin_username", "admin")
	viper.SetDefault("rbac.direct_auth_is_admin", true)
	viper.SetDefault("ratelimit.enabled", true)
	viper.SetDefault("ratelimit.cache_refresh_interval", 30*time.Second)
	viper.SetDefault("ratelimit.cleanup_interval", time.Hour)
	viper.SetDefault("ratelimit.counter_retention_days", 3)
	viper.SetDefault("ratelimit.timezone", "UTC")
	viper.SetDefault("ratelimit.trust_proxy_headers", false)
	viper.SetDefault("telemetry.enabled", false)
	viper.SetDefault("telemetry.service_name", "all-in-one")
	viper.SetDefault("telemetry.service_version", "1.0.0")
	viper.SetDefault("telemetry.environment", "local")
	viper.SetDefault("telemetry.otlp_endpoint", "localhost:4318")
	viper.SetDefault("telemetry.otlp_insecure", true)
	viper.SetDefault("telemetry.sample_ratio", 1.0)
	viper.SetDefault("telemetry.metric_interval", 15*time.Second)

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
	viper.BindEnv("storage.postgres.host", "ALLINONE_STORAGE_POSTGRES_HOST")
	viper.BindEnv("storage.postgres.port", "ALLINONE_STORAGE_POSTGRES_PORT")
	viper.BindEnv("storage.postgres.user", "ALLINONE_STORAGE_POSTGRES_USER")
	viper.BindEnv("storage.postgres.password", "ALLINONE_STORAGE_POSTGRES_PASSWORD")
	viper.BindEnv("storage.postgres.dbname", "ALLINONE_STORAGE_POSTGRES_DBNAME")
	viper.BindEnv("storage.postgres.sslmode", "ALLINONE_STORAGE_POSTGRES_SSLMODE")
	viper.BindEnv("rbac.admin_username", "ALLINONE_RBAC_ADMIN_USERNAME")
	viper.BindEnv("rbac.direct_auth_is_admin", "ALLINONE_RBAC_DIRECT_AUTH_IS_ADMIN")
	viper.BindEnv("ratelimit.enabled", "ALLINONE_RATELIMIT_ENABLED")
	viper.BindEnv("ratelimit.cache_refresh_interval", "ALLINONE_RATELIMIT_CACHE_REFRESH_INTERVAL")
	viper.BindEnv("ratelimit.cleanup_interval", "ALLINONE_RATELIMIT_CLEANUP_INTERVAL")
	viper.BindEnv("ratelimit.counter_retention_days", "ALLINONE_RATELIMIT_COUNTER_RETENTION_DAYS")
	viper.BindEnv("ratelimit.timezone", "ALLINONE_RATELIMIT_TIMEZONE")
	viper.BindEnv("ratelimit.trust_proxy_headers", "ALLINONE_RATELIMIT_TRUST_PROXY_HEADERS")
	viper.BindEnv("telemetry.enabled", "ALLINONE_TELEMETRY_ENABLED")
	viper.BindEnv("telemetry.service_name", "ALLINONE_TELEMETRY_SERVICE_NAME")
	viper.BindEnv("telemetry.service_version", "ALLINONE_TELEMETRY_SERVICE_VERSION")
	viper.BindEnv("telemetry.environment", "ALLINONE_TELEMETRY_ENVIRONMENT")
	viper.BindEnv("telemetry.otlp_endpoint", "ALLINONE_TELEMETRY_OTLP_ENDPOINT")
	viper.BindEnv("telemetry.otlp_insecure", "ALLINONE_TELEMETRY_OTLP_INSECURE")
	viper.BindEnv("telemetry.sample_ratio", "ALLINONE_TELEMETRY_SAMPLE_RATIO")
	viper.BindEnv("telemetry.metric_interval", "ALLINONE_TELEMETRY_METRIC_INTERVAL")

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
