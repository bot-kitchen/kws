package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

var (
	once     sync.Once
	instance *Config
)

// Config holds all application configuration
type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Server      ServerConfig      `mapstructure:"server"`
	MongoDB     MongoDBConfig     `mapstructure:"mongodb"`
	Keycloak    KeycloakConfig    `mapstructure:"keycloak"`
	Certificate CertificateConfig `mapstructure:"certificate"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	CORS        CORSConfig        `mapstructure:"cors"`
}

type AppConfig struct {
	Name  string `mapstructure:"name"`
	Env   string `mapstructure:"env"`
	Debug bool   `mapstructure:"debug"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ExternalURL     string        `mapstructure:"external_url"` // Public URL for KOS to connect
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type MongoDBConfig struct {
	URI            string        `mapstructure:"uri"`
	Database       string        `mapstructure:"database"`
	MaxPoolSize    uint64        `mapstructure:"max_pool_size"`
	MinPoolSize    uint64        `mapstructure:"min_pool_size"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

type KeycloakConfig struct {
	URL                  string `mapstructure:"url"`
	AdminRealm           string `mapstructure:"admin_realm"`
	AdminClientID        string `mapstructure:"admin_client_id"`
	AdminUsername        string `mapstructure:"admin_username"`
	AdminPassword        string `mapstructure:"admin_password"`
	DefaultRealmTemplate string `mapstructure:"default_realm_template"`
}

type CertificateConfig struct {
	CADir            string `mapstructure:"ca_dir"`
	CACert           string `mapstructure:"ca_cert"` // CA certificate PEM
	CAKey            string `mapstructure:"ca_key"`  // CA private key PEM
	CertValidityDays int    `mapstructure:"cert_validity_days"`
	CAValidityDays   int    `mapstructure:"ca_validity_days"`
}

type JWTConfig struct {
	Secret           string        `mapstructure:"secret"` // Secret for KOS JWT tokens
	AccessTokenTTL   time.Duration `mapstructure:"access_token_ttl"`
	RefreshThreshold time.Duration `mapstructure:"refresh_threshold"`
	Issuer           string        `mapstructure:"issuer"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

// Initialize sets up Viper with default configuration paths and environment bindings
func Initialize() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/kws")
	viper.AddConfigPath("$HOME/.kws")

	// Environment variable support
	viper.SetEnvPrefix("KWS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, using defaults and env vars
	}

	return nil
}

func setDefaults() {
	// App defaults
	viper.SetDefault("app.name", "kws")
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.debug", true)

	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.external_url", "https://kws.example.com")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "10s")

	// MongoDB defaults
	viper.SetDefault("mongodb.uri", "mongodb://localhost:27017")
	viper.SetDefault("mongodb.database", "kws")
	viper.SetDefault("mongodb.max_pool_size", 100)
	viper.SetDefault("mongodb.min_pool_size", 10)
	viper.SetDefault("mongodb.connect_timeout", "10s")

	// Keycloak defaults
	viper.SetDefault("keycloak.url", "http://localhost:8180")
	viper.SetDefault("keycloak.admin_realm", "master")
	viper.SetDefault("keycloak.admin_client_id", "admin-cli")

	// Certificate defaults
	viper.SetDefault("certificate.ca_dir", "/etc/kws/ca")
	viper.SetDefault("certificate.cert_validity_days", 365)
	viper.SetDefault("certificate.ca_validity_days", 3650)

	// JWT defaults
	viper.SetDefault("jwt.secret", "change-this-secret-in-production")
	viper.SetDefault("jwt.access_token_ttl", "15m")
	viper.SetDefault("jwt.refresh_threshold", "2m")
	viper.SetDefault("jwt.issuer", "kws")

	// Logging defaults
	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "console")
	viper.SetDefault("logging.output", "stdout")

	// CORS defaults
	viper.SetDefault("cors.allowed_origins", []string{"*"})
	viper.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allowed_headers", []string{"Authorization", "Content-Type", "X-Tenant-ID"})
}

// Load returns the singleton config instance
func Load() (*Config, error) {
	var err error
	once.Do(func() {
		if err = Initialize(); err != nil {
			return
		}
		instance = &Config{}
		if err = viper.Unmarshal(instance); err != nil {
			err = fmt.Errorf("failed to unmarshal config: %w", err)
			return
		}
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

// GetAddress returns the server address string
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}
