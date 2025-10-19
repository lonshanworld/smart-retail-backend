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
	admin.Put("/profile", handlers.HandleUpdateAdminProfile)

	// Dashboard
	admin.Get("/dashboard/summary", handlers.HandleGetAdminDashboardSummary)

	// User Management (Merchants, Staff, Admins)
	admin.Get("/users/merchants-for-selection", handlers.HandleGetMerchantsForSelection) // Must be before /users/:userId
	admin.Post("/users", handlers.HandleCreateUserV2)
	admin.Get("/users", handlers.HandleGetUsers)
	admin.Get("/users/:userId", handlers.HandleGetUserByID)
	admin.Put("/users/:userId", handlers.HandleAdminUpdateUser)
	admin.Put("/users/:userId/status", handlers.HandleSetUserStatus)
	admin.Delete("/users/:userId", handlers.HandleDeleteUserMerchant) // This is a soft delete
	admin.Delete("/users/:userId/permanent-delete", handlers.HandlePermanentDeleteUser)

	// Specific Admin-related routes
	admin.Get("/admins", handlers.HandleGetAdmins)
	admin.Get("/staff", handlers.HandleGetAllStaff)

	// Merchant Management (as seen by Admin)
	admin.Get("/merchants", handlers.HandleListMerchants)
	admin.Get("/merchants/:merchantIdOrUserId", handlers.HandleGetMerchantByID)

	// Shop Management
	admin.Get("/shops", handlers.HandleListShops)
	admin.Get("/shops/:shopId", handlers.HandleGetShopByID)
	admin.Post("/shops", handlers.HandleCreateShop)
	admin.Put("/shops/:shopId", handlers.HandleUpdateShop)
	admin.Put("/shops/:shopId/status", handlers.HandleSetShopActiveStatus)
	admin.Delete("/shops/:shopId", handlers.HandleDeleteShop)

	// --- Merchant Routes ---
	merchant := api.Group("/merchant", middleware.JWTMiddleware, middleware.MerchantRequired)
	merchant.Get("/dashboard/summary", handlers.HandleGetMerchantDashboardSummary)
	merchant.Post("/shops", handlers.HandleCreateShop)

	customers := merchant.Group("/customers")
	customers.Get("/search", handlers.HandleSearchCustomers)
	customers.Post("/", handlers.HandleCreateCustomer)

	suppliers := merchant.Group("/suppliers")
	suppliers.Get("/", handlers.HandleListMerchantSuppliers)
	suppliers.Get("/:supplierId", handlers.HandleGetSupplierDetails)
	suppliers.Post("/", handlers.HandleCreateNewSupplier)
	suppliers.Put("/:supplierId", handlers.HandleUpdateExistingSupplier)
	suppliers.Delete("/:supplierId", handlers.HandleDeleteExistingSupplier)

	// --- Gemini Routes ---
	gemini := api.Group("/gemini", middleware.JWTMiddleware)
	gemini.Post("/generate", handlers.HandleGenerateText)
}
