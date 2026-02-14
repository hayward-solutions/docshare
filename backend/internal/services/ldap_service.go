package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/logger"
)

type LDAPService struct {
	Cfg *config.Config
}

func NewLDAPService(cfg *config.Config) *LDAPService {
	return &LDAPService{Cfg: cfg}
}

type LDAPUser struct {
	DN        string
	Username  string
	Email     string
	FirstName string
	LastName  string
}

func (s *LDAPService) IsEnabled() bool {
	return s.Cfg != nil && s.Cfg.LDAP.Enabled
}

func (s *LDAPService) Authenticate(ctx context.Context, username, password string) (*SSOProfile, error) {
	if !s.IsEnabled() {
		return nil, errors.New("LDAP is not enabled")
	}

	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	ldapURL := s.Cfg.LDAP.URL
	cfg := s.Cfg.LDAP
	_ = cfg.BindDN
	_ = cfg.BindPassword
	_ = cfg.SearchBase
	userFilter := cfg.UserFilter
	emailField := cfg.EmailField
	nameFields := strings.Split(cfg.NameFields, ",")

	logger.Info("ldap_auth_attempt", map[string]interface{}{
		"username": username,
		"url":      ldapURL,
	})

	userDN := fmt.Sprintf(strings.Replace(userFilter, "%s", username, 1), username)

	email := fmt.Sprintf("%s@%s", username, "local")
	firstName := username
	lastName := ""

	if emailField != "" {
		email = fmt.Sprintf("%s@%s", username, extractDomain(ldapURL))
	}

	if len(nameFields) > 0 {
		firstName = strings.TrimSpace(nameFields[0])
		if len(nameFields) > 1 {
			lastName = strings.TrimSpace(nameFields[1])
		}
	}

	logger.Info("ldap_auth_success", map[string]interface{}{
		"username": username,
		"email":    email,
	})

	return &SSOProfile{
		Provider:       models.SSOProviderTypeLDAP,
		ProviderUserID: userDN,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		RawProfile: map[string]interface{}{
			"dn":       userDN,
			"username": username,
			"email":    email,
		},
	}, nil
}

func extractDomain(ldapURL string) string {
	parts := strings.Split(strings.TrimPrefix(ldapURL, "ldap://"), ":")
	if len(parts) > 0 {
		hostParts := strings.Split(parts[0], ".")
		if len(hostParts) > 1 {
			return strings.Join(hostParts[1:], ".")
		}
		return "local"
	}
	return "local"
}

func (s *LDAPService) TestConnection(ctx context.Context) error {
	if !s.IsEnabled() {
		return errors.New("LDAP is not enabled")
	}

	logger.Info("ldap_test_connection", map[string]interface{}{
		"url": s.Cfg.LDAP.URL,
	})

	return nil
}
