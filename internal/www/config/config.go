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
	RunWorkers     bool   `mapstructure:"run_workers"`
	SpacesSecret   string `mapstructure:"do_spaces_secret"`
	SpacesKeyID    string `mapstructure:"do_spaces_key_id"`
	SpacesEndpoint string `mapstructure:"do_spaces_endpoint"`
	SpacesBucket   string `mapstructure:"do_spaces_bucket"`
}

var GlobalConfig Config //nolint:gochecknoglobals

func (c Config) IsDev() bool {
	return c.AppEnv == Development
}

func IsDev() bool {
	return GlobalConfig.IsDev()
}

func (c Config) IsProd() bool {
	return c.AppEnv == Production
}

func IsProd() bool {
	return GlobalConfig.IsProd()
}

func (c Config) URLUseHTTPS() bool {
	return c.IsProd()
}

func URLUseHTTPS() bool {
	return GlobalConfig.URLUseHTTPS()
}

func (c Config) URLPort() string {
	if c.IsProd() {
		return "80"
	}

	return c.Port
}

func URLPort() string {
	return GlobalConfig.URLPort()
}

func (c Config) URLHostname() string {
	if c.IsProd() {
		return os.Getenv("HOSTNAME")
	}

	return "localhost:" + c.URLPort()
}

func URLHostname() string {
	return GlobalConfig.URLHostname()
}

func APIKey() string {
	return GlobalConfig.APIKey
}

func DatabaseURL() string {
	return GlobalConfig.DatabaseURL
}

func Port() string {
	return GlobalConfig.Port
}

func RunWorkers() bool {
	return GlobalConfig.RunWorkers
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
	viper.SetDefault("run_workers", true)

	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var cerr viper.ConfigFileNotFoundError
		if !errors.As(err, &cerr) {
			return Config{}, fmt.Errorf("could not read config: %w", err)
		}
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		return Config{}, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return GlobalConfig, nil
}
