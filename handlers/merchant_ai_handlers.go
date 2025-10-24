package handlers

import (
	"app/database"
	"app/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// HandleAIAssistant provides AI-powered insights based on a user's prompt.
func HandleAIAssistant(c *fiber.Ctx) error {
	var req models.AIAssistantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	// 1. Classify the user's intent
	intent, err := classifyIntent(req.Prompt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// 2. Fetch data based on the intent
	data, err := fetchDataForIntent(intent, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// 3. Generate a human-readable analysis
	analysis, err := generateAnalysis(req.Prompt, intent, data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "analysis": analysis})
}

// classifyIntent uses Gemini to determine the user's intent.
func classifyIntent(prompt string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		return "", fmt.Errorf("failed to create AI client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")
	classificationPrompt := fmt.Sprintf(
		`You are an intent classification system. Classify the user's prompt into one of the following categories: 'best_sellers', 'highest_sales', 'peak_hours', or 'unknown'. The user prompt is: "%s"`,
		prompt,
	)

	resp, err := model.GenerateContent(ctx, genai.Text(classificationPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to classify intent: %w", err)
	}

	intent := strings.TrimSpace(fmt.Sprint(resp.Candidates[0].Content.Parts[0]))
	if intent == "best_sellers" || intent == "highest_sales" || intent == "peak_hours" {
		return intent, nil
	}
	return "unknown", nil
}

// fetchDataForIntent queries the database based on the classified intent.
func fetchDataForIntent(intent, merchantID string) (interface{}, error) {
	db := database.GetDB()
	ctx := context.Background()

	switch intent {
	case "best_sellers":
		query := `
            SELECT i.name, SUM(si.quantity_sold) as total_sold
            FROM sale_items si
            JOIN inventory_items i ON si.inventory_item_id = i.id
            JOIN sales s ON si.sale_id = s.id
            WHERE s.merchant_id = $1
            GROUP BY i.name
            ORDER BY total_sold DESC
            LIMIT 10
        `
		rows, err := db.Query(ctx, query, merchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to query best sellers: %w", err)
		}
		defer rows.Close()

		var bestSellers []models.BestSeller
		for rows.Next() {
			var seller models.BestSeller
			if err := rows.Scan(&seller.ProductName, &seller.TotalSold); err != nil {
				continue
			}
			bestSellers = append(bestSellers, seller)
		}
		return bestSellers, nil

	case "highest_sales":
		query := `
            SELECT sh.name, SUM(s.total_amount) as total_sales
            FROM sales s
            JOIN shops sh ON s.shop_id = sh.id
            WHERE s.merchant_id = $1
            GROUP BY sh.name
            ORDER BY total_sales DESC
            LIMIT 10
        `
		rows, err := db.Query(ctx, query, merchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to query highest sales: %w", err)
		}
		defer rows.Close()

		var shopSales []models.ShopSales
		for rows.Next() {
			var shop models.ShopSales
			if err := rows.Scan(&shop.ShopName, &shop.TotalSales); err != nil {
				continue
			}
			shopSales = append(shopSales, shop)
		}
		return shopSales, nil

	case "peak_hours":
		query := `
            SELECT EXTRACT(HOUR FROM sale_date) as hour, COUNT(*) as total_sales
            FROM sales
            WHERE merchant_id = $1
            GROUP BY hour
            ORDER BY total_sales DESC
        `
		rows, err := db.Query(ctx, query, merchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to query peak hours: %w", err)
		}
		defer rows.Close()

		var peakHours []models.PeakHour
		for rows.Next() {
			var hour models.PeakHour
			if err := rows.Scan(&hour.Hour, &hour.TotalSales); err != nil {
				continue
			}
			peakHours = append(peakHours, hour)
		}
		return peakHours, nil
	}

	return nil, nil // No data for 'unknown' intent
}

// generateAnalysis uses Gemini to create a human-readable analysis.
func generateAnalysis(originalPrompt, intent string, data interface{}) (string, error) {
	if intent == "unknown" {
		return "Sorry, I can't answer that question yet. Try asking about 'best sellers', 'highest sales', or 'peak hours'.", nil
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		return "", fmt.Errorf("failed to create AI client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize data: %w", err)
	}

	analysisPrompt := fmt.Sprintf(
		`You are a helpful AI assistant for a retail business. The user asked: "%s". The intent of the query was determined to be '%s'. Based on the following data, provide a concise and helpful analysis:

		Data: %s`,
		originalPrompt,
		intent,
		string(jsonData),
	)

	resp, err := model.GenerateContent(ctx, genai.Text(analysisPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate analysis: %w", err)
	}

	return fmt.Sprint(resp.Candidates[0].Content.Parts[0]), nil
}
