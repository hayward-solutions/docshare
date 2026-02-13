package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docshare/cli/internal/api"
)

// JSON prints v as indented JSON to stdout.
func JSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// FileTable prints a slice of files as a human-readable table.
func FileTable(files []api.File) {
	if len(files) == 0 {
		fmt.Println("No files found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tTYPE\tSHARED\tMODIFIED")

	for _, f := range files {
		name := f.Name
		if f.IsDirectory {
			name += "/"
		}

		size := FormatSize(f.Size)
		if f.IsDirectory {
			size = "-"
		}

		kind := shortMIME(f.MimeType)
		if f.IsDirectory {
			kind = "dir"
		}

		shared := "-"
		if f.SharedWith > 0 {
			shared = fmt.Sprintf("%d", f.SharedWith)
		}

		modified := RelativeTime(f.UpdatedAt)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, size, kind, shared, modified)
	}
	w.Flush()
}

// FileDetail prints a single file's details.
func FileDetail(f api.File) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Name:\t%s\n", f.Name)
	fmt.Fprintf(w, "ID:\t%s\n", f.ID)
	fmt.Fprintf(w, "Type:\t%s\n", f.MimeType)
	if !f.IsDirectory {
		fmt.Fprintf(w, "Size:\t%s\n", FormatSize(f.Size))
	}
	fmt.Fprintf(w, "Directory:\t%v\n", f.IsDirectory)
	if f.ParentID != nil {
		fmt.Fprintf(w, "Parent ID:\t%s\n", *f.ParentID)
	}
	fmt.Fprintf(w, "Owner:\t%s\n", f.OwnerID)
	if f.Owner != nil {
		fmt.Fprintf(w, "Owner Email:\t%s\n", f.Owner.Email)
	}
	if f.SharedWith > 0 {
		fmt.Fprintf(w, "Shared With:\t%d user(s)\n", f.SharedWith)
	}
	fmt.Fprintf(w, "Created:\t%s\n", f.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Modified:\t%s\n", f.UpdatedAt.Format(time.RFC3339))
	w.Flush()
}

// ShareTable prints a slice of shares.
func ShareTable(shares []api.Share) {
	if len(shares) == 0 {
		fmt.Println("No shares found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tSHARED BY\tPERMISSION\tCREATED")
	for _, s := range shares {
		name := s.FileID
		if s.File != nil {
			name = s.File.Name
		}
		by := s.SharedByID
		if s.SharedBy != nil {
			by = s.SharedBy.Email
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, by, s.Permission, RelativeTime(s.CreatedAt))
	}
	w.Flush()
}

// UserInfo prints user details.
func UserInfo(u api.User) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Email:\t%s\n", u.Email)
	fmt.Fprintf(w, "Name:\t%s %s\n", u.FirstName, u.LastName)
	fmt.Fprintf(w, "Role:\t%s\n", u.Role)
	fmt.Fprintf(w, "ID:\t%s\n", u.ID)
	w.Flush()
}

// FormatSize converts bytes to a human-readable string.
func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// RelativeTime formats a timestamp relative to now (e.g. "2h ago", "3d ago").
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func shortMIME(mime string) string {
	// "application/pdf" -> "pdf", "image/png" -> "png"
	parts := strings.Split(mime, "/")
	if len(parts) == 2 {
		s := parts[1]
		// strip "vnd.openxmlformats..." prefixes
		if idx := strings.LastIndex(s, "."); idx >= 0 {
			s = s[idx+1:]
		}
		return s
	}
	return mime
}
