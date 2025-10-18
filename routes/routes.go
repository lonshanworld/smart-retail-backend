package routes

import (
	"app/handlers"
	"app/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes defines all the routes for the application.
func SetupRoutes(app *fiber.App) {
	api := app.Group("/api/v1")

	// Authentication routes
	auth := api.Group("/auth")
	auth.Post("/login", handlers.HandleLogin)
	auth.Post("/shop-login", handlers.HandleShopLogin)
	auth.Post("/users", handlers.HandleCreateUser)

	// Admin routes
	admin := api.Group("/admin", middleware.JWTMiddleware, middleware.AdminRequired)
	admin.Get("/dashboard/summary", handlers.HandleGetAdminDashboardSummary)
	admin.Get("/users", handlers.HandleGetUsers)
	admin.Put("/users/:userId", handlers.HandleAdminUpdateUser)
	admin.Get("/admins", handlers.HandleGetAdmins) // New route for fetching admin users

	// Merchant routes
	merchant := api.Group("/merchant", middleware.JWTMiddleware, middleware.MerchantRequired)
	merchant.Post("/shops", handlers.HandleCreateShop)
}
