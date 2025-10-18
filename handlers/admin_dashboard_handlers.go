package handlers

import (
	"app/database"
	"log"

	"github.com/gofiber/fiber/v2"
)

// AdminDashboardSummaryModel defines the structure for the admin dashboard summary.
type AdminDashboardSummaryModel struct {
	TotalActiveMerchants int `json:"total_active_merchants"`
	TotalActiveStaff     int `json:"total_active_staff"`
	TotalActiveShops     int `json:"total_active_shops"`
	TotalProductsListed  int `json:"total_products_listed"`
}

// HandleGetAdminDashboardSummary fetches summary data for the admin dashboard.
func HandleGetAdminDashboardSummaryV2(c *fiber.Ctx) error {
	db := database.GetDB()

	var summary AdminDashboardSummaryModel

	// Query for TotalActiveMerchants
	queryMerchants := `SELECT COUNT(*) FROM users WHERE role = 'merchant' AND is_active = true`
	err := db.QueryRow(queryMerchants).Scan(&summary.TotalActiveMerchants)
	if err != nil {
		log.Printf("Error fetching total active merchants: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch active merchants count"})
	}

	// Query for TotalActiveStaff
	queryStaff := `SELECT COUNT(*) FROM users WHERE role = 'staff' AND is_active = true`
	err = db.QueryRow(queryStaff).Scan(&summary.TotalActiveStaff)
	if err != nil {
		log.Printf("Error fetching total active staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch active staff count"})
	}

	// Query for TotalActiveShops
	queryShops := `SELECT COUNT(*) FROM shops WHERE is_active = true`
	err = db.QueryRow(queryShops).Scan(&summary.TotalActiveShops)
	if err != nil {
		log.Printf("Error fetching total active shops: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch active shops count"})
	}

	// Query for TotalProductsListed
	queryProducts := `SELECT COUNT(*) FROM inventory_items`
	err = db.QueryRow(queryProducts).Scan(&summary.TotalProductsListed)
	if err != nil {
		log.Printf("Error fetching total products listed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch products count"})
	}

	return c.JSON(summary)
}
