package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetSalesReport generates a sales report based on filters.
func HandleGetSalesReport(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantID := claims.UserID

	// Parse query parameters
	startDateStr := c.Query("startDate", time.Now().AddDate(0, -1, 0).Format(time.RFC3339))
	endDateStr := c.Query("endDate", time.Now().Format(time.RFC3339))
	shopID := c.Query("shopId")
	groupBy := c.Query("groupBy", "daily") // Not used for now, but kept for future

	log.Printf("📊 [SALES REPORT] Request - Merchant: %s, StartDate: %s, EndDate: %s, ShopID: %s, GroupBy: %s",
		merchantID, startDateStr, endDateStr, shopID, groupBy)

	// Parse dates - try multiple formats
	parseDate := func(dateStr string) (time.Time, error) {
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05",
			"2006-01-02",
		}
		var lastErr error
		for _, format := range formats {
			if t, err := time.Parse(format, dateStr); err == nil {
				return t, nil
			} else {
				lastErr = err
			}
		}
		return time.Time{}, lastErr
	}

	startDate, err := parseDate(startDateStr)
	if err != nil {
		log.Printf("❌ [SALES REPORT] Invalid startDate: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid startDate format"})
	}
	endDate, err := parseDate(endDateStr)
	if err != nil {
		log.Printf("❌ [SALES REPORT] Invalid endDate: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid endDate format"})
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	usePagination := true

	// Fetch the selected sales and all of their items in one query. The previous
	// implementation issued one additional query per sale (N+1), which made
	// larger date ranges unnecessarily slow.
	query := `
		WITH selected_sales AS (
			SELECT id, shop_id, merchant_id, sale_date, total_amount,
			       applied_promotion_id, discount_amount, payment_type, payment_status,
			       created_at, updated_at
			FROM sales
			WHERE merchant_id = $1 AND sale_date BETWEEN $2 AND $3
	`
	args := []interface{}{merchantID, startDate, endDate}

	// Add shop filter if provided
	if shopID != "" {
		query += " AND shop_id = $4"
		args = append(args, shopID)
	}

	query += " ORDER BY sale_date DESC"
	if usePagination {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, pageSize, (page-1)*pageSize)
	}
	query += `
		)
		SELECT ss.id, ss.shop_id, ss.merchant_id, ss.sale_date, ss.total_amount,
		       ss.applied_promotion_id, ss.discount_amount, ss.payment_type,
		       ss.payment_status, ss.created_at, ss.updated_at,
		       si.id, si.sale_id, si.inventory_item_id, si.item_name, si.item_sku,
		       si.quantity_sold, si.selling_price_at_sale, si.original_price_at_sale,
		       si.subtotal, si.created_at, si.updated_at
		FROM selected_sales ss
		LEFT JOIN sale_items si ON si.sale_id = ss.id
		ORDER BY ss.sale_date DESC, si.created_at ASC
	`

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("❌ [SALES REPORT] Query error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate sales report"})
	}
	defer rows.Close()

	sales := make([]models.Sale, 0)
	saleIndexes := make(map[string]int)
	for rows.Next() {
		var sale models.Sale
		var itemID, itemSaleID, itemInventoryID, itemName, itemSKU sql.NullString
		// quantity_sold is NUMERIC(15,3) in the live schema. PostgreSQL may
		// return values such as "4000e-3", which cannot be scanned into int64.
		var itemQuantity sql.NullFloat64
		var itemSellingPrice, itemOriginalPrice, itemSubtotal sql.NullFloat64
		var itemCreatedAt, itemUpdatedAt sql.NullTime
		if err := rows.Scan(
			&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.SaleDate,
			&sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount,
			&sale.PaymentType, &sale.PaymentStatus, &sale.CreatedAt, &sale.UpdatedAt,
			&itemID, &itemSaleID, &itemInventoryID, &itemName, &itemSKU,
			&itemQuantity, &itemSellingPrice, &itemOriginalPrice, &itemSubtotal,
			&itemCreatedAt, &itemUpdatedAt,
		); err != nil {
			log.Printf("❌ [SALES REPORT] Scan error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to process sales report data"})
		}

		index, exists := saleIndexes[sale.ID]
		if !exists {
			sale.Items = make([]models.SaleItem, 0)
			sales = append(sales, sale)
			index = len(sales) - 1
			saleIndexes[sale.ID] = index
		}

		if itemID.Valid {
			item := models.SaleItem{ID: itemID.String}
			if itemSaleID.Valid {
				item.SaleID = itemSaleID.String
			}
			if itemInventoryID.Valid {
				item.InventoryItemID = itemInventoryID.String
			}
			if itemQuantity.Valid {
				item.QuantitySold = int(itemQuantity.Float64)
			}
			if itemSellingPrice.Valid {
				item.SellingPriceAtSale = itemSellingPrice.Float64
			}
			if itemOriginalPrice.Valid {
				value := itemOriginalPrice.Float64
				item.OriginalPriceAtSale = &value
			}
			if itemSubtotal.Valid {
				item.Subtotal = itemSubtotal.Float64
			}
			if itemCreatedAt.Valid {
				item.CreatedAt = itemCreatedAt.Time
			}
			if itemUpdatedAt.Valid {
				item.UpdatedAt = itemUpdatedAt.Time
			}
			if itemName.Valid {
				value := itemName.String
				item.ItemName = &value
			}
			if itemSKU.Valid {
				value := itemSKU.String
				item.ItemSKU = &value
			}
			sales[index].Items = append(sales[index].Items, item)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("❌ [SALES REPORT] Row iteration error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to process sales report data"})
	}

	if usePagination {
		countQuery := `
			SELECT COUNT(*)
			FROM sales
			WHERE merchant_id = $1 AND sale_date BETWEEN $2 AND $3
		`
		countArgs := []interface{}{merchantID, startDate, endDate}
		if shopID != "" {
			countQuery += " AND shop_id = $4"
			countArgs = append(countArgs, shopID)
		}

		var totalItems int
		if err := db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalItems); err != nil {
			log.Printf("❌ [SALES REPORT] Count query error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count sales report data"})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"items": sales,
				"sales": sales,
				"meta": fiber.Map{
					"totalItems":  totalItems,
					"currentPage": page,
					"pageSize":    pageSize,
					"totalPages":  (totalItems + pageSize - 1) / pageSize,
				},
			},
		})
	}

	log.Printf("✅ [SALES REPORT] Returning %d sales", len(sales))
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"sales": sales}})
}

// HandleGetSalesForecast generates a sales forecast for a product using Gemini.
func HandleGetSalesForecast(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}
	merchantID := claims.UserID

	shopID := c.Query("shopId")
	itemID := c.Query("itemId")
	provider := resolveAIProvider(c.Query("provider"))

	if shopID == "" || itemID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "shopId and itemId are required"})
	}
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}
	var owned bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inventory_items WHERE shop_id=$1 AND stock_item_id=$2 AND merchant_id=$3)`, shopID, itemID, merchantID).Scan(&owned); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to verify inventory ownership"})
	}
	if !owned {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Inventory item not found in shop"})
	}

	// 1. Fetch historical sales data for the item
	query := `
        SELECT s.sale_date, si.quantity_sold
        FROM sales s
        JOIN sale_items si ON s.id = si.sale_id
        WHERE s.shop_id = $1 AND si.stock_item_id = $2 AND s.merchant_id = $3
        AND s.sale_date >= NOW() - INTERVAL '90 days'
		ORDER BY s.sale_date
		LIMIT 5000
    `
	rows, err := db.Query(ctx, query, shopID, itemID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to get historical data"})
	}
	defer rows.Close()

	var historicalData []models.HistoricalSale
	for rows.Next() {
		var hs models.HistoricalSale
		var quantity sql.NullFloat64
		if err := rows.Scan(&hs.SaleDate, &quantity); err != nil {
			continue
		}
		if quantity.Valid {
			hs.QuantitySold = int(quantity.Float64)
		}
		historicalData = append(historicalData, hs)
	}

	// 2. Fetch product and shop details for context
	var productName, shopName string
	var currentStock int
	_ = db.QueryRow(ctx, "SELECT name FROM stock_items WHERE id = $1 AND merchant_id = $2", itemID, merchantID).Scan(&productName)
	_ = db.QueryRow(ctx, "SELECT name FROM shops WHERE id = $1 AND merchant_id = $2", shopID, merchantID).Scan(&shopName)
	_ = db.QueryRow(ctx, "SELECT quantity_on_hand FROM inventory_items WHERE shop_id = $1 AND stock_item_id = $2", shopID, itemID).Scan(&currentStock)

	// 3. Construct the prompt for the Gemini API
	prompt := constructForecastPrompt(productName, shopName, currentStock, historicalData)

	// 4. Call the configured AI provider
	resp, err := generateAIText(ctx, provider, "You are an expert retail data analyst. Return only valid minified JSON.", prompt)
	if err != nil {
		log.Printf("Error from AI provider: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate forecast from AI"})
	}

	// 5. Parse the response and format for the frontend
	forecastResponse, err := parseForecastResponse(resp, productName, shopName, currentStock)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "data": forecastResponse})
}

// constructForecastPrompt creates a detailed prompt for the Gemini API.
func constructForecastPrompt(productName, shopName string, currentStock int, data []models.HistoricalSale) string {
	dataStr := ""
	for _, d := range data {
		dataStr += fmt.Sprintf("On %s, %d units were sold.\n", d.SaleDate.Format("2006-01-02"), d.QuantitySold)
	}
	if dataStr == "" {
		dataStr = "No sales data available for the last 90 days."
	}

	jsonFormat := `{"forecast":[{"date":"YYYY-MM-DD","predicted_sales":integer},...],"summary":"string","analysis_html":"<div>...</div>","positive_factors":["string",...],"negative_factors":["string",...]}`

	return fmt.Sprintf(`
        You are an expert retail data analyst. Your task is to generate a 7-day sales forecast and provide a brief analysis based on the data provided.

        **Analysis Context:**
        - Product Name: %s
        - Shop Name: %s
        - Current Stock Level: %d units
        - Today's Date: %s

        **Historical Sales Data (last 90 days):**
        %s

        **Required Output:**
        You must provide a single, minified JSON object with the following exact structure. Do not include any markdown formatting, backticks, or explanatory text before or after the JSON object.

        %s
    `, productName, shopName, currentStock, time.Now().Format("2006-01-02"), dataStr, jsonFormat)
}

func extractJSON(rawString string) string {
	start := strings.Index(rawString, "{")
	end := strings.LastIndex(rawString, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return rawString[start : end+1]
}

// parseForecastResponse parses the JSON from an AI provider into a structured response.
func parseForecastResponse(rawText, productName, shopName string, currentStock int) (*models.SalesForecastResponse, error) {
	if rawText == "" {
		return nil, fmt.Errorf("no text content received from AI")
	}

	// Clean the response to get only the JSON object
	jsonStr := extractJSON(rawText)
	if jsonStr == "" {
		log.Printf("Could not extract JSON from AI response")
		return nil, fmt.Errorf("failed to parse AI response format")
	}

	var geminiJSON struct {
		Forecast        []models.DailyForecast `json:"forecast"`
		Summary         string                 `json:"summary"`
		AnalysisHTML    string                 `json:"analysis_html"`
		PositiveFactors []string               `json:"positive_factors"`
		NegativeFactors []string               `json:"negative_factors"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &geminiJSON); err != nil {
		log.Printf("Error parsing AI JSON: %v", err)
		return nil, fmt.Errorf("failed to parse AI forecast data")
	}

	return &models.SalesForecastResponse{
		ReportName:   "7-Day Sales Forecast",
		GeneratedAt:  time.Now(),
		ProductName:  productName,
		ShopName:     shopName,
		CurrentStock: currentStock,
		ForecastPeriod: models.ForecastPeriod{
			StartDate: time.Now(),
			EndDate:   time.Now().AddDate(0, 0, 7),
		},
		DailyForecast: geminiJSON.Forecast,
		AiAnalysis: models.AiAnalysis{
			Summary:         geminiJSON.Summary,
			AnalysisHTML:    escapeAIHTML(geminiJSON.AnalysisHTML),
			PositiveFactors: geminiJSON.PositiveFactors,
			NegativeFactors: geminiJSON.NegativeFactors,
		},
	}, nil
}
