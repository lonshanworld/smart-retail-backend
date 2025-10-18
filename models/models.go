package models

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// --- Structs ---

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	UserType string `json:"userType"`
}

type ShopLoginRequest struct {
	ShopID   string `json:"shopId"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	MerchantID *int   `json:"merchantId,omitempty"`
}

// CreateShopRequest defines the body for creating a new shop.
type CreateShopRequest struct {
	Name string `json:"name"`
}

type AdminDashboardSummary struct {
	TotalMerchants    int     `json:"totalMerchants"`
	ActiveMerchants   int     `json:"activeMerchants"`
	TotalStaff        int     `json:"totalStaff"`
	ActiveStaff       int     `json:"activeStaff"`
	TotalShops        int     `json:"totalShops"`
	TotalSalesValue   float64 `json:"totalSalesValue"`
	SalesToday        float64 `json:"salesToday"`
	TransactionsToday int     `json:"transactionsToday"`
}

// Pagination details for paginated responses.
type Pagination struct {
	TotalItems  int `json:"totalItems"`
	TotalPages  int `json:"totalPages"`
	CurrentPage int `json:"currentPage"`
}

// PaginatedAdminsResponse is the structure for the GET /api/admin/admins endpoint.
type PaginatedAdminsResponse struct {
	Data       []User     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type AdminPaginatedUsersResponse struct {
	Users       []User `json:"users"`
	CurrentPage int    `json:"current_page"`
	TotalPages  int    `json:"total_pages"`
	PageSize    int    `json:"page_size"`
	TotalCount  int    `json:"total_count"`
}

type User struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	IsActive       bool      `json:"is_active"`
	Phone          *string   `json:"phone,omitempty"`
	AssignedShopID *string   `json:"assigned_shop_id,omitempty"`
	MerchantID     *string   `json:"merchant_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Shop           *Shop     `json:"Shop,omitempty"`
}

type Shop struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MerchantID string    `json:"merchantId"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type JwtClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
