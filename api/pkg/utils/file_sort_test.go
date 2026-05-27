package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/docshare/api/internal/models"
	"github.com/gofiber/fiber/v2"
)

func parseFileSortForTest(t *testing.T, query string) FileSort {
	t.Helper()

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		s := ParseFileSort(c)
		return c.JSON(fiber.Map{"column": s.Column, "direction": s.Direction})
	})

	path := "/"
	if query != "" {
		path = fmt.Sprintf("/?%s", query)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var parsed FileSort
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	return parsed
}

func TestParseFileSort(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantCol   string
		wantOrder string
	}{
		{"default", "", "name", "ASC"},
		{"name asc", "sort=name&order=asc", "name", "ASC"},
		{"name desc", "sort=name&order=desc", "name", "DESC"},
		{"size", "sort=size", "size", "ASC"},
		{"size desc", "sort=size&order=DESC", "size", "DESC"},
		{"modified", "sort=modified", "updated_at", "ASC"},
		{"modified desc", "sort=modified&order=desc", "updated_at", "DESC"},
		{"unknown column", "sort=bogus", "name", "ASC"},
		{"unknown order", "sort=size&order=sideways", "size", "ASC"},
		{"mixed case", "sort=Modified&order=Desc", "updated_at", "DESC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseFileSortForTest(t, tc.query)
			if got.Column != tc.wantCol || got.Direction != tc.wantOrder {
				t.Fatalf("got %+v, want column=%s direction=%s", got, tc.wantCol, tc.wantOrder)
			}
		})
	}
}

func TestFileSortSQLClause(t *testing.T) {
	got := FileSort{Column: "size", Direction: "DESC"}.SQLClause()
	want := "is_directory DESC, size DESC, name ASC"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSortFiles(t *testing.T) {
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now()
	files := []models.File{
		{Name: "zeta.txt", Size: 100, IsDirectory: false},
		{Name: "alpha.txt", Size: 50, IsDirectory: false},
		{Name: "B-folder", Size: 0, IsDirectory: true},
		{Name: "a-folder", Size: 0, IsDirectory: true},
	}
	files[0].UpdatedAt = newer
	files[1].UpdatedAt = older
	files[2].UpdatedAt = newer
	files[3].UpdatedAt = older

	t.Run("name asc keeps folders first", func(t *testing.T) {
		got := append([]models.File(nil), files...)
		FileSort{Column: "name", Direction: "ASC"}.SortFiles(got)
		want := []string{"B-folder", "a-folder", "alpha.txt", "zeta.txt"}
		for i, f := range got {
			if f.Name != want[i] {
				t.Fatalf("pos %d: got %s, want %s (full=%+v)", i, f.Name, want[i], got)
			}
		}
	})

	t.Run("size desc among files", func(t *testing.T) {
		got := append([]models.File(nil), files...)
		FileSort{Column: "size", Direction: "DESC"}.SortFiles(got)
		// folders still first (both size 0, broken by name asc), then files by size desc
		want := []string{"B-folder", "a-folder", "zeta.txt", "alpha.txt"}
		for i, f := range got {
			if f.Name != want[i] {
				t.Fatalf("pos %d: got %s, want %s", i, f.Name, want[i])
			}
		}
	})

	t.Run("modified asc", func(t *testing.T) {
		got := append([]models.File(nil), files...)
		FileSort{Column: "updated_at", Direction: "ASC"}.SortFiles(got)
		// folders first ordered by updated_at asc (a-folder older), then files asc
		want := []string{"a-folder", "B-folder", "alpha.txt", "zeta.txt"}
		for i, f := range got {
			if f.Name != want[i] {
				t.Fatalf("pos %d: got %s, want %s", i, f.Name, want[i])
			}
		}
	})
}
