package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docshare/api/internal/config"
	"github.com/docshare/api/internal/models"
	"github.com/docshare/api/internal/storage"
)

// ExportFormat names a target format for the editor's "Export as…" menu.
// Keep this set conservative — anything we list here must round-trip
// safely through pandoc (or, for PDF, through Gotenberg).
type ExportFormat string

const (
	ExportPDF  ExportFormat = "pdf"
	ExportDOCX ExportFormat = "docx"
	ExportODT  ExportFormat = "odt"
	ExportRTF  ExportFormat = "rtf"
	ExportHTML ExportFormat = "html"
	ExportEPUB ExportFormat = "epub"
	ExportMD   ExportFormat = "md"
	ExportTXT  ExportFormat = "txt"
)

// pandocExecTimeout caps a single conversion. Pandoc on a sub-MB doc
// finishes in well under a second; if we hit the timeout something is
// wedged and the caller should surface a 500.
const pandocExecTimeout = 30 * time.Second

// maxConvertedBytes guards against pandoc producing a pathological output
// (e.g. a malformed input that blows up image embedding). Editor source
// caps at 1 MiB; allow 10× headroom for output, then refuse.
const maxConvertedBytes = 10 * 1024 * 1024

var (
	ErrFormatNotSupported = errors.New("export format not supported for this file type")
	ErrPandocMissing      = errors.New("pandoc binary not available on server")
)

type ExportService struct {
	Storage    *storage.S3Client
	Gotenberg  config.GotenbergConfig
	HTTPClient *http.Client
	// PandocPath is the resolved absolute path to the pandoc binary, or ""
	// if pandoc was not found at startup. Conversions that require pandoc
	// return ErrPandocMissing in that case so the handler can return a
	// clean 503.
	PandocPath string
}

func NewExportService(storageClient *storage.S3Client, gotenberg config.GotenbergConfig) *ExportService {
	path, _ := exec.LookPath("pandoc")
	return &ExportService{
		Storage:    storageClient,
		Gotenberg:  gotenberg,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		PandocPath: path,
	}
}

// Result carries the converted bytes plus the MIME type and filename the
// handler should hand back to the browser.
type ExportResult struct {
	Body     []byte
	MimeType string
	Filename string
}

// IsExportableSource returns true if the file's stored MIME type is one
// the export pipeline knows how to read. Today that's markdown and any
// text/* type — pandoc treats plain text as markdown with no formatting,
// which is the right behavior for the cases we surface in the UI.
func IsExportableSource(mimeType string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(m, ';'); i >= 0 {
		m = m[:i]
	}
	if m == "text/markdown" || m == "text/x-markdown" {
		return true
	}
	return strings.HasPrefix(m, "text/")
}

// SupportedFormatsFor returns the formats the editor should offer for a
// given source MIME type. Markdown gets the full set; non-markdown text
// gets a narrower set because the typography of e.g. EPUB on plain text
// is misleading.
func SupportedFormatsFor(mimeType string) []ExportFormat {
	if !IsExportableSource(mimeType) {
		return nil
	}
	m := strings.ToLower(mimeType)
	if strings.HasPrefix(m, "text/markdown") || strings.HasPrefix(m, "text/x-markdown") {
		return []ExportFormat{ExportPDF, ExportDOCX, ExportODT, ExportRTF, ExportHTML, ExportEPUB, ExportMD}
	}
	return []ExportFormat{ExportPDF, ExportDOCX, ExportTXT}
}

// ParseFormat normalizes user input ("PDF", "  docx ") into a known
// ExportFormat value. Returns false if the value is unknown.
func ParseFormat(raw string) (ExportFormat, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pdf":
		return ExportPDF, true
	case "docx":
		return ExportDOCX, true
	case "odt":
		return ExportODT, true
	case "rtf":
		return ExportRTF, true
	case "html", "htm":
		return ExportHTML, true
	case "epub":
		return ExportEPUB, true
	case "md", "markdown":
		return ExportMD, true
	case "txt":
		return ExportTXT, true
	}
	return "", false
}

// mimeFor returns the Content-Type a browser should see for a given
// export target. We need this because Go's mime.TypeByExtension doesn't
// always have the OOXML/ODF values on minimal Linux images.
func mimeFor(format ExportFormat) string {
	switch format {
	case ExportPDF:
		return "application/pdf"
	case ExportDOCX:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ExportODT:
		return "application/vnd.oasis.opendocument.text"
	case ExportRTF:
		return "application/rtf"
	case ExportHTML:
		return "text/html; charset=utf-8"
	case ExportEPUB:
		return "application/epub+zip"
	case ExportMD:
		return "text/markdown; charset=utf-8"
	case ExportTXT:
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// Export reads file's bytes from S3, converts them to the requested
// format, and returns the result ready to stream to the browser. The
// caller is responsible for permission checks; this method assumes the
// user is allowed to download the source.
func (e *ExportService) Export(ctx context.Context, file *models.File, format ExportFormat) (*ExportResult, error) {
	if file == nil {
		return nil, errors.New("nil file")
	}
	if !IsExportableSource(file.MimeType) {
		return nil, ErrFormatNotSupported
	}

	supported := false
	for _, f := range SupportedFormatsFor(file.MimeType) {
		if f == format {
			supported = true
			break
		}
	}
	if !supported {
		return nil, ErrFormatNotSupported
	}

	source, err := e.readSource(ctx, file)
	if err != nil {
		return nil, err
	}

	outName := exportFilename(file.Name, format)

	switch format {
	case ExportMD, ExportTXT:
		return &ExportResult{Body: source, MimeType: mimeFor(format), Filename: outName}, nil
	case ExportPDF:
		body, err := e.renderPDF(ctx, source, file.Name)
		if err != nil {
			return nil, err
		}
		return &ExportResult{Body: body, MimeType: mimeFor(format), Filename: outName}, nil
	default:
		body, err := e.runPandoc(ctx, source, sourceFormatFor(file.MimeType), string(format))
		if err != nil {
			return nil, err
		}
		return &ExportResult{Body: body, MimeType: mimeFor(format), Filename: outName}, nil
	}
}

func (e *ExportService) readSource(ctx context.Context, file *models.File) ([]byte, error) {
	obj, err := e.Storage.Download(ctx, file.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("download source: %w", err)
	}
	defer obj.Close()
	// Editor save path caps content at 1 MiB; read up to that + 1 so a
	// corrupted/oversized blob is rejected rather than truncated.
	body, err := io.ReadAll(io.LimitReader(obj, maxConvertedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	if int64(len(body)) > maxConvertedBytes {
		return nil, fmt.Errorf("source file exceeds export maximum of %d bytes", maxConvertedBytes)
	}
	return body, nil
}

// sourceFormatFor maps a stored MIME type to the format flag pandoc
// expects on its -f argument. Markdown is `gfm` so tables and task lists
// from the TipTap editor round-trip correctly.
func sourceFormatFor(mimeType string) string {
	m := strings.ToLower(mimeType)
	if strings.HasPrefix(m, "text/markdown") || strings.HasPrefix(m, "text/x-markdown") {
		return "gfm"
	}
	return "plain"
}

// runPandoc invokes pandoc, piping the source on stdin and capturing the
// converted bytes from stdout. Stderr is captured so a failure surfaces a
// useful message rather than just an exit code.
func (e *ExportService) runPandoc(ctx context.Context, source []byte, fromFmt, toFmt string) ([]byte, error) {
	if e.PandocPath == "" {
		return nil, ErrPandocMissing
	}

	execCtx, cancel := context.WithTimeout(ctx, pandocExecTimeout)
	defer cancel()

	args := []string{"-f", fromFmt, "-t", toFmt, "--standalone"}
	// HTML output is self-contained so the downloaded file renders without
	// the browser needing to fetch our editor's stylesheet. Pandoc inlines
	// CSS and embeds images.
	if toFmt == "html" {
		args = append(args, "--embed-resources")
	}

	cmd := exec.CommandContext(execCtx, e.PandocPath, args...)
	cmd.Stdin = bytes.NewReader(source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("pandoc timeout after %s", pandocExecTimeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("pandoc failed: %s", msg)
	}

	if stdout.Len() > maxConvertedBytes {
		return nil, fmt.Errorf("converted output exceeds maximum of %d bytes", maxConvertedBytes)
	}
	return stdout.Bytes(), nil
}

// renderPDF takes the raw source bytes, has pandoc emit a self-contained
// HTML document, and posts that HTML to Gotenberg's chromium HTML route
// to produce the PDF. This avoids pulling in LaTeX just for PDF output.
func (e *ExportService) renderPDF(ctx context.Context, source []byte, sourceName string) ([]byte, error) {
	html, err := e.runPandoc(ctx, source, sourceFormatFor(mimeForExtension(sourceName)), "html")
	if err != nil {
		return nil, err
	}
	return e.htmlToPDF(ctx, html)
}

// mimeForExtension is a tiny helper so renderPDF can pass a sensible
// format flag to pandoc when called purely from a filename context — the
// caller (Export) already has the MIME type, but we keep readPDF stand-
// alone for testability.
func mimeForExtension(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return "text/markdown"
	}
	return "text/plain"
}

func (e *ExportService) htmlToPDF(ctx context.Context, html []byte) ([]byte, error) {
	if strings.TrimSpace(e.Gotenberg.URL) == "" {
		return nil, errors.New("gotenberg url not configured")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		// Gotenberg's chromium route requires the main file to be named
		// index.html — other names are ignored.
		part, err := writer.CreateFormFile("files", "index.html")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := part.Write(html); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	url := strings.TrimRight(e.Gotenberg.URL, "/") + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("gotenberg conversion failed: %s", strings.TrimSpace(string(body)))
	}

	out, err := io.ReadAll(io.LimitReader(resp.Body, maxConvertedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read gotenberg response: %w", err)
	}
	if int64(len(out)) > maxConvertedBytes {
		return nil, fmt.Errorf("converted PDF exceeds maximum of %d bytes", maxConvertedBytes)
	}
	return out, nil
}

// exportFilename replaces the source filename's extension with the
// target format's natural extension, falling back to the format name
// if the source has none.
func exportFilename(sourceName string, format ExportFormat) string {
	base := filepath.Base(strings.TrimSpace(sourceName))
	if base == "" || base == "." || base == "/" {
		base = "document"
	}
	ext := filepath.Ext(base)
	stem := base
	if ext != "" {
		stem = strings.TrimSuffix(base, ext)
	}
	return stem + "." + extensionFor(format)
}

func extensionFor(format ExportFormat) string {
	switch format {
	case ExportPDF:
		return "pdf"
	case ExportDOCX:
		return "docx"
	case ExportODT:
		return "odt"
	case ExportRTF:
		return "rtf"
	case ExportHTML:
		return "html"
	case ExportEPUB:
		return "epub"
	case ExportMD:
		return "md"
	case ExportTXT:
		return "txt"
	}
	return string(format)
}
