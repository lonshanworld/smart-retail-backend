package routes

import (
	"app/handlers"
	"app/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes defines all the routes for the application.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

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
	admin.Get("/users/merchants-for-selection", handlers.HandleGetMerchantsForSelection) // Must be before /users/:userId
	admin.Post("/users", handlers.HandleCreateUser)
	admin.Get("/users", handlers.HandleListUsers)
	admin.Get("/users/:userId", handlers.HandleGetUserByID)
	admin.Put("/users/:userId", handlers.HandleUpdateUser)
	admin.Put("/users/:userId/status", handlers.HandleSetUserStatus)
	admin.Delete("/users/:userId/permanent-delete", handlers.HandleHardDeleteUser)

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

	// Merchant Profile
	merchant.Get("/profile", handlers.HandleGetMerchantProfile)
	merchant.Put("/profile", handlers.HandleUpdateMerchantProfile)

	// Merchant Shops
	merchantShops := merchant.Group("/shops")
	merchantShops.Get("/", handlers.HandleListMerchantShops)
	merchantShops.Post("/", handlers.HandleCreateShop) // This was already correct
	merchantShops.Put("/:shopId", handlers.HandleUpdateMerchantShop)
	merchantShops.Delete("/:shopId", handlers.HandleDeleteMerchantShop)
	merchantShops.Get("/:shopId/products", handlers.HandleListProductsForShop) // New route

	// Merchant Promotions
	promotions := merchant.Group("/promotions")
	promotions.Get("/", handlers.HandleListPromotions)
	promotions.Post("/", handlers.HandleCreatePromotion)
	promotions.Put("/:id", handlers.HandleUpdatePromotion)
	promotions.Delete("/:id", handlers.HandleDeletePromotion)

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
	notifications.Get("/unread-count", handlers.HandleGetUnreadNotificationsCount)
	notifications.Patch("/:notificationId/read", handlers.HandleMarkNotificationAsRead)

	// Merchant Payments
	payments := merchant.Group("/payments")
	payments.Post("/create-intent", handlers.HandleCreatePaymentIntent)

	// Merchant POS
	pos := merchant.Group("/pos")
	pos.Get("/products", handlers.HandleSearchProductsForPOS)
	pos.Post("/checkout", handlers.HandleCheckout)

	customers := merchant.Group("/customers")
	customers.Get("/search", handlers.HandleSearchCustomers)
	customers.Post("/", handlers.HandleCreateCustomer)

	suppliers := merchant.Group("/suppliers")
	suppliers.Get("/", handlers.HandleListMerchantSuppliers)
	suppliers.Get("/:supplierId", handlers.HandleGetSupplierDetails)
	suppliers.Post("/", handlers.HandleCreateNewSupplier)
	suppliers.Put("/:supplierId", handlers.HandleUpdateExistingSupplier)
	suppliers.Delete("/:supplierId", handlers.HandleDeleteExistingSupplier)

	inventory := merchant.Group("/inventory")
	inventory.Get("/", handlers.HandleListInventoryItems)
	inventory.Post("/", handlers.HandleCreateInventoryItem)
	inventory.Get("/:itemId", handlers.HandleGetInventoryItemByID)
	inventory.Put("/:itemId", handlers.HandleUpdateInventoryItem)
	inventory.Delete("/:itemId", handlers.HandleDeleteInventoryItem)
	inventory.Patch("/:itemId/archive", handlers.HandleArchiveInventoryItem)
	inventory.Patch("/:itemId/unarchive", handlers.HandleUnarchiveInventoryItem)


	// --- Gemini Routes ---
	gemini := api.Group("/gemini", middleware.JWTMiddleware)
	gemini.Post("/generate", handlers.HandleGenerateText)
}
