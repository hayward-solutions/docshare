package models

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestBaseModel_BeforeCreate(t *testing.T) {
	t.Run("generates UUID if not set", func(t *testing.T) {
		model := &BaseModel{}
		err := model.BeforeCreate(nil)
		if err != nil {
			t.Fatalf("BeforeCreate returned error: %v", err)
		}
		if model.ID == uuid.Nil {
			t.Error("expected ID to be generated, got nil UUID")
		}
	})

	t.Run("preserves existing UUID", func(t *testing.T) {
		existingID := uuid.New()
		model := &BaseModel{ID: existingID}
		err := model.BeforeCreate(nil)
		if err != nil {
			t.Fatalf("BeforeCreate returned error: %v", err)
		}
		if model.ID != existingID {
			t.Errorf("expected ID to remain %s, got %s", existingID, model.ID)
		}
	})
}

func TestShare_IsPublic(t *testing.T) {
	tests := []struct {
		name      string
		shareType ShareType
		want      bool
	}{
		{"private share", ShareTypePrivate, false},
		{"public_anyone share", ShareTypePublicAnyone, true},
		{"public_logged_in share", ShareTypePublicLoggedIn, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			share := &Share{ShareType: tt.shareType}
			if got := share.IsPublic(); got != tt.want {
				t.Errorf("Share.IsPublic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShare_TableName(t *testing.T) {
	share := Share{}
	if share.TableName() != "shares" {
		t.Errorf("expected table name 'shares', got %s", share.TableName())
	}
}

func TestUser_ModelFields(t *testing.T) {
	t.Run("user role constants", func(t *testing.T) {
		if UserRoleAdmin != "admin" {
			t.Errorf("expected UserRoleAdmin to be 'admin', got %s", UserRoleAdmin)
		}
		if UserRoleUser != "user" {
			t.Errorf("expected UserRoleUser to be 'user', got %s", UserRoleUser)
		}
	})
}

func TestFile_ModelFields(t *testing.T) {
	t.Run("file with parent", func(t *testing.T) {
		parentID := uuid.New()
		ownerID := uuid.New()
		thumbnailPath := "thumbnails/thumb.png"

		file := File{
			Name:          "test.txt",
			MimeType:      "text/plain",
			Size:          100,
			IsDirectory:   false,
			ParentID:      &parentID,
			OwnerID:       ownerID,
			StoragePath:   "storage/test.txt",
			ThumbnailPath: &thumbnailPath,
		}

		if file.Name != "test.txt" {
			t.Errorf("expected Name 'test.txt', got %s", file.Name)
		}
		if *file.ParentID != parentID {
			t.Errorf("expected ParentID %s, got %s", parentID, *file.ParentID)
		}
		if file.OwnerID != ownerID {
			t.Errorf("expected OwnerID %s, got %s", ownerID, file.OwnerID)
		}
	})

	t.Run("directory file", func(t *testing.T) {
		file := File{
			Name:        "documents",
			MimeType:    "inode/directory",
			Size:        0,
			IsDirectory: true,
			OwnerID:     uuid.New(),
			StoragePath: "",
		}

		if !file.IsDirectory {
			t.Error("expected IsDirectory to be true")
		}
		if file.StoragePath != "" {
			t.Errorf("expected empty StoragePath for directory, got %s", file.StoragePath)
		}
	})
}

func TestSharePermission_Constants(t *testing.T) {
	if SharePermissionView != "view" {
		t.Errorf("expected SharePermissionView to be 'view', got %s", SharePermissionView)
	}
	if SharePermissionDownload != "download" {
		t.Errorf("expected SharePermissionDownload to be 'download', got %s", SharePermissionDownload)
	}
	if SharePermissionEdit != "edit" {
		t.Errorf("expected SharePermissionEdit to be 'edit', got %s", SharePermissionEdit)
	}
}

func TestShareType_Constants(t *testing.T) {
	if ShareTypePrivate != "private" {
		t.Errorf("expected ShareTypePrivate to be 'private', got %s", ShareTypePrivate)
	}
	if ShareTypePublicAnyone != "public_anyone" {
		t.Errorf("expected ShareTypePublicAnyone to be 'public_anyone', got %s", ShareTypePublicAnyone)
	}
	if ShareTypePublicLoggedIn != "public_logged_in" {
		t.Errorf("expected ShareTypePublicLoggedIn to be 'public_logged_in', got %s", ShareTypePublicLoggedIn)
	}
}

func TestGroup_Model(t *testing.T) {
	createdByID := uuid.New()
	description := "Engineering department members"
	group := Group{
		Name:        "Engineering Team",
		CreatedByID: createdByID,
		Description: &description,
	}

	if group.Name != "Engineering Team" {
		t.Errorf("expected Name 'Engineering Team', got %s", group.Name)
	}
	if group.CreatedByID != createdByID {
		t.Errorf("expected CreatedByID %s, got %s", createdByID, group.CreatedByID)
	}
}

func TestGroupMembership_RoleConstants(t *testing.T) {
	if GroupRoleOwner != "owner" {
		t.Errorf("expected GroupRoleOwner to be 'owner', got %s", GroupRoleOwner)
	}
	if GroupRoleAdmin != "admin" {
		t.Errorf("expected GroupRoleAdmin to be 'admin', got %s", GroupRoleAdmin)
	}
	if GroupRoleMember != "member" {
		t.Errorf("expected GroupRoleMember to be 'member', got %s", GroupRoleMember)
	}
}

func TestAPIToken_Model(t *testing.T) {
	userID := uuid.New()
	token := APIToken{
		Name:      "CLI Token",
		TokenHash: "hashed-token-value",
		UserID:    userID,
		ExpiresAt: nil,
	}

	if token.Name != "CLI Token" {
		t.Errorf("expected Name 'CLI Token', got %s", token.Name)
	}
	if token.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, token.UserID)
	}
}

func TestActivity_Model(t *testing.T) {
	userID := uuid.New()
	actorID := uuid.New()
	activity := Activity{
		UserID:       userID,
		ActorID:      actorID,
		Action:       "file.shared",
		ResourceType: "file",
		IsRead:       false,
	}

	if activity.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, activity.UserID)
	}
	if activity.ActorID != actorID {
		t.Errorf("expected ActorID %s, got %s", actorID, activity.ActorID)
	}
	if activity.IsRead {
		t.Error("expected IsRead to be false")
	}
}

func TestAuditLog_NoBaseModel(t *testing.T) {
	log := AuditLog{
		Action:       "user.login",
		ResourceType: "user",
	}

	if log.ID != uuid.Nil {
		t.Error("AuditLog should use UUID ID, not int")
	}
}

func TestTransfer_Model(t *testing.T) {
	senderID := uuid.New()
	recipientID := uuid.New()
	code := "ABC123"

	transfer := Transfer{
		Code:        code,
		SenderID:    senderID,
		RecipientID: &recipientID,
		FileName:    "document.pdf",
		FileSize:    1024,
		Status:      TransferStatusPending,
		Timeout:     300,
	}

	if transfer.Code != code {
		t.Errorf("expected Code %s, got %s", code, transfer.Code)
	}
	if transfer.Status != TransferStatusPending {
		t.Errorf("expected Status %s, got %s", TransferStatusPending, transfer.Status)
	}
}

func TestTransferStatus_Constants(t *testing.T) {
	if TransferStatusPending != "pending" {
		t.Errorf("expected TransferStatusPending to be 'pending', got %s", TransferStatusPending)
	}
	if TransferStatusActive != "active" {
		t.Errorf("expected TransferStatusActive to be 'active', got %s", TransferStatusActive)
	}
	if TransferStatusCompleted != "completed" {
		t.Errorf("expected TransferStatusCompleted to be 'completed', got %s", TransferStatusCompleted)
	}
	if TransferStatusCancelled != "cancelled" {
		t.Errorf("expected TransferStatusCancelled to be 'cancelled', got %s", TransferStatusCancelled)
	}
	if TransferStatusExpired != "expired" {
		t.Errorf("expected TransferStatusExpired to be 'expired', got %s", TransferStatusExpired)
	}
}

func TestDeviceCode_Model(t *testing.T) {
	userID := uuid.New()
	deviceCode := DeviceCode{
		DeviceCodeHash: "hashed-device-code-123",
		UserCode:       "ABCD-EFGH",
		UserID:         &userID,
		Status:         DeviceCodePending,
		Interval:       5,
	}

	if deviceCode.UserCode != "ABCD-EFGH" {
		t.Errorf("expected UserCode 'ABCD-EFGH', got %s", deviceCode.UserCode)
	}
	if *deviceCode.UserID != userID {
		t.Errorf("expected UserID %s, got %s", userID, *deviceCode.UserID)
	}
}

func TestBaseModel_DeletedAt(t *testing.T) {
	model := BaseModel{}

	if model.DeletedAt.Valid {
		t.Error("expected DeletedAt to be invalid (null) by default")
	}

	var deletedAt gorm.DeletedAt
	deletedAt.Valid = true

	model.DeletedAt = deletedAt
	if !model.DeletedAt.Valid {
		t.Error("expected DeletedAt to be valid after setting")
	}
}
