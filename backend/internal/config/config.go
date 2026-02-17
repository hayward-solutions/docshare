package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type Config struct {
	DB        DBConfig
	S3        S3Config
	JWT       JWTConfig
	Server    ServerConfig
	Gotenberg GotenbergConfig
	Audit     AuditConfig
	Preview   PreviewConfig
	SSO       SSOConfig
	SAML      SAMLConfig
	LDAP      LDAPConfig
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type S3Config struct {
	Endpoint       string
	PublicEndpoint string
	Region         string
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
	Port        string
	FrontendURL string
	BackendURL  string
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

type SSOConfig struct {
	AutoRegister bool
	DefaultRole  string
	Google       OAuthProviderConfig
	GitHub       OAuthProviderConfig
	OIDC         OIDCProviderConfig
}

type OAuthProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       string
}

func (c OAuthProviderConfig) ClientConfig(ctx context.Context) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       strings.Split(c.Scopes, ","),
	}
}

type OIDCProviderConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       string
	IssuerURL    string
}

func (c OIDCProviderConfig) ClientConfig(ctx context.Context) *oauth2.Config {
	endpoint := oauth2.Endpoint{
		AuthURL:  c.IssuerURL + "/authorize",
		TokenURL: c.IssuerURL + "/token",
	}
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       strings.Split(c.Scopes, ","),
		Endpoint:     endpoint,
	}
}

type SAMLConfig struct {
	Enabled        bool
	IDPMetadataURL string
	SPEntityID     string
	SPACSURL       string
	SPKeyPath      string
	SPCertPath     string
}

type LDAPConfig struct {
	Enabled      bool
	URL          string
	BindDN       string
	BindPassword string
	SearchBase   string
	UserFilter   string
	EmailField   string
	NameFields   string
}

func Load() *Config {
	cfg := &Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "docshare"),
			Password: getEnv("DB_PASSWORD", "docshare_secret"),
			Name:     getEnv("DB_NAME", "docshare"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		S3: S3Config{
			Region:         getEnv("S3_REGION", "us-east-1"),
			Endpoint:       getEnv("S3_ENDPOINT", ""),
			PublicEndpoint: getEnv("S3_PUBLIC_ENDPOINT", ""),
			AccessKey:      getEnv("S3_ACCESS_KEY", ""),
			SecretKey:      getEnv("S3_SECRET_KEY", ""),
			Bucket:         getEnv("S3_BUCKET", "docshare"),
			UseSSL:         getEnvAsBool("S3_USE_SSL", true),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "change-me-in-production"),
			ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		},
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3001"),
			BackendURL:  getEnv("BACKEND_URL", "http://localhost:8080/api"),
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
		SSO: SSOConfig{
			AutoRegister: getEnvAsBool("SSO_AUTO_REGISTER", true),
			DefaultRole:  getEnv("SSO_DEFAULT_ROLE", "user"),
			Google: OAuthProviderConfig{
				Enabled:      getEnvAsBool("OAUTH_GOOGLE_ENABLED", false),
				ClientID:     getEnv("OAUTH_GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("OAUTH_GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("OAUTH_GOOGLE_REDIRECT_URL", ""),
				Scopes:       getEnv("OAUTH_GOOGLE_SCOPES", "openid,email,profile"),
			},
			GitHub: OAuthProviderConfig{
				Enabled:      getEnvAsBool("OAUTH_GITHUB_ENABLED", false),
				ClientID:     getEnv("OAUTH_GITHUB_CLIENT_ID", ""),
				ClientSecret: getEnv("OAUTH_GITHUB_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("OAUTH_GITHUB_REDIRECT_URL", ""),
				Scopes:       getEnv("OAUTH_GITHUB_SCOPES", "read:user,user:email"),
			},
			OIDC: OIDCProviderConfig{
				Enabled:      getEnvAsBool("OAUTH_OIDC_ENABLED", false),
				ClientID:     getEnv("OAUTH_OIDC_CLIENT_ID", ""),
				ClientSecret: getEnv("OAUTH_OIDC_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("OAUTH_OIDC_REDIRECT_URL", ""),
				Scopes:       getEnv("OAUTH_OIDC_SCOPES", "openid,profile,email"),
				IssuerURL:    getEnv("OAUTH_OIDC_ISSUER_URL", ""),
			},
		},
		SAML: SAMLConfig{
			Enabled:        getEnvAsBool("SAML_ENABLED", false),
			IDPMetadataURL: getEnv("SAML_IDP_METADATA_URL", ""),
			SPEntityID:     getEnv("SAML_SP_ENTITY_ID", ""),
			SPACSURL:       getEnv("SAML_SP_ACS_URL", ""),
			SPKeyPath:      getEnv("SAML_SP_KEY_PATH", ""),
			SPCertPath:     getEnv("SAML_SP_CERT_PATH", ""),
		},
		LDAP: LDAPConfig{
			Enabled:      getEnvAsBool("LDAP_ENABLED", false),
			URL:          getEnv("LDAP_URL", "ldap://localhost:389"),
			BindDN:       getEnv("LDAP_BIND_DN", ""),
			BindPassword: getEnv("LDAP_BIND_PASSWORD", ""),
			SearchBase:   getEnv("LDAP_SEARCH_BASE", ""),
			UserFilter:   getEnv("LDAP_USER_FILTER", "(uid=%s)"),
			EmailField:   getEnv("LDAP_EMAIL_FIELD", "mail"),
			NameFields:   getEnv("LDAP_NAME_FIELDS", "givenName,sn"),
		},
	}

	if cfg.S3.Endpoint == "" {
		cfg.S3.Endpoint = fmt.Sprintf("s3.%s.amazonaws.com", cfg.S3.Region)
	}

	backendURL := strings.TrimRight(cfg.Server.BackendURL, "/")
	if cfg.SSO.Google.Enabled && cfg.SSO.Google.RedirectURL == "" {
		cfg.SSO.Google.RedirectURL = backendURL + "/auth/sso/oauth/google/callback"
	}
	if cfg.SSO.GitHub.Enabled && cfg.SSO.GitHub.RedirectURL == "" {
		cfg.SSO.GitHub.RedirectURL = backendURL + "/auth/sso/oauth/github/callback"
	}
	if cfg.SSO.OIDC.Enabled && cfg.SSO.OIDC.RedirectURL == "" {
		cfg.SSO.OIDC.RedirectURL = backendURL + "/auth/sso/oauth/oidc/callback"
	}
	if cfg.SAML.Enabled && cfg.SAML.SPACSURL == "" {
		cfg.SAML.SPACSURL = backendURL + "/auth/sso/saml/acs"
	}

	return cfg
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
