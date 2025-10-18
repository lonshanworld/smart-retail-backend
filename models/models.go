package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// --- Custom JSON Type for database/sql ---

// JSONB allows storing JSON data in a PostgreSQL jsonb column.
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, &j)
}

// --- JWT & Auth ---

type JwtClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

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

// --- Core Models ---

// User represents a user in the system (Admin, Merchant, or Staff)
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

// Shop represents a single retail location owned by a merchant.
type Shop struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MerchantID string    `json:"merchantId"`
	Address    *string   `json:"address,omitempty"`
	Phone      *string   `json:"phone,omitempty"`
	IsActive   bool      `json:"isActive"`
	IsPrimary  bool      `json:"isPrimary"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Supplier provides items to merchants.
type Supplier struct {
	ID           string    `json:"id"`
	MerchantID   string    `json:"merchantId"`
	Name         string    `json:"name"`
	ContactName  *string   `json:"contactName,omitempty"`
	ContactEmail *string   `json:"contactEmail,omitempty"`
	ContactPhone *string   `json:"contactPhone,omitempty"`
	Address      *string   `json:"address,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// InventoryItem represents an item in the master inventory of a merchant.
type InventoryItem struct {
	ID                string    `json:"id"`
	MerchantID        string    `json:"merchantId"`
	Name              string    `json:"name"`
	Description       *string   `json:"description,omitempty"`
	SKU               *string   `json:"sku,omitempty"`
	SellingPrice      float64   `json:"sellingPrice"`
	OriginalPrice     *float64  `json:"originalPrice,omitempty"`
	LowStockThreshold *int      `json:"lowStockThreshold,omitempty"`
	Category          *string   `json:"category,omitempty"`
	SupplierID        *string   `json:"supplierId,omitempty"`
	IsArchived        bool      `json:"isArchived"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ShopStock represents the quantity of an inventory item at a specific shop.
type ShopStock struct {
	ID              string    `json:"id"`
	ShopID          string    `json:"shopId"`
	InventoryItemID string    `json:"inventoryItemId"`
	Quantity        int       `json:"quantity"`
	LastStockedInAt time.Time `json:"lastStockedInAt"`
}

// StockMovement logs any change in stock quantity.
type StockMovement struct {
	ID              string    `json:"id"`
	InventoryItemID string    `json:"inventory_item_id"`
	ShopID          string    `json:"shop_id"`
	UserID          string    `json:"user_id"`
	MovementType    string    `json:"movement_type"`
	QuantityChanged int       `json:"quantity_changed"`
	NewQuantity     int       `json:"new_quantity"`
	Reason          *string   `json:"reason,omitempty"`
	MovementDate    time.Time `json:"movement_date"`
	Notes           *string   `json:"notes,omitempty"`
}

// ShopCustomer represents a customer associated with a specific shop.
type ShopCustomer struct {
	ID         string    `json:"id"`
	ShopID     string    `json:"shopId"`
	MerchantID string    `json:"merchantId"`
	Name       string    `json:"name"`
	Email      *string   `json:"email,omitempty"`
	Phone      *string   `json:⚫`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Promotion represents a discount or offer.
type Promotion struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchantId"`
	ShopID      *string   `json:"shopId,omitempty"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Type        string    `json:"type"`
	Value       float64   `json:"value"`
	MinSpend    float64   `json:"minSpend"`
	Conditions  JSONB     `json:"conditions,omitempty"`
	StartDate   time.Time `json:"startDate"`
	EndDate     time.Time `json:"endDate"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Sale represents a single transaction.
type Sale struct {
	ID                   string     `json:"id"`
	ShopID               string     `json:"shopId"`
	MerchantID           string     `json:"merchantId"`
	SaleDate             time.Time  `json:"saleDate"`
	TotalAmount          float64    `json:"totalAmount"`
	AppliedPromotionID   *string    `json:"appliedPromotionId,omitempty"`
	DiscountAmount       *float64   `json:"discountAmount,omitempty"`
	PaymentType          string     `json:"paymentType"`
	PaymentStatus        string     `json:"paymentStatus"`
	StripePaymentIntentID *string    `json:"stripePaymentIntentId,omitempty"`
	Notes                *string    `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	Items                []SaleItem `json:"items,omitempty"`
}

// SaleItem is an individual item within a Sale.
type SaleItem struct {
	ID                  string    `json:"id"`
	SaleID              string    `json:"saleId"`
	InventoryItemID     string    `json:"inventoryItemId"`
	QuantitySold        int       `json:"quantitySold"`
	SellingPriceAtSale  float64   `json:"sellingPriceAtSale"`
	OriginalPriceAtSale *float64  `json:"originalPriceAtSale,omitempty"`
	Subtotal            float64   `json:"subtotal"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	ItemName            *string   `json:"itemName,omitempty"`
	ItemSKU             *string   `json:"itemSku,omitempty"`
}

// Salary represents a salary payment to a staff member.
type Salary struct {
	ID           string    `json:"id"`
	StaffID      string    `json:"staffId"`
	Amount       float64   `json:"amount"`
	PaymentDate  time.Time `json:"paymentDate"`
	Notes        *string   `json:"notes,omitempty"`
}

// Notification for a user.
type Notification struct {
	ID                string    `json:"id"`
	RecipientUserID   string    `json:"recipientUserId"`
	Title             string    `json:"title"`
	Message           string    `json:"message"`
	Type              string    `json:"type"`
	RelatedEntityID   *string   `json:"relatedEntityId,omitempty"`
	RelatedEntityType *string   `json:"relatedEntityType,omitempty"`
	IsRead            bool      `json:"isRead"`
	CreatedAt         time.Time `json:"createdAt"`
}


// --- API Request/Response Structs ---

// CreateUserRequest defines the body for creating a new user.
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

// AdminDashboardSummary holds metrics for the admin dashboard.
type AdminDashboardSummary struct {
	TotalMerchants    int     `json:"totalMerchants"`
	ActiveMerchants   int     `json:⚫`
	TotalStaff        int     `json:"totalStaff"`
	ActiveStaff       int     `json:"activeStaff"`
	TotalShops        int     `json:"totalShops"`
	TotalSalesValue   float64 `json:"totalSalesValue"`
	SalesToday        float64 `json:"salesToday"`
	TransactionsToday int     `json:"transactionsToday"`
}


// --- Paginated Responses ---

// Pagination details for paginated responses.
type Pagination struct {
	TotalItems  int `json:"totalItems"`
	TotalPages  int `json:"totalPages"`
	CurrentPage int `json:"currentPage"`
    PageSize    int `json:"pageSize"`
}

// PaginatedAdminsResponse is the structure for the GET /api/admin/admins endpoint.
type PaginatedAdminsResponse struct {
	Data       []User     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// AdminPaginatedUsersResponse is the old structure for paginated users.
type AdminPaginatedUsersResponse struct {
	Users       []User `json:"users"`
	CurrentPage int    `json:"current_page"`
	TotalPages  int    `json:"total_pages"`
	PageSize    int    `json:"page_size"`
	TotalCount  int    `json:"total_count"`
}

// PaginatedSalesResponse for sales history.
type PaginatedSalesResponse struct {
	Data       []Sale     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// PaginatedPromotionsResponse for promotions.
type PaginatedPromotionsResponse struct {
	Items      []Promotion `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

// PaginatedSuppliersResponse for suppliers.
type PaginatedSuppliersResponse struct {
	Data       []Supplier `json:"data"`
	Pagination Pagination   `json:"pagination"`
}

// PaginatedInventoryResponse for inventory items.
type PaginatedInventoryResponse struct {
	Items      []InventoryItem `json:"items"`
	Pagination Pagination      `json:"pagination"`
}

// PaginatedNotificationsResponse for notifications.
type PaginatedNotificationsResponse struct {
	Data       []Notification `json:"data"`
	Pagination Pagination       `json:"meta"`
}
