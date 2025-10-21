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

// Merchant represents a business entity using the service.
type Merchant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
    ShopName  *string   `json:"shop_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Shop represents a single retail location owned by a merchant.
type Shop struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MerchantID string    `json:"merchant_id"`
	Address    *string   `json:"address,omitempty"`
	Phone      *string   `json:"phone,omitempty"`
	IsActive   bool      `json:"is_active"`
	IsPrimary  bool      `json:"is_primary"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Supplier provides items to merchants.
type Supplier struct {
	ID           string    `json:"id"`
	MerchantID   string    `json:"merchant_id"`
	Name         string    `json:"name"`
	ContactName  *string   `json:"contact_name,omitempty"`
	ContactEmail *string   `json:"contact_email,omitempty"`
	ContactPhone *string   `json:"contact_phone,omitempty"`
	Address      *string   `json:"address,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// InventoryItem represents an item in the master inventory of a merchant.
type InventoryItem struct {
	ID                string    `json:"id"`
	MerchantID        string    `json:"merchant_id"`
	Name              string    `json:"name"`
	Description       *string   `json:"description,omitempty"`
	SKU               *string   `json:"sku,omitempty"`
	SellingPrice      float64   `json:"selling_price"`
	OriginalPrice     *float64  `json:"original_price,omitempty"`
	LowStockThreshold *int      `json:"low_stock_threshold,omitempty"`
	Category          *string   `json:"category,omitempty"`
	SupplierID        *string   `json:"supplier_id,omitempty"`
	IsArchived        bool      `json:"is_archived"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// InventoryItemWithQuantity extends InventoryItem with shop-specific stock quantity.
type InventoryItemWithQuantity struct {
	InventoryItem
	Quantity int `json:"quantity"`
}


// ShopStock represents the quantity of an inventory item at a specific shop.
type ShopStock struct {
	ID              string    `json:"id"`
	ShopID          string    `json:"shop_id"`
	InventoryItemID string    `json:"inventory_item_id"`
	Quantity        int       `json:"quantity"`
	LastStockedInAt time.Time `json:"last_stocked_in_at"`
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
	ShopID     string    `json:"shop_id"`
	MerchantID string    `json:"merchant_id"`
	Name       string    `json:"name"`
	Email      *string   `json:"email,omitempty"`
	Phone      *string   `json:"phone,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Promotion represents a discount or offer.
type Promotion struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	ShopID      *string   `json:"shop_id,omitempty"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	PromoType   string    `json:"promo_type"`
	PromoValue  float64   `json:"promo_value"`
	MinSpend    float64   `json:"min_spend"`
	Conditions  JSONB     `json:"conditions,omitempty"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Sale represents a single transaction.
type Sale struct {
	ID                    string     `json:"id"`
	ShopID                string     `json:"shop_id"`
	MerchantID            string     `json:"merchant_id"`
	SaleDate              time.Time  `json:"sale_date"`
	TotalAmount           float64    `json:"total_amount"`
	AppliedPromotionID    *string    `json:"applied_promotion_id,omitempty"`
	DiscountAmount        *float64   `json:"discount_amount,omitempty"`
	PaymentType           string     `json:"payment_type"`
	PaymentStatus         string     `json:"payment_status"`
	StripePaymentIntentID *string    `json:"stripe_payment_intent_id,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Items                 []SaleItem `json:"items,omitempty"`
}

// SaleItem is an individual item within a Sale.
type SaleItem struct {
	ID                  string    `json:"id"`
	SaleID              string    `json:"sale_id"`
	InventoryItemID     string    `json:"inventory_item_id"`
	QuantitySold        int       `json:"quantity_sold"`
	SellingPriceAtSale  float64   `json:"selling_price_at_sale"`
	OriginalPriceAtSale *float64  `json:"original_price_at_sale,omitempty"`
	Subtotal            float64   `json:"subtotal"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	ItemName            *string   `json:"item_name,omitempty"`
	ItemSKU             *string   `json:"item_sku,omitempty"`
}

// Salary represents a salary payment to a staff member.
type Salary struct {
	ID          string    `json:"id"`
	StaffID     string    `json:"staff_id"`
	Amount      float64   `json:"amount"`
	PaymentDate time.Time `json:"payment_date"`
	Notes       *string   `json:"notes,omitempty"`
}

// Notification for a user.
type Notification struct {
	ID                string    `json:"id"`
	RecipientUserID   string    `json:"recipient_user_id"`
	Title             string    `json:"title"`
	Message           string    `json:"message"`
	Type              string    `json:"type"`
	RelatedEntityID   *string   `json:"related_entity_id,omitempty"`
	RelatedEntityType *string   `json:"related_entity_type,omitempty"`
	IsRead            bool      `json:"is_read"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// --- API Request/Response Structs ---

// CreateUserRequest defines the body for creating a new user.
type CreateUserRequest struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Role       string  `json:"role"`
	MerchantID *string `json:"merchant_id,omitempty"`
}

// CreateShopRequest defines the body for creating a new shop.
type CreateShopRequest struct {
	Name       string  `json:"name"`
	MerchantID string  `json:"merchant_id"`
	Address    *string `json:"address,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	IsActive   bool    `json:"is_active"`
	IsPrimary  bool    `json:"is_primary"`
}

// AdminDashboardSummary defines the structure for the admin dashboard summary.
type AdminDashboardSummary struct {
	TotalActiveMerchants int `json:"total_active_merchants"`
	TotalActiveStaff     int `json:"total_active_staff"`
	TotalActiveShops     int `json:"total_active_shops"`
	TotalProductsListed  int `json:"total_products_listed"`
}


// KpiData represents a single Key Performance Indicator.
type KpiData struct {
	Value float64 `json:"value"`
}

// ProductSummary represents a summary of a single product's performance.
type ProductSummary struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	QuantitySold int     `json:"quantity_sold"`
	Revenue     float64 `json:"revenue"`
}

// MerchantDashboardSummary defines the structure for the merchant dashboard summary.
type MerchantDashboardSummary struct {
	TotalSalesRevenue    KpiData          `json:"total_sales_revenue"`
	NumberOfTransactions KpiData          `json:"number_of_transactions"`
	AverageOrderValue    KpiData          `json:"average_order_value"`
	TopSellingProducts   []ProductSummary `json:"top_selling_products"`
}

// CombinedStockItem represents a flattened view of an inventory item in a specific shop.
type CombinedStockItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SKU           *string  `json:"sku,omitempty"`
	Quantity      int      `json:"quantity"`
	SellingPrice  float64  `json:"sellingPrice"`
	OriginalPrice *float64 `json:"originalPrice,omitempty"`
	ShopName      string   `json:"shopName"`
	ShopID        string   `json:"shopId"`
}

// --- Paginated Responses ---

// Pagination details for paginated responses.
type Pagination struct {
	TotalItems  int `json:"total_items"`
	TotalPages  int `json:"total_pages"`
	CurrentPage int `json:"current_page"`
	PageSize    int `json:"page_size"`
}

// PaginatedUsersResponse is the generic structure for paginated users.
type PaginatedUsersResponse struct {
	Data       []User     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// PaginatedAdminShopsResponse is the structure for the GET /api/admin/shops endpoint.
type PaginatedAdminShopsResponse struct {
	Data       []Shop     `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// PaginatedAdminMerchantsResponse is the structure for the GET /api/admin/merchants endpoint.
type PaginatedAdminMerchantsResponse struct {
	Data       []Merchant `json:"data"`
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
	Data struct {
		Items []Sale     `json:"items"`
		Meta  Pagination `json:"meta"`
	} `json:"data"`
}

// PaginatedPromotionsResponse for promotions.
type PaginatedPromotionsResponse struct {
	Items      []Promotion `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

// PaginatedSuppliersResponse for suppliers.
type PaginatedSuppliersResponse struct {
	Data       []Supplier `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// PaginatedInventoryResponse for inventory items.
type PaginatedInventoryResponse struct {
	Items      []InventoryItem `json:"items"`
	Pagination Pagination      `json:"pagination"`
}

// PaginatedNotificationsResponse for notifications.
type PaginatedNotificationsResponse struct {
	Data       []Notification `json:"data"`
	Pagination Pagination     `json:"meta"`
}

// UserSelectionItem for dropdowns
type UserSelectionItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
