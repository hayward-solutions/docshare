package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	ldap "github.com/go-ldap/ldap/v3"
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

	cfg := s.Cfg.LDAP
	ldapURL := cfg.URL

	logger.Info("ldap_auth_attempt", map[string]interface{}{
		"username": username,
		"url":      ldapURL,
	})

	l, err := ldap.DialURL(ldapURL)
	if err != nil {
		logger.Warn("ldap_dial_failed", map[string]interface{}{
			"username": username,
			"url":      ldapURL,
			"error":    err.Error(),
		})
		return nil, errors.New("failed to connect to LDAP server")
	}
	defer l.Close()

	useTLS := strings.HasPrefix(strings.ToLower(ldapURL), "ldaps://")
	if useTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		err = l.StartTLS(tlsConfig)
		if err != nil {
			logger.Warn("ldap_starttls_failed", map[string]interface{}{
				"username": username,
				"error":    err.Error(),
			})
			return nil, errors.New("failed to start TLS")
		}
	}

	bindDN := cfg.BindDN
	bindPassword := cfg.BindPassword

	if bindDN != "" && bindPassword != "" {
		err = l.Bind(bindDN, bindPassword)
		if err != nil {
			logger.Warn("ldap_bind_failed", map[string]interface{}{
				"bind_dn": bindDN,
				"error":   err.Error(),
			})
			return nil, errors.New("failed to bind to LDAP server")
		}
	}

	searchFilter := fmt.Sprintf(cfg.UserFilter, username)
	emailField := cfg.EmailField
	if emailField == "" {
		emailField = "mail"
	}
	nameFields := strings.Split(cfg.NameFields, ",")

	attrs := []string{"dn", emailField}
	if len(nameFields) > 0 {
		for _, f := range nameFields {
			if f != "" {
				attrs = append(attrs, strings.TrimSpace(f))
			}
		}
	}
	if emailField != "mail" {
		attrs = append(attrs, "mail")
	}
	attrs = append(attrs, "cn", "uid", "sn", "givenName", "displayName")

	searchRequest := ldap.NewSearchRequest(
		cfg.SearchBase,
		ldap.ScopeWholeSubtree,
		ldap.DerefAlways,
		0,
		0,
		false,
		searchFilter,
		attrs,
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		logger.Warn("ldap_search_failed", map[string]interface{}{
			"username":   username,
			"filter":     searchFilter,
			"searchBase": cfg.SearchBase,
			"error":      err.Error(),
		})
		return nil, errors.New("failed to search for user")
	}

	if len(sr.Entries) == 0 {
		logger.Warn("ldap_user_not_found", map[string]interface{}{
			"username": username,
			"filter":   searchFilter,
		})
		return nil, errors.New("invalid credentials")
	}

	entry := sr.Entries[0]
	userDN := entry.DN

	err = l.Bind(userDN, password)
	if err != nil {
		logger.Warn("ldap_user_bind_failed", map[string]interface{}{
			"user_dn": userDN,
			"error":   err.Error(),
		})
		return nil, errors.New("invalid credentials")
	}

	email := getAttributeValue(entry, emailField)
	if email == "" {
		email = getAttributeValue(entry, "mail")
	}
	if email == "" {
		email = fmt.Sprintf("%s@%s", username, extractDomain(ldapURL))
	}

	firstName := username
	lastName := ""

	for _, nf := range nameFields {
		nf = strings.TrimSpace(nf)
		if nf == "" {
			continue
		}
		val := getAttributeValue(entry, nf)
		if val != "" {
			if firstName == username {
				firstName = val
			} else {
				lastName = val
			}
		}
	}

	if cn := getAttributeValue(entry, "cn"); cn != "" && firstName == username {
		parts := strings.SplitN(cn, " ", 2)
		if len(parts) > 0 && firstName == username {
			firstName = parts[0]
		}
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	if givenName := getAttributeValue(entry, "givenName"); givenName != "" {
		firstName = givenName
	}
	if sn := getAttributeValue(entry, "sn"); sn != "" {
		lastName = sn
	}

	logger.Info("ldap_auth_success", map[string]interface{}{
		"username": username,
		"email":    email,
		"user_dn":  userDN,
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

func getAttributeValue(entry *ldap.Entry, attr string) string {
	values := entry.GetAttributeValues(attr)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func extractDomain(ldapURL string) string {
	cleanURL := strings.TrimPrefix(strings.TrimPrefix(ldapURL, "ldap://"), "ldaps://")
	parts := strings.Split(cleanURL, ":")
	hostPart := strings.Split(parts[0], ".")[0]
	if len(parts) > 0 {
		hostParts := strings.Split(parts[0], ".")
		if len(hostParts) > 1 {
			return strings.Join(hostParts[1:], ".")
		}
		return hostPart + ".local"
	}
	return "local"
}

func (s *LDAPService) TestConnection(ctx context.Context) error {
	if !s.IsEnabled() {
		return errors.New("LDAP is not enabled")
	}

	cfg := s.Cfg.LDAP
	ldapURL := cfg.URL

	logger.Info("ldap_test_connection", map[string]interface{}{
		"url": ldapURL,
	})

	l, err := ldap.DialURL(ldapURL)
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer l.Close()

	useTLS := strings.HasPrefix(strings.ToLower(ldapURL), "ldaps://")
	if useTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		err = l.StartTLS(tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	bindDN := cfg.BindDN
	bindPassword := cfg.BindPassword

	if bindDN != "" && bindPassword != "" {
		err = l.Bind(bindDN, bindPassword)
		if err != nil {
			return fmt.Errorf("failed to bind to LDAP server: %w", err)
		}
	}

	searchRequest := ldap.NewSearchRequest(
		cfg.SearchBase,
		ldap.ScopeBaseObject,
		ldap.DerefAlways,
		0,
		0,
		false,
		"(objectClass=*)",
		[]string{"namingContexts"},
		nil,
	)

	_, err = l.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to search LDAP: %w", err)
	}

	return nil
}
