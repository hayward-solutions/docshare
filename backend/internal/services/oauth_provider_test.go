package services

import (
	"context"
	"testing"
	"time"

	"github.com/docshare/backend/internal/config"
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
