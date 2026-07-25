package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	if strings.TrimSpace(req.Prompt) == "" || len(req.Prompt) > 1000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Prompt must contain between 1 and 1000 characters"})
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
	log.Printf("   User prompt accepted (%d characters)", len(req.Prompt))

	// 1. Use AI to generate SQL query from natural language
	log.Printf("🔄 [AI ASSISTANT] Step 1: Generating SQL from prompt...")
	sqlQuery, err := generateSQLFromPrompt(req.Prompt, merchantID, provider)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to generate SQL: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"success": false, "message": "AI provider is currently unavailable"})
	}
	log.Printf("✅ [AI ASSISTANT] Generated SQL accepted for validation")

	// 2. Validate the SQL query (security check)
	log.Printf("🔄 [AI ASSISTANT] Step 2: Validating SQL query...")
	if err := validateSQLQuery(sqlQuery, merchantID); err != nil {
		log.Printf("❌ [AI ASSISTANT] SQL validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Invalid query: %s", err.Error())})
	}
	log.Printf("✅ [AI ASSISTANT] SQL validation passed")

	// 3. Execute the query safely
	log.Printf("🔄 [AI ASSISTANT] Step 3: Executing SQL query...")
	queryResult, err := executeSafeQuery(sqlQuery)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Query execution failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Query execution failed"})
	}
	log.Printf("✅ [AI ASSISTANT] Query executed successfully, returned %d rows", len(queryResult))

	// 4. Use AI to format the results in a human-readable way
	log.Printf("🔄 [AI ASSISTANT] Step 4: Formatting results with AI...")
	analysisHTML, err := formatResultsWithAI(req.Prompt, queryResult, provider)
	if err != nil {
		log.Printf("❌ [AI ASSISTANT] Failed to format results: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"success": false, "message": "AI provider is currently unavailable"})
	}
	log.Printf("✅ [AI ASSISTANT] Analysis generated successfully")

	log.Printf("🎉 [AI ASSISTANT] Request completed successfully")
	return c.JSON(fiber.Map{"success": true, "analysis": stripHTMLTags(analysisHTML), "analysis_html": escapeAIHTML(analysisHTML), "analysis_format": "html", "provider": string(provider), "sql": sqlQuery, "data": queryResult})
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
- users (id, name, email, role, merchant_id, assigned_shop_id, is_active, created_at, updated_at)
- shops (id, merchant_id, name, address, phone, is_active, is_primary, created_at, updated_at)
- products (id, merchant_id, name, slug, product_type, brand_id, is_active, created_at, updated_at)
- product_variants (id, merchant_id, product_id, name, sku, barcode, attributes, is_active)
- product_categories (merchant_id, product_id, category_id)
- product_prices (id, merchant_id, product_id, variant_id, shop_id, price_type, cost_price, selling_price, starts_at, ends_at)
- stock_items (id, merchant_id, product_id, variant_id, name, sku, tracking_mode, base_unit_id, is_active)
- inventory_items (id, merchant_id, shop_id, product_id, stock_item_id, variant_id, quantity_on_hand, reserved_quantity, low_stock_threshold, is_active)
- inventory_batches (id, merchant_id, shop_id, inventory_item_id, product_id, stock_item_id, batch_code, quantity_received, quantity_remaining, expiry_date)
- inventory_movements (id, merchant_id, shop_id, inventory_item_id, product_id, stock_item_id, movement_type, quantity, base_quantity, unit_cost, reference_type, reference_id, movement_date)
- inventory_reservations (id, merchant_id, shop_id, inventory_item_id, quantity, status)
- barcode_registry (id, merchant_id, code, normalized_code, owner_type, owner_id, is_active)
- suppliers (id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at)
- promotions (id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, start_date, end_date, is_active)
- promotion_products (merchant_id, promotion_id, product_id)
- shop_customers (id, shop_id, merchant_id, name, email, phone, created_at, updated_at)
- sales (id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, notes, created_at, updated_at)
- sale_items (id, sale_id, inventory_item_id, product_id, variant_id, stock_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
- invoices (id, merchant_id, shop_id, sale_id, invoice_number, payment_status, total_amount, invoice_date)
- payments (id, sale_id, method, amount, status, reference, created_at)
- purchase_orders (id, merchant_id, shop_id, supplier_id, status, total_amount, ordered_at)
- purchase_order_items (id, purchase_order_id, stock_item_id, quantity, unit_cost)
- goods_receipts (id, merchant_id, shop_id, purchase_order_id, supplier_id, status, received_at)
- goods_receipt_items (id, goods_receipt_id, stock_item_id, quantity_received, unit_cost)
- accounts (id, merchant_id, shop_id, code, name, account_type, is_active)
- journal_entries (id, merchant_id, shop_id, entry_date, reference_type, reference_id, description)
- journal_lines (id, journal_entry_id, account_id, debit, credit)

CRITICAL - Table naming (use EXACT names from schema):
- Use "inventory_items" for shop-level stock balances
- Use "inventory_movements" for stock adjustments and stock history
- Use "shop_customers" for customer records
- Use "sale_items" for sale line items

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
	log.Printf("   📝 [SQL GEN] User question accepted (%d characters)", len(prompt))
	sqlQuery, err := generateAIText(ctx, provider, "You are a PostgreSQL expert. Return only one safe SELECT query.", sqlPrompt)
	if err != nil {
		log.Printf("   ❌ [SQL GEN] AI provider error: %v", err)
		return "", fmt.Errorf("failed to generate SQL: %w", err)
	}

	sqlQuery = strings.TrimSpace(sqlQuery)
	log.Printf("   📋 [SQL GEN] AI response received (%d characters)", len(sqlQuery))

	// Clean up the SQL (remove markdown code blocks if present)
	sqlQuery = strings.TrimPrefix(sqlQuery, "```sql")
	sqlQuery = strings.TrimPrefix(sqlQuery, "```")
	sqlQuery = strings.TrimSuffix(sqlQuery, "```")
	sqlQuery = strings.TrimSpace(sqlQuery)

	log.Printf("   ✨ [SQL GEN] Cleaned SQL: %s", sqlQuery)
	return sqlQuery, nil
}

// validateSQLQuery ensures the query is safe (only SELECT, no dangerous operations)
func validateSQLQuery(query string, merchantID string) error {
	query = strings.TrimSpace(query)
	// A single trailing semicolon is a normal SQL statement terminator. Strip
	// it before checking for embedded semicolons, which would indicate more
	// than one statement.
	query = strings.TrimSpace(strings.TrimSuffix(query, ";"))
	if query == "" || len(query) > 4000 {
		return fmt.Errorf("query is empty or too large")
	}
	if !regexp.MustCompile(`(?i)^SELECT\b`).MatchString(query) ||
		strings.Contains(query, ";") || strings.Contains(query, "\x00") ||
		strings.Contains(query, "--") || strings.Contains(query, "/*") || strings.Contains(query, "*/") ||
		regexp.MustCompile(`(?is)\bWITH\b|\(\s*SELECT\b`).MatchString(query) {
		return fmt.Errorf("only one plain SELECT query is allowed")
	}

	for _, keyword := range []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "CREATE",
		"REPLACE", "GRANT", "REVOKE", "EXEC", "EXECUTE", "UNION", "OR", "PG_SLEEP",
	} {
		if regexp.MustCompile(`(?i)\b` + keyword + `\b`).MatchString(query) {
			return fmt.Errorf("forbidden SQL operation")
		}
	}
	if !regexp.MustCompile(`(?i)\bWHERE\b`).MatchString(query) {
		return fmt.Errorf("query must include a WHERE clause")
	}
	merchantFilter := regexp.MustCompile(`(?i)\bmerchant_id\s*=\s*'` + regexp.QuoteMeta(merchantID) + `'`)
	if !merchantFilter.MatchString(query) {
		return fmt.Errorf("query must be scoped to the authenticated merchant")
	}
	limitMatch := regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)`).FindStringSubmatch(query)
	if len(limitMatch) != 2 {
		return fmt.Errorf("query must include LIMIT between 1 and 100")
	}
	limit, limitErr := strconv.Atoi(limitMatch[1])
	if limitErr != nil || limit < 1 || limit > 100 {
		return fmt.Errorf("query must include LIMIT between 1 and 100")
	}
	allowedTables := map[string]bool{
		"users": true, "shops": true, "products": true, "product_variants": true,
		"product_categories": true, "product_prices": true, "stock_items": true,
		"inventory_items": true, "inventory_batches": true,
		"inventory_movements": true, "inventory_reservations": true,
		"barcode_registry": true, "suppliers": true, "promotions": true,
		"promotion_products": true, "sales": true, "sale_items": true,
		"shop_customers": true, "invoices": true, "payments": true,
		"purchase_orders": true, "purchase_order_items": true,
		"goods_receipts": true, "goods_receipt_items": true, "accounts": true,
		"journal_entries": true, "journal_lines": true,
	}
	for _, table := range regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-z_][a-z0-9_]*)`).FindAllStringSubmatch(query, -1) {
		if len(table) != 2 || !allowedTables[strings.ToLower(table[1])] {
			return fmt.Errorf("query references a table outside the reporting schema")
		}
	}
	return nil
}

// executeSafeQuery runs the validated SQL query and returns results
func executeSafeQuery(query string) ([]map[string]interface{}, error) {
	log.Printf("   🔍 [SQL EXEC] Connecting to database...")
	db := database.GetDB()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("   🔍 [SQL EXEC] Executing query...")
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("query transaction error: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
		return nil, fmt.Errorf("query timeout configuration error: %w", err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL transaction_read_only = on`); err != nil {
		return nil, fmt.Errorf("query read-only configuration error: %w", err)
	}
	rows, err := tx.Query(ctx, query)
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query iteration error: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("query commit error: %w", err)
	}
	log.Printf("   ✅ [SQL EXEC] Retrieved %d rows", rowCount)
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
