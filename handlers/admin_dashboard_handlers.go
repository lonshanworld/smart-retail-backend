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

	// Total Merchants
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'merchant'").Scan(&summary.TotalMerchants); err != nil {
		log.Printf("Error counting total merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count total merchants"})
	}

	// Active Merchants
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'merchant' AND is_active = true").Scan(&summary.ActiveMerchants); err != nil {
		log.Printf("Error counting active merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count active merchants"})
	}

	// Total Staff
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'staff'").Scan(&summary.TotalStaff); err != nil {
		log.Printf("Error counting total staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count total staff"})
	}

	// Active Staff
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'staff' AND is_active = true").Scan(&summary.ActiveStaff); err != nil {
		log.Printf("Error counting active staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count active staff"})
	}

	// Total Shops
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shops").Scan(&summary.TotalShops); err != nil {
		log.Printf("Error counting total shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count total shops"})
	}

	// Total Sales Value (all time)
	if err := db.QueryRow(ctx, "SELECT COALESCE(SUM(total_amount), 0) FROM sales").Scan(&summary.TotalSalesValue); err != nil {
		log.Printf("Error calculating total sales value: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to calculate total sales"})
	}

	// Sales Today
	if err := db.QueryRow(ctx, "SELECT COALESCE(SUM(total_amount), 0) FROM sales WHERE DATE(created_at) = CURRENT_DATE").Scan(&summary.SalesToday); err != nil {
		log.Printf("Error calculating today's sales: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to calculate today's sales"})
	}

	// Transactions Today
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM sales WHERE DATE(created_at) = CURRENT_DATE").Scan(&summary.TransactionsToday); err != nil {
		log.Printf("Error counting today's transactions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count today's transactions"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}
