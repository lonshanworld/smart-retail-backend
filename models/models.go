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
	Email    string  `json:"email"`
	Password string  `json:"password"`
	UserType string  `json:"userType"`
	ShopID   *string `json:"shopId,omitempty"` // Optional: for merchant shop verification
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
	Password       string    `json:"password,omitempty"` // Used only for creating users
	PasswordHash   string    `json:"-"`                  // Stored in DB, never sent to client
	Role           string    `json:"role"`
	IsActive       bool      `json:"is_active"`
	Phone          *string   `json:"phone,omitempty"`
	AssignedShopID *string   `json:"assigned_shop_id,omitempty"`
	MerchantID     *string   `json:"merchant_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Shop           *Shop     `json:"shop,omitempty"`

	// Fields for staff profile
	ShopName     *string  `json:"shopName,omitempty"`
	Salary       *float64 `json:"salary,omitempty"`
	PayFrequency *string  `json:"payFrequency,omitempty"`
}

// Merchant represents a business entity using the service.
type Merchant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	IsActive  bool      `json:"isActive"`
	ShopName  *string   `json:"shopName,omitempty"` // For backward compatibility
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	ID                string     `json:"id"`
	MerchantID        string     `json:"merchantId"`
	Name              string     `json:"name"`
	Description       *string    `json:"description,omitempty"`
	SKU               *string    `json:"sku,omitempty"`
	SellingPrice      float64    `json:"sellingPrice"`
	OriginalPrice     *float64   `json:"originalPrice,omitempty"`
	LowStockThreshold *int       `json:"lowStockThreshold,omitempty"`
	Category          *string    `json:"category,omitempty"`
	SupplierID        *string    `json:"supplierId,omitempty"`
	IsArchived        bool       `json:"isArchived"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	Stock             *ShopStock `json:"stock,omitempty"`
}

// InventoryItemWithQuantity extends InventoryItem with shop-specific stock quantity.
type InventoryItemWithQuantity struct {
	InventoryItem
	Quantity int `json:"quantity"`
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
	InventoryItemID string    `json:"inventoryItemId"`
	ShopID          string    `json:"shopId"`
	UserID          string    `json:"userId"`
	MovementType    string    `json:"movementType"`
	QuantityChanged int       `json:"quantityChanged"`
	NewQuantity     int       `json:"newQuantity"`
	Reason          *string   `json:"reason,omitempty"`
	MovementDate    time.Time `json:"movementDate"`
	Notes           *string   `json:"notes,omitempty"`
}

// ShopCustomer represents a customer associated with a specific shop.
type ShopCustomer struct {
	ID         string    `json:"id"`
	ShopID     string    `json:"shopId"`
	MerchantID string    `json:"merchantId"`
	Name       string    `json:"name"`
	Email      *string   `json:"email,omitempty"`
	Phone      *string   `json:"phone,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Promotion represents a discount or offer.
type Promotion struct {
	ID          string     `json:"id"`
	MerchantID  string     `json:"merchantId"`
	ShopID      *string    `json:"shopId,omitempty"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	PromoType   string     `json:"promoType"`
	PromoValue  float64    `json:"promoValue"`
	MinSpend    float64    `json:"minSpend"`
	Conditions  JSONB      `json:"conditions,omitempty"`
	StartDate   *time.Time `json:"startDate,omitempty"`
	EndDate     *time.Time `json:"endDate,omitempty"`
	IsActive    bool       `json:"isActive"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Sale represents a single transaction.
type Sale struct {
	ID                    string     `json:"id"`
	ShopID                string     `json:"shopId"`
	MerchantID            string     `json:"merchantId"`
	StaffID               *string    `json:"staffId,omitempty"`
	CustomerID            *string    `json:"customerId,omitempty"`
	SaleDate              time.Time  `json:"saleDate"`
	TotalAmount           float64    `json:"totalAmount"`
	AppliedPromotionID    *string    `json:"appliedPromotionId,omitempty"`
	DiscountAmount        *float64   `json:"discountAmount,omitempty"`
	PaymentType           string     `json:"paymentType"`
	PaymentStatus         string     `json:"paymentStatus"`
	StripePaymentIntentID *string    `json:"stripePaymentIntentId,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	Items                 []SaleItem `json:"items,omitempty"`
}

// Invoice represents an invoice generated for a sale.
type Invoice struct {
	ID             string     `json:"id"`
	SaleID         string     `json:"saleId"`
	InvoiceNumber  string     `json:"invoiceNumber"`
	MerchantID     string     `json:"merchantId"`
	ShopID         string     `json:"shopId"`
	CustomerID     *string    `json:"customerId,omitempty"`
	InvoiceDate    time.Time  `json:"invoiceDate"`
	DueDate        *time.Time `json:"dueDate,omitempty"`
	Subtotal       float64    `json:"subtotal"`
	DiscountAmount float64    `json:"discountAmount"`
	TaxAmount      float64    `json:"taxAmount"`
	TotalAmount    float64    `json:"totalAmount"`
	PaymentStatus  string     `json:"paymentStatus"`
	Notes          *string    `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
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
	ID          string    `json:"id"`
	StaffID     string    `json:"staffId"`
	Amount      float64   `json:"amount"`
	PaymentDate time.Time `json:"paymentDate"`
	Notes       *string   `json:"notes,omitempty"`
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
	UpdatedAt         time.Time `json:"updatedAt"`
}

// --- API Request/Response Structs ---

// CreateUserRequest defines the body for creating a new user.
type CreateUserRequest struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Role       string  `json:"role"`
	MerchantID *string `json:"merchantId,omitempty"`
}

// CreateShopRequest defines the body for creating a new shop.
type CreateShopRequest struct {
	Name       string  `json:"name"`
	MerchantID string  `json:"merchantId"`
	Address    *string `json:"address,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	IsActive   bool    `json:"isActive"`
	IsPrimary  bool    `json:"isPrimary"`
}

// AdminDashboardSummary defines the structure for the admin dashboard summary.
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

// KpiData represents a single Key Performance Indicator.
type KpiData struct {
	Value float64 `json:"value"`
}

// ProductSummary represents a summary of a single product's performance.
type ProductSummary struct {
	ProductID    string  `json:"productId"`
	ProductName  string  `json:"productName"`
	QuantitySold int     `json:"quantitySold"`
	Revenue      float64 `json:"revenue"`
}

// MerchantDashboardSummary defines the structure for the merchant dashboard summary.
type MerchantDashboardSummary struct {
	TotalSalesRevenue    KpiData          `json:"totalSalesRevenue"`
	NumberOfTransactions KpiData          `json:"numberOfTransactions"`
	AverageOrderValue    KpiData          `json:"averageOrderValue"`
	TopSellingProducts   []ProductSummary `json:"topSellingProducts"`
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
	TotalItems  int `json:"totalItems"`
	TotalPages  int `json:"totalPages"`
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
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
	CurrentPage int    `json:"currentPage"`
	TotalPages  int    `json:"totalPages"`
	PageSize    int    `json:"pageSize"`
	TotalCount  int    `json:"totalCount"`
}

// PaginatedSalesResponse for sales history.
type PaginatedSalesResponse struct {
	Items      []Sale     `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// PaginatedInvoicesResponse for invoices.
type PaginatedInvoicesResponse struct {
	Items      []Invoice  `json:"items"`
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

type PaginatedShopStockResponse struct {
	Items       []ShopStockItem `json:"items"`
	TotalItems  int             `json:"totalItems"`
	CurrentPage int             `json:"currentPage"`
	PageSize    int             `json:"pageSize"`
	TotalPages  int             `json:"totalPages"`
}

// PaginatedShopCustomersResponse for shop customers.
type PaginatedShopCustomersResponse struct {
	Data       []ShopCustomer `json:"data"`
	Pagination Pagination     `json:"pagination"`
}

type ShopStockItem struct {
	ID              string    `json:"id"`
	ShopID          string    `json:"shopId"`
	InventoryItemID string    `json:"inventoryItemId"`
	ItemName        string    `json:"itemName"`
	ItemSku         string    `json:"itemSku"`
	ItemUnitPrice   float64   `json:"itemUnitPrice"`
	Quantity        int       `json:"quantity"`
	LastStockedInAt time.Time `json:"lastStockedInAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Receipt struct {
	SaleID         string        `json:"saleId"`
	SaleDate       time.Time     `json:"saleDate"`
	ShopName       string        `json:"shopName"`
	ShopAddress    string        `json:"shopAddress"`
	MerchantName   string        `json:"merchantName"`
	OriginalTotal  float64       `json:"originalTotal"`
	DiscountAmount float64       `json:"discountAmount"`
	FinalTotal     float64       `json:"finalTotal"`
	PaymentType    string        `json:"paymentType"`
	PaymentStatus  string        `json:"paymentStatus"`
	Items          []ReceiptItem `json:"items"`
}

type ShopDashboardSummary struct {
	SalesToday        float64 `json:"salesToday"`
	TransactionsToday int     `json:"transactionsToday"`
	LowStockItems     int     `json:"lowStockItems"`
}

type ReceiptItem struct {
	ItemName  string  `json:"itemName"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
	Total     float64 `json:"total"`
}

type StaffDashboardSummaryResponse struct {
	AssignedShopName  string                `json:"assignedShopName"`
	SalesToday        float64               `json:"salesToday"`
	TransactionsToday int                   `json:"transactionsToday"`
	RecentActivities  []StaffRecentActivity `json:"recentActivities"`
}

type StaffRecentActivity struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
	RelatedID string    `json:"relatedId"`
}

// --- POS --- //

// CheckoutItem represents a single item in the checkout request.
type CheckoutItem struct {
	ProductID          string  `json:"productId"`
	Quantity           int     `json:"quantity"`
	SellingPriceAtSale float64 `json:"sellingPriceAtSale"`
}

// CheckoutRequest is the full request body for the checkout endpoint.
type CheckoutRequest struct {
	ShopID                string         `json:"shopId"`
	Items                 []CheckoutItem `json:"items"`
	TotalAmount           float64        `json:"totalAmount"`
	DiscountAmount        float64        `json:"discountAmount"`
	AppliedPromotionID    *string        `json:"appliedPromotionId,omitempty"`
	PaymentType           string         `json:"paymentType"`
	CustomerID            *string        `json:"customerId,omitempty"`
	CustomerName          *string        `json:"customerName,omitempty"`
	StripePaymentIntentID *string        `json:"stripePaymentIntentId,omitempty"`
}

// ShopInventoryItem is a simplified view of an inventory item for the shop interface.
type ShopInventoryItem struct {
	ID           string  `json:"id"`
	ProductID    string  `json:"productId"`
	Name         string  `json:"name"`
	SKU          string  `json:"sku"`
	Quantity     int     `json:"quantity"`
	SellingPrice float64 `json:"sellingPrice"`
}

// StockInItem represents a single item in a stock-in request.
type StockInItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

// StockInRequest is the request body for the stock-in endpoint.
type StockInRequest struct {
	Items []StockInItem `json:"items"`
}

// StaffCheckoutItem represents a single item in a staff checkout request.
type StaffCheckoutItem struct {
	ProductID          string  `json:"productId"`
	Quantity           int     `json:"quantity"`
	SellingPriceAtSale float64 `json:"sellingPriceAtSale"`
}

// StaffCheckoutRequest is the request body for the staff checkout endpoint.
type StaffCheckoutRequest struct {
	Items        []StaffCheckoutItem `json:"items"`
	TotalAmount  float64             `json:"totalAmount"`
	PaymentType  string              `json:"paymentType"`
	CustomerID   *string             `json:"customerId,omitempty"`
	CustomerName *string             `json:"customerName,omitempty"`
}
