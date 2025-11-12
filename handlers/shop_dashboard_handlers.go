package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetShopDashboardSummary retrieves a summary for the shop dashboard.
// Accessible by both merchants (with shopId query param) and staff (using assigned_shop_id).
func HandleGetShopDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID
	userRole := claims.Role

	var shopID string

	// Both merchant and staff should provide shopId query parameter
	shopID = c.Query("shopId")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId query parameter is required"})
	}

	if userRole == "merchant" {
		// Verify merchant owns the shop
		var merchantID string
		shopCheckQuery := "SELECT merchant_id FROM shops WHERE id = $1"
		if err := db.QueryRow(ctx, shopCheckQuery, shopID).Scan(&merchantID); err != nil {
			log.Printf("Error verifying shop ownership: %v", err)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		if merchantID != userID {
			log.Printf("Access denied: Shop %s belongs to merchant %s, not %s", shopID, merchantID, userID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied to this shop"})
		}
	} else if userRole == "staff" {
		// Verify staff is assigned to this shop
		var assignedShopID *string
		staffQuery := "SELECT assigned_shop_id FROM users WHERE id = $1"
		if err := db.QueryRow(ctx, staffQuery, userID).Scan(&assignedShopID); err != nil {
			log.Printf("Error getting staff's assigned shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve staff details"})
		}
		if assignedShopID == nil || *assignedShopID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Staff member has no assigned shop"})
		}
		if *assignedShopID != shopID {
			log.Printf("Access denied: Staff %s is assigned to shop %s, not %s", userID, *assignedShopID, shopID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied to this shop"})
		}
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied"})
	}

	// Get sales and transactions for today
	var salesToday float64
	var transactionsToday int
	salesQuery := `
		SELECT COALESCE(SUM(total_amount), 0), COUNT(id)
		FROM sales
		WHERE shop_id = $1 AND sale_date >= $2
	`
	today := time.Now().Truncate(24 * time.Hour)
	if err := db.QueryRow(ctx, salesQuery, shopID, today).Scan(&salesToday, &transactionsToday); err != nil {
		log.Printf("Error getting shop sales summary: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales summary"})
	}

	// Get count of low stock items
	var lowStockItems int
	lowStockQuery := `
		SELECT count(*)
		FROM shop_stock ss
		JOIN inventory_items ii ON ss.inventory_item_id = ii.id
		WHERE ss.shop_id = $1 AND ii.low_stock_threshold IS NOT NULL AND ss.quantity <= ii.low_stock_threshold
	`
	if err := db.QueryRow(ctx, lowStockQuery, shopID).Scan(&lowStockItems); err != nil {
		log.Printf("Error getting low stock items count: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve low stock items count"})
	}

	summary := models.ShopDashboardSummary{
		SalesToday:        salesToday,
		TransactionsToday: transactionsToday,
		LowStockItems:     lowStockItems,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}
