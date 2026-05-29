package handlers

import "testing"

// TestResolveMimeType_PrefersExtensionOverOctetStream covers the CLI upload
// regression: Go's mime/multipart writes parts with Content-Type
// "application/octet-stream" by default, so without the extension fallback
// a .jpg uploaded via the CLI was stored as octet-stream and skipped
// downstream gates (image-thumbnail enqueue, viewer routing).
func TestResolveMimeType_PrefersExtensionOverOctetStream(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		declared string
		wantPrefix string
	}{
		{"jpg upload via CLI", "photo.jpg", "application/octet-stream", "image/jpeg"},
		{"png upload via CLI", "shot.png", "application/octet-stream", "image/png"},
		{"pdf upload via CLI", "doc.pdf", "application/octet-stream", "application/pdf"},
		{"empty declared honors extension", "photo.gif", "", "image/gif"},
		{"web upload honors specific declared type", "photo.jpg", "image/jpeg", "image/jpeg"},
		// Authoritative non-octet declared types win even when extension would
		// disagree — the client knows better than the extension here.
		{"explicit declared wins over extension mismatch", "photo.jpg", "image/png", "image/png"},
		// Unknown extension with octet-stream stays octet-stream.
		{"unknown extension falls back to declared", "blob.xyz123", "application/octet-stream", "application/octet-stream"},
		// Custom override branch still wins (defined in resolveMimeType).
		{".md special case", "notes.md", "application/octet-stream", "text/markdown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveMimeType(tc.filename, tc.declared)
			if got != tc.wantPrefix && !startsWith(got, tc.wantPrefix+";") {
				t.Errorf("resolveMimeType(%q, %q) = %q, want %q", tc.filename, tc.declared, got, tc.wantPrefix)
			}
		})
	}
}

// startsWith handles mime.TypeByExtension returning e.g. "text/markdown; charset=utf-8".
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
