package utils

import "math"

// Pagination represents the pagination details.
type Pagination struct {
	TotalItems  int `json:"totalItems"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalPages  int `json:"totalPages"`
}

// CreatePagination creates a Pagination object.
func CreatePagination(totalItems, page, pageSize int) *Pagination {
	if pageSize <= 0 {
		pageSize = 10 // Default page size
	}
	if page <= 0 {
		page = 1 // Default page
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	return &Pagination{
		TotalItems:  totalItems,
		CurrentPage: page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
	}
}
