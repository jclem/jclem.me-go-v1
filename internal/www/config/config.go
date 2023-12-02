package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type AppEnv string

const (
	Development AppEnv = "development"
	Production  AppEnv = "production"
)

type Config struct {
	Port           string `mapstructure:"port"`
	AppEnv         AppEnv `mapstructure:"app_env"`
	DatabaseURL    string `mapstructure:"database_url"`
	APIKey         string `mapstructure:"api_key"`
	SpacesSecret   string `mapstructure:"do_spaces_secret"`
	SpacesKeyID    string `mapstructure:"do_spaces_key_id"`
	SpacesEndpoint string `mapstructure:"do_spaces_endpoint"`
	SpacesBucket   string `mapstructure:"do_spaces_bucket"`
}

func (c Config) IsDev() bool {
	return c.AppEnv == Development
}

func (c Config) IsProd() bool {
	return c.AppEnv == Production
}

func (c Config) URLUseHTTPS() bool {
	return c.IsProd()
}

func (c Config) URLPort() string {
	if c.IsProd() {
		return "80"
	}

	return c.Port
}

func (c Config) URLHostname() string {
	if c.IsProd() {
		return os.Getenv("HOSTNAME")
	}

	return "localhost:" + c.URLPort()
}

// LoadConfig loads the configuration from flags and configuration files into
// the given context.
func LoadConfig() (Config, error) {
	viper.SetDefault("port", "8080")
	viper.SetDefault("app_env", Development)
	viper.SetDefault("database_url", "")
	viper.SetDefault("api_key", "")
	viper.SetDefault("do_spaces_secret", "")
	viper.SetDefault("do_spaces_key_id", "")
	viper.SetDefault("do_spaces_endpoint", "")
	viper.SetDefault("do_spaces_bucket", "")

	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var cerr viper.ConfigFileNotFoundError
		if !errors.As(err, &cerr) {
			return Config{}, fmt.Errorf("could not read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return cfg, nil
}
