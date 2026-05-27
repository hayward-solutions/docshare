package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/docshare/api/internal/models"
	"github.com/gofiber/fiber/v2"
)

// FileSort describes how a file/folder listing should be ordered.
// Column is a whitelisted SQL identifier; Direction is "ASC" or "DESC".
type FileSort struct {
	Column    string
	Direction string
}

// ParseFileSort reads the "sort" and "order" query params and returns a
// validated FileSort. Unknown values fall back to name ASC.
func ParseFileSort(c *fiber.Ctx) FileSort {
	column := "name"
	switch strings.ToLower(c.Query("sort")) {
	case "size":
		column = "size"
	case "modified":
		column = "updated_at"
	}
	direction := "ASC"
	if strings.EqualFold(c.Query("order"), "desc") {
		direction = "DESC"
	}
	return FileSort{Column: column, Direction: direction}
}

// SQLClause returns an ORDER BY fragment safe to pass to gorm.Order.
// Folders always come before files; the chosen column follows in the
// requested direction; name is the deterministic tiebreaker.
func (f FileSort) SQLClause() string {
	return fmt.Sprintf("is_directory DESC, %s %s, name ASC", f.Column, f.Direction)
}

// SortFiles orders the slice in place to match SQLClause semantics.
// Use this when results have been merged in memory and cannot be ordered
// in a single SQL statement.
func (f FileSort) SortFiles(files []models.File) {
	asc := f.Direction != "DESC"
	sort.SliceStable(files, func(i, j int) bool {
		a, b := files[i], files[j]
		if a.IsDirectory != b.IsDirectory {
			return a.IsDirectory
		}
		cmp := 0
		switch f.Column {
		case "size":
			switch {
			case a.Size < b.Size:
				cmp = -1
			case a.Size > b.Size:
				cmp = 1
			}
		case "updated_at":
			switch {
			case a.UpdatedAt.Before(b.UpdatedAt):
				cmp = -1
			case a.UpdatedAt.After(b.UpdatedAt):
				cmp = 1
			}
		default:
			cmp = strings.Compare(a.Name, b.Name)
		}
		if !asc {
			cmp = -cmp
		}
		// Tiebreaker is always name ASC, matching the SQLClause fragment.
		if cmp == 0 {
			cmp = strings.Compare(a.Name, b.Name)
		}
		return cmp < 0
	})
}
