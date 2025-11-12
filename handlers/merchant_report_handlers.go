package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
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

	// Query to get actual sale records (not aggregated)
	query := `
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

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("❌ [SALES REPORT] Query error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate sales report"})
	}
	defer rows.Close()

	sales := make([]models.Sale, 0)
	for rows.Next() {
		var sale models.Sale
		if err := rows.Scan(
			&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.SaleDate,
			&sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount,
			&sale.PaymentType, &sale.PaymentStatus, &sale.CreatedAt, &sale.UpdatedAt,
		); err != nil {
			log.Printf("❌ [SALES REPORT] Scan error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to process sales report data"})
		}

		// Fetch sale items for each sale
		itemsQuery := `
			SELECT id, sale_id, inventory_item_id, item_name, item_sku, 
			       quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal,
			       created_at, updated_at
			FROM sale_items 
			WHERE sale_id = $1
		`
		itemRows, err := db.Query(ctx, itemsQuery, sale.ID)
		if err != nil {
			log.Printf("⚠️  [SALES REPORT] Failed to fetch items for sale %s: %v", sale.ID, err)
			sale.Items = []models.SaleItem{} // Empty items if fetch fails
		} else {
			items := make([]models.SaleItem, 0)
			for itemRows.Next() {
				var item models.SaleItem
				if err := itemRows.Scan(
					&item.ID, &item.SaleID, &item.InventoryItemID, &item.ItemName,
					&item.ItemSKU, &item.QuantitySold, &item.SellingPriceAtSale,
					&item.OriginalPriceAtSale, &item.Subtotal, &item.CreatedAt, &item.UpdatedAt,
				); err != nil {
					log.Printf("⚠️  [SALES REPORT] Failed to scan item: %v", err)
					continue
				}
				items = append(items, item)
			}
			itemRows.Close()
			sale.Items = items
		}

		sales = append(sales, sale)
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

	if shopID == "" || itemID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "shopId and itemId are required"})
	}

	// 1. Fetch historical sales data for the item
	query := `
        SELECT s.sale_date, si.quantity_sold
        FROM sales s
        JOIN sale_items si ON s.id = si.sale_id
        WHERE s.shop_id = $1 AND si.inventory_item_id = $2 AND s.merchant_id = $3
        AND s.sale_date >= NOW() - INTERVAL '90 days'
        ORDER BY s.sale_date
    `
	rows, err := db.Query(ctx, query, shopID, itemID, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to get historical data"})
	}
	defer rows.Close()

	var historicalData []models.HistoricalSale
	for rows.Next() {
		var hs models.HistoricalSale
		if err := rows.Scan(&hs.SaleDate, &hs.QuantitySold); err != nil {
			continue
		}
		historicalData = append(historicalData, hs)
	}

	// 2. Fetch product and shop details for context
	var productName, shopName string
	var currentStock int
	_ = db.QueryRow(ctx, "SELECT name FROM inventory_items WHERE id = $1", itemID).Scan(&productName)
	_ = db.QueryRow(ctx, "SELECT name FROM shops WHERE id = $1", shopID).Scan(&shopName)
	_ = db.QueryRow(ctx, "SELECT quantity FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2", shopID, itemID).Scan(&currentStock)

	// 3. Construct the prompt for the Gemini API
	prompt := constructForecastPrompt(productName, shopName, currentStock, historicalData)

	// 4. Call the Gemini API
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Printf("Error creating Gemini client: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to connect to AI service"})
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Error from Gemini API: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate forecast from AI"})
	}

	// 5. Parse the response and format for the frontend
	forecastResponse, err := parseGeminiResponse(resp, productName, shopName, currentStock)
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

	jsonFormat := `{"forecast":[{"date":"YYYY-MM-DD","predicted_sales":integer},...],"summary":"string","positive_factors":["string",...],"negative_factors":["string",...]}`

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

// parseGeminiResponse parses the JSON from Gemini into a structured response.
func parseGeminiResponse(resp *genai.GenerateContentResponse, productName, shopName string, currentStock int) (*models.SalesForecastResponse, error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content received from AI")
	}

	var geminiText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			geminiText += string(txt)
		}
	}

	if geminiText == "" {
		return nil, fmt.Errorf("no text content received from AI")
	}

	// Clean the response to get only the JSON object
	jsonStr := extractJSON(geminiText)
	if jsonStr == "" {
		log.Printf("Could not extract JSON from Gemini response: %s", geminiText)
		return nil, fmt.Errorf("failed to parse AI response format")
	}

	var geminiJSON struct {
		Forecast        []models.DailyForecast `json:"forecast"`
		Summary         string                 `json:"summary"`
		PositiveFactors []string               `json:"positive_factors"`
		NegativeFactors []string               `json:"negative_factors"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &geminiJSON); err != nil {
		log.Printf("Error parsing Gemini JSON: %v\nRaw JSON: %s", err, jsonStr)
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
			PositiveFactors: geminiJSON.PositiveFactors,
			NegativeFactors: geminiJSON.NegativeFactors,
		},
	}, nil
}
