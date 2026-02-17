package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func parsePaginationForTest(t *testing.T, query string) PaginationParams {
	t.Helper()

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		params := ParsePagination(c)
		return c.JSON(fiber.Map{
			"page":   params.Page,
			"limit":  params.Limit,
			"offset": params.Offset,
		})
	})

	path := "/"
	if query != "" {
		path = fmt.Sprintf("/?%s", query)
	}

	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("pagination request failed for query %q: %v", query, err)
	}
	defer resp.Body.Close()

	var parsed PaginationParams
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("failed to decode pagination response for query %q: %v", query, err)
	}

	return parsed
}

func TestParsePagination(t *testing.T) {
	testCases := []struct {
		name       string
		query      string
		wantPage   int
		wantLimit  int
		wantOffset int
	}{
		{name: "defaults when no query params", query: "", wantPage: 1, wantLimit: 20, wantOffset: 0},
		{name: "uses explicit page and limit", query: "page=2&limit=10", wantPage: 2, wantLimit: 10, wantOffset: 10},
		{name: "normalizes page less than one", query: "page=0&limit=10", wantPage: 1, wantLimit: 10, wantOffset: 0},
		{name: "normalizes invalid page string", query: "page=abc&limit=10", wantPage: 1, wantLimit: 10, wantOffset: 0},
		{name: "normalizes limit less than one", query: "page=3&limit=0", wantPage: 3, wantLimit: 20, wantOffset: 40},
		{name: "caps limit above maximum", query: "page=1&limit=500", wantPage: 1, wantLimit: 100, wantOffset: 0},
		{name: "normalizes invalid limit string", query: "page=4&limit=abc", wantPage: 4, wantLimit: 20, wantOffset: 60},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePaginationForTest(t, tc.query)

			if got.Page != tc.wantPage {
				t.Fatalf("expected page=%d, got %d", tc.wantPage, got.Page)
			}
			if got.Limit != tc.wantLimit {
				t.Fatalf("expected limit=%d, got %d", tc.wantLimit, got.Limit)
			}
			if got.Offset != tc.wantOffset {
				t.Fatalf("expected offset=%d, got %d", tc.wantOffset, got.Offset)
			}
		})
	}
}

func TestApplyPagination(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 user=test password=test dbname=test port=5432 sslmode=disable",
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to create dry-run gorm db: %v", err)
	}

	params := PaginationParams{Page: 3, Limit: 15, Offset: 30}
	paginatedDB := ApplyPagination(db.Table("users"), params)

	if paginatedDB == nil {
		t.Fatal("expected ApplyPagination to return non-nil *gorm.DB")
	}
	if paginatedDB.Statement == nil {
		t.Fatal("expected paginated db to have a statement")
	}

	limitClause, ok := paginatedDB.Statement.Clauses["LIMIT"]
	if !ok {
		t.Fatal("expected LIMIT clause to be set by ApplyPagination")
	}

	limitExpr, ok := limitClause.Expression.(clause.Limit)
	if !ok {
		t.Fatalf("expected LIMIT clause expression type %T, got %T", clause.Limit{}, limitClause.Expression)
	}
	if limitExpr.Limit == nil {
		t.Fatal("expected LIMIT value to be set")
	}
	if *limitExpr.Limit != params.Limit {
		t.Fatalf("expected limit=%d, got %d", params.Limit, *limitExpr.Limit)
	}
	if limitExpr.Offset != params.Offset {
		t.Fatalf("expected offset=%d, got %d", params.Offset, limitExpr.Offset)
	}
}
