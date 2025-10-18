package handlers

import (
	"app/database"
	"app/middleware"
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
)

// --- Structs for handler responses ---

type KpiData struct {
	Value float64 `json:"value"`
}

type ProductSummaryModel struct {
	ProductID    string  `json:"product_id"`
	ProductName  string  `json:"product_name"`
	QuantitySold int     `json:"quantity_sold"`
	Revenue      float64 `json:"revenue"`
}

type MerchantDashboardSummaryModel struct {
	TotalSalesRevenue    KpiData               `json:"total_sales_revenue"`
	NumberOfTransactions KpiData               `json:"number_of_transactions"`
	AverageOrderValue    KpiData               `json:"average_order_value"`
	TopSellingProducts   []ProductSummaryModel `json:"top_selling_products"`
}

// HandleGetMerchantDashboardSummary calculates and returns the summary for the merchant's dashboard.
// GET /api/v1/merchant/dashboard/summary
func HandleGetMerchantDashboardSummary(c *fiber.Ctx) error {
	// Get merchant details from JWT
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Unauthorized",
		})
	}
	merchantID := claims.UserID

	db := database.GetDB()
	var summary MerchantDashboardSummaryModel

	// --- Calculate KPIs ---

	// Get total sales revenue
	revenueQuery := "SELECT COALESCE(SUM(total_amount), 0) FROM sales WHERE merchant_id = $1"
	if err := db.QueryRow(revenueQuery, merchantID).Scan(&summary.TotalSalesRevenue.Value); err != nil {
		log.Printf("Error getting total sales revenue for merchant %s: %v", merchantID, err)
	}

	// Get number of transactions
	transactionsQuery := "SELECT COUNT(*) FROM sales WHERE merchant_id = $1"
	if err := db.QueryRow(transactionsQuery, merchantID).Scan(&summary.NumberOfTransactions.Value); err != nil {
		log.Printf("Error getting transaction count for merchant %s: %v", merchantID, err)
	}

	// Calculate average order value
	if summary.NumberOfTransactions.Value > 0 {
		summary.AverageOrderValue.Value = summary.TotalSalesRevenue.Value / summary.NumberOfTransactions.Value
	} else {
		summary.AverageOrderValue.Value = 0
	}

	// --- Get Top 5 Selling Products ---
	topProductsQuery := `
		SELECT 
			i.id,
			i.name,
			SUM(si.quantity_sold) as total_quantity_sold,
			SUM(si.subtotal) as total_revenue
		FROM sale_items si
		JOIN inventory_items i ON si.inventory_item_id = i.id
		JOIN sales s ON si.sale_id = s.id
		WHERE s.merchant_id = $1
		GROUP BY i.id, i.name
		ORDER BY total_revenue DESC
		LIMIT 5
	`

	rows, err := db.Query(topProductsQuery, merchantID)
	if err != nil {
		log.Printf("Error querying top selling products for merchant %s: %v", merchantID, err)
		// Do not fail the whole request, just return empty top products
		summary.TopSellingProducts = []ProductSummaryModel{}
		return c.JSON(fiber.Map{"status": "success", "data": summary})
	}
	defer rows.Close()

	products := []ProductSummaryModel{}
	for rows.Next() {
		var p ProductSummaryModel
		var quantitySold sql.NullInt64
		var revenue sql.NullFloat64

		if err := rows.Scan(&p.ProductID, &p.ProductName, &quantitySold, &revenue); err != nil {
			log.Printf("Error scanning top product row for merchant %s: %v", merchantID, err)
			continue
		}
		if quantitySold.Valid {
			p.QuantitySold = int(quantitySold.Int64)
		}
		if revenue.Valid {
			p.Revenue = revenue.Float64
		}
		products = append(products, p)
	}

	summary.TopSellingProducts = products

	return c.JSON(fiber.Map{"status": "success", "data": summary})
}
