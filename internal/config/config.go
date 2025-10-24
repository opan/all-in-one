package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Storage StorageConfig `mapstructure:"storage"`
	Logging LoggingConfig `mapstructure:"log"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
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

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config/listing")

	// Set default values
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("storage.type", "memory")
	viper.SetDefault("log.level", "debug")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ALLINONE_LISTING")

	// Try to read config file (it's okay if it doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No config file found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
