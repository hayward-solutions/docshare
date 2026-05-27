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

// compareNatural compares two strings byte-by-byte, but treats consecutive
// digit runs as numbers so that "file2" sorts before "file10". Leading zeros
// don't change numeric value but break ties (so "file2" < "file02").
func compareNatural(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ai, bj := a[i], b[j]
		aDigit := ai >= '0' && ai <= '9'
		bDigit := bj >= '0' && bj <= '9'
		if aDigit && bDigit {
			iEnd := i
			for iEnd < len(a) && a[iEnd] >= '0' && a[iEnd] <= '9' {
				iEnd++
			}
			jEnd := j
			for jEnd < len(b) && b[jEnd] >= '0' && b[jEnd] <= '9' {
				jEnd++
			}
			aTrim := strings.TrimLeft(a[i:iEnd], "0")
			bTrim := strings.TrimLeft(b[j:jEnd], "0")
			if len(aTrim) != len(bTrim) {
				if len(aTrim) < len(bTrim) {
					return -1
				}
				return 1
			}
			if c := strings.Compare(aTrim, bTrim); c != 0 {
				return c
			}
			// Numerically equal — fall back to the raw run length so
			// "01" and "1" are still distinguishable in a stable way.
			if (iEnd - i) != (jEnd - j) {
				if (iEnd - i) < (jEnd - j) {
					return -1
				}
				return 1
			}
			i, j = iEnd, jEnd
			continue
		}
		if ai != bj {
			if ai < bj {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case i < len(a):
		return 1
	case j < len(b):
		return -1
	}
	return 0
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
			cmp = compareNatural(a.Name, b.Name)
		}
		if !asc {
			cmp = -cmp
		}
		// Tiebreaker is always name ASC, matching the SQLClause fragment.
		if cmp == 0 {
			cmp = compareNatural(a.Name, b.Name)
		}
		return cmp < 0
	})
}
