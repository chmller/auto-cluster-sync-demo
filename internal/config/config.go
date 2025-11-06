package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Node     NodeConfig    `yaml:"node"`
	Cluster  ClusterConfig `yaml:"cluster"`
	LogLevel string        `yaml:"log_level,omitempty"` // debug, info, warn, error
}

// NodeConfig contains node-specific configuration
type NodeConfig struct {
	Name     string      `yaml:"name"`
	Serf     SerfConfig  `yaml:"serf"`
	HTTP     HTTPConfig  `yaml:"http"`
	Database DBConfig    `yaml:"database"`
}

// SerfConfig contains Serf-specific configuration
type SerfConfig struct {
	BindAddr      string `yaml:"bind_addr"`
	AdvertiseAddr string `yaml:"advertise_addr,omitempty"`
}

// HTTPConfig contains HTTP server configuration
type HTTPConfig struct {
	Port int `yaml:"port"`
}

// DBConfig contains database configuration
type DBConfig struct {
	Path string `yaml:"path"`
}

// ClusterConfig contains cluster configuration
type ClusterConfig struct {
	Seeds       []string `yaml:"seeds"`
	EncryptKey  string   `yaml:"encrypt_key,omitempty"`
	JoinTimeout int      `yaml:"join_timeout,omitempty"` // seconds
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Node.HTTP.Port == 0 {
		config.Node.HTTP.Port = 8080
	}
	if config.Node.Database.Path == "" {
		config.Node.Database.Path = "./todos.db"
	}
	if config.Cluster.JoinTimeout == 0 {
		config.Cluster.JoinTimeout = 10
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	return &config, nil
}

// ParseLogLevel converts a log level string to slog.Level
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
