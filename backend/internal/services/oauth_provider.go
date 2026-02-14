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
	"time"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/logger"
	"golang.org/x/oauth2"
	github "golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuthProviderService struct {
	Cfg *config.Config
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
		endpoint := oauth2.Endpoint{
			AuthURL:  s.Cfg.SSO.OIDC.IssuerURL + "/authorize",
			TokenURL: s.Cfg.SSO.OIDC.IssuerURL + "/token",
		}
		return &oauth2.Config{
			ClientID:     s.Cfg.SSO.OIDC.ClientID,
			ClientSecret: s.Cfg.SSO.OIDC.ClientSecret,
			RedirectURL:  s.Cfg.SSO.OIDC.RedirectURL,
			Scopes:       strings.Split(s.Cfg.SSO.OIDC.Scopes, ","),
			Endpoint:     endpoint,
		}, "oidc", nil

	default:
		return nil, "", errors.New("unknown oauth provider: " + provider)
	}
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
	switch strings.ToLower(provider) {
	case "google":
		return s.getGoogleUserInfo(ctx, token)
	case "github":
		return s.getGitHubUserInfo(ctx, token)
	case "oidc":
		return s.getOIDCUserInfo(ctx, token)
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

func (s *OAuthProviderService) getOIDCUserInfo(ctx context.Context, token *oauth2.Token) (*SSOProfile, error) {
	client := s.Cfg.SSO.OIDC.ClientConfig(ctx).Client(ctx, token)

	userInfoURL := s.Cfg.SSO.OIDC.IssuerURL + "/userinfo"
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oidc userinfo returned status %d: %s", resp.StatusCode, string(body))
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	email, _ := data["email"].(string)
	sub, _ := data["sub"].(string)
	name, _ := data["name"].(string)
	givenName, _ := data["given_name"].(string)
	familyName, _ := data["family_name"].(string)
	picture, _ := data["picture"].(string)

	if email == "" {
		email, _ = data["email"].(string)
	}
	if sub == "" {
		return nil, errors.New("oidc: subject claim is required")
	}

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
		RawProfile: data,
	}, nil
}
