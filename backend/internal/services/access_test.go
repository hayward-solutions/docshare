package services

import (
	"context"
	"testing"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite database: %v", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMembership{},
		&models.File{},
		&models.Share{},
	)
	if err != nil {
		t.Fatalf("failed automigrating models: %v", err)
	}

	return db
}

func TestAccessService_HasAccess(t *testing.T) {
	db := setupAccessTestDB(t)
	service := NewAccessService(db)

	owner := &models.User{
		Email:        "owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Owner",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("failed creating owner: %v", err)
	}

	otherUser := &models.User{
		Email:        "other@test.com",
		PasswordHash: "hash",
		FirstName:    "Other",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	if err := db.Create(otherUser).Error; err != nil {
		t.Fatalf("failed creating other user: %v", err)
	}

	rootFile := &models.File{
		Name:        "root.txt",
		MimeType:    "text/plain",
		Size:        100,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "root.txt",
	}
	if err := db.Create(rootFile).Error; err != nil {
		t.Fatalf("failed creating root file: %v", err)
	}

	nestedFile := &models.File{
		Name:        "nested.txt",
		MimeType:    "text/plain",
		Size:        50,
		IsDirectory: false,
		ParentID:    &rootFile.ID,
		OwnerID:     owner.ID,
		StoragePath: "nested.txt",
	}
	if err := db.Create(nestedFile).Error; err != nil {
		t.Fatalf("failed creating nested file: %v", err)
	}

	t.Run("owner has full access to own files", func(t *testing.T) {
		if !service.HasAccess(context.TODO(), owner.ID, rootFile.ID, models.SharePermissionView) {
			t.Error("owner should have view access")
		}
		if !service.HasAccess(context.TODO(), owner.ID, rootFile.ID, models.SharePermissionDownload) {
			t.Error("owner should have download access")
		}
		if !service.HasAccess(context.TODO(), owner.ID, rootFile.ID, models.SharePermissionEdit) {
			t.Error("owner should have edit access")
		}
	})

	t.Run("non-owner without share has no access", func(t *testing.T) {
		if service.HasAccess(context.TODO(), otherUser.ID, rootFile.ID, models.SharePermissionView) {
			t.Error("non-owner should not have view access without share")
		}
	})

	t.Run("user with direct share has access", func(t *testing.T) {
		share := &models.Share{
			FileID:           rootFile.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &otherUser.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := db.Create(share).Error; err != nil {
			t.Fatalf("failed creating share: %v", err)
		}

		if !service.HasAccess(nil, otherUser.ID, rootFile.ID, models.SharePermissionView) {
			t.Error("user with view share should have view access")
		}
		if service.HasAccess(nil, otherUser.ID, rootFile.ID, models.SharePermissionEdit) {
			t.Error("user with view share should not have edit access")
		}
	})

	t.Run("user with group share has access", func(t *testing.T) {
		groupUser := &models.User{
			Email:        "groupuser@test.com",
			PasswordHash: "hash",
			FirstName:    "Group",
			LastName:     "User",
			Role:         models.UserRoleUser,
		}
		if err := db.Create(groupUser).Error; err != nil {
			t.Fatalf("failed creating group user: %v", err)
		}

		group := &models.Group{
			Name:        "Test Group",
			CreatedByID: owner.ID,
		}
		if err := db.Create(group).Error; err != nil {
			t.Fatalf("failed creating group: %v", err)
		}

		membership := &models.GroupMembership{
			GroupID: group.ID,
			UserID:  groupUser.ID,
			Role:    models.GroupRoleMember,
		}
		if err := db.Create(membership).Error; err != nil {
			t.Fatalf("failed creating membership: %v", err)
		}

		groupFile := &models.File{
			Name:        "group-file.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "group-file.txt",
		}
		if err := db.Create(groupFile).Error; err != nil {
			t.Fatalf("failed creating group file: %v", err)
		}

		groupShare := &models.Share{
			FileID:            groupFile.ID,
			SharedByID:        owner.ID,
			SharedWithGroupID: &group.ID,
			ShareType:         models.ShareTypePrivate,
			Permission:        models.SharePermissionDownload,
		}
		if err := db.Create(groupShare).Error; err != nil {
			t.Fatalf("failed creating group share: %v", err)
		}

		if !service.HasAccess(nil, groupUser.ID, groupFile.ID, models.SharePermissionView) {
			t.Error("group member should have view access via group share")
		}
		if !service.HasAccess(nil, groupUser.ID, groupFile.ID, models.SharePermissionDownload) {
			t.Error("group member should have download access via group share")
		}
		if service.HasAccess(nil, groupUser.ID, groupFile.ID, models.SharePermissionEdit) {
			t.Error("group member should not have edit access with download share")
		}
	})

	t.Run("access to nested file inherits from parent share", func(t *testing.T) {
		nestedAccessUser := &models.User{
			Email:        "nested@test.com",
			PasswordHash: "hash",
			FirstName:    "Nested",
			LastName:     "User",
			Role:         models.UserRoleUser,
		}
		if err := db.Create(nestedAccessUser).Error; err != nil {
			t.Fatalf("failed creating nested user: %v", err)
		}

		parentDir := &models.File{
			Name:        "parent-dir",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     owner.ID,
			StoragePath: "",
		}
		if err := db.Create(parentDir).Error; err != nil {
			t.Fatalf("failed creating parent dir: %v", err)
		}

		childFile := &models.File{
			Name:        "child.txt",
			MimeType:    "text/plain",
			Size:        50,
			IsDirectory: false,
			ParentID:    &parentDir.ID,
			OwnerID:     owner.ID,
			StoragePath: "child.txt",
		}
		if err := db.Create(childFile).Error; err != nil {
			t.Fatalf("failed creating child file: %v", err)
		}

		parentShare := &models.Share{
			FileID:           parentDir.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &nestedAccessUser.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
		}
		if err := db.Create(parentShare).Error; err != nil {
			t.Fatalf("failed creating parent share: %v", err)
		}

		if !service.HasAccess(nil, nestedAccessUser.ID, childFile.ID, models.SharePermissionView) {
			t.Error("user should have view access to child via parent share")
		}
	})

	t.Run("expired share denies access", func(t *testing.T) {
		expiredUser := &models.User{
			Email:        "expired@test.com",
			PasswordHash: "hash",
			FirstName:    "Expired",
			LastName:     "User",
			Role:         models.UserRoleUser,
		}
		if err := db.Create(expiredUser).Error; err != nil {
			t.Fatalf("failed creating expired user: %v", err)
		}

		expiredFile := &models.File{
			Name:        "expired-file.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "expired-file.txt",
		}
		if err := db.Create(expiredFile).Error; err != nil {
			t.Fatalf("failed creating expired file: %v", err)
		}

		pastTime := time.Now().Add(-24 * time.Hour)
		expiredShare := &models.Share{
			FileID:           expiredFile.ID,
			SharedByID:       owner.ID,
			SharedWithUserID: &expiredUser.ID,
			ShareType:        models.ShareTypePrivate,
			Permission:       models.SharePermissionView,
			ExpiresAt:        &pastTime,
		}
		if err := db.Create(expiredShare).Error; err != nil {
			t.Fatalf("failed creating expired share: %v", err)
		}

		if service.HasAccess(nil, expiredUser.ID, expiredFile.ID, models.SharePermissionView) {
			t.Error("user with expired share should not have access")
		}
	})

	t.Run("invalid permission returns false", func(t *testing.T) {
		if service.HasAccess(nil, owner.ID, rootFile.ID, "invalid") {
			t.Error("invalid permission should return false")
		}
	})

	t.Run("non-existent file returns false", func(t *testing.T) {
		fakeID := uuid.New()
		if service.HasAccess(nil, owner.ID, fakeID, models.SharePermissionView) {
			t.Error("non-existent file should return false")
		}
	})
}

func TestAccessService_HasPublicAccess(t *testing.T) {
	db := setupAccessTestDB(t)
	service := NewAccessService(db)

	owner := &models.User{
		Email:        "owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Owner",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("failed creating owner: %v", err)
	}

	publicFile := &models.File{
		Name:        "public.txt",
		MimeType:    "text/plain",
		Size:        100,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "public.txt",
	}
	if err := db.Create(publicFile).Error; err != nil {
		t.Fatalf("failed creating public file: %v", err)
	}

	loggedInFile := &models.File{
		Name:        "loggedin.txt",
		MimeType:    "text/plain",
		Size:        100,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "loggedin.txt",
	}
	if err := db.Create(loggedInFile).Error; err != nil {
		t.Fatalf("failed creating logged-in file: %v", err)
	}

	privateFile := &models.File{
		Name:        "private.txt",
		MimeType:    "text/plain",
		Size:        100,
		IsDirectory: false,
		OwnerID:     owner.ID,
		StoragePath: "private.txt",
	}
	if err := db.Create(privateFile).Error; err != nil {
		t.Fatalf("failed creating private file: %v", err)
	}

	t.Run("public_anyone share allows anonymous access", func(t *testing.T) {
		share := &models.Share{
			FileID:     publicFile.ID,
			SharedByID: owner.ID,
			ShareType:  models.ShareTypePublicAnyone,
			Permission: models.SharePermissionDownload,
		}
		if err := db.Create(share).Error; err != nil {
			t.Fatalf("failed creating public share: %v", err)
		}

		if !service.HasPublicAccess(nil, publicFile.ID, models.SharePermissionView, false) {
			t.Error("public file should be viewable by anyone")
		}
		if !service.HasPublicAccess(nil, publicFile.ID, models.SharePermissionDownload, false) {
			t.Error("public file should be downloadable by anyone")
		}
		if service.HasPublicAccess(nil, publicFile.ID, models.SharePermissionEdit, false) {
			t.Error("public file should not be editable by anonymous users")
		}
	})

	t.Run("public_logged_in share requires login flag", func(t *testing.T) {
		share := &models.Share{
			FileID:     loggedInFile.ID,
			SharedByID: owner.ID,
			ShareType:  models.ShareTypePublicLoggedIn,
			Permission: models.SharePermissionView,
		}
		if err := db.Create(share).Error; err != nil {
			t.Fatalf("failed creating logged-in share: %v", err)
		}

		if service.HasPublicAccess(nil, loggedInFile.ID, models.SharePermissionView, false) {
			t.Error("logged-in file should not be accessible without login flag")
		}
		if !service.HasPublicAccess(nil, loggedInFile.ID, models.SharePermissionView, true) {
			t.Error("logged-in file should be accessible with login flag")
		}
	})

	t.Run("private file has no public access", func(t *testing.T) {
		if service.HasPublicAccess(nil, privateFile.ID, models.SharePermissionView, false) {
			t.Error("private file should not have public access")
		}
		if service.HasPublicAccess(nil, privateFile.ID, models.SharePermissionView, true) {
			t.Error("private file should not have public access even with login flag")
		}
	})
}

func TestAccessService_GetPublicShareType(t *testing.T) {
	db := setupAccessTestDB(t)
	service := NewAccessService(db)

	owner := &models.User{
		Email:        "owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Owner",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("failed creating owner: %v", err)
	}

	t.Run("returns nil for private file", func(t *testing.T) {
		privateFile := &models.File{
			Name:        "private.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "private.txt",
		}
		if err := db.Create(privateFile).Error; err != nil {
			t.Fatalf("failed creating private file: %v", err)
		}

		shareType := service.GetPublicShareType(nil, privateFile.ID)
		if shareType != nil {
			t.Errorf("expected nil share type for private file, got %v", shareType)
		}
	})

	t.Run("returns public_anyone for publicly shared file", func(t *testing.T) {
		publicFile := &models.File{
			Name:        "public.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "public.txt",
		}
		if err := db.Create(publicFile).Error; err != nil {
			t.Fatalf("failed creating public file: %v", err)
		}

		share := &models.Share{
			FileID:     publicFile.ID,
			SharedByID: owner.ID,
			ShareType:  models.ShareTypePublicAnyone,
			Permission: models.SharePermissionView,
		}
		if err := db.Create(share).Error; err != nil {
			t.Fatalf("failed creating share: %v", err)
		}

		shareType := service.GetPublicShareType(nil, publicFile.ID)
		if shareType == nil {
			t.Fatal("expected non-nil share type")
		}
		if *shareType != models.ShareTypePublicAnyone {
			t.Errorf("expected public_anyone, got %s", *shareType)
		}
	})

	t.Run("prefers public_anyone over public_logged_in", func(t *testing.T) {
		bothFile := &models.File{
			Name:        "both.txt",
			MimeType:    "text/plain",
			Size:        100,
			IsDirectory: false,
			OwnerID:     owner.ID,
			StoragePath: "both.txt",
		}
		if err := db.Create(bothFile).Error; err != nil {
			t.Fatalf("failed creating both file: %v", err)
		}

		loggedInShare := &models.Share{
			FileID:     bothFile.ID,
			SharedByID: owner.ID,
			ShareType:  models.ShareTypePublicLoggedIn,
			Permission: models.SharePermissionView,
		}
		if err := db.Create(loggedInShare).Error; err != nil {
			t.Fatalf("failed creating logged-in share: %v", err)
		}

		anyoneShare := &models.Share{
			FileID:     bothFile.ID,
			SharedByID: owner.ID,
			ShareType:  models.ShareTypePublicAnyone,
			Permission: models.SharePermissionView,
		}
		if err := db.Create(anyoneShare).Error; err != nil {
			t.Fatalf("failed creating anyone share: %v", err)
		}

		shareType := service.GetPublicShareType(nil, bothFile.ID)
		if shareType == nil {
			t.Fatal("expected non-nil share type")
		}
		if *shareType != models.ShareTypePublicAnyone {
			t.Errorf("expected public_anyone to be preferred, got %s", *shareType)
		}
	})

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		fakeID := uuid.New()
		shareType := service.GetPublicShareType(nil, fakeID)
		if shareType != nil {
			t.Error("expected nil for non-existent file")
		}
	})
}

func TestPermissionLevel(t *testing.T) {
	tests := []struct {
		permission models.SharePermission
		wantLevel  int
		wantOK     bool
	}{
		{models.SharePermissionView, 1, true},
		{models.SharePermissionDownload, 2, true},
		{models.SharePermissionEdit, 3, true},
		{"invalid", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.permission), func(t *testing.T) {
			level, ok := permissionLevel(tt.permission)
			if level != tt.wantLevel {
				t.Errorf("permissionLevel(%s) = %d, want %d", tt.permission, level, tt.wantLevel)
			}
			if ok != tt.wantOK {
				t.Errorf("permissionLevel(%s) ok = %v, want %v", tt.permission, ok, tt.wantOK)
			}
		})
	}
}
