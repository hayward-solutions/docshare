package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if val, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() { os.Setenv(key, val) })
	}
	os.Unsetenv(key)
}

func TestLoad(t *testing.T) {
	t.Run("returns config with defaults when no env vars set", func(t *testing.T) {
		cfg := Load()
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.DB.Host != "localhost" {
			t.Errorf("expected DB.Host 'localhost', got %s", cfg.DB.Host)
		}
		if cfg.DB.Port != "5432" {
			t.Errorf("expected DB.Port '5432', got %s", cfg.DB.Port)
		}
		if cfg.Server.Port != "8080" {
			t.Errorf("expected Server.Port '8080', got %s", cfg.Server.Port)
		}
		if cfg.JWT.ExpirationHours != 24 {
			t.Errorf("expected JWT.ExpirationHours 24, got %d", cfg.JWT.ExpirationHours)
		}
		if cfg.Preview.MaxAttempts != 3 {
			t.Errorf("expected Preview.MaxAttempts 3, got %d", cfg.Preview.MaxAttempts)
		}
		if cfg.Audit.ExportInterval != 1*time.Hour {
			t.Errorf("expected Audit.ExportInterval 1h, got %v", cfg.Audit.ExportInterval)
		}
	})

	t.Run("reads environment variables", func(t *testing.T) {
		t.Setenv("DB_HOST", "custom-host")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_USER", "custom-user")
		t.Setenv("DB_PASSWORD", "custom-pass")
		t.Setenv("DB_NAME", "custom-db")
		t.Setenv("DB_SSLMODE", "require")
		t.Setenv("SERVER_PORT", "9090")
		t.Setenv("JWT_SECRET", "my-secret")
		t.Setenv("JWT_EXPIRATION_HOURS", "48")
		t.Setenv("FRONTEND_URL", "https://app.example.com")
		t.Setenv("BACKEND_URL", "https://api.example.com/api")

		cfg := Load()

		if cfg.DB.Host != "custom-host" {
			t.Errorf("expected DB.Host 'custom-host', got %s", cfg.DB.Host)
		}
		if cfg.DB.Port != "5433" {
			t.Errorf("expected DB.Port '5433', got %s", cfg.DB.Port)
		}
		if cfg.DB.User != "custom-user" {
			t.Errorf("expected DB.User 'custom-user', got %s", cfg.DB.User)
		}
		if cfg.DB.Password != "custom-pass" {
			t.Errorf("expected DB.Password 'custom-pass', got %s", cfg.DB.Password)
		}
		if cfg.DB.Name != "custom-db" {
			t.Errorf("expected DB.Name 'custom-db', got %s", cfg.DB.Name)
		}
		if cfg.DB.SSLMode != "require" {
			t.Errorf("expected DB.SSLMode 'require', got %s", cfg.DB.SSLMode)
		}
		if cfg.Server.Port != "9090" {
			t.Errorf("expected Server.Port '9090', got %s", cfg.Server.Port)
		}
		if cfg.JWT.Secret != "my-secret" {
			t.Errorf("expected JWT.Secret 'my-secret', got %s", cfg.JWT.Secret)
		}
		if cfg.JWT.ExpirationHours != 48 {
			t.Errorf("expected JWT.ExpirationHours 48, got %d", cfg.JWT.ExpirationHours)
		}
		if cfg.Server.FrontendURL != "https://app.example.com" {
			t.Errorf("expected Server.FrontendURL 'https://app.example.com', got %s", cfg.Server.FrontendURL)
		}
	})

	t.Run("S3 endpoint defaults to AWS format when not set", func(t *testing.T) {
		t.Setenv("S3_REGION", "eu-west-1")
		unsetEnv(t, "S3_ENDPOINT")

		cfg := Load()

		expected := "s3.eu-west-1.amazonaws.com"
		if cfg.S3.Endpoint != expected {
			t.Errorf("expected S3.Endpoint %q, got %q", expected, cfg.S3.Endpoint)
		}
	})

	t.Run("S3 uses custom endpoint when set", func(t *testing.T) {
		t.Setenv("S3_ENDPOINT", "minio.local:9000")

		cfg := Load()

		if cfg.S3.Endpoint != "minio.local:9000" {
			t.Errorf("expected S3.Endpoint 'minio.local:9000', got %s", cfg.S3.Endpoint)
		}
	})

	t.Run("SSO Google redirect URL auto-generates from backend URL", func(t *testing.T) {
		t.Setenv("OAUTH_GOOGLE_ENABLED", "true")
		t.Setenv("OAUTH_GOOGLE_CLIENT_ID", "client-id")
		t.Setenv("OAUTH_GOOGLE_CLIENT_SECRET", "client-secret")
		t.Setenv("BACKEND_URL", "https://api.example.com/api")
		unsetEnv(t, "OAUTH_GOOGLE_REDIRECT_URL")

		cfg := Load()

		expected := "https://api.example.com/api/auth/sso/oauth/google/callback"
		if cfg.SSO.Google.RedirectURL != expected {
			t.Errorf("expected Google RedirectURL %q, got %q", expected, cfg.SSO.Google.RedirectURL)
		}
	})

	t.Run("SSO GitHub redirect URL auto-generates from backend URL", func(t *testing.T) {
		t.Setenv("OAUTH_GITHUB_ENABLED", "true")
		t.Setenv("OAUTH_GITHUB_CLIENT_ID", "client-id")
		t.Setenv("OAUTH_GITHUB_CLIENT_SECRET", "client-secret")
		t.Setenv("BACKEND_URL", "https://api.example.com/api")
		unsetEnv(t, "OAUTH_GITHUB_REDIRECT_URL")

		cfg := Load()

		expected := "https://api.example.com/api/auth/sso/oauth/github/callback"
		if cfg.SSO.GitHub.RedirectURL != expected {
			t.Errorf("expected GitHub RedirectURL %q, got %q", expected, cfg.SSO.GitHub.RedirectURL)
		}
	})

	t.Run("SSO OIDC redirect URL auto-generates from backend URL", func(t *testing.T) {
		t.Setenv("OAUTH_OIDC_ENABLED", "true")
		t.Setenv("OAUTH_OIDC_CLIENT_ID", "client-id")
		t.Setenv("OAUTH_OIDC_CLIENT_SECRET", "client-secret")
		t.Setenv("BACKEND_URL", "https://api.example.com/api")
		unsetEnv(t, "OAUTH_OIDC_REDIRECT_URL")

		cfg := Load()

		expected := "https://api.example.com/api/auth/sso/oauth/oidc/callback"
		if cfg.SSO.OIDC.RedirectURL != expected {
			t.Errorf("expected OIDC RedirectURL %q, got %q", expected, cfg.SSO.OIDC.RedirectURL)
		}
	})

	t.Run("SAML ACS URL auto-generates from backend URL", func(t *testing.T) {
		t.Setenv("SAML_ENABLED", "true")
		t.Setenv("BACKEND_URL", "https://api.example.com/api")
		unsetEnv(t, "SAML_SP_ACS_URL")

		cfg := Load()

		expected := "https://api.example.com/api/auth/sso/saml/acs"
		if cfg.SAML.SPACSURL != expected {
			t.Errorf("expected SAML.SPACSURL %q, got %q", expected, cfg.SAML.SPACSURL)
		}
	})

	t.Run("preview config reads from env", func(t *testing.T) {
		t.Setenv("PREVIEW_QUEUE_BUFFER_SIZE", "50")
		t.Setenv("PREVIEW_JOB_MAX_ATTEMPTS", "5")

		cfg := Load()

		if cfg.Preview.QueueBufferSize != 50 {
			t.Errorf("expected Preview.QueueBufferSize 50, got %d", cfg.Preview.QueueBufferSize)
		}
		if cfg.Preview.MaxAttempts != 5 {
			t.Errorf("expected Preview.MaxAttempts 5, got %d", cfg.Preview.MaxAttempts)
		}
	})

	t.Run("S3 UseSSL defaults to true", func(t *testing.T) {
		unsetEnv(t, "S3_USE_SSL")
		cfg := Load()
		if !cfg.S3.UseSSL {
			t.Error("expected S3.UseSSL to default to true")
		}
	})

	t.Run("S3 UseSSL can be disabled", func(t *testing.T) {
		t.Setenv("S3_USE_SSL", "false")
		cfg := Load()
		if cfg.S3.UseSSL {
			t.Error("expected S3.UseSSL to be false")
		}
	})

	t.Run("audit export interval reads from env", func(t *testing.T) {
		t.Setenv("AUDIT_EXPORT_INTERVAL", "30m")
		cfg := Load()
		if cfg.Audit.ExportInterval != 30*time.Minute {
			t.Errorf("expected Audit.ExportInterval 30m, got %v", cfg.Audit.ExportInterval)
		}
	})

	t.Run("LDAP config defaults", func(t *testing.T) {
		unsetEnv(t, "LDAP_ENABLED")
		cfg := Load()
		if cfg.LDAP.Enabled {
			t.Error("expected LDAP.Enabled to default to false")
		}
		if cfg.LDAP.UserFilter != "(uid=%s)" {
			t.Errorf("expected default LDAP.UserFilter '(uid=%%s)', got %s", cfg.LDAP.UserFilter)
		}
		if cfg.LDAP.EmailField != "mail" {
			t.Errorf("expected default LDAP.EmailField 'mail', got %s", cfg.LDAP.EmailField)
		}
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_GET_ENV", "value123")
		if got := getEnv("TEST_GET_ENV", "fallback"); got != "value123" {
			t.Errorf("expected 'value123', got %s", got)
		}
	})

	t.Run("returns fallback when not set", func(t *testing.T) {
		unsetEnv(t, "TEST_GET_ENV_MISSING")
		if got := getEnv("TEST_GET_ENV_MISSING", "fallback"); got != "fallback" {
			t.Errorf("expected 'fallback', got %s", got)
		}
	})
}

func TestGetEnvAsInt(t *testing.T) {
	t.Run("returns parsed int", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		if got := getEnvAsInt("TEST_INT", 0); got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("returns fallback for invalid int", func(t *testing.T) {
		t.Setenv("TEST_INT_BAD", "not-a-number")
		if got := getEnvAsInt("TEST_INT_BAD", 10); got != 10 {
			t.Errorf("expected 10, got %d", got)
		}
	})

	t.Run("returns fallback when not set", func(t *testing.T) {
		unsetEnv(t, "TEST_INT_MISSING")
		if got := getEnvAsInt("TEST_INT_MISSING", 99); got != 99 {
			t.Errorf("expected 99, got %d", got)
		}
	})
}

func TestGetEnvAsBool(t *testing.T) {
	t.Run("returns parsed bool", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "true")
		if got := getEnvAsBool("TEST_BOOL", false); !got {
			t.Error("expected true")
		}
	})

	t.Run("returns fallback for invalid bool", func(t *testing.T) {
		t.Setenv("TEST_BOOL_BAD", "maybe")
		if got := getEnvAsBool("TEST_BOOL_BAD", true); !got {
			t.Error("expected true (fallback)")
		}
	})

	t.Run("returns fallback when not set", func(t *testing.T) {
		unsetEnv(t, "TEST_BOOL_MISSING")
		if got := getEnvAsBool("TEST_BOOL_MISSING", false); got {
			t.Error("expected false (fallback)")
		}
	})
}

func TestGetEnvAsDuration(t *testing.T) {
	t.Run("returns parsed duration", func(t *testing.T) {
		t.Setenv("TEST_DUR", "5m")
		if got := getEnvAsDuration("TEST_DUR", time.Hour); got != 5*time.Minute {
			t.Errorf("expected 5m, got %v", got)
		}
	})

	t.Run("returns fallback for invalid duration", func(t *testing.T) {
		t.Setenv("TEST_DUR_BAD", "invalid")
		if got := getEnvAsDuration("TEST_DUR_BAD", time.Hour); got != time.Hour {
			t.Errorf("expected 1h (fallback), got %v", got)
		}
	})

	t.Run("returns fallback when not set", func(t *testing.T) {
		unsetEnv(t, "TEST_DUR_MISSING")
		if got := getEnvAsDuration("TEST_DUR_MISSING", 2*time.Hour); got != 2*time.Hour {
			t.Errorf("expected 2h (fallback), got %v", got)
		}
	})
}

func TestOAuthProviderConfig_ClientConfig(t *testing.T) {
	cfg := OAuthProviderConfig{
		ClientID:     "my-client-id",
		ClientSecret: "my-client-secret",
		RedirectURL:  "https://example.com/callback",
		Scopes:       "openid,email,profile",
	}

	oauthConfig := cfg.ClientConfig(context.TODO())
	if oauthConfig.ClientID != "my-client-id" {
		t.Errorf("expected ClientID 'my-client-id', got %s", oauthConfig.ClientID)
	}
	if oauthConfig.ClientSecret != "my-client-secret" {
		t.Errorf("expected ClientSecret 'my-client-secret', got %s", oauthConfig.ClientSecret)
	}
	if oauthConfig.RedirectURL != "https://example.com/callback" {
		t.Errorf("expected RedirectURL 'https://example.com/callback', got %s", oauthConfig.RedirectURL)
	}
	if len(oauthConfig.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(oauthConfig.Scopes))
	}
}

func TestOIDCProviderConfig_ClientConfig(t *testing.T) {
	cfg := OIDCProviderConfig{
		ClientID:     "oidc-client-id",
		ClientSecret: "oidc-client-secret",
		RedirectURL:  "https://example.com/oidc/callback",
		Scopes:       "openid,profile",
		IssuerURL:    "https://idp.example.com",
	}

	oauthConfig := cfg.ClientConfig(context.TODO())
	if oauthConfig.ClientID != "oidc-client-id" {
		t.Errorf("expected ClientID 'oidc-client-id', got %s", oauthConfig.ClientID)
	}
	if oauthConfig.Endpoint.AuthURL != "https://idp.example.com/authorize" {
		t.Errorf("expected AuthURL 'https://idp.example.com/authorize', got %s", oauthConfig.Endpoint.AuthURL)
	}
	if oauthConfig.Endpoint.TokenURL != "https://idp.example.com/token" {
		t.Errorf("expected TokenURL 'https://idp.example.com/token', got %s", oauthConfig.Endpoint.TokenURL)
	}
	if len(oauthConfig.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(oauthConfig.Scopes))
	}
}
