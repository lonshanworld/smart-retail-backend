package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdminDashboardSummary fetches comprehensive summary data for the admin dashboard.
func HandleGetAdminDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var summary models.AdminDashboardSummary

	// --- User and Shop Counts ---
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'merchant'`).Scan(&summary.TotalMerchants); err != nil {
		log.Printf("Error fetching total merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch total merchants count"})
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'merchant' AND is_active = true`).Scan(&summary.ActiveMerchants); err != nil {
		log.Printf("Error fetching active merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch active merchants count"})
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'staff'`).Scan(&summary.TotalStaff); err != nil {
		log.Printf("Error fetching total staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch total staff count"})
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'staff' AND is_active = true`).Scan(&summary.ActiveStaff); err != nil {
		log.Printf("Error fetching active staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch active staff count"})
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM shops`).Scan(&summary.TotalShops); err != nil {
		log.Printf("Error fetching total shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch total shops count"})
	}

	// --- Sales and Transaction Metrics ---
	if err := db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount), 0) FROM sales`).Scan(&summary.TotalSalesValue); err != nil {
		log.Printf("Error fetching total sales value: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch total sales value"})
	}

	// Today's metrics
	today := time.Now().Truncate(24 * time.Hour)
	if err := db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount), 0) FROM sales WHERE sale_date >= $1`, today).Scan(&summary.SalesToday); err != nil {
		log.Printf("Error fetching sales today: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch sales today"})
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM sales WHERE sale_date >= $1`, today).Scan(&summary.TransactionsToday); err != nil {
		log.Printf("Error fetching transactions today: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch transactions today"})
	}

	return c.JSON(summary)
}
