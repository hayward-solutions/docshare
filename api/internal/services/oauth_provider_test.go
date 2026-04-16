package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"
	"time"

	"github.com/docshare/api/internal/config"
	"golang.org/x/oauth2"
)

func TestOAuthProviderService_GenerateState(t *testing.T) {
	cfg := &config.Config{
		SSO: config.SSOConfig{
			Google: config.OAuthProviderConfig{
				ClientID:     "test-client-id",
				ClientSecret: "test-secret",
			},
		},
	}
	service := NewOAuthProviderService(cfg)

	t.Run("generates valid state", func(t *testing.T) {
		state, err := service.GenerateState("google")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state == nil {
			t.Fatal("expected non-nil state")
		}
		if state.Provider != "google" {
			t.Errorf("expected provider 'google', got %s", state.Provider)
		}
		if state.ExpiresAt.Before(time.Now()) {
			t.Error("expected expiresAt to be in the future")
		}
		if state.Nonce == "" {
			t.Error("expected non-empty nonce")
		}
	})

	t.Run("generates unique nonces", func(t *testing.T) {
		state1, _ := service.GenerateState("google")
		state2, _ := service.GenerateState("google")
		if state1.Nonce == state2.Nonce {
			t.Error("expected unique nonces")
		}
	})
}

func TestOAuthProviderService_GetOAuthConfig(t *testing.T) {
	t.Run("returns config for google", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				BackendURL: "http://localhost:8080",
			},
			SSO: config.SSOConfig{
				Google: config.OAuthProviderConfig{
					Enabled:      true,
					ClientID:     "google-client-id",
					ClientSecret: "google-secret",
				},
			},
		}
		service := NewOAuthProviderService(cfg)

		oauthCfg, providerName, err := service.GetOAuthConfig("google")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if providerName != "google" {
			t.Errorf("expected provider name 'google', got %s", providerName)
		}
		if oauthCfg.ClientID != "google-client-id" {
			t.Errorf("expected client ID 'google-client-id', got %s", oauthCfg.ClientID)
		}
	})

	t.Run("returns config for github", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				BackendURL: "http://localhost:8080",
			},
			SSO: config.SSOConfig{
				GitHub: config.OAuthProviderConfig{
					Enabled:      true,
					ClientID:     "github-client-id",
					ClientSecret: "github-secret",
				},
			},
		}
		service := NewOAuthProviderService(cfg)

		_, providerName, err := service.GetOAuthConfig("github")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if providerName != "github" {
			t.Errorf("expected provider name 'github', got %s", providerName)
		}
	})

	t.Run("returns error for unknown provider", func(t *testing.T) {
		cfg := &config.Config{}
		service := NewOAuthProviderService(cfg)

		_, _, err := service.GetOAuthConfig("unknown")
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}

func TestOAuthProviderService_ExchangeCode(t *testing.T) {
	t.Run("returns error for invalid code", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				BackendURL: "http://localhost:8080",
			},
			SSO: config.SSOConfig{
				Google: config.OAuthProviderConfig{
					ClientID:     "test-client",
					ClientSecret: "test-secret",
				},
			},
		}
		service := NewOAuthProviderService(cfg)

		_, err := service.ExchangeCode(context.Background(), "google", "invalid-code")
		if err == nil {
			t.Fatal("expected error for invalid code")
		}
	})
}

func TestOAuthProviderService_GetUserInfo(t *testing.T) {
	t.Run("returns error for unknown provider", func(t *testing.T) {
		cfg := &config.Config{}
		service := NewOAuthProviderService(cfg)

		_, err := service.GetUserInfo(context.Background(), "unknown", &oauth2.Token{})
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}

func TestOAuthProviderService_OIDCDiscovery(t *testing.T) {
	var issuer string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/protocol/openid-connect/auth",
			"token_endpoint":         issuer + "/protocol/openid-connect/token",
			"userinfo_endpoint":      issuer + "/protocol/openid-connect/userinfo",
			"jwks_uri":               issuer + "/protocol/openid-connect/certs",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer = srv.URL

	cfg := &config.Config{
		SSO: config.SSOConfig{
			OIDC: config.OIDCProviderConfig{
				Enabled:      true,
				ClientID:     "docshare",
				ClientSecret: "secret",
				RedirectURL:  "http://app.example.com/callback",
				Scopes:       "openid,email,profile",
				IssuerURL:    issuer,
			},
		},
	}
	svc := NewOAuthProviderService(cfg)

	oauthCfg, name, err := svc.GetOAuthConfig("oidc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "oidc" {
		t.Fatalf("expected provider 'oidc', got %s", name)
	}
	wantAuth := issuer + "/protocol/openid-connect/auth"
	if oauthCfg.Endpoint.AuthURL != wantAuth {
		t.Errorf("expected discovered AuthURL %q, got %q", wantAuth, oauthCfg.Endpoint.AuthURL)
	}
	wantToken := issuer + "/protocol/openid-connect/token"
	if oauthCfg.Endpoint.TokenURL != wantToken {
		t.Errorf("expected discovered TokenURL %q, got %q", wantToken, oauthCfg.Endpoint.TokenURL)
	}

	state, err := svc.GenerateState("oidc")
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	authURL, err := svc.AuthCodeURL(context.Background(), "oidc", state)
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	parsed, err := neturl.Parse(authURL)
	if err != nil {
		t.Fatalf("invalid AuthCodeURL: %v", err)
	}
	if got := parsed.Query().Get("nonce"); got != state.Nonce {
		t.Errorf("expected auth URL nonce %q, got %q (full URL: %s)", state.Nonce, got, authURL)
	}
	if got := parsed.Query().Get("state"); got == "" {
		t.Errorf("expected auth URL to carry state param, got: %s", authURL)
	}
}

func TestOAuthProviderService_OIDCDiscoveryFailure(t *testing.T) {
	cfg := &config.Config{
		SSO: config.SSOConfig{
			OIDC: config.OIDCProviderConfig{
				Enabled:   true,
				IssuerURL: "http://127.0.0.1:1/does-not-exist",
			},
		},
	}
	svc := NewOAuthProviderService(cfg)

	_, _, err := svc.GetOAuthConfig("oidc")
	if err == nil {
		t.Fatal("expected discovery to fail for unreachable issuer")
	}
}
