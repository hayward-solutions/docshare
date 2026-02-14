package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB        DBConfig
	MinIO     MinIOConfig
	JWT       JWTConfig
	Server    ServerConfig
	Gotenberg GotenbergConfig
	Audit     AuditConfig
	Preview   PreviewConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type MinIOConfig struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

type ServerConfig struct {
	Port string
}

type GotenbergConfig struct {
	URL string
}

type AuditConfig struct {
	ExportInterval time.Duration
}

type PreviewConfig struct {
	QueueBufferSize int
	MaxAttempts     int
	RetryDelays     []time.Duration
}

func Load() *Config {
	return &Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "docshare"),
			Password: getEnv("DB_PASSWORD", "docshare_secret"),
			Name:     getEnv("DB_NAME", "docshare"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		MinIO: MinIOConfig{
			Endpoint:       getEnv("MINIO_ENDPOINT", "localhost:9000"),
			PublicEndpoint: getEnv("MINIO_PUBLIC_ENDPOINT", getEnv("MINIO_ENDPOINT", "localhost:9000")),
			AccessKey:      getEnv("MINIO_ACCESS_KEY", "docshare"),
			SecretKey:      getEnv("MINIO_SECRET_KEY", "docshare_secret"),
			Bucket:         getEnv("MINIO_BUCKET", "docshare"),
			UseSSL:         getEnvAsBool("MINIO_USE_SSL", false),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "change-me-in-production"),
			ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Gotenberg: GotenbergConfig{
			URL: getEnv("GOTENBERG_URL", "http://localhost:3000"),
		},
		Audit: AuditConfig{
			ExportInterval: getEnvAsDuration("AUDIT_EXPORT_INTERVAL", 1*time.Hour),
		},
		Preview: PreviewConfig{
			QueueBufferSize: getEnvAsInt("PREVIEW_QUEUE_BUFFER_SIZE", 100),
			MaxAttempts:     getEnvAsInt("PREVIEW_JOB_MAX_ATTEMPTS", 3),
			RetryDelays:     []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute},
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
