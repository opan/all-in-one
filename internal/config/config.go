package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Storage StorageConfig `mapstructure:"storage"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type StorageConfig struct {
	Type string `mapstructure:"type"` // "memory" or "sqlite"
	Path string `mapstructure:"path"` // used for sqlite storage
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set default values
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("storage.type", "memory")
	viper.SetDefault("storage.path", "./data/listings.db")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ALLINONE")

	// Allow command-line flags to override config
	viper.BindEnv("storage.type", "ALLINONE_STORAGE_TYPE")
	viper.BindEnv("storage.path", "ALLINONE_STORAGE_PATH")
	viper.BindEnv("server.port", "ALLINONE_SERVER_PORT")

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
