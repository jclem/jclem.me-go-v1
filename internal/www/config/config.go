package config

import "os"

type AppEnv string

const (
	Development AppEnv = "development"
	Production  AppEnv = "production"
)

func Env() AppEnv {
	return AppEnv(os.Getenv("APP_ENV"))
}

func IsDev() bool {
	return Env() == Development
}

func IsProd() bool {
	return Env() == Production
}

func DatabaseURL() string {
	return os.Getenv("DATABASE_URL")
}

func APIKey() string {
	return os.Getenv("API_KEY")
}

type SpacesConfig struct {
	Secret   string
	KeyID    string
	Endpoint string
	Bucket   string
}

func Spaces() *SpacesConfig {
	return &SpacesConfig{
		KeyID:    os.Getenv("DO_SPACES_KEY_ID"),
		Secret:   os.Getenv("DO_SPACES_SECRET"),
		Endpoint: os.Getenv("DO_SPACES_ENDPOINT"),
		Bucket:   os.Getenv("DO_SPACES_BUCKET"),
	}
}

func Port() string {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	return port
}

func URLUseHTTPS() bool {
	return IsProd()
}

func URLPort() string {
	if IsProd() {
		return "80"
	}

	return Port()
}

func URLHostname() string {
	if IsProd() {
		return os.Getenv("HOSTNAME")
	}

	return "localhost:" + URLPort()
}
