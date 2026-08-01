package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func itoa(value int) string { return strconv.Itoa(value) }

type catalogListQuery struct {
	Page      int
	PageSize  int
	Offset    int
	Search    string
	SortField string
	SortOrder string
}

func getCatalogListQuery(c *fiber.Ctx, defaultSort string, allowed map[string]string) catalogListQuery {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	sortField := c.Query("sort", defaultSort)
	if _, ok := allowed[sortField]; !ok {
		sortField = defaultSort
	}
	sortOrder := strings.ToUpper(c.Query("order", "ASC"))
	if sortOrder != "DESC" {
		sortOrder = "ASC"
	}
	return catalogListQuery{
		Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize,
		Search: strings.TrimSpace(c.Query("search")), SortField: allowed[sortField], SortOrder: sortOrder,
	}
}

func (q catalogListQuery) orderBy() string {
	return fmt.Sprintf(" ORDER BY %s %s, id ASC", q.SortField, q.SortOrder)
}

func paginatedResponse(data interface{}, total int64, q catalogListQuery) fiber.Map {
	pages := 0
	if total > 0 {
		pages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return fiber.Map{
		"status": "success", "success": true, "data": data,
		"pagination": fiber.Map{"totalItems": total, "totalPages": pages, "currentPage": q.Page, "pageSize": q.PageSize},
	}
}
