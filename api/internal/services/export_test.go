package services

import (
	"bytes"
	"context"
	"strings"
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

func TestIsMarkdownMime(t *testing.T) {
	cases := map[string]bool{
		"text/markdown":              true,
		"text/x-markdown":            true,
		"text/markdown; charset=utf-8": true,
		"TEXT/MARKDOWN":              true,
		"text/plain":                 false,
		"text/csv":                   false,
		"text/typescript":            false,
		"":                           false,
	}
	for mime, want := range cases {
		if got := isMarkdownMime(mime); got != want {
			t.Errorf("isMarkdownMime(%q) = %v, want %v", mime, got, want)
		}
	}
}

func TestPrepareForPandoc(t *testing.T) {
	// Markdown source must pass through verbatim.
	md := []byte("# Heading\n\nText with **bold**.")
	if got := prepareForPandoc(md, "text/markdown"); !bytes.Equal(got, md) {
		t.Errorf("markdown should pass through unchanged, got %q", got)
	}

	// Non-markdown source must be wrapped in a fenced code block so
	// gfm doesn't interpret `# not a heading` etc.
	plain := []byte("# not a heading\n_not_ italics\n| a | b |")
	out := prepareForPandoc(plain, "text/plain")
	s := string(out)
	if !strings.HasPrefix(s, "~~~") {
		t.Errorf("expected tilde fence prefix, got %q", s)
	}
	if !strings.Contains(s, "# not a heading") {
		t.Errorf("source content missing from output")
	}
	// Round-trip through the actual gfm reader → html writer would
	// preserve the literal `#`; here we just assert the wrapper shape.
	if !strings.HasSuffix(strings.TrimRight(s, "\n"), "~~~") {
		t.Errorf("expected tilde fence suffix, got %q", s)
	}
}

func TestTildeFenceFor(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"no tildes", "hello world", "~~~"},
		{"short tilde run", "x~~", "~~~"},
		{"line-leading 3 tildes need 4", "~~~\nbody", "~~~~"},
		{"line-leading 5 tildes need 6", "~~~~~\nbody", "~~~~~~"},
		{"tildes mid-line ignored", "foo ~~~~~ bar", "~~~"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tildeFenceFor([]byte(tt.source)); got != tt.want {
				t.Errorf("tildeFenceFor(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
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

func TestIsRemoteURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"data:image/png;base64,abc", false},
		{"DATA:image/png;base64,abc", false},
		{"cid:foo@bar", false},
		{"#section-1", false},
		{"http://example.com/x.png", true},
		{"https://example.com/x.png", true},
		{"//example.com/x.png", true},
		{"http://169.254.169.254/latest/meta-data/", true},
		{"file:///etc/passwd", true},
		{"./image.png", true},
		{"images/foo.png", true},
		{"  http://example.com  ", true},
	}
	for _, tt := range tests {
		if got := isRemoteURL(tt.in); got != tt.want {
			t.Errorf("isRemoteURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeHTMLForChromium(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContain    []string
		wantNotContain []string
	}{
		{
			name:           "strips remote img src (SSRF target)",
			input:          `<html><body><img src="http://169.254.169.254/latest/meta-data/"></body></html>`,
			wantNotContain: []string{"169.254.169.254", `src="http`},
		},
		{
			name:           "strips https img src",
			input:          `<html><body><img src="https://evil.com/track.png" alt="x"></body></html>`,
			wantContain:    []string{`alt="x"`},
			wantNotContain: []string{"evil.com"},
		},
		{
			name:        "preserves data URI img",
			input:       `<html><body><img src="data:image/png;base64,iVBORw0KGgo="></body></html>`,
			wantContain: []string{"data:image/png;base64,iVBORw0KGgo="},
		},
		{
			name:        "preserves anchor href so PDFs stay clickable",
			input:       `<html><body><a href="https://example.com">link</a></body></html>`,
			wantContain: []string{`href="https://example.com"`, "link"},
		},
		{
			name:        "preserves fragment anchors",
			input:       `<html><body><a href="#section">jump</a><img src="#foo"></body></html>`,
			wantContain: []string{`href="#section"`, `src="#foo"`},
		},
		{
			name:           "strips iframe src",
			input:          `<html><body><iframe src="http://internal/admin"></iframe></body></html>`,
			wantNotContain: []string{"internal/admin"},
		},
		{
			name:           "strips relative img path",
			input:          `<html><body><img src="./local.png"></body></html>`,
			wantNotContain: []string{"./local.png"},
		},
		{
			name:        "preserves headings and paragraphs",
			input:       `<html><body><h1>Title</h1><p>Hello <strong>world</strong></p></body></html>`,
			wantContain: []string{"<h1>Title</h1>", "<strong>world</strong>"},
		},
		{
			name:           "drops script element entirely",
			input:          `<html><body><p>before</p><script>new Image().src='http://169.254.169.254/'</script><p>after</p></body></html>`,
			wantContain:    []string{"before", "after"},
			wantNotContain: []string{"<script", "169.254.169.254", "Image()"},
		},
		{
			name:           "drops link element entirely",
			input:          `<html><head><link rel="stylesheet" href="http://cdn.example.com/x.css"></head><body><p>hi</p></body></html>`,
			wantContain:    []string{"hi"},
			wantNotContain: []string{"<link", "cdn.example.com", "stylesheet"},
		},
		{
			name:           "strips style attribute",
			input:          `<html><body><div style="background-image:url(http://169.254.169.254/)">x</div></body></html>`,
			wantContain:    []string{"x"},
			wantNotContain: []string{"style=", "background-image", "169.254.169.254"},
		},
		{
			name:           "strips inline event handler",
			input:          `<html><body><div onclick="fetch('http://internal')" onload="x()">x</div></body></html>`,
			wantContain:    []string{"x"},
			wantNotContain: []string{"onclick", "onload", "fetch(", "internal"},
		},
		{
			name:           "scrubs url() in style block",
			input:          `<html><head><style>body { background: url(http://169.254.169.254/); color: red; }</style></head><body></body></html>`,
			wantContain:    []string{"<style", "color: red", "url()"},
			wantNotContain: []string{"169.254.169.254"},
		},
		{
			name:           "scrubs @import in style block",
			input:          `<html><head><style>@import url('http://evil.com/x.css'); body { color: red; }</style></head><body></body></html>`,
			wantContain:    []string{"color: red"},
			wantNotContain: []string{"@import", "evil.com"},
		},
		{
			name:        "preserves data: url() in style block",
			input:       `<html><head><style>.icon { background: url(data:image/png;base64,iVBOR); }</style></head><body></body></html>`,
			wantContain: []string{"data:image/png;base64,iVBOR"},
		},
		{
			name:           "drops srcset entirely (multi-candidate bypass)",
			input:          `<html><body><img src="data:image/png;base64,iV" srcset="data:,x 1w, http://169.254.169.254/ 9999w"></body></html>`,
			wantContain:    []string{"data:image/png;base64,iV"},
			wantNotContain: []string{"srcset", "169.254.169.254"},
		},
		{
			name:           "drops imagesrcset on link/img",
			input:          `<html><body><img imagesrcset="http://internal/x 1x"></body></html>`,
			wantNotContain: []string{"imagesrcset", "internal"},
		},
		{
			name:           "drops base element (rebase attack)",
			input:          `<html><head><base href="http://169.254.169.254/"></head><body><img src="#x"></body></html>`,
			wantNotContain: []string{"<base", "169.254.169.254"},
		},
		{
			name:           "drops svg element with image href",
			input:          `<html><body><p>text</p><svg><image href="http://169.254.169.254/"></image></svg></body></html>`,
			wantContain:    []string{"text"},
			wantNotContain: []string{"<svg", "<image", "169.254.169.254"},
		},
		{
			name:           "drops iframe with srcdoc (HTML injection)",
			input:          `<html><body><iframe srcdoc="&lt;script&gt;fetch('http://internal')&lt;/script&gt;"></iframe></body></html>`,
			wantNotContain: []string{"<iframe", "srcdoc", "internal"},
		},
		{
			name:           "drops meta refresh redirect",
			input:          `<html><head><meta http-equiv="refresh" content="0;url=http://169.254.169.254/"></head><body></body></html>`,
			wantNotContain: []string{"<meta", "169.254.169.254"},
		},
		{
			name:           "drops math (MathML foreign content)",
			input:          `<html><body><math><mi>x</mi></math></body></html>`,
			wantNotContain: []string{"<math"},
		},
		{
			name:           "scrubs comment-obfuscated @import",
			input:          `<html><head><style>@import/**/"http://169.254.169.254/x.css";body{color:red}</style></head><body></body></html>`,
			wantContain:    []string{"color:red"},
			wantNotContain: []string{"@import", "169.254.169.254"},
		},
		{
			name:           "scrubs url() with comment-obfuscated remote",
			input:          `<html><head><style>body{background:url(/**/http://169.254.169.254/)}</style></head><body></body></html>`,
			wantNotContain: []string{"169.254.169.254"},
		},
		{
			name:           "drops body background attribute (HTML4 legacy)",
			input:          `<html><body background="http://169.254.169.254/x.png"><p>hi</p></body></html>`,
			wantContain:    []string{"hi"},
			wantNotContain: []string{"background=", "169.254.169.254"},
		},
		{
			name:           "drops td background attribute",
			input:          `<html><body><table><tr><td background="http://169.254.169.254/">x</td></tr></table></body></html>`,
			wantContain:    []string{">x<"},
			wantNotContain: []string{"background=", "169.254.169.254"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := sanitizeHTMLForChromium([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(out)
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\noutput: %s", want, s)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(s, notWant) {
					t.Errorf("output unexpectedly contains %q\noutput: %s", notWant, s)
				}
			}
		})
	}
}
