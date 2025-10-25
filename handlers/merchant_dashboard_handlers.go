package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleGetMerchantDashboardSummary fetches summary data for the merchant dashboard.
func HandleGetMerchantDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Authorization: Get merchantID from JWT claims ---
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	shopID := c.Query("shop_id") // Optional shop_id from query parameter

	var summary models.MerchantDashboardSummary

	// 1. Total Sales Revenue
	querySales := `
		SELECT COALESCE(SUM(total_amount), 0)
		FROM sales
		WHERE merchant_id = $1
	`
	argsSales := []interface{}{merchantID}
	if shopID != "" {
		querySales += " AND shop_id = $2"
		argsSales = append(argsSales, shopID)
	}
	err := db.QueryRow(ctx, querySales, argsSales...).Scan(&summary.TotalSalesRevenue.Value)
	if err != nil {
		log.Printf("Error fetching total sales revenue: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch total sales revenue"})
	}

	// 2. Number of Transactions
	queryTransactions := `
		SELECT COUNT(*)
		FROM sales
		WHERE merchant_id = $1
	`
	argsTransactions := []interface{}{merchantID}
	if shopID != "" {
		queryTransactions += " AND shop_id = $2"
		argsTransactions = append(argsTransactions, shopID)
	}
	err = db.QueryRow(ctx, queryTransactions, argsTransactions...).Scan(&summary.NumberOfTransactions.Value)
	if err != nil {
		log.Printf("Error fetching number of transactions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch number of transactions"})
	}

	// 3. Average Order Value
	if summary.NumberOfTransactions.Value > 0 {
		summary.AverageOrderValue.Value = summary.TotalSalesRevenue.Value / summary.NumberOfTransactions.Value
	} else {
		summary.AverageOrderValue.Value = 0
	}

	// 4. Top Selling Products
	queryTopProducts := `
		SELECT
			i.id AS product_id,
			i.name AS product_name,
			COALESCE(SUM(si.quantity_sold), 0) AS quantity_sold,
			COALESCE(SUM(si.subtotal), 0) AS revenue
		FROM sales s
		JOIN sale_items si ON s.id = si.sale_id
		JOIN inventory_items i ON si.inventory_item_id = i.id
		WHERE s.merchant_id = $1
	`
	argsTopProducts := []interface{}{merchantID}
	if shopID != "" {
		queryTopProducts += " AND s.shop_id = $2"
		argsTopProducts = append(argsTopProducts, shopID)
	}
	queryTopProducts += `
		GROUP BY i.id, i.name
		ORDER BY revenue DESC
		LIMIT 5
	`

	rows, err := db.Query(ctx, queryTopProducts, argsTopProducts...)
	if err != nil {
		log.Printf("Error fetching top selling products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch top selling products"})
	}
	defer rows.Close()

	products := []models.ProductSummary{}
	for rows.Next() {
		var p models.ProductSummary
		if err := rows.Scan(&p.ProductID, &p.ProductName, &p.QuantitySold, &p.Revenue); err != nil {
			log.Printf("Error scanning top product row: %v", err)
			continue
		}
		products = append(products, p)
	}
	summary.TopSellingProducts = products

	return c.JSON(summary)
}
