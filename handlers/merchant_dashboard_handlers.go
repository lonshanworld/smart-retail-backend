package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleGetMerchantDashboardSummary fetches summary data for the merchant dashboard.
func HandleGetMerchantDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	// --- Authorization: Get merchantID from JWT claims ---
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		log.Printf("[MerchantDashboard] Failed to extract claims: %v", err)
		return err
	}
	merchantID := claims.UserID
	log.Printf("[MerchantDashboard] Fetching dashboard for merchant: %s", merchantID)

	shopID := c.Query("shop_id") // Optional shop_id from query parameter
	if shopID != "" {
		log.Printf("[MerchantDashboard] Filtering by shop_id: %s", shopID)
	}

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
	err = db.QueryRow(ctx, querySales, argsSales...).Scan(&summary.TotalSalesRevenue.Value)
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
	var transactionCount int64
	err = db.QueryRow(ctx, queryTransactions, argsTransactions...).Scan(&transactionCount)
	if err != nil {
		log.Printf("Error fetching number of transactions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch number of transactions"})
	}
	summary.NumberOfTransactions.Value = float64(transactionCount)

	// 3. Average Order Value
	if summary.NumberOfTransactions.Value > 0 {
		summary.AverageOrderValue.Value = summary.TotalSalesRevenue.Value / summary.NumberOfTransactions.Value
	} else {
		summary.AverageOrderValue.Value = 0
	}

	// 4. Top Selling Products
	queryTopProducts := `
		SELECT
			COALESCE(i.id, p.id) AS product_id,
			COALESCE(i.name, p.name) AS product_name,
			COALESCE(SUM(si.quantity_sold), 0) AS quantity_sold,
			COALESCE(SUM(si.subtotal), 0) AS revenue
		FROM sales s
		JOIN sale_items si ON s.id = si.sale_id
		LEFT JOIN stock_items i ON si.stock_item_id = i.id
		LEFT JOIN products p ON si.product_id = p.id
		WHERE s.merchant_id = $1
	`
	argsTopProducts := []interface{}{merchantID}
	if shopID != "" {
		queryTopProducts += " AND s.shop_id = $2"
		argsTopProducts = append(argsTopProducts, shopID)
	}
	topPage, _ := strconv.Atoi(c.Query("topProductsPage", "1"))
	topPageSize, _ := strconv.Atoi(c.Query("topProductsPageSize", "5"))
	if topPage < 1 {
		topPage = 1
	}
	if topPageSize < 1 || topPageSize > 100 {
		topPageSize = 5
	}
	groupedTopProducts := queryTopProducts + ` GROUP BY COALESCE(i.id, p.id), COALESCE(i.name, p.name)`
	var topTotal int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM ("+groupedTopProducts+") top_products", argsTopProducts...).Scan(&topTotal); err != nil {
		log.Printf("Error counting top selling products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count top selling products"})
	}
	queryTopProducts += `
		GROUP BY COALESCE(i.id, p.id), COALESCE(i.name, p.name)
		ORDER BY revenue DESC
		LIMIT $` + strconv.Itoa(len(argsTopProducts)+1) + ` OFFSET $` + strconv.Itoa(len(argsTopProducts)+2)
	argsTopProducts = append(argsTopProducts, topPageSize, (topPage-1)*topPageSize)

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
	summary.TopSellingProductsPagination = models.Pagination{TotalItems: topTotal, TotalPages: (topTotal + topPageSize - 1) / topPageSize, CurrentPage: topPage, PageSize: topPageSize}

	log.Printf("[MerchantDashboard] Returning summary - Revenue: %.2f, Transactions: %.0f, Products: %d",
		summary.TotalSalesRevenue.Value, summary.NumberOfTransactions.Value, len(summary.TopSellingProducts))

	return c.JSON(summary)
}
