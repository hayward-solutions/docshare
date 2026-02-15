package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/docshare/backend/internal/middleware"
	"github.com/docshare/backend/internal/models"
	"github.com/docshare/backend/pkg/logger"
	"github.com/docshare/backend/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TransfersHandler struct {
	DB             *gorm.DB
	DefaultTimeout int
}

func NewTransfersHandler(db *gorm.DB, defaultTimeout int) *TransfersHandler {
	return &TransfersHandler{DB: db, DefaultTimeout: defaultTimeout}
}

func generateTransferCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes)[:length]), nil
}

type createTransferRequest struct {
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	Timeout  *int   `json:"timeout,omitempty"`
}

func (h *TransfersHandler) Create(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req createTransferRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.FileName == "" {
		return utils.Error(c, fiber.StatusBadRequest, "fileName is required")
	}
	if req.FileSize <= 0 {
		return utils.Error(c, fiber.StatusBadRequest, "fileSize must be positive")
	}

	code, err := generateTransferCode(6)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed generating code")
	}

	timeout := h.DefaultTimeout
	if req.Timeout != nil && *req.Timeout > 0 {
		timeout = *req.Timeout
	}

	transfer := models.Transfer{
		Code:      code,
		SenderID:  currentUser.ID,
		FileName:  req.FileName,
		FileSize:  req.FileSize,
		Status:    models.TransferStatusPending,
		Timeout:   timeout,
		ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Second),
	}

	if err := h.DB.Create(&transfer).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed creating transfer")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_created", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
		"file_name":   req.FileName,
		"file_size":   req.FileSize,
	})

	return utils.Success(c, fiber.StatusCreated, fiber.Map{
		"code":      code,
		"fileName":  transfer.FileName,
		"fileSize":  transfer.FileSize,
		"expiresAt": transfer.ExpiresAt,
	})
}

func (h *TransfersHandler) Get(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.Preload("Sender").First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if transfer.Status == models.TransferStatusExpired {
		return utils.Error(c, fiber.StatusGone, "transfer has expired")
	}
	if transfer.Status == models.TransferStatusCancelled {
		return utils.Error(c, fiber.StatusGone, "transfer was cancelled")
	}
	if transfer.Status == models.TransferStatusCompleted {
		return utils.Error(c, fiber.StatusGone, "transfer already completed")
	}

	senderPolling := transfer.SenderID == currentUser.ID
	receiverPolling := transfer.RecipientID != nil && *transfer.RecipientID == currentUser.ID

	if !senderPolling && !receiverPolling {
		return utils.Error(c, fiber.StatusForbidden, "not authorized for this transfer")
	}

	if senderPolling && transfer.Status == models.TransferStatusActive {
		return utils.Success(c, fiber.StatusOK, fiber.Map{
			"status":      "receiver_connected",
			"code":        transfer.Code,
			"fileName":    transfer.FileName,
			"fileSize":    transfer.FileSize,
			"recipientID": transfer.RecipientID,
		})
	}

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"status":    string(transfer.Status),
		"code":      transfer.Code,
		"fileName":  transfer.FileName,
		"fileSize":  transfer.FileSize,
		"expiresAt": transfer.ExpiresAt,
	})
}

func (h *TransfersHandler) Connect(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if time.Now().After(transfer.ExpiresAt) {
		h.DB.Model(&transfer).Update("status", models.TransferStatusExpired)
		return utils.Error(c, fiber.StatusGone, "transfer has expired")
	}

	if transfer.Status != models.TransferStatusPending {
		return utils.Error(c, fiber.StatusConflict, "transfer is not pending")
	}

	if transfer.SenderID == currentUser.ID {
		return utils.Error(c, fiber.StatusBadRequest, "cannot connect to your own transfer")
	}

	recipientID := currentUser.ID
	if err := h.DB.Model(&transfer).Updates(map[string]interface{}{
		"status":       models.TransferStatusActive,
		"recipient_id": recipientID,
	}).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed connecting to transfer")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_connected", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{
		"status":   "connected",
		"fileName": transfer.FileName,
		"fileSize": transfer.FileSize,
	})
}

func (h *TransfersHandler) Upload(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if transfer.SenderID != currentUser.ID {
		return utils.Error(c, fiber.StatusForbidden, "not the sender")
	}

	if transfer.Status != models.TransferStatusActive {
		return utils.Error(c, fiber.StatusBadRequest, "receiver not connected")
	}

	chunkIndex := c.Get("X-Chunk-Index")
	chunkTotal := c.Get("X-Chunk-Total")

	contentLength := c.Get("Content-Length")
	if contentLength == "" {
		return utils.Error(c, fiber.StatusBadRequest, "Content-Length required")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_chunk_received", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
		"chunk_index": chunkIndex,
		"chunk_total": chunkTotal,
		"size":        contentLength,
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"received": true})
}

func (h *TransfersHandler) Download(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if transfer.RecipientID == nil || *transfer.RecipientID != currentUser.ID {
		return utils.Error(c, fiber.StatusForbidden, "not the recipient")
	}

	if transfer.Status != models.TransferStatusActive {
		return utils.Error(c, fiber.StatusBadRequest, "transfer not active")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_download_started", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
		"file_name":   transfer.FileName,
	})

	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", transfer.FileName))
	c.Set("Content-Length", fmt.Sprintf("%d", transfer.FileSize))
	c.Set("X-Filename", transfer.FileName)
	c.Set("X-FileSize", fmt.Sprintf("%d", transfer.FileSize))

	return c.Status(fiber.StatusOK).SendString("")
}

func (h *TransfersHandler) Complete(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if transfer.SenderID != currentUser.ID && (transfer.RecipientID == nil || *transfer.RecipientID != currentUser.ID) {
		return utils.Error(c, fiber.StatusForbidden, "not authorized")
	}

	if err := h.DB.Model(&transfer).Update("status", models.TransferStatusCompleted).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed completing transfer")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_completed", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"status": "completed"})
}

func (h *TransfersHandler) Cancel(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	code := c.Params("code")
	if code == "" {
		return utils.Error(c, fiber.StatusBadRequest, "code is required")
	}

	var transfer models.Transfer
	if err := h.DB.First(&transfer, "code = ?", code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, fiber.StatusNotFound, "transfer not found")
		}
		return utils.Error(c, fiber.StatusInternalServerError, "failed loading transfer")
	}

	if transfer.SenderID != currentUser.ID {
		return utils.Error(c, fiber.StatusForbidden, "only sender can cancel")
	}

	if transfer.Status == models.TransferStatusCompleted {
		return utils.Error(c, fiber.StatusBadRequest, "transfer already completed")
	}

	if err := h.DB.Model(&transfer).Update("status", models.TransferStatusCancelled).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed cancelling transfer")
	}

	logger.InfoWithUser(currentUser.ID.String(), "transfer_cancelled", map[string]interface{}{
		"transfer_id": transfer.ID.String(),
		"code":        code,
	})

	return utils.Success(c, fiber.StatusOK, fiber.Map{"status": "cancelled"})
}

func (h *TransfersHandler) List(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var transfers []models.Transfer
	if err := h.DB.Where("sender_id = ? AND status IN ?", currentUser.ID, []string{"pending", "active"}).Find(&transfers).Error; err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed listing transfers")
	}

	type transferListItem struct {
		Code      string    `json:"code"`
		FileName  string    `json:"fileName"`
		FileSize  int64     `json:"fileSize"`
		Status    string    `json:"status"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	items := make([]transferListItem, len(transfers))
	for i, t := range transfers {
		items[i] = transferListItem{
			Code:      t.Code,
			FileName:  t.FileName,
			FileSize:  t.FileSize,
			Status:    string(t.Status),
			ExpiresAt: t.ExpiresAt,
		}
	}

	return utils.Success(c, fiber.StatusOK, items)
}

func (h *TransfersHandler) CleanupExpired() {
	h.DB.Model(&models.Transfer{}).
		Where("status IN ? AND expires_at < ?", []string{"pending", "active"}, time.Now()).
		Update("status", models.TransferStatusExpired)
}

func CleanupExpiredTransfers(db *gorm.DB) {
	db.Model(&models.Transfer{}).
		Where("status IN ? AND expires_at < ?", []string{"pending", "active"}, time.Now()).
		Update("status", models.TransferStatusExpired)
}
