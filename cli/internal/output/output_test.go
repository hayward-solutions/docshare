package output

import (
	"testing"
	"time"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatSize(tt.input)
			if got != tt.want {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	t.Run("just now", func(t *testing.T) {
		got := RelativeTime(time.Now())
		if got != "just now" {
			t.Errorf("expected 'just now', got %q", got)
		}
	})

	t.Run("minutes ago", func(t *testing.T) {
		got := RelativeTime(time.Now().Add(-5 * time.Minute))
		if got != "5m ago" {
			t.Errorf("expected '5m ago', got %q", got)
		}
	})

	t.Run("hours ago", func(t *testing.T) {
		got := RelativeTime(time.Now().Add(-3 * time.Hour))
		if got != "3h ago" {
			t.Errorf("expected '3h ago', got %q", got)
		}
	})

	t.Run("days ago", func(t *testing.T) {
		got := RelativeTime(time.Now().Add(-7 * 24 * time.Hour))
		if got != "7d ago" {
			t.Errorf("expected '7d ago', got %q", got)
		}
	})

	t.Run("date format for old timestamps", func(t *testing.T) {
		old := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		got := RelativeTime(old)
		if got != "2024-01-15" {
			t.Errorf("expected date format, got %q", got)
		}
	})
}

func TestShortMIME(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"application/pdf", "pdf"},
		{"image/png", "png"},
		{"text/plain", "plain"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "sheet"},
		{"inode/directory", "directory"},
		{"plaintext", "plaintext"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shortMIME(tt.input)
			if got != tt.want {
				t.Errorf("shortMIME(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
