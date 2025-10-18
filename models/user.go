package models

// PaginatedAdminUsersResponse is the response structure for paginated admin users.
type PaginatedAdminUsersResponse struct {
	Data       []User         `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo holds metadata for paginated responses.
type PaginationInfo struct {
	TotalItems  int `json:"totalItems"`
	TotalPages  int `json:"totalPages"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
}

// UserSelectionItem represents a user in a format suitable for selection lists (e.g., dropdowns).
type UserSelectionItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
