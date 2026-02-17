package services

import (
	"testing"
	"time"

	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/pkg/logger"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	logger.Init()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMembership{},
		&models.File{},
		&models.Share{},
		&models.AuditLog{},
		&models.Activity{},
		&models.AuditExportCursor{},
	)
	if err != nil {
		t.Fatalf("failed automigrating: %v", err)
	}

	return db
}

func TestNewAuditService(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.DB != db {
		t.Fatal("expected DB to be set")
	}
}

func TestAuditService_LogAsync(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)

	userID := uuid.New()
	user := &models.User{
		BaseModel:    models.BaseModel{ID: userID},
		Email:        "audit@test.com",
		PasswordHash: "hash",
		FirstName:    "Audit",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(user)

	t.Run("logs entry asynchronously", func(t *testing.T) {
		service.LogAsync(AuditEntry{
			UserID:       &userID,
			Action:       "test.action",
			ResourceType: "test",
			Details:      map[string]interface{}{"key": "value"},
			IPAddress:    "127.0.0.1",
			RequestID:    "req-123",
		})

		time.Sleep(200 * time.Millisecond)

		var count int64
		db.Model(&models.AuditLog{}).Where("action = ?", "test.action").Count(&count)
		if count == 0 {
			t.Error("expected audit log to be created")
		}
	})
}

func TestAuditService_SelfActivity(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)

	userID := uuid.New()
	user := &models.User{
		BaseModel:    models.BaseModel{ID: userID},
		Email:        "activity@test.com",
		PasswordHash: "hash",
		FirstName:    "Activity",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(user)

	actions := []struct {
		action  string
		hasBody bool
	}{
		{"file.upload", true},
		{"file.download", true},
		{"file.delete", true},
		{"file.update", true},
		{"folder.create", true},
		{"share.create", true},
		{"share.delete", true},
		{"share.update", true},
		{"user.login", true},
		{"user.register", true},
		{"user.password_change", true},
		{"user.profile_update", true},
		{"group.create", true},
		{"group.delete", true},
		{"group.member_add", true},
		{"group.member_remove", true},
		{"admin.user_delete", true},
		{"admin.user_update", true},
		{"api_token.create", true},
		{"api_token.revoke", true},
		{"auth.device_flow_approve", true},
		{"auth.device_flow_login", true},
		{"unknown.action", false},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			fileID := uuid.New()
			log := models.AuditLog{
				UserID:       &userID,
				Action:       tc.action,
				ResourceType: "test",
				ResourceID:   &fileID,
				Details: map[string]interface{}{
					"file_name":   "test.txt",
					"folder_name": "docs",
					"group_name":  "team",
					"name":        "token-name",
				},
			}

			activity := service.selfActivityForAction(log)
			if tc.hasBody && activity == nil {
				t.Errorf("expected self activity for %s", tc.action)
			}
			if !tc.hasBody && activity != nil {
				t.Errorf("expected nil activity for %s", tc.action)
			}
		})
	}

	t.Run("nil user returns nil", func(t *testing.T) {
		log := models.AuditLog{
			UserID: nil,
			Action: "file.upload",
		}
		activity := service.selfActivityForAction(log)
		if activity != nil {
			t.Error("expected nil activity for nil user")
		}
	})
}

func TestAuditService_ShareCreateActivity(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)

	ownerID := uuid.New()
	recipientID := uuid.New()
	owner := &models.User{
		BaseModel:    models.BaseModel{ID: ownerID},
		Email:        "owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Owner",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	recipient := &models.User{
		BaseModel:    models.BaseModel{ID: recipientID},
		Email:        "recipient@test.com",
		PasswordHash: "hash",
		FirstName:    "Recipient",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(recipient)

	fileID := uuid.New()

	t.Run("share with user generates activity", func(t *testing.T) {
		log := models.AuditLog{
			UserID:       &ownerID,
			Action:       "share.create",
			ResourceType: "share",
			ResourceID:   &fileID,
			Details: map[string]interface{}{
				"file_name":           "shared.txt",
				"shared_with_user_id": recipientID.String(),
			},
		}

		activities := service.activitiesForShareCreate(log)
		if len(activities) != 1 {
			t.Fatalf("expected 1 activity, got %d", len(activities))
		}
		if activities[0].UserID != recipientID {
			t.Errorf("expected activity for recipient")
		}
	})

	t.Run("share with group generates activities", func(t *testing.T) {
		group := &models.Group{
			Name:        "Test Group",
			CreatedByID: ownerID,
		}
		db.Create(group)

		membership := &models.GroupMembership{
			GroupID: group.ID,
			UserID:  recipientID,
			Role:    models.GroupRoleMember,
		}
		db.Create(membership)

		log := models.AuditLog{
			UserID:       &ownerID,
			Action:       "share.create",
			ResourceType: "share",
			ResourceID:   &fileID,
			Details: map[string]interface{}{
				"file_name":            "group-shared.txt",
				"shared_with_group_id": group.ID.String(),
				"group_name":           "Test Group",
			},
		}

		activities := service.activitiesForShareCreate(log)
		if len(activities) == 0 {
			t.Fatal("expected at least 1 activity for group share")
		}
	})

	t.Run("nil user returns nil", func(t *testing.T) {
		log := models.AuditLog{
			UserID:     nil,
			Action:     "share.create",
			ResourceID: &fileID,
		}
		activities := service.activitiesForShareCreate(log)
		if activities != nil {
			t.Error("expected nil for nil user")
		}
	})
}

func TestAuditService_ShareDeleteActivity(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)

	ownerID := uuid.New()
	recipientID := uuid.New()
	owner := &models.User{
		BaseModel:    models.BaseModel{ID: ownerID},
		Email:        "share-delete-owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Owner",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	fileID := uuid.New()

	t.Run("generates activity for user share deletion", func(t *testing.T) {
		log := models.AuditLog{
			UserID:       &ownerID,
			Action:       "share.delete",
			ResourceType: "share",
			ResourceID:   &fileID,
			Details: map[string]interface{}{
				"file_name":           "deleted.txt",
				"shared_with_user_id": recipientID.String(),
			},
		}

		activities := service.activitiesForShareDelete(log)
		if len(activities) != 1 {
			t.Fatalf("expected 1 activity, got %d", len(activities))
		}
	})

	t.Run("nil user returns nil", func(t *testing.T) {
		log := models.AuditLog{
			UserID:     nil,
			ResourceID: &fileID,
			Action:     "share.delete",
		}
		if service.activitiesForShareDelete(log) != nil {
			t.Error("expected nil")
		}
	})
}

func TestAuditService_GroupMemberActivities(t *testing.T) {
	db := setupAuditTestDB(t)
	service := NewAuditService(db, nil)

	ownerID := uuid.New()
	targetID := uuid.New()
	groupID := uuid.New()
	owner := &models.User{
		BaseModel:    models.BaseModel{ID: ownerID},
		Email:        "grp-owner@test.com",
		PasswordHash: "hash",
		FirstName:    "Group",
		LastName:     "Owner",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	t.Run("member add generates activity", func(t *testing.T) {
		log := models.AuditLog{
			UserID:       &ownerID,
			Action:       "group.member_add",
			ResourceType: "group",
			ResourceID:   &groupID,
			Details: map[string]interface{}{
				"target_user_id": targetID.String(),
				"group_name":     "Engineering",
			},
		}

		activities := service.activitiesForGroupMemberAdd(log)
		if len(activities) != 1 {
			t.Fatalf("expected 1 activity, got %d", len(activities))
		}
		if activities[0].UserID != targetID {
			t.Error("expected activity for target user")
		}
	})

	t.Run("member remove generates activity", func(t *testing.T) {
		log := models.AuditLog{
			UserID:       &ownerID,
			Action:       "group.member_remove",
			ResourceType: "group",
			ResourceID:   &groupID,
			Details: map[string]interface{}{
				"target_user_id": targetID.String(),
				"group_name":     "Engineering",
			},
		}

		activities := service.activitiesForGroupMemberRemove(log)
		if len(activities) != 1 {
			t.Fatalf("expected 1 activity, got %d", len(activities))
		}
	})

	t.Run("missing target_user_id returns nil", func(t *testing.T) {
		log := models.AuditLog{
			UserID:  &ownerID,
			Action:  "group.member_add",
			Details: map[string]interface{}{},
		}
		if service.activitiesForGroupMemberAdd(log) != nil {
			t.Error("expected nil for missing target")
		}
	})
}

func TestDetailString(t *testing.T) {
	tests := []struct {
		name    string
		details map[string]interface{}
		key     string
		want    string
	}{
		{"nil details", nil, "key", ""},
		{"missing key", map[string]interface{}{"other": "val"}, "key", ""},
		{"string value", map[string]interface{}{"key": "hello"}, "key", "hello"},
		{"non-string value", map[string]interface{}{"key": 42}, "key", "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detailString(tt.details, tt.key)
			if got != tt.want {
				t.Errorf("detailString() = %q, want %q", got, tt.want)
			}
		})
	}
}
