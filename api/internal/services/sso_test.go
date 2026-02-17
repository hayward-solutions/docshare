package services

import (
	"context"
	"testing"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSSOTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(&models.User{}, &models.Group{}, &models.GroupMembership{}, &models.LinkedAccount{}, &models.SSOProvider{})
	if err != nil {
		t.Fatalf("failed automigrating: %v", err)
	}

	return db
}

func TestSSOService_FindOrCreateUser_ExistingUser(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{
		SSO: config.SSOConfig{
			AutoRegister: true,
			DefaultRole:  "user",
		},
	}
	service := NewSSOService(db, cfg)

	existingUser := &models.User{
		Email:        "existing@test.com",
		PasswordHash: "somehash",
		FirstName:    "Existing",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(existingUser)

	profile := &SSOProfile{
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: "google-123",
		Email:          "existing@test.com",
		FirstName:      "New",
		LastName:       "Name",
	}

	t.Run("returns existing user without creating new", func(t *testing.T) {
		user, err := service.FindOrCreateUser(context.Background(), profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != existingUser.ID {
			t.Errorf("expected user ID %s, got %s", existingUser.ID, user.ID)
		}
		if user.FirstName != "Existing" {
			t.Errorf("expected first name to remain 'Existing', got %s", user.FirstName)
		}
	})

	t.Run("creates linked account for existing user", func(t *testing.T) {
		var linkedAccount models.LinkedAccount
		err := db.First(&linkedAccount, "user_id = ? AND provider = ?", existingUser.ID, models.SSOProviderTypeGoogle).Error
		if err != nil {
			t.Fatalf("linked account not found: %v", err)
		}
		if linkedAccount.ProviderUserID != "google-123" {
			t.Errorf("expected provider user ID 'google-123', got %s", linkedAccount.ProviderUserID)
		}
	})
}

func TestSSOService_FindOrCreateUser_NewUser(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{
		SSO: config.SSOConfig{
			AutoRegister: true,
			DefaultRole:  "user",
		},
	}
	service := NewSSOService(db, cfg)

	profile := &SSOProfile{
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: "google-456",
		Email:          "new@test.com",
		FirstName:      "New",
		LastName:       "User",
	}

	t.Run("creates new user when auto-register enabled", func(t *testing.T) {
		user, err := service.FindOrCreateUser(context.Background(), profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "new@test.com" {
			t.Errorf("expected email 'new@test.com', got %s", user.Email)
		}
		if user.FirstName != "New" {
			t.Errorf("expected first name 'New', got %s", user.FirstName)
		}
		if user.Role != models.UserRoleUser {
			t.Errorf("expected role 'user', got %s", user.Role)
		}
	})

	t.Run("creates linked account for new user", func(t *testing.T) {
		var user models.User
		err := db.First(&user, "email = ?", "new@test.com").Error
		if err != nil {
			t.Fatalf("user not found: %v", err)
		}

		var linkedAccount models.LinkedAccount
		err = db.First(&linkedAccount, "user_id = ? AND provider = ?", user.ID, models.SSOProviderTypeGoogle).Error
		if err != nil {
			t.Fatalf("linked account not found: %v", err)
		}
	})
}

func TestSSOService_FindOrCreateUser_AutoRegisterDisabled(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{
		SSO: config.SSOConfig{
			AutoRegister: false,
		},
	}
	service := NewSSOService(db, cfg)

	profile := &SSOProfile{
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: "google-789",
		Email:          "notregistered@test.com",
		FirstName:      "Not",
		LastName:       "Registered",
	}

	t.Run("returns error when auto-register disabled and user not found", func(t *testing.T) {
		_, err := service.FindOrCreateUser(context.Background(), profile)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "auto-registration is disabled" {
			t.Errorf("expected 'auto-registration is disabled', got %v", err)
		}
	})
}

func TestSSOService_GetEnabledProviders(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{}
	service := NewSSOService(db, cfg)

	t.Run("returns empty when no providers configured", func(t *testing.T) {
		providers, err := service.GetEnabledProviders(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(providers) != 0 {
			t.Errorf("expected 0 providers, got %d", len(providers))
		}
	})

	t.Run("returns configured providers", func(t *testing.T) {
		googleProvider := models.SSOProvider{
			Name:        "google-provider",
			Type:        models.SSOProviderTypeGoogle,
			DisplayName: "Google",
			ClientID:    "test-client-id",
			Enabled:     true,
		}
		db.Create(&googleProvider)

		githubProvider := models.SSOProvider{
			Name:        "github-provider",
			Type:        models.SSOProviderTypeGitHub,
			DisplayName: "GitHub",
			ClientID:    "github-client-id",
			Enabled:     true,
		}
		db.Create(&githubProvider)

		providers, err := service.GetEnabledProviders(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(providers) != 2 {
			t.Errorf("expected 2 providers, got %d", len(providers))
		}
	})
}

func TestSSOService_LinkAccount(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{}
	service := NewSSOService(db, cfg)

	user := &models.User{
		Email:        "link@test.com",
		PasswordHash: "hash",
		FirstName:    "Link",
		LastName:     "Test",
		Role:         models.UserRoleUser,
	}
	db.Create(user)

	profile := &SSOProfile{
		Provider:       models.SSOProviderTypeGitHub,
		ProviderUserID: "github-999",
		Email:          "link@test.com",
	}

	t.Run("links account to existing user", func(t *testing.T) {
		err := service.LinkAccount(context.Background(), user.ID, profile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var linkedAccount models.LinkedAccount
		err = db.First(&linkedAccount, "user_id = ? AND provider = ?", user.ID, models.SSOProviderTypeGitHub).Error
		if err != nil {
			t.Fatalf("linked account not found: %v", err)
		}
		if linkedAccount.ProviderUserID != "github-999" {
			t.Errorf("expected provider user ID 'github-999', got %s", linkedAccount.ProviderUserID)
		}
	})
}

func TestSSOService_UnlinkAccount(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{}
	service := NewSSOService(db, cfg)

	user := &models.User{
		Email:        "unlink@test.com",
		PasswordHash: "hash",
		FirstName:    "Unlink",
		LastName:     "Test",
		Role:         models.UserRoleUser,
	}
	db.Create(user)

	linkedAccount := models.LinkedAccount{
		UserID:         user.ID,
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: "google-555",
		Email:          "unlink@test.com",
	}
	db.Create(&linkedAccount)

	t.Run("unlinks account successfully", func(t *testing.T) {
		err := service.UnlinkAccount(context.Background(), user.ID, linkedAccount.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var count int64
		db.Model(&models.LinkedAccount{}).Where("id = ?", linkedAccount.ID).Count(&count)
		if count != 0 {
			t.Errorf("expected linked account to be deleted, found %d", count)
		}
	})
}

func TestSSOService_FindLinkedAccount(t *testing.T) {
	db := setupSSOTestDB(t)
	cfg := &config.Config{}
	service := NewSSOService(db, cfg)

	user := &models.User{
		Email:        "find@test.com",
		PasswordHash: "hash",
		FirstName:    "Find",
		LastName:     "Test",
		Role:         models.UserRoleUser,
	}
	db.Create(user)

	linkedAccount := models.LinkedAccount{
		UserID:         user.ID,
		Provider:       models.SSOProviderTypeGoogle,
		ProviderUserID: "google-find-123",
		Email:          "find@test.com",
	}
	db.Create(&linkedAccount)

	t.Run("finds linked account", func(t *testing.T) {
		found, err := service.FindLinkedAccount(context.Background(), models.SSOProviderTypeGoogle, "google-find-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found == nil {
			t.Fatal("expected to find linked account")
		}
		if found.ID != linkedAccount.ID {
			t.Errorf("expected ID %s, got %s", linkedAccount.ID, found.ID)
		}
	})

	t.Run("returns nil for non-existent account", func(t *testing.T) {
		found, err := service.FindLinkedAccount(context.Background(), models.SSOProviderTypeGoogle, "non-existent")
		if err != nil && err.Error() != "record not found" {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != nil {
			t.Error("expected nil for non-existent account")
		}
	})
}
