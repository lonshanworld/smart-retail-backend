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
	admin.Get("/dashboard/summary", handlers.HandleGetAdminDashboardSummaryV2)

	// User Management (including Merchants and other Admins)
	admin.Post("/users", handlers.HandleCreateUserV2)
	admin.Get("/users", handlers.HandleGetUsers)
	admin.Put("/users/:userId", handlers.HandleAdminUpdateUser) // Corrected handler
	admin.Put("/users/:userId/status", handlers.HandleSetMerchantUserActiveStatus)
	admin.Delete("/users/:userId", handlers.HandleDeleteUserMerchant)

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
}
