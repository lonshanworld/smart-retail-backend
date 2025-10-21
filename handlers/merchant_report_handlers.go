package handlers

import (
	"app/database"
	"app/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/genai"
	"google.golang.org/api/option"
)

// HandleGetSalesReport generates a sales report based on filters.
func HandleGetSalesReport(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	// Parse query parameters
	startDate := c.Query("startDate", time.Now().AddDate(0, -1, 0).Format(time.RFC3339))
	endDate := c.Query("endDate", time.Now().Format(time.RFC3339))
	shopID := c.Query("shopId")
	groupBy := c.Query("groupBy", "daily") // daily, weekly, monthly

	// Base query
	query := `
        SELECT 
            date_trunc($4, sale_date) as period,
            SUM(total_amount) as total_sales
        FROM sales
        WHERE merchant_id = $1 AND sale_date BETWEEN $2 AND $3
    `
	args := []interface{}{merchantID, startDate, endDate, groupBy}

	// Add shop filter if provided
	if shopID != "" {
		query += " AND shop_id = $5"
		args = append(args, shopID)
	}

	query += " GROUP BY period ORDER BY period"

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error getting sales report: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate sales report"})
	}
	defer rows.Close()

	var results []models.SalesReportData
	for rows.Next() {
		var r models.SalesReportData
		if err := rows.Scan(&r.Period, &r.TotalSales); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to process sales report data"})
		}
		results = append(results, r)
	}

	return c.JSON(fiber.Map{"success": true, "data": results})
}

// HandleGetSalesForecast generates a sales forecast for a product using Gemini.
func HandleGetSalesForecast(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

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

	model := client.GenerativeModel("gemini-1.5-pro")
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:    genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:    genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:    genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:    genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Error from Gemini API: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate forecast"})
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
	// Simplified: In a real app, you'd format the data more carefully
	dataStr := ""
	for _, d := range data {
		dataStr += fmt.Sprintf("On %s, %d units were sold.\n", d.SaleDate.Format("2006-01-02"), d.QuantitySold)
	}
	if dataStr == "" {
		dataStr = "No sales data available for the last 90 days."
	}

	return fmt.Sprintf(`
        As a retail data analyst, generate a 7-day sales forecast and analysis.

        **Context:**
        - Product: %s
        - Shop: %s
        - Current Stock: %d units
        - Today's Date: %s

        **Historical Sales Data (last 90 days):**
        %s

        **Your Task:**
        Provide a JSON object with the following structure. Do not include any introductory text or backticks.
        - `+"`forecast`"+`: An array of 7 objects, each with `+"`date`"+` (YYYY-MM-DD) and `+"`predicted_sales`"+` (integer).
        - `+"`summary`"+`: A brief, data-driven summary of the forecast.
        - `+"`positive_factors`"+`: An array of strings listing factors that could boost sales.
        - `+"`negative_factors`"+`: An array of strings listing risks or factors that could hurt sales.

        **Example JSON Output:**
        {
          "forecast": [
            { "date": "2023-11-25", "predicted_sales": 15 },
            { "date": "2023-11-26", "predicted_sales": 18 }
          ],
          "summary": "Sales are expected to be strong over the weekend.",
          "positive_factors": ["Upcoming holiday", "Recent marketing campaign"],
          "negative_factors": ["Potential stock shortages", "Competitor sale"]
        }
    `, productName, shopName, currentStock, time.Now().Format("2006-01-02"), dataStr)
}

// parseGeminiResponse parses the JSON from Gemini into a structured response.
func parseGeminiResponse(resp *genai.GenerateContentResponse, productName, shopName string, currentStock int) (*models.SalesForecastResponse, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content received from AI")
	}

	geminiText, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected AI response format")
	}

	var geminiJSON struct {
		Forecast        []models.DailyForecast `json:"forecast"`
		Summary         string               `json:"summary"`
		PositiveFactors []string             `json:"positive_factors"`
		NegativeFactors []string             `json:"negative_factors"`
	}

	if err := json.Unmarshal([]byte(geminiText), &geminiJSON); err != nil {
		log.Printf("Error parsing Gemini JSON: %v\nRaw Response: %s", err, geminiText)
		return nil, fmt.Errorf("failed to parse AI forecast data")
	}

	return &models.SalesForecastResponse{
		ReportName:     "7-Day Sales Forecast",
		GeneratedAt:    time.Now(),
		ProductName:    productName,
		ShopName:       shopName,
		CurrentStock:   currentStock,
		ForecastPeriod: models.ForecastPeriod{
			StartDate: time.Now(),
			EndDate:   time.Now().AddDate(0, 0, 7),
		},
		DailyForecast:  geminiJSON.Forecast,
		AiAnalysis: models.AiAnalysis{
			Summary:         geminiJSON.Summary,
			PositiveFactors: geminiJSON.PositiveFactors,
			NegativeFactors: geminiJSON.NegativeFactors,
		},
	}, nil
}
