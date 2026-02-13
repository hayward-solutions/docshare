package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/internal/storage"
	"github.com/docshare/backend/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditEntry struct {
	UserID       *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Details      map[string]interface{}
	IPAddress    string
	RequestID    string
}

type AuditService struct {
	DB      *gorm.DB
	Storage *storage.MinIOClient
	queue   chan models.AuditLog
}

func NewAuditService(db *gorm.DB, storageClient *storage.MinIOClient) *AuditService {
	s := &AuditService{
		DB:      db,
		Storage: storageClient,
		queue:   make(chan models.AuditLog, 1000),
	}
	go s.processQueue()
	return s
}

func (s *AuditService) LogAsync(entry AuditEntry) {
	row := models.AuditLog{
		UserID:       entry.UserID,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Details:      entry.Details,
		IPAddress:    entry.IPAddress,
		RequestID:    entry.RequestID,
		CreatedAt:    time.Now().UTC(),
	}

	select {
	case s.queue <- row:
	default:
		logger.Warn("audit_queue_full", map[string]interface{}{
			"action":  entry.Action,
			"dropped": true,
		})
	}
}

func (s *AuditService) processQueue() {
	for row := range s.queue {
		if err := s.DB.Create(&row).Error; err != nil {
			logger.Error("audit_log_insert_failed", err, map[string]interface{}{
				"action": row.Action,
			})
			continue
		}
		s.generateActivities(row)
	}
}

func (s *AuditService) generateActivities(log models.AuditLog) {
	if log.UserID == nil {
		return
	}

	var otherActivities []models.Activity

	switch log.Action {
	case "share.create":
		otherActivities = s.activitiesForShareCreate(log)
	case "share.delete":
		otherActivities = s.activitiesForShareDelete(log)
	case "file.upload":
		otherActivities = s.activitiesForFileUpload(log)
	case "file.delete":
		otherActivities = s.activitiesForFileDelete(log)
	case "group.member_add":
		otherActivities = s.activitiesForGroupMemberAdd(log)
	case "group.member_remove":
		otherActivities = s.activitiesForGroupMemberRemove(log)
	}

	for i := range otherActivities {
		if otherActivities[i].UserID == *log.UserID {
			continue
		}
		if err := s.DB.Create(&otherActivities[i]).Error; err != nil {
			logger.Error("activity_insert_failed", err, map[string]interface{}{
				"action":  log.Action,
				"user_id": otherActivities[i].UserID.String(),
			})
		}
	}

	selfActivity := s.selfActivityForAction(log)
	if selfActivity != nil {
		if err := s.DB.Create(selfActivity).Error; err != nil {
			logger.Error("self_activity_insert_failed", err, map[string]interface{}{
				"action": log.Action,
			})
		}
	}
}

func (s *AuditService) selfActivityForAction(log models.AuditLog) *models.Activity {
	if log.UserID == nil {
		return nil
	}

	actorID := *log.UserID
	resourceName := detailString(log.Details, "file_name")
	if resourceName == "" {
		resourceName = detailString(log.Details, "folder_name")
	}
	if resourceName == "" {
		resourceName = detailString(log.Details, "group_name")
	}

	var message string
	var resourceType string

	switch log.Action {
	case "file.upload":
		message = fmt.Sprintf("You uploaded \"%s\"", resourceName)
		resourceType = "file"
	case "file.download":
		message = fmt.Sprintf("You downloaded \"%s\"", resourceName)
		resourceType = "file"
	case "file.delete":
		message = fmt.Sprintf("You deleted \"%s\"", resourceName)
		resourceType = "file"
	case "file.update":
		message = fmt.Sprintf("You updated \"%s\"", resourceName)
		resourceType = "file"
	case "folder.create":
		message = fmt.Sprintf("You created folder \"%s\"", resourceName)
		resourceType = "file"
	case "share.create":
		message = fmt.Sprintf("You shared \"%s\"", resourceName)
		resourceType = "file"
	case "share.delete":
		message = fmt.Sprintf("You revoked a share on \"%s\"", resourceName)
		resourceType = "file"
	case "share.update":
		message = fmt.Sprintf("You updated sharing on \"%s\"", resourceName)
		resourceType = "file"
	case "user.login":
		message = "You signed in"
		resourceType = "user"
		resourceName = "Account"
	case "user.register":
		message = "Welcome to DocShare"
		resourceType = "user"
		resourceName = "Account"
	case "user.password_change":
		message = "You changed your password"
		resourceType = "user"
		resourceName = "Account"
	case "user.profile_update":
		message = "You updated your profile"
		resourceType = "user"
		resourceName = "Account"
	case "group.create":
		message = fmt.Sprintf("You created group \"%s\"", resourceName)
		resourceType = "group"
	case "group.delete":
		message = fmt.Sprintf("You deleted group \"%s\"", resourceName)
		resourceType = "group"
	case "group.member_add":
		message = fmt.Sprintf("You added a member to \"%s\"", resourceName)
		resourceType = "group"
	case "group.member_remove":
		message = fmt.Sprintf("You removed a member from \"%s\"", resourceName)
		resourceType = "group"
	case "admin.user_delete":
		message = "You deleted a user account"
		resourceType = "user"
		resourceName = "Admin"
	case "admin.user_update":
		message = "You updated a user account"
		resourceType = "user"
		resourceName = "Admin"
	case "api_token.create":
		tokenName := detailString(log.Details, "name")
		if tokenName == "" {
			tokenName = "API token"
		}
		message = fmt.Sprintf("You created API token \"%s\"", tokenName)
		resourceType = "api_token"
		resourceName = tokenName
	case "api_token.revoke":
		tokenName := detailString(log.Details, "name")
		if tokenName == "" {
			tokenName = "API token"
		}
		message = fmt.Sprintf("You revoked API token \"%s\"", tokenName)
		resourceType = "api_token"
		resourceName = tokenName
	case "auth.device_flow_approve":
		message = "You approved a device login"
		resourceType = "user"
		resourceName = "Account"
	case "auth.device_flow_login":
		message = "You signed in via device flow"
		resourceType = "user"
		resourceName = "Account"
	default:
		return nil
	}

	return &models.Activity{
		UserID:       actorID,
		ActorID:      actorID,
		Action:       log.Action,
		ResourceType: resourceType,
		ResourceID:   log.ResourceID,
		ResourceName: resourceName,
		Message:      message,
	}
}

func (s *AuditService) activitiesForShareCreate(log models.AuditLog) []models.Activity {
	if log.UserID == nil || log.ResourceID == nil {
		return nil
	}

	fileName := detailString(log.Details, "file_name")
	actorName := s.getActorName(*log.UserID)

	if userIDStr, ok := log.Details["shared_with_user_id"].(string); ok {
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil
		}
		return []models.Activity{{
			UserID:       uid,
			ActorID:      *log.UserID,
			Action:       log.Action,
			ResourceType: "file",
			ResourceID:   log.ResourceID,
			ResourceName: fileName,
			Message:      fmt.Sprintf("%s shared \"%s\" with you", actorName, fileName),
		}}
	}

	if groupIDStr, ok := log.Details["shared_with_group_id"].(string); ok {
		gid, err := uuid.Parse(groupIDStr)
		if err != nil {
			return nil
		}
		groupName := detailString(log.Details, "group_name")
		if groupName == "" {
			groupName = "a group"
		}
		members := s.getGroupMemberIDs(gid)
		result := make([]models.Activity, 0, len(members))
		for _, memberID := range members {
			result = append(result, models.Activity{
				UserID:       memberID,
				ActorID:      *log.UserID,
				Action:       log.Action,
				ResourceType: "file",
				ResourceID:   log.ResourceID,
				ResourceName: fileName,
				Message:      fmt.Sprintf("%s shared \"%s\" with %s", actorName, fileName, groupName),
			})
		}
		return result
	}

	return nil
}

func (s *AuditService) activitiesForShareDelete(log models.AuditLog) []models.Activity {
	if log.UserID == nil || log.ResourceID == nil {
		return nil
	}

	fileName := detailString(log.Details, "file_name")
	actorName := s.getActorName(*log.UserID)

	if userIDStr, ok := log.Details["shared_with_user_id"].(string); ok {
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil
		}
		return []models.Activity{{
			UserID:       uid,
			ActorID:      *log.UserID,
			Action:       log.Action,
			ResourceType: "file",
			ResourceID:   log.ResourceID,
			ResourceName: fileName,
			Message:      fmt.Sprintf("%s revoked your access to \"%s\"", actorName, fileName),
		}}
	}

	return nil
}

func (s *AuditService) activitiesForFileUpload(log models.AuditLog) []models.Activity {
	if log.UserID == nil || log.ResourceID == nil {
		return nil
	}

	parentIDStr := detailString(log.Details, "parent_id")
	if parentIDStr == "" {
		return nil
	}
	parentID, err := uuid.Parse(parentIDStr)
	if err != nil {
		return nil
	}

	fileName := detailString(log.Details, "file_name")
	actorName := s.getActorName(*log.UserID)
	recipients := s.getShareRecipients(parentID, *log.UserID)

	result := make([]models.Activity, 0, len(recipients))
	for _, uid := range recipients {
		result = append(result, models.Activity{
			UserID:       uid,
			ActorID:      *log.UserID,
			Action:       log.Action,
			ResourceType: "file",
			ResourceID:   log.ResourceID,
			ResourceName: fileName,
			Message:      fmt.Sprintf("%s uploaded \"%s\" to a shared folder", actorName, fileName),
		})
	}
	return result
}

func (s *AuditService) activitiesForFileDelete(log models.AuditLog) []models.Activity {
	if log.UserID == nil || log.ResourceID == nil {
		return nil
	}

	fileName := detailString(log.Details, "file_name")
	actorName := s.getActorName(*log.UserID)

	rawIDs, ok := log.Details["notify_user_ids"]
	if !ok || rawIDs == nil {
		return nil
	}

	idSlice, ok := rawIDs.([]string)
	if !ok {
		return nil
	}

	result := make([]models.Activity, 0, len(idSlice))
	for _, idStr := range idSlice {
		uid, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		result = append(result, models.Activity{
			UserID:       uid,
			ActorID:      *log.UserID,
			Action:       log.Action,
			ResourceType: "file",
			ResourceID:   log.ResourceID,
			ResourceName: fileName,
			Message:      fmt.Sprintf("%s deleted \"%s\"", actorName, fileName),
		})
	}
	return result
}

func (s *AuditService) activitiesForGroupMemberAdd(log models.AuditLog) []models.Activity {
	if log.UserID == nil {
		return nil
	}

	targetIDStr := detailString(log.Details, "target_user_id")
	if targetIDStr == "" {
		return nil
	}
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		return nil
	}

	groupName := detailString(log.Details, "group_name")
	actorName := s.getActorName(*log.UserID)

	return []models.Activity{{
		UserID:       targetID,
		ActorID:      *log.UserID,
		Action:       log.Action,
		ResourceType: "group",
		ResourceID:   log.ResourceID,
		ResourceName: groupName,
		Message:      fmt.Sprintf("%s added you to \"%s\"", actorName, groupName),
	}}
}

func (s *AuditService) activitiesForGroupMemberRemove(log models.AuditLog) []models.Activity {
	if log.UserID == nil {
		return nil
	}

	targetIDStr := detailString(log.Details, "target_user_id")
	if targetIDStr == "" {
		return nil
	}
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		return nil
	}

	groupName := detailString(log.Details, "group_name")
	actorName := s.getActorName(*log.UserID)

	return []models.Activity{{
		UserID:       targetID,
		ActorID:      *log.UserID,
		Action:       log.Action,
		ResourceType: "group",
		ResourceID:   log.ResourceID,
		ResourceName: groupName,
		Message:      fmt.Sprintf("%s removed you from \"%s\"", actorName, groupName),
	}}
}

func (s *AuditService) getActorName(userID uuid.UUID) string {
	var user models.User
	if err := s.DB.Select("first_name", "last_name").First(&user, "id = ?", userID).Error; err != nil {
		return "Someone"
	}
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}

func (s *AuditService) getGroupMemberIDs(groupID uuid.UUID) []uuid.UUID {
	var memberships []models.GroupMembership
	s.DB.Select("user_id").Where("group_id = ?", groupID).Find(&memberships)

	ids := make([]uuid.UUID, len(memberships))
	for i, m := range memberships {
		ids[i] = m.UserID
	}
	return ids
}

func (s *AuditService) getShareRecipients(fileID uuid.UUID, excludeUserID uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{excludeUserID: true}
	var result []uuid.UUID

	var shares []models.Share
	s.DB.Where("file_id = ?", fileID).
		Where("expires_at IS NULL OR expires_at > NOW()").
		Find(&shares)

	for _, share := range shares {
		if share.SharedWithUserID != nil && !seen[*share.SharedWithUserID] {
			seen[*share.SharedWithUserID] = true
			result = append(result, *share.SharedWithUserID)
		}
		if share.SharedWithGroupID != nil {
			members := s.getGroupMemberIDs(*share.SharedWithGroupID)
			for _, mid := range members {
				if !seen[mid] {
					seen[mid] = true
					result = append(result, mid)
				}
			}
		}
	}
	return result
}

// StartExporter runs a background goroutine that periodically exports
// new audit log rows to S3/MinIO as NDJSON files.
func (s *AuditService) StartExporter(interval time.Duration) {
	if s.Storage == nil {
		logger.Info("audit_exporter_disabled", map[string]interface{}{
			"reason": "no storage client configured",
		})
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			s.exportToS3()
		}
	}()

	logger.Info("audit_exporter_started", map[string]interface{}{
		"interval": interval.String(),
	})
}

func (s *AuditService) exportToS3() {
	var cursor models.AuditExportCursor
	err := s.DB.First(&cursor).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			cursor = models.AuditExportCursor{
				LastExportAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			}
			if createErr := s.DB.Create(&cursor).Error; createErr != nil {
				logger.Error("audit_export_cursor_create_failed", createErr, nil)
				return
			}
		} else {
			logger.Error("audit_export_cursor_load_failed", err, nil)
			return
		}
	}

	var logs []models.AuditLog
	if err := s.DB.Where("created_at > ?", cursor.LastExportAt).
		Order("created_at ASC").
		Limit(10000).
		Find(&logs).Error; err != nil {
		logger.Error("audit_export_query_failed", err, nil)
		return
	}

	if len(logs) == 0 {
		return
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, log := range logs {
		if err := enc.Encode(log); err != nil {
			logger.Error("audit_export_encode_failed", err, map[string]interface{}{
				"log_id": log.ID.String(),
			})
			continue
		}
	}

	now := time.Now().UTC()
	objectName := fmt.Sprintf("audit-logs/%s/%s.ndjson",
		now.Format("2006/01/02"),
		now.Format("15-04-05"),
	)

	if err := s.Storage.Upload(
		context.Background(),
		objectName,
		&buf,
		int64(buf.Len()),
		"application/x-ndjson",
	); err != nil {
		logger.Error("audit_export_upload_failed", err, map[string]interface{}{
			"object_name": objectName,
			"count":       len(logs),
		})
		return
	}

	lastCreatedAt := logs[len(logs)-1].CreatedAt
	s.DB.Model(&cursor).Updates(map[string]interface{}{
		"last_export_at": lastCreatedAt,
		"exported_count": gorm.Expr("exported_count + ?", len(logs)),
	})

	logger.Info("audit_export_success", map[string]interface{}{
		"object_name": objectName,
		"count":       len(logs),
	})
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	v, ok := details[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
