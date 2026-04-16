package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"golang.org/x/oauth2"
	github "golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuthProviderService struct {
	Cfg *config.Config

	oidcMu       sync.Mutex
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
}

func NewOAuthProviderService(cfg *config.Config) *OAuthProviderService {
	return &OAuthProviderService{Cfg: cfg}
}

type OAuthState struct {
	Provider    string
	Nonce       string
	ExpiresAt   time.Time
	RedirectURL string
}

func (s *OAuthProviderService) GetOAuthConfig(provider string) (*oauth2.Config, string, error) {
	switch strings.ToLower(provider) {
	case "google":
		if !s.Cfg.SSO.Google.Enabled {
			return nil, "", errors.New("google oauth is not enabled")
		}
		return &oauth2.Config{
			ClientID:     s.Cfg.SSO.Google.ClientID,
			ClientSecret: s.Cfg.SSO.Google.ClientSecret,
			RedirectURL:  s.Cfg.SSO.Google.RedirectURL,
			Scopes:       strings.Split(s.Cfg.SSO.Google.Scopes, ","),
			Endpoint:     google.Endpoint,
		}, "google", nil

	case "github":
		if !s.Cfg.SSO.GitHub.Enabled {
			return nil, "", errors.New("github oauth is not enabled")
		}
		return &oauth2.Config{
			ClientID:     s.Cfg.SSO.GitHub.ClientID,
			ClientSecret: s.Cfg.SSO.GitHub.ClientSecret,
			RedirectURL:  s.Cfg.SSO.GitHub.RedirectURL,
			Scopes:       strings.Split(s.Cfg.SSO.GitHub.Scopes, ","),
			Endpoint:     github.Endpoint,
		}, "github", nil

	case "oidc":
		if !s.Cfg.SSO.OIDC.Enabled {
			return nil, "", errors.New("oidc is not enabled")
		}
		provider, err := s.getOIDCProvider(context.Background())
		if err != nil {
			return nil, "", err
		}
		scopes := splitScopes(s.Cfg.SSO.OIDC.Scopes)
		if !containsScope(scopes, oidc.ScopeOpenID) {
			scopes = append([]string{oidc.ScopeOpenID}, scopes...)
		}
		return &oauth2.Config{
			ClientID:     s.Cfg.SSO.OIDC.ClientID,
			ClientSecret: s.Cfg.SSO.OIDC.ClientSecret,
			RedirectURL:  s.Cfg.SSO.OIDC.RedirectURL,
			Scopes:       scopes,
			Endpoint:     provider.Endpoint(),
		}, "oidc", nil

	default:
		return nil, "", errors.New("unknown oauth provider: " + provider)
	}
}

func splitScopes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func containsScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
}

func (s *OAuthProviderService) getOIDCProvider(ctx context.Context) (*oidc.Provider, error) {
	s.oidcMu.Lock()
	defer s.oidcMu.Unlock()

	if s.oidcProvider != nil {
		return s.oidcProvider, nil
	}

	issuer := strings.TrimRight(s.Cfg.SSO.OIDC.IssuerURL, "/")
	if issuer == "" {
		return nil, errors.New("oidc issuer URL is required")
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.Cfg.SSO.OIDC.SkipIssuerVerification {
		discoveryCtx = oidc.InsecureIssuerURLContext(discoveryCtx, issuer)
	}

	provider, err := oidc.NewProvider(discoveryCtx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed for %s: %w", issuer, err)
	}

	s.oidcProvider = provider
	s.oidcVerifier = provider.Verifier(&oidc.Config{
		ClientID:        s.Cfg.SSO.OIDC.ClientID,
		SkipIssuerCheck: s.Cfg.SSO.OIDC.SkipIssuerVerification,
	})
	return provider, nil
}

func (s *OAuthProviderService) getOIDCVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	if _, err := s.getOIDCProvider(ctx); err != nil {
		return nil, err
	}
	return s.oidcVerifier, nil
}

func (s *OAuthProviderService) GenerateState(provider string) (*OAuthState, error) {
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	nonce := base64.URLEncoding.EncodeToString(nonceBytes)

	state := &OAuthState{
		Provider:  provider,
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	return state, nil
}

func (s *OAuthProviderService) AuthCodeURL(ctx context.Context, provider string, state *OAuthState) (string, error) {
	oauthCfg, providerName, err := s.GetOAuthConfig(provider)
	if err != nil {
		return "", err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	stateEncoded := base64.URLEncoding.EncodeToString(stateJSON)

	opts := []oauth2.AuthCodeOption{}
	if providerName == "oidc" {
		opts = append(opts, oidc.Nonce(state.Nonce))
	}

	return oauthCfg.AuthCodeURL(stateEncoded, opts...), nil
}

func (s *OAuthProviderService) ExchangeCode(ctx context.Context, provider string, code string) (*oauth2.Token, error) {
	oauthCfg, _, err := s.GetOAuthConfig(provider)
	if err != nil {
		return nil, err
	}

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		logger.Warn("oauth_exchange_failed", map[string]interface{}{
			"provider": provider,
			"error":    err.Error(),
		})
		return nil, errors.New("failed to exchange code for token")
	}

	return token, nil
}

func (s *OAuthProviderService) GetUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*SSOProfile, error) {
	return s.GetUserInfoWithState(ctx, provider, token, nil)
}

func (s *OAuthProviderService) GetUserInfoWithState(ctx context.Context, provider string, token *oauth2.Token, state *OAuthState) (*SSOProfile, error) {
	switch strings.ToLower(provider) {
	case "google":
		return s.getGoogleUserInfo(ctx, token)
	case "github":
		return s.getGitHubUserInfo(ctx, token)
	case "oidc":
		return s.getOIDCUserInfo(ctx, token, state)
	default:
		return nil, errors.New("unknown provider: " + provider)
	}
}

func (s *OAuthProviderService) getGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*SSOProfile, error) {
	client := s.Cfg.SSO.Google.ClientConfig(ctx).Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google api returned status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	avatarURL := data.Picture
	return &SSOProfile{
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: data.ID,
		Email:          data.Email,
		FirstName:      data.GivenName,
		LastName:       data.FamilyName,
		AvatarURL: func() *string {
			if avatarURL != "" {
				return &avatarURL
			}
			return nil
		}(),
		RawProfile: map[string]interface{}{
			"id":             data.ID,
			"email":          data.Email,
			"name":           data.Name,
			"given_name":     data.GivenName,
			"family_name":    data.FamilyName,
			"picture":        data.Picture,
			"verified_email": data.VerifiedEmail,
		},
	}, nil
}

func (s *OAuthProviderService) getGitHubUserInfo(ctx context.Context, token *oauth2.Token) (*SSOProfile, error) {
	client := s.Cfg.SSO.GitHub.ClientConfig(ctx).Client(ctx, token)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if json.NewDecoder(emailResp.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						data.Email = e.Email
						break
					}
				}
			}
		}
	}

	if data.Email == "" {
		return nil, errors.New("github email not available")
	}

	parts := strings.SplitN(data.Name, " ", 2)
	firstName := parts[0]
	lastName := ""
	if len(parts) > 1 {
		lastName = parts[1]
	}

	avatarURL := data.AvatarURL
	return &SSOProfile{
		Provider:       models.SSOProviderTypeGitHub,
		ProviderUserID: fmt.Sprintf("%d", data.ID),
		Email:          data.Email,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarURL: func() *string {
			if avatarURL != "" {
				return &avatarURL
			}
			return nil
		}(),
		RawProfile: map[string]interface{}{
			"id":         data.ID,
			"login":      data.Login,
			"name":       data.Name,
			"email":      data.Email,
			"avatar_url": data.AvatarURL,
		},
	}, nil
}

func (s *OAuthProviderService) getOIDCUserInfo(ctx context.Context, token *oauth2.Token, state *OAuthState) (*SSOProfile, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oidc: id_token missing from token response")
	}

	verifier, err := s.getOIDCVerifier(ctx)
	if err != nil {
		return nil, err
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", err)
	}

	if state != nil && state.Nonce != "" && idToken.Nonce != state.Nonce {
		return nil, errors.New("oidc: id_token nonce mismatch")
	}

	claims := map[string]interface{}{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: failed to parse id_token claims: %w", err)
	}

	provider, err := s.getOIDCProvider(ctx)
	if err == nil {
		userInfo, uerr := provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if uerr == nil {
			extra := map[string]interface{}{}
			if err := userInfo.Claims(&extra); err == nil {
				for k, v := range extra {
					if _, exists := claims[k]; !exists {
						claims[k] = v
					}
				}
			}
		} else {
			logger.Warn("oidc_userinfo_unavailable", map[string]interface{}{
				"error": uerr.Error(),
			})
		}
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("oidc: subject claim is required")
	}

	email, _ := claims["email"].(string)
	if email == "" {
		return nil, errors.New("oidc: email claim is required (request 'email' scope)")
	}

	name, _ := claims["name"].(string)
	givenName, _ := claims["given_name"].(string)
	familyName, _ := claims["family_name"].(string)
	picture, _ := claims["picture"].(string)

	firstName := givenName
	lastName := familyName
	if firstName == "" && name != "" {
		parts := strings.SplitN(name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	return &SSOProfile{
		Provider:       models.SSOProviderTypeOIDC,
		ProviderUserID: sub,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarURL: func() *string {
			if picture != "" {
				return &picture
			}
			return nil
		}(),
		RawProfile: claims,
	}, nil
}
