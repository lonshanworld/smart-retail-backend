package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetShopDashboardSummary retrieves a summary for the shop dashboard.
func HandleGetShopDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)

	// Get assigned shop ID for the staff member
	var shopID string
	shopQuery := "SELECT assigned_shop_id FROM users WHERE id = $1"
	if err := db.QueryRow(ctx, shopQuery, staffID).Scan(&shopID); err != nil {
		log.Printf("Error getting staff's assigned shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop details"})
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

	return c.JSON(summary)
}
