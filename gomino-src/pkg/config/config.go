package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:",squash"`
	Database DatabaseConfig `mapstructure:",squash"`
	JWT      JWTConfig      `mapstructure:",squash"`
	SMTP     SMTPConfig     `mapstructure:",squash"`
	LiveKit  LiveKitConfig  `mapstructure:",squash"`
	Media    MediaConfig    `mapstructure:",squash"`
	Logging  LoggingConfig  `mapstructure:",squash"`
	Firebase FirebaseConfig `mapstructure:",squash"`
}

type ServerConfig struct {
	Port       string `mapstructure:"SERVER_PORT"`
	APIHost    string `mapstructure:"API_HOST"`
	EnableDocs bool   `mapstructure:"ENABLE_DOCS"`
}

type LoggingConfig struct {
	ErrorLogFile string `mapstructure:"ERROR_LOG_FILE"`
}

type JWTConfig struct {
	Secret string `mapstructure:"JWT_SECRET"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"DATABASE_DSN"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"SMTP_HOST"`
	Port     string `mapstructure:"SMTP_PORT"`
	Username string `mapstructure:"SMTP_USERNAME"`
	Password string `mapstructure:"SMTP_PASSWORD"`
	From     string `mapstructure:"SMTP_FROM"`
}

type LiveKitConfig struct {
	APIKey    string `mapstructure:"LIVEKIT_API_KEY"`
	APISecret string `mapstructure:"LIVEKIT_API_SECRET"`
	URL       string `mapstructure:"LIVEKIT_URL"`
}

type MediaConfig struct {
	StoragePath string `mapstructure:"MEDIA_STORAGE_PATH"`
	ServerURL   string `mapstructure:"MEDIA_SERVER_URL"`
}

type FirebaseConfig struct {
	ServiceAccountPath string `mapstructure:"FIREBASE_SERVICE_ACCOUNT_PATH"`
	Enabled            bool   `mapstructure:"FIREBASE_PUSH_ENABLED"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	// Bind environment variables explicitly
	viper.BindEnv("ENABLE_DOCS")
	viper.BindEnv("ERROR_LOG_FILE")
	viper.BindEnv("FIREBASE_PUSH_ENABLED")
	viper.BindEnv("FIREBASE_SERVICE_ACCOUNT_PATH")

	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}
