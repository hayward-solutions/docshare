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
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

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

	prepared := prepareForPandoc(source, file.MimeType)

	switch format {
	case ExportMD, ExportTXT:
		return &ExportResult{Body: source, MimeType: mimeFor(format), Filename: outName}, nil
	case ExportPDF:
		body, err := e.renderPDF(ctx, prepared)
		if err != nil {
			return nil, err
		}
		return &ExportResult{Body: body, MimeType: mimeFor(format), Filename: outName}, nil
	default:
		body, err := e.runPandoc(ctx, prepared, "gfm", string(format))
		if err != nil {
			return nil, err
		}
		return &ExportResult{Body: body, MimeType: mimeFor(format), Filename: outName}, nil
	}
}

// prepareForPandoc wraps non-markdown text in a fenced code block so the
// gfm reader doesn't reinterpret literal `#`, `_`, `|`, etc. as markdown
// syntax. Markdown sources pass through verbatim.
//
// The fence is built from tildes — one more than the longest run of `~`
// at the start of any line in the source — so a pathological input that
// contains `~~~~` can't terminate the wrapper early.
func prepareForPandoc(source []byte, mimeType string) []byte {
	if isMarkdownMime(mimeType) {
		return source
	}
	fence := tildeFenceFor(source)
	var buf bytes.Buffer
	buf.Grow(len(source) + 2*len(fence) + 4)
	buf.WriteString(fence)
	buf.WriteByte('\n')
	buf.Write(source)
	if len(source) == 0 || source[len(source)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(fence)
	buf.WriteByte('\n')
	return buf.Bytes()
}

func isMarkdownMime(mimeType string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(m, ';'); i >= 0 {
		m = m[:i]
	}
	return m == "text/markdown" || m == "text/x-markdown"
}

// tildeFenceFor returns a tilde fence guaranteed to be longer than any
// line-leading tilde run in source, so a wrapped fenced code block
// cannot be closed prematurely by content.
func tildeFenceFor(source []byte) string {
	longest := 0
	atLineStart := true
	run := 0
	for _, b := range source {
		switch b {
		case '\n':
			atLineStart = true
			run = 0
		case '~':
			if atLineStart {
				run++
				if run > longest {
					longest = run
				}
			}
		default:
			atLineStart = false
			run = 0
		}
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("~", n)
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


// runPandoc invokes pandoc, piping the source on stdin and capturing the
// converted bytes from stdout. Stderr is captured so a failure surfaces a
// useful message rather than just an exit code.
func (e *ExportService) runPandoc(ctx context.Context, source []byte, fromFmt, toFmt string) ([]byte, error) {
	if e.PandocPath == "" {
		return nil, ErrPandocMissing
	}

	execCtx, cancel := context.WithTimeout(ctx, pandocExecTimeout)
	defer cancel()

	// `--sandbox` blocks all filesystem and network access except stdin/
	// stdout and the explicit `--resource-path`. Without it, pandoc's
	// DOCX/ODT/EPUB writers transparently fetch `<img src="http://…">`
	// references to embed image bytes into the output — the same SSRF
	// primitive that motivated dropping `--embed-resources` from the
	// HTML path, but on a writer the flag doesn't reach. With sandbox
	// on, embedded images must come from data URIs (which the TipTap
	// editor already produces for pasted/dropped images).
	//
	// `-o -` forces pandoc to write to stdout. Binary formats (docx, odt,
	// epub) refuse a TTY-bound stdout in some versions and require either
	// a file path or this explicit dash; including it for all formats
	// keeps behavior consistent.
	args := []string{"--sandbox", "-f", fromFmt, "-t", toFmt, "--standalone", "-o", "-"}

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

// renderPDF takes the prepared source bytes (markdown or fenced text —
// see prepareForPandoc), has pandoc emit a self-contained HTML
// document, and posts that HTML to Gotenberg's chromium HTML route to
// produce the PDF. This avoids pulling in LaTeX just for PDF output.
//
// The intermediate HTML is run through sanitizeHTMLForChromium first so
// remote `<img src>` / `<link href>` / similar attributes don't trigger
// SSRF when Chromium renders the page — markdown like
// `![](http://169.254.169.254/...)` would otherwise reach internal hosts
// from inside the gotenberg container.
func (e *ExportService) renderPDF(ctx context.Context, source []byte) ([]byte, error) {
	rendered, err := e.runPandoc(ctx, source, "gfm", "html")
	if err != nil {
		return nil, err
	}
	safe, err := sanitizeHTMLForChromium(rendered)
	if err != nil {
		return nil, fmt.Errorf("sanitize html: %w", err)
	}
	return e.htmlToPDF(ctx, safe)
}

func (e *ExportService) htmlToPDF(ctx context.Context, body []byte) ([]byte, error) {
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
		if _, err := part.Write(body); err != nil {
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

// remoteFetchAttrs maps element names to the attributes Chromium would
// hit the network for when rendering. Anchor `<a href>` is intentionally
// excluded — those are user clicks, not auto-fetches, and stripping them
// would degrade the PDF as a usable document. Elements that have no
// safe rendering at all in our pipeline (script, link, base, svg,
// iframe, embed, object, meta) are dropped wholesale in sanitizeNode
// rather than appearing here.
var remoteFetchAttrs = map[string][]string{
	"img":    {"src", "longdesc"},
	"source": {"src"},
	"track":  {"src"},
	"video":  {"src", "poster"},
	"audio":  {"src"},
	"input":  {"src"},
}

// dropElements names HTML elements that are removed wholesale before the
// rendered HTML reaches Gotenberg's Chromium. Each entry exists because
// the element either has no legitimate place in pandoc's standalone
// output or carries an attack surface (srcdoc, base href, foreign-content
// SVG image refs, http-equiv refresh, etc.) that's costly to whitelist.
var dropElements = map[string]bool{
	"script": true,
	"link":   true,
	"base":   true, // <base href="http://internal"> would re-anchor #fragment refs
	"iframe": true, // srcdoc carries arbitrary HTML; src is a network fetch
	"embed":  true,
	"object": true,
	"meta":   true, // http-equiv="refresh" content="0;url=http://internal"
	"svg":    true, // <image href="..."> and <use href="..."> bypass img sanitization
	"math":   true, // MathML parallels SVG's foreign-content attack surface
	"form":   true, // not auto-fetching but no use in PDFs
}

// cssURLPattern matches CSS `url(...)` references — quoted or bare, with
// or without surrounding whitespace. Used to rewrite `<style>` content so
// user-injected raw HTML can't smuggle SSRF through CSS.
var cssURLPattern = regexp.MustCompile(`(?i)url\s*\(\s*['"]?\s*([^'")\s]*)\s*['"]?\s*\)`)

// cssImportPattern matches CSS `@import` directives. We drop these
// entirely — there's no reason for user-controlled @import in our
// pandoc-rendered output. Note: CSS comments are stripped before this
// pattern runs so `@import/**/"..."` style obfuscation can't slip past
// the whitespace requirement here.
var cssImportPattern = regexp.MustCompile(`(?is)@import\b[^;]*;?`)

// cssCommentPattern matches CSS block comments (`/* ... */`, including
// multi-line). Chromium treats comments as whitespace, so
// `@import/**/"http://..."` would bypass cssImportPattern unless we
// strip comments first.
var cssCommentPattern = regexp.MustCompile(`/\*[\s\S]*?\*/`)

// sanitizeHTMLForChromium walks the parsed HTML tree and removes anything
// that would cause Chromium to issue outbound requests, execute scripts,
// or trigger event handlers when rendering for PDF export. This blunts an
// SSRF primitive: without it, an authenticated user could drop a remote
// image, inline script, `url()` reference, `<base href>`, or inline SVG
// `<image href>` in markdown (pandoc's gfm reader passes raw HTML
// through) and the gotenberg container would dutifully fetch internal
// services.
//
// What it does:
//   - removes elements in dropElements wholesale (script, link, base,
//     iframe, embed, object, meta, svg, math, form)
//   - scrubs <style> body content of url() and @import references
//   - drops `style=`, `srcset=`, `imagesrcset=`, and `on*=` attributes
//     from every element
//   - strips src/href/data attributes on auto-fetching tags when the
//     URL isn't a data:/cid:/fragment URL. Fragments survive because
//     `<base>` is in dropElements, so there is no injected base URL
//     for a `#frag` ref to rebase against — the `<base href="http://
//     internal">` + `<img src="#x">` attack pattern is closed by the
//     base drop, not by reclassifying fragments
//
// `data:` and `cid:` URLs survive so editor-pasted images (encoded as
// data URIs by the TipTap upload path) still render.
func sanitizeHTMLForChromium(input []byte) ([]byte, error) {
	doc, err := html.Parse(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	sanitizeNode(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sanitizeNode(n *html.Node) {
	// Snapshot children before walking — we mutate the tree as we go.
	var children []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		children = append(children, c)
	}

	for _, c := range children {
		if c.Type != html.ElementNode {
			sanitizeNode(c)
			continue
		}

		data := strings.ToLower(c.Data)

		// Drop entire elements with no safe rendering. See dropElements
		// for why each one is here.
		if dropElements[data] {
			n.RemoveChild(c)
			continue
		}

		// Strip inline event handlers, `style=` attributes (which can
		// embed `background:url(...)`), and `srcset` (parsed as multiple
		// candidates by Chromium, so a `data:,x 1w, http://... 9999w`
		// bypass slips past a naïve src-style check).
		c.Attr = filterDangerousAttrs(c.Attr)

		// Strip remote URLs in src/href/data attributes for auto-fetching
		// tags. data:/cid:/fragment URLs survive.
		if attrs, ok := remoteFetchAttrs[data]; ok {
			for _, name := range attrs {
				stripRemoteAttr(c, name)
			}
		}

		// Inside <style>, rewrite url(...) refs to url() and drop @import
		// directives. Pandoc's own default stylesheet doesn't use either,
		// so this only affects user-injected raw <style> blocks.
		if data == "style" {
			scrubStyleContent(c)
		}

		sanitizeNode(c)
	}
}

func filterDangerousAttrs(attrs []html.Attribute) []html.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		switch {
		case key == "style":
			// inline CSS can embed url(http://internal)
			continue
		case key == "srcset", key == "imagesrcset":
			// comma-separated candidate list — Chromium can pick any
			// candidate, so a single safe data: entry doesn't make the
			// whole attribute safe. Drop unconditionally.
			continue
		case key == "background":
			// Legacy HTML4 `<body background="…">` and `<td background="…">`
			// are still honored by Chromium and would trigger a fetch.
			continue
		case strings.HasPrefix(key, "on"):
			// onclick, onload, onerror, etc.
			continue
		}
		out = append(out, a)
	}
	return out
}

func scrubStyleContent(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.TextNode {
			continue
		}
		c.Data = scrubCSSResources(c.Data)
	}
}

// scrubCSSResources rewrites `url(...)` references in a CSS snippet to
// `url()` when the referenced URL would trigger a fetch, and removes
// `@import` directives entirely. CSS comments are stripped first so
// constructs like `@import/**/"http://..."` (Chromium parses comments
// as whitespace) can't bypass the import-removal pattern.
func scrubCSSResources(css string) string {
	css = cssCommentPattern.ReplaceAllString(css, "")
	css = cssImportPattern.ReplaceAllString(css, "")
	css = cssURLPattern.ReplaceAllStringFunc(css, func(match string) string {
		sub := cssURLPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return "url()"
		}
		ref := strings.TrimSpace(sub[1])
		low := strings.ToLower(ref)
		if strings.HasPrefix(low, "data:") {
			return match
		}
		return "url()"
	})
	return css
}

func stripRemoteAttr(n *html.Node, attrName string) {
	for i := len(n.Attr) - 1; i >= 0; i-- {
		if !strings.EqualFold(n.Attr[i].Key, attrName) {
			continue
		}
		if isRemoteURL(n.Attr[i].Val) {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
		}
	}
}

// isRemoteURL returns true if v points at a resource Chromium would
// fetch over the network. Conservative: anything not obviously a
// safe data/cid URL is treated as remote, including relative paths
// (which Chromium would resolve against the document base — pandoc's
// standalone HTML has no useful base, so stripping these costs nothing).
func isRemoteURL(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	low := strings.ToLower(v)
	if strings.HasPrefix(low, "data:") || strings.HasPrefix(low, "cid:") {
		return false
	}
	// Pure fragments (#section) are intra-document and safe.
	if strings.HasPrefix(low, "#") {
		return false
	}
	return true
}
