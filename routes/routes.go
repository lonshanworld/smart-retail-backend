package routes

import (
	"app/handlers"
	"app/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes defines all the routes for the application.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// --- System Initialization ---
	api.Post("/init/admin", handlers.HandleInitializeAdmin)

	// --- Authentication Routes ---
	auth := api.Group("/auth")
	auth.Post("/login", handlers.HandleLogin)
	auth.Post("/shop-login", handlers.HandleShopLogin)

	// --- Admin Routes ---
	admin := api.Group("/admin", middleware.JWTMiddleware, middleware.AdminRequired)

	// Admin Profile
	admin.Get("/profile", handlers.HandleGetAdminProfile)
	admin.Put("profile", handlers.HandleUpdateAdminProfile)

	// Dashboard
	admin.Get("/dashboard/summary", handlers.HandleGetAdminDashboardSummary)

	// User Management (Staff, Admins)
	adminUsers := admin.Group("/users")
	adminUsers.Get("/merchants-for-selection", handlers.HandleGetMerchantsForSelection) // Must be before /:userId
	adminUsers.Post("/", handlers.HandleCreateUser)
	adminUsers.Get("/", handlers.HandleListUsers)
	adminUsers.Get("/:userId", handlers.HandleGetUserByID)
	adminUsers.Put("/:userId", handlers.HandleUpdateUser)
	adminUsers.Delete("/:userId", handlers.HandleHardDeleteUser) // Corrected path

	// Specific Admin-related routes
	admin.Get("/admins", handlers.HandleGetAdmins)
	admin.Get("/staff", handlers.HandleGetAllStaff)

	// Merchant Management (as seen by Admin)
	admin.Get("/merchants", handlers.HandleListMerchants)
	admin.Get("/merchants/:merchantIdOrUserId", handlers.HandleGetMerchantByID)

	// Shop Management (Admin)
	adminShops := admin.Group("/shops")
	adminShops.Get("/", handlers.HandleListShops)
	adminShops.Get("/:shopId", handlers.HandleGetShopByID)
	adminShops.Post("/", handlers.HandleCreateShop)
	adminShops.Put("/:shopId", handlers.HandleUpdateShop)
	adminShops.Put("/:shopId/status", handlers.HandleSetShopActiveStatus)
	adminShops.Delete("/:shopId", handlers.HandleDeleteShop)

	// --- Merchant Routes ---
	merchant := api.Group("/merchant", middleware.JWTMiddleware, middleware.MerchantRequired)
	merchant.Get("/dashboard/summary", handlers.HandleGetMerchantDashboardSummary)

	// AI Assistant
	merchant.Post("/ai-analysis", handlers.HandleAIAssistant)

	// Merchant Profile
	merchant.Get("/profile", handlers.HandleGetMerchantProfile)
	merchant.Put("/profile", handlers.HandleUpdateMerchantProfile)

	// Merchant Shops
	merchantShops := merchant.Group("/shops")
	merchantShops.Get("/", handlers.HandleListMerchantShops)
	merchantShops.Post("/", handlers.HandleCreateShop) // This was already correct
	merchantShops.Put("/:shopId", handlers.HandleUpdateMerchantShop)
	merchantShops.Delete("/:shopId", handlers.HandleDeleteMerchantShop)
	merchantShops.Get("/:shopId/products", handlers.HandleListProductsForShop)
	merchantShops.Patch("/:shopId/set-primary", handlers.HandleSetPrimaryShop)
	merchantShops.Get("/:shopId/inventory", handlers.HandleListInventoryForShop)
	merchantShops.Post("/:shopId/stock-in", handlers.HandleShopStockIn)
	merchantShops.Post("/:shopId/inventory/:inventoryItemId/stock-in", handlers.HandleStockInItem)
	merchantShops.Patch("/:shopId/inventory/:inventoryItemId/adjust-stock", handlers.HandleAdjustStockItem)
	merchantShops.Get("/:shopId/sales", handlers.HandleListSalesForShop)

	// New routes for stock adjustment and history
	merchantShops.Post("/:shopId/inventory/:itemId/adjust", handlers.HandleAdjustStock)
	merchantShops.Get("/:shopId/inventory/:itemId/history", handlers.HandleGetStockMovementHistory)

	// Merchant Sales
	merchantSales := merchant.Group("/sales")
	merchantSales.Post("/", handlers.HandleCreateSale)
	merchantSales.Get("/:saleId", handlers.HandleGetSaleByID)
	merchantSales.Get("/:saleId/receipt", handlers.HandleGetReceipt)

	// Merchant Promotions
	promotions := merchant.Group("/promotions")
	promotions.Get("/", handlers.HandleListPromotions)
	promotions.Post("/", handlers.HandleCreatePromotion)
	promotions.Put("/:id", handlers.HandleUpdatePromotion)
	promotions.Delete("/:id", handlers.HandleDeletePromotion)

	// Merchant Reports
	reports := merchant.Group("/reports")
	reports.Get("/sales", handlers.HandleGetSalesReport)
	reports.Get("/sales-forecast", handlers.HandleGetSalesForecast)

	// Merchant Staff
	merchantStaff := merchant.Group("/staff")
	merchantStaff.Get("/", handlers.HandleListMerchantStaff)
	merchantStaff.Post("/", handlers.HandleCreateMerchantStaff)
	merchantStaff.Put("/:staffId", handlers.HandleUpdateMerchantStaff)
	merchantStaff.Delete("/:staffId", handlers.HandleDeleteMerchantStaff)

	// Merchant Stocks
	merchant.Get("/stocks", handlers.HandleGetCombinedStocks)

	// Merchant Notifications
	notifications := merchant.Group("/notifications")
	notifications.Get("/", handlers.HandleGetNotifications)
	notifications.Get("/unread-count", handlers.HandleGetUnreadNotificationCount)
	notifications.Patch("/:notificationId/read", handlers.HandleMarkNotificationAsRead)

	// Merchant Payments
	payments := merchant.Group("/payments")
	payments.Post("/create-intent", handlers.HandleCreatePaymentIntent)

	// Merchant POS
	pos := merchant.Group("/pos")
	pos.Get("/products", handlers.HandleSearchProductsForPOS)
	pos.Get("/promotions", handlers.HandleGetActivePromotionsForPOS)
	pos.Post("/checkout", handlers.HandleCheckout)
	pos.Post("/sync", handlers.HandleSyncOfflineSales)

	customers := merchant.Group("/customers")
	customers.Get("/search", handlers.HandleSearchCustomers)
	customers.Post("/", handlers.HandleCreateCustomer)

	suppliers := merchant.Group("/suppliers")
	suppliers.Get("/", handlers.HandleListMerchantSuppliers)
	suppliers.Post("/", handlers.HandleCreateNewSupplier)
	suppliers.Get("/:supplierId", handlers.HandleGetSupplierDetails)
	suppliers.Put("/:supplierId", handlers.HandleUpdateExistingSupplier)
	suppliers.Delete("/:supplierId", handlers.HandleDeleteExistingSupplier)

	inventory := merchant.Group("/inventory")
	inventory.Get("/", handlers.HandleListInventoryItems)
	inventory.Post("/", handlers.HandleCreateInventoryItem)
	inventory.Post("/stock-in", handlers.HandleMerchantStockIn)
	inventory.Post("/move-stock", handlers.HandleMoveStock)
	inventory.Get("/:itemId", handlers.HandleGetInventoryItemByID)
	inventory.Put("/:itemId", handlers.HandleUpdateInventoryItem)
	inventory.Delete("/:itemId", handlers.HandleDeleteInventoryItem)
	inventory.Patch("/:itemId/archive", handlers.HandleArchiveInventoryItem)
	inventory.Patch("/:itemId/unarchive", handlers.HandleUnarchiveInventoryItem)

	// --- Staff Routes ---
	staff := api.Group("/staff", middleware.JWTMiddleware, middleware.StaffRequired)
	staff.Get("/dashboard/summary", handlers.HandleGetStaffDashboardSummary)
	staff.Get("/assigned-shop", handlers.HandleGetAssignedShop)
	staff.Get("/profile", handlers.HandleGetStaffProfile)
	staff.Get("/salary", handlers.HandleGetSalaryHistory)

	// --- Staff POS Routes ---
	staffPOS := staff.Group("/pos")
	staffPOS.Get("/products", handlers.HandleSearchProductsForStaff)
	staffPOS.Get("/promotions", handlers.HandleGetActivePromotionsForStaff)
	staffPOS.Post("/checkout", handlers.HandleStaffCheckout)

	// --- Staff Items Routes ---
	staffItems := staff.Group("/items")
	staffItems.Get("/", handlers.HandleGetStaffItems)

	// --- Shop Routes ---
	// Shop routes are accessible by both merchants (with shopId param) and staff (with assigned shop)
	shop := api.Group("/shop", middleware.JWTMiddleware)
	shop.Get("/dashboard/summary", handlers.HandleGetShopDashboardSummary)
	shop.Get("/profile", handlers.HandleGetShopProfile) // Corrected route

	// New routes for shop inventory management
	shop.Get("/items", handlers.HandleGetShopItems)
	shop.Put("/items/:itemId/stock", handlers.HandleUpdateShopItemStock)
	shop.Get("/inventory", handlers.HandleGetShopInventory)
	shop.Post("/inventory/stock-in", handlers.HandleStockIn)

	// Shop customers routes (accessible by both merchant and staff)
	shopCustomers := shop.Group("/customers")
	shopCustomers.Get("/search", handlers.HandleSearchCustomers)
	shopCustomers.Post("/", handlers.HandleCreateCustomer)

	// Shop sales routes (accessible by both merchant and staff)
	shopSales := shop.Group("/shops/:shopId/sales")
	shopSales.Get("/", handlers.HandleListSalesForShop)

	// --- Shop POS Routes ---
	shopPOS := shop.Group("/pos")
	shopPOS.Get("/:shopId/products", handlers.HandleSearchShopProducts)
	shopPOS.Get("/promotions", handlers.HandleGetActivePromotionsForShop)
	shopPOS.Post("/:shopId/checkout", handlers.HandleShopCheckout)

	// --- Gemini Routes ---
	gemini := api.Group("/gemini", middleware.JWTMiddleware)
	gemini.Post("/generate", handlers.HandleGenerateText)
}
