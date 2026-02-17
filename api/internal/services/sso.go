package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SSOService struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewSSOService(db *gorm.DB, cfg *config.Config) *SSOService {
	return &SSOService{DB: db, Cfg: cfg}
}

type SSOProfile struct {
	Provider       models.SSOProviderType
	ProviderUserID string
	Email          string
	FirstName      string
	LastName       string
	AvatarURL      *string
	RawProfile     map[string]interface{}
}

func (s *SSOService) FindOrCreateUser(ctx context.Context, profile *SSOProfile) (*models.User, error) {
	var user models.User

	err := s.DB.WithContext(ctx).First(&user, "email = ?", profile.Email).Error
	if err == nil {
		linkedAccount := models.LinkedAccount{
			UserID:         user.ID,
			Provider:       profile.Provider,
			ProviderUserID: profile.ProviderUserID,
			Email:          profile.Email,
		}
		profileJSON, _ := json.Marshal(profile.RawProfile)
		linkedAccount.ProfileData = string(profileJSON)

		if err := s.DB.WithContext(ctx).Create(&linkedAccount).Error; err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				logger.Warn("sso_link_account_failed", map[string]interface{}{
					"user_id":  user.ID.String(),
					"provider": string(profile.Provider),
					"error":    err.Error(),
				})
			}
		}

		if user.AuthProvider == nil {
			provider := string(profile.Provider)
			if err := s.DB.WithContext(ctx).Model(&user).Update("auth_provider", provider).Error; err != nil {
				logger.Warn("sso_update_auth_provider_failed", map[string]interface{}{
					"user_id":  user.ID.String(),
					"provider": string(profile.Provider),
				})
			}
		}

		return &user, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if !s.Cfg.SSO.AutoRegister {
		return nil, errors.New("auto-registration is disabled")
	}

	role := models.UserRole(s.Cfg.SSO.DefaultRole)
	if role == "" {
		role = models.UserRoleUser
	}

	user = models.User{
		Email:           profile.Email,
		FirstName:       profile.FirstName,
		LastName:        profile.LastName,
		Role:            role,
		AvatarURL:       profile.AvatarURL,
		IsEmailVerified: true,
		AuthProvider:    func() *string { p := string(profile.Provider); return &p }(),
		ExternalID:      func() *string { p := profile.ProviderUserID; return &p }(),
	}

	if err := s.DB.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, err
	}

	linkedAccount := models.LinkedAccount{
		UserID:         user.ID,
		Provider:       profile.Provider,
		ProviderUserID: profile.ProviderUserID,
		Email:          profile.Email,
	}
	profileJSON, _ := json.Marshal(profile.RawProfile)
	linkedAccount.ProfileData = string(profileJSON)

	if err := s.DB.WithContext(ctx).Create(&linkedAccount).Error; err != nil {
		logger.Warn("sso_create_linked_account_failed", map[string]interface{}{
			"user_id":  user.ID.String(),
			"provider": string(profile.Provider),
			"error":    err.Error(),
		})
	}

	logger.Info("sso_user_created", map[string]interface{}{
		"user_id":  user.ID.String(),
		"email":    user.Email,
		"provider": string(profile.Provider),
	})

	return &user, nil
}

func (s *SSOService) GetEnabledProviders(ctx context.Context) ([]models.SSOProvider, error) {
	var providers []models.SSOProvider
	err := s.DB.WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority ASC").
		Find(&providers).Error
	return providers, err
}

func (s *SSOService) GetProviderByName(ctx context.Context, name string) (*models.SSOProvider, error) {
	var provider models.SSOProvider
	err := s.DB.WithContext(ctx).First(&provider, "name = ? AND enabled = ?", name, true).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (s *SSOService) LinkAccount(ctx context.Context, userID uuid.UUID, profile *SSOProfile) error {
	linkedAccount := models.LinkedAccount{
		UserID:         userID,
		Provider:       profile.Provider,
		ProviderUserID: profile.ProviderUserID,
		Email:          profile.Email,
	}
	profileJSON, _ := json.Marshal(profile.RawProfile)
	linkedAccount.ProfileData = string(profileJSON)

	return s.DB.WithContext(ctx).Create(&linkedAccount).Error
}

func (s *SSOService) GetLinkedAccounts(ctx context.Context, userID uuid.UUID) ([]models.LinkedAccount, error) {
	var accounts []models.LinkedAccount
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&accounts).Error
	return accounts, err
}

func (s *SSOService) UnlinkAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) error {
	return s.DB.WithContext(ctx).
		Where("id = ? AND user_id = ?", accountID, userID).
		Delete(&models.LinkedAccount{}).Error
}

func (s *SSOService) FindLinkedAccount(ctx context.Context, provider models.SSOProviderType, providerUserID string) (*models.LinkedAccount, error) {
	var account models.LinkedAccount
	err := s.DB.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}
