package services

import (
	"context"
	"testing"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPreviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed opening in-memory sqlite: %v", err)
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = db.AutoMigrate(&models.User{}, &models.File{})
	if err != nil {
		t.Fatalf("failed automigrating: %v", err)
	}

	return db
}

func TestIsOfficeDocument(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"docx file", "document.docx", true},
		{"xlsx file", "spreadsheet.xlsx", true},
		{"pptx file", "presentation.pptx", true},
		{"odt file", "document.odt", true},
		{"ods file", "spreadsheet.ods", true},
		{"odp file", "presentation.odp", true},
		{"txt file", "readme.txt", false},
		{"pdf file", "document.pdf", false},
		{"png file", "image.png", false},
		{"jpg file", "photo.jpg", false},
		{"empty extension", "file.", false},
		{"no extension", "file", false},
		{"uppercase DOCX", "file.DOCX", true},
		{"path with directory", "folder/document.docx", true},
		{"path with multiple dots", "folder/file.test.docx", true},
		{"old doc format", "document.doc", false},
		{"old xls format", "spreadsheet.xls", false},
		{"old ppt format", "presentation.ppt", false},
		{"rtf file", "document.rtf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOfficeDocument(tt.filename); got != tt.want {
				t.Errorf("isOfficeDocument(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestPreviewService_ConvertToPreview_Directory(t *testing.T) {
	db := setupPreviewTestDB(t)
	service := NewPreviewService(db, nil, config.GotenbergConfig{})

	owner := &models.User{
		Email:        "preview-dir@test.com",
		PasswordHash: "hash",
		FirstName:    "Test",
		LastName:     "User",
		Role:         models.UserRoleUser,
	}
	db.Create(owner)

	dir := &models.File{
		Name:        "folder",
		MimeType:    "inode/directory",
		Size:        0,
		IsDirectory: true,
		OwnerID:     owner.ID,
		StoragePath: "",
	}
	db.Create(dir)

	t.Run("returns error for directory", func(t *testing.T) {
		_, err := service.ConvertToPreview(context.Background(), dir)
		if err == nil {
			t.Fatal("expected error for directory")
		}
		if err.Error() != "cannot preview a directory" {
			t.Errorf("expected 'cannot preview a directory', got %v", err)
		}
	})
}
