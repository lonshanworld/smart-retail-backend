package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdminDashboardSummary handles the GET /api/v1/admin/dashboard/summary endpoint.
func HandleGetAdminDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var summary models.AdminDashboardSummary

	// Total Active Merchants
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'merchant' AND is_active = true").Scan(&summary.TotalActiveMerchants); err != nil {
		log.Printf("Error counting active merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count active merchants"})
	}

	// Total Active Staff
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'staff' AND is_active = true").Scan(&summary.TotalActiveStaff); err != nil {
		log.Printf("Error counting active staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count active staff"})
	}

	// Total Active Shops
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shops WHERE is_active = true").Scan(&summary.TotalActiveShops); err != nil {
		log.Printf("Error counting active shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count active shops"})
	}

	// Total Products Listed
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM inventory_items WHERE is_archived = false").Scan(&summary.TotalProductsListed); err != nil {
		log.Printf("Error counting products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count products"})
	}

	return c.JSON(summary)
}
