package services

import (
	"context"
	"testing"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in   string
		ok   bool
		want ExportFormat
	}{
		{"pdf", true, ExportPDF},
		{"PDF", true, ExportPDF},
		{"  docx ", true, ExportDOCX},
		{"odt", true, ExportODT},
		{"rtf", true, ExportRTF},
		{"html", true, ExportHTML},
		{"htm", true, ExportHTML},
		{"epub", true, ExportEPUB},
		{"md", true, ExportMD},
		{"markdown", true, ExportMD},
		{"txt", true, ExportTXT},
		{"", false, ""},
		{"docx2", false, ""},
		{"doc", false, ""},
	}
	for _, tt := range tests {
		got, ok := ParseFormat(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseFormat(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsExportableSource(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"text/markdown", true},
		{"text/x-markdown", true},
		{"text/markdown; charset=utf-8", true},
		{"TEXT/MARKDOWN", true},
		{"text/plain", true},
		{"text/csv", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", false},
		{"application/pdf", false},
		{"image/png", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsExportableSource(tt.mime); got != tt.want {
			t.Errorf("IsExportableSource(%q) = %v, want %v", tt.mime, got, tt.want)
		}
	}
}

func TestSupportedFormatsFor(t *testing.T) {
	mdFormats := SupportedFormatsFor("text/markdown")
	if len(mdFormats) == 0 {
		t.Fatal("expected non-empty formats for markdown")
	}
	wantInMd := map[ExportFormat]bool{
		ExportPDF: false, ExportDOCX: false, ExportODT: false,
		ExportRTF: false, ExportHTML: false, ExportEPUB: false, ExportMD: false,
	}
	for _, f := range mdFormats {
		if _, ok := wantInMd[f]; ok {
			wantInMd[f] = true
		}
	}
	for f, present := range wantInMd {
		if !present {
			t.Errorf("markdown formats missing %q", f)
		}
	}

	textFormats := SupportedFormatsFor("text/plain")
	wantInText := map[ExportFormat]bool{ExportPDF: false, ExportDOCX: false, ExportTXT: false}
	for _, f := range textFormats {
		if _, ok := wantInText[f]; ok {
			wantInText[f] = true
		}
	}
	for f, present := range wantInText {
		if !present {
			t.Errorf("text formats missing %q", f)
		}
	}

	if formats := SupportedFormatsFor("application/pdf"); len(formats) != 0 {
		t.Errorf("expected no formats for non-text mime, got %v", formats)
	}
}

func TestExportFilename(t *testing.T) {
	tests := []struct {
		name   string
		format ExportFormat
		want   string
	}{
		{"notes.md", ExportPDF, "notes.pdf"},
		{"notes.md", ExportDOCX, "notes.docx"},
		{"notes", ExportDOCX, "notes.docx"},
		{"a.b.c.md", ExportPDF, "a.b.c.pdf"},
		{"", ExportPDF, "document.pdf"},
		{".", ExportPDF, "document.pdf"},
	}
	for _, tt := range tests {
		if got := exportFilename(tt.name, tt.format); got != tt.want {
			t.Errorf("exportFilename(%q, %q) = %q, want %q", tt.name, tt.format, got, tt.want)
		}
	}
}

func TestMimeFor(t *testing.T) {
	if mimeFor(ExportPDF) != "application/pdf" {
		t.Errorf("pdf mime mismatch")
	}
	if mimeFor(ExportDOCX) != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("docx mime mismatch")
	}
	if mimeFor(ExportFormat("nope")) != "application/octet-stream" {
		t.Errorf("unknown format should fall back to octet-stream")
	}
}

func TestSourceFormatFor(t *testing.T) {
	if sourceFormatFor("text/markdown") != "gfm" {
		t.Errorf("markdown should map to gfm")
	}
	if sourceFormatFor("text/x-markdown; charset=utf-8") != "gfm" {
		t.Errorf("text/x-markdown should map to gfm")
	}
	if sourceFormatFor("text/plain") != "plain" {
		t.Errorf("plain text should map to plain")
	}
}

func TestExport_RejectsNonExportable(t *testing.T) {
	svc := NewExportService(nil, config.GotenbergConfig{})
	file := &models.File{Name: "image.png", MimeType: "image/png"}
	if _, err := svc.Export(context.Background(), file, ExportPDF); err != ErrFormatNotSupported {
		t.Errorf("expected ErrFormatNotSupported, got %v", err)
	}
}

func TestExport_RejectsUnsupportedFormatForSource(t *testing.T) {
	svc := NewExportService(nil, config.GotenbergConfig{})
	file := &models.File{Name: "doc.txt", MimeType: "text/plain"}
	// EPUB is only offered for markdown, not plain text.
	if _, err := svc.Export(context.Background(), file, ExportEPUB); err != ErrFormatNotSupported {
		t.Errorf("expected ErrFormatNotSupported for txt → epub, got %v", err)
	}
}
