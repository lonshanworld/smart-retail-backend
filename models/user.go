package models

import "time"

// User represents a user in the system (Admin, Merchant, or Staff).
type User struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	IsActive       bool      `json:"isActive"`
	Phone          *string   `json:"phone,omitempty"`
	AssignedShopID *string   `json:"assignedShopId,omitempty"`
	MerchantID     *string   `json:"merchantId,omitempty"`
    MerchantName   *string   `json:"merchantName,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// CreateUserRequest defines the shape of a request to create a new user.
type CreateUserRequest struct {
	Name           string  `json:"name"`
	Email          string  `json:"email"`
	Password       string  `json:"password"`
	Role           string  `json:"role"`
	IsActive       bool    `json:"is_active"`
	Phone          *string `json:"phone,omitempty"`
	AssignedShopID *string `json:"assigned_shop_id,omitempty"`
	MerchantID     *string `json:"merchant_id,omitempty"`
}

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
