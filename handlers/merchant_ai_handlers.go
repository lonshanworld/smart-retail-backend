package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleAIAssistant provides AI-powered insights based on a user's prompt.
// It uses Gemini to generate safe SQL queries from natural language.
func HandleAIAssistant(c *fiber.Ctx) error {
	var req models.AIAssistantRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to extract claims: %v", err)
		return err
	}
	merchantID := claims.UserID
	provider := resolveAIProvider(req.Provider)

	log.Printf("🤖 [AI ASSISTANT] Starting request")
	log.Printf("   Merchant ID: %s", merchantID)
	log.Printf("   Provider: %s", provider)
	log.Printf("   User Prompt: %s", req.Prompt)

	// 1. Use AI to generate SQL query from natural language
	log.Printf("🔄 [AI ASSISTANT] Step 1: Generating SQL from prompt...")
	sqlQuery, err := generateSQLFromPrompt(req.Prompt, merchantID, provider)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to generate SQL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}
	log.Printf("✅ [AI ASSISTANT] Generated SQL: %s", sqlQuery)

	// 2. Validate the SQL query (security check)
	log.Printf("🔄 [AI ASSISTANT] Step 2: Validating SQL query...")
	if err := validateSQLQuery(sqlQuery); err != nil {
		log.Printf("❌ [AI ASSISTANT] SQL validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Invalid query: %s", err.Error())})
	}
	log.Printf("✅ [AI ASSISTANT] SQL validation passed")

	// 3. Execute the query safely
	log.Printf("🔄 [AI ASSISTANT] Step 3: Executing SQL query...")
	queryResult, err := executeSafeQuery(sqlQuery)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Query execution failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Query execution failed: %s", err.Error())})
	}
	log.Printf("✅ [AI ASSISTANT] Query executed successfully, returned %d rows", len(queryResult))

	// 4. Use AI to format the results in a human-readable way
	log.Printf("🔄 [AI ASSISTANT] Step 4: Formatting results with AI...")
	analysisHTML, err := formatResultsWithAI(req.Prompt, queryResult, provider)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to format results: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": err.Error()})
	}
	log.Printf("✅ [AI ASSISTANT] Analysis generated successfully")

	log.Printf("🎉 [AI ASSISTANT] Request completed successfully")
	return c.JSON(fiber.Map{"success": true, "analysis": stripHTMLTags(analysisHTML), "analysis_html": analysisHTML, "analysis_format": "html", "provider": string(provider), "sql": sqlQuery, "data": queryResult})
}

// generateSQLFromPrompt uses Gemini to convert natural language to SQL
func generateSQLFromPrompt(prompt string, merchantID string, provider aiProvider) (string, error) {
	log.Printf("   🔍 [SQL GEN] Fetching available tables from database...")
	ctx := context.Background()
	db := database.GetDB()

	// Query actual table names from the database
	tableRows, err := db.Query(ctx, `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		log.Printf("   ⚠️  [SQL GEN] Failed to fetch table names: %v", err)
		// Continue with hardcoded schema as fallback
	} else {
		defer tableRows.Close()
		var tableNames []string
		for tableRows.Next() {
			var tableName string
			if err := tableRows.Scan(&tableName); err == nil {
				tableNames = append(tableNames, tableName)
			}
		}
		log.Printf("   ✅ [SQL GEN] Available tables: %v", tableNames)
	}

	// Database schema information (from actual schema.sql)
	schemaInfo := `
Database Schema:
- users (id, name, email, password_hash, phone, role, is_active, merchant_id, assigned_shop_id, created_at, updated_at)
- shops (id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at)
- staff_contracts (id, staff_id, salary, pay_frequency, start_date, end_date, is_active, created_at, updated_at)
- suppliers (id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at)
- inventory_items (id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at)
- shop_stock (id, shop_id, inventory_item_id, quantity, last_stocked_in_at)
- stock_movements (id, inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason, movement_date, notes)
- shop_customers (id, shop_id, merchant_id, name, email, phone, created_at, updated_at)
- promotions (id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, start_date, end_date, is_active, created_at, updated_at)
- promotion_products (promotion_id, inventory_item_id)
- sales (id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at)
- sale_items (id, sale_id, inventory_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, created_at, updated_at)
- salary_payments (id, staff_id, payment_date, amount_paid, payment_period_start, payment_period_end, payment_method, notes, created_at)
- notifications (id, recipient_user_id, title, message, notification_type, related_entity_type, related_entity_id, is_read, created_at)

CRITICAL - Table naming (use EXACT names from schema):
- Use "shop_stock" (SINGULAR), NOT "shop_stocks"
- Use "shop_customers" (plural), NOT "customers"
- Use "inventory_items" (plural), NOT "inventory"
- Use "sale_items" (plural), NOT "sale_item"

Important:
- The merchant_id for this query is: '` + merchantID + `'
- ALWAYS filter by merchant_id = '` + merchantID + `' to ensure data security
- Use PostgreSQL syntax
- Return ONLY a valid SELECT query, nothing else
- Do NOT use INSERT, UPDATE, DELETE, DROP, ALTER, or any DDL/DML commands
- Limit results to 100 rows maximum using LIMIT clause
`

	sqlPrompt := fmt.Sprintf(`%s

User Question: "%s"

Generate a PostgreSQL SELECT query that answers this question. 
Return ONLY the SQL query without any explanation, code blocks, or markdown.
The query MUST include WHERE merchant_id = '%s' for security.`, schemaInfo, prompt, merchantID)

	log.Printf("   🔍 [SQL GEN] Sending prompt to Gemini AI...")
	log.Printf("   📝 [SQL GEN] User question: %s", prompt)
	sqlQuery, err := generateAIText(ctx, provider, "You are a PostgreSQL expert. Return only one safe SELECT query.", sqlPrompt)
	if err != nil {
		log.Printf("   ❌ [SQL GEN] AI provider error: %v", err)
		return "", fmt.Errorf("failed to generate SQL: %w", err)
	}

	sqlQuery = strings.TrimSpace(sqlQuery)
	log.Printf("   📋 [SQL GEN] Raw AI response: %s", sqlQuery)

	// Clean up the SQL (remove markdown code blocks if present)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimPrefix(sqlQuery, "```")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	log.Printf("   ✨ [SQL GEN] Cleaned SQL: %s", sqlQuery)
	return sqlQuery, nil
}

// validateSQLQuery ensures the query is safe (only SELECT, no dangerous operations)
func validateSQLQuery(query string) error {
	log.Printf("   🔍 [SQL VALIDATE] Starting validation...")
	log.Printf("   📝 [SQL VALIDATE] Query: %s", query)

	queryUpper := strings.ToUpper(strings.TrimSpace(query))

	// Must start with SELECT
	if !strings.HasPrefix(queryUpper, "SELECT") {
		log.Printf("   ❌ [SQL VALIDATE] Query does not start with SELECT")
		return fmt.Errorf("only SELECT queries are allowed")
	}
	log.Printf("   ✅ [SQL VALIDATE] Query starts with SELECT")

	// Forbidden keywords
	forbiddenKeywords := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE",
		"CREATE", "REPLACE", "GRANT", "REVOKE", "EXEC", "EXECUTE",
		"--", "/*", "*/", ";--", "UNION", "INTO OUTFILE", "INTO DUMPFILE",
	}

	for _, keyword := range forbiddenKeywords {
		if strings.Contains(queryUpper, keyword) {
			log.Printf("   ❌ [SQL VALIDATE] Forbidden keyword detected: %s", keyword)
			return fmt.Errorf("forbidden keyword detected: %s", keyword)
		}
	}
	log.Printf("   ✅ [SQL VALIDATE] No forbidden keywords found")

	// Must contain merchant_id filter for security
	if !strings.Contains(queryUpper, "MERCHANT_ID") {
		log.Printf("   ❌ [SQL VALIDATE] Query missing merchant_id filter")
		return fmt.Errorf("query must filter by merchant_id for security")
	}
	log.Printf("   ✅ [SQL VALIDATE] merchant_id filter present")

	// Must have LIMIT clause
	if !strings.Contains(queryUpper, "LIMIT") {
		log.Printf("   ❌ [SQL VALIDATE] Query missing LIMIT clause")
		return fmt.Errorf("query must include LIMIT clause")
	}
	log.Printf("   ✅ [SQL VALIDATE] LIMIT clause present")

	log.Printf("   🎉 [SQL VALIDATE] All validations passed!")
	return nil
}

// executeSafeQuery runs the validated SQL query and returns results
func executeSafeQuery(query string) ([]map[string]interface{}, error) {
	log.Printf("   🔍 [SQL EXEC] Connecting to database...")
	db := database.GetDB()
	ctx := context.Background()

	log.Printf("   🔍 [SQL EXEC] Executing query...")
	rows, err := db.Query(ctx, query)
	if err != nil {
		log.Printf("   ❌ [SQL EXEC] Query failed: %v", err)
		return nil, fmt.Errorf("query execution error: %w", err)
	}
	defer rows.Close()
	log.Printf("   ✅ [SQL EXEC] Query executed successfully")

	// Get column names
	fieldDescriptions := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columnNames[i] = string(fd.Name)
	}
	log.Printf("   📋 [SQL EXEC] Column names: %v", columnNames)

	// Fetch all rows
	var results []map[string]interface{}
	rowCount := 0
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Printf("   ⚠️  [SQL EXEC] Failed to scan row %d: %v", rowCount, err)
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columnNames {
			row[col] = values[i]
		}
		results = append(results, row)
		rowCount++
	}

	log.Printf("   ✅ [SQL EXEC] Retrieved %d rows", rowCount)
	if rowCount > 0 {
		log.Printf("   📋 [SQL EXEC] Sample row 1: %+v", results[0])
	}
	return results, nil
}

// formatResultsWithAI uses the configured AI provider to create a HTML response.
func formatResultsWithAI(originalPrompt string, queryResults []map[string]interface{}, provider aiProvider) (string, error) {
	// Convert results to JSON string
	resultsJSON, _ := json.Marshal(queryResults)
	log.Printf("   📋 [FORMAT] Data size: %d bytes, %d rows", len(resultsJSON), len(queryResults))

	analysisPrompt := fmt.Sprintf(`
User asked: "%s"

Query returned this data:
%s

Return a single HTML fragment only. Use semantic HTML tags and inline CSS styles.
No markdown, no code fences, and no surrounding explanation.
The HTML should summarize the answer clearly with headings, short paragraphs, bullets, tables, or cards if useful.
If the data is empty, return a short HTML fragment explaining that no data was found.
`, originalPrompt, string(resultsJSON))
	log.Printf("   🔍 [FORMAT] Sending to AI provider for formatting...")
	resp, err := generateAIText(context.Background(), provider, "You are a retail business analyst that returns only HTML fragments.", analysisPrompt)
	if err != nil {
		log.Printf("   ❌ [FORMAT] AI provider error: %v", err)
		return "", fmt.Errorf("failed to generate analysis: %w", err)
	}

	analysis := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(resp, "```"), "```html"))
	log.Printf("   ✅ [FORMAT] Analysis generated successfully (%d chars)", len(analysis))
	return analysis, nil
}
