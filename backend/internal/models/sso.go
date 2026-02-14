package models

import (
	"github.com/google/uuid"
)

// SSOProviderType represents the type of SSO provider
type SSOProviderType string

const (
	SSOProviderTypeGoogle SSOProviderType = "google"
	SSOProviderTypeGitHub SSOProviderType = "github"
	SSOProviderTypeOIDC   SSOProviderType = "oidc"
	SSOProviderTypeSAML   SSOProviderType = "saml"
	SSOProviderTypeLDAP   SSOProviderType = "ldap"
)

// SSOProvider represents a configured SSO/Identity provider
type SSOProvider struct {
	BaseModel
	Name         string          `json:"name" gorm:"type:varchar(50);uniqueIndex;not null"`
	DisplayName  string          `json:"displayName" gorm:"type:varchar(100);not null"`
	Type         SSOProviderType `json:"type" gorm:"type:varchar(20);not null"`
	Enabled      bool            `json:"enabled" gorm:"default:false"`
	Priority     int             `json:"priority" gorm:"default:100"`
	ClientID     string          `json:"-" gorm:"type:varchar(255)"`
	ClientSecret string          `json:"-" gorm:"type:text"`
	RedirectURL  string          `json:"redirectURL" gorm:"type:varchar(500)"`
	Scopes       string          `json:"scopes" gorm:"type:varchar(500)"`      // Comma-separated scopes
	IssuerURL    string          `json:"issuerURL" gorm:"type:varchar(500)"`   // For OIDC
	MetadataURL  string          `json:"metadataURL" gorm:"type:varchar(500)"` // For SAML
	EntityID     string          `json:"entityID" gorm:"type:varchar(500)"`    // For SAML SP
	// LDAP specific
	LDAPURL          string `json:"ldapURL" gorm:"type:varchar(500)"`
	LDAPBindDN       string `json:"-" gorm:"type:varchar(255)"`
	LDAPBindPassword string `json:"-" gorm:"type:text"`
	LDAPSearchBase   string `json:"ldapSearchBase" gorm:"type:varchar(255)"`
	LDAPUserFilter   string `json:"ldapUserFilter" gorm:"type:varchar(255)"`
	LDAPEmailField   string `json:"ldapEmailField" gorm:"type:varchar(100)"`
	LDAPNameFields   string `json:"ldapNameFields" gorm:"type:varchar(255)"` // Comma-separated
	// SAML attribute mapping (stored as JSON)
	AttributeMapping string `json:"attributeMapping" gorm:"type:text"`
}

func (SSOProvider) TableName() string {
	return "sso_providers"
}

// LinkedAccount links a local user to an external identity provider
type LinkedAccount struct {
	BaseModel
	UserID         uuid.UUID       `json:"userId" gorm:"type:uuid;not null;index"`
	Provider       SSOProviderType `json:"provider" gorm:"type:varchar(20);not null;index"`
	ProviderUserID string          `json:"providerUserId" gorm:"type:varchar(255);not null"`
	Email          string          `json:"email" gorm:"type:varchar(255)"`
	ProfileData    string          `json:"profileData" gorm:"type:text"` // JSON stored as string

	// Relation
	User User `json:"-" gorm:"foreignKey:UserID"`
}

func (LinkedAccount) TableName() string {
	return "linked_accounts"
}
