package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"app/utils"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// HandleGetActivePromotionsForPOS retrieves active promotions for a shop.
func HandleGetActivePromotionsForPOS(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Query("shopId")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId query parameter is required"})
	}

	log.Printf("🔍 [POS PROMOTION REQUEST] Merchant: %s, Shop: %s", merchantID, shopID)

	// Verify merchant owns the shop
	var foundMerchantID string
	shopCheckQuery := "SELECT merchant_id FROM shops WHERE id = $1"
	if err := db.QueryRow(ctx, shopCheckQuery, shopID).Scan(&foundMerchantID); err != nil {
		log.Printf("❌ [POS PROMOTION] Shop not found: %s", shopID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}
	if foundMerchantID != merchantID {
		log.Printf("❌ [POS PROMOTION] Access denied. Shop %s belongs to merchant %s, not %s", shopID, foundMerchantID, merchantID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access to shop denied"})
	}

	log.Printf("✅ [POS PROMOTION] Shop ownership verified")

	// Get active promotions (shop-specific or merchant-wide)
	query := `
		SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, 
		       min_spend, start_date, end_date, is_active, created_at, updated_at
		FROM promotions
		WHERE merchant_id = $1
		  AND (shop_id IS NULL OR shop_id = $2)
		  AND is_active = TRUE
		  AND (start_date IS NULL OR start_date <= NOW())
		  AND (end_date IS NULL OR end_date >= NOW())
		ORDER BY created_at DESC
	`

	rows, err := db.Query(ctx, query, merchantID, shopID)
	if err != nil {
		log.Printf("Error fetching active promotions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch promotions"})
	}
	defer rows.Close()

	promotions := make([]models.Promotion, 0)
	for rows.Next() {
		var promo models.Promotion
		if err := rows.Scan(
			&promo.ID, &promo.MerchantID, &promo.ShopID, &promo.Name, &promo.Description,
			&promo.PromoType, &promo.PromoValue, &promo.MinSpend, &promo.StartDate,
			&promo.EndDate, &promo.IsActive, &promo.CreatedAt, &promo.UpdatedAt,
		); err != nil {
			log.Printf("❌ [POS PROMOTION] Error scanning promotion: %v", err)
			continue
		}

		// Log each promotion found
		shopDisplay := "merchant-wide"
		if promo.ShopID != nil {
			shopDisplay = fmt.Sprintf("shop: %s", *promo.ShopID)
		}
		dateDisplay := "always available"
		if promo.StartDate != nil || promo.EndDate != nil {
			if promo.StartDate != nil && promo.EndDate != nil {
				dateDisplay = fmt.Sprintf("%s to %s", promo.StartDate.Format("2006-01-02"), promo.EndDate.Format("2006-01-02"))
			} else if promo.StartDate != nil {
				dateDisplay = fmt.Sprintf("from %s", promo.StartDate.Format("2006-01-02"))
			} else {
				dateDisplay = fmt.Sprintf("until %s", promo.EndDate.Format("2006-01-02"))
			}
		}
		minSpendDisplay := "no minimum"
		if promo.MinSpend > 0 {
			minSpendDisplay = fmt.Sprintf("min $%.2f", promo.MinSpend)
		}

		log.Printf("   ✓ %s: %s %.0f%s (%s, %s, %s)",
			promo.ID, promo.Name, promo.PromoValue,
			map[string]string{"percentage": "%", "fixed_amount": "$"}[promo.PromoType],
			shopDisplay, dateDisplay, minSpendDisplay)

		promotions = append(promotions, promo)
	}

	// Debug logging
	log.Printf("📋 [POS PROMOTION RESULT] Found %d active promotion(s)", len(promotions))
	if len(promotions) == 0 {
		// Check total promotions for debugging
		var totalCount int
		countQuery := "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1"
		db.QueryRow(ctx, countQuery, merchantID).Scan(&totalCount)
		log.Printf("⚠️  [POS PROMOTION] No active promotions found. Total promotions for merchant: %d", totalCount)

		if totalCount > 0 {
			log.Printf("🔎 [POS PROMOTION] Analyzing why promotions were filtered out:")

			// Show why promotions were filtered out
			var inactiveCount, expiredCount, notStartedCount, wrongShopCount int
			db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1 AND is_active = FALSE", merchantID).Scan(&inactiveCount)
			db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1 AND (end_date IS NOT NULL AND end_date < NOW())", merchantID).Scan(&expiredCount)
			db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1 AND (start_date IS NOT NULL AND start_date > NOW())", merchantID).Scan(&notStartedCount)
			db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions WHERE merchant_id = $1 AND shop_id IS NOT NULL AND shop_id != $2", merchantID, shopID).Scan(&wrongShopCount)

			log.Printf("   ❌ Inactive (is_active=false): %d", inactiveCount)
			log.Printf("   ⏱️  Not started yet (start_date in future): %d", notStartedCount)
			log.Printf("   📅 Expired (end_date passed): %d", expiredCount)
			log.Printf("   🏪 Wrong shop (different shop_id): %d", wrongShopCount)

			// Show actual promotion details for debugging
			debugQuery := `SELECT id, name, is_active, start_date, end_date, shop_id FROM promotions WHERE merchant_id = $1`
			debugRows, _ := db.Query(ctx, debugQuery, merchantID)
			if debugRows != nil {
				defer debugRows.Close()
				log.Printf("   📝 All promotions for this merchant:")
				for debugRows.Next() {
					var id, name string
					var isActive bool
					var startDate, endDate *time.Time
					var shopIDDebug *string
					debugRows.Scan(&id, &name, &isActive, &startDate, &endDate, &shopIDDebug)
					shopStr := "all shops"
					if shopIDDebug != nil {
						shopStr = *shopIDDebug
					}
					log.Printf("      • %s: active=%v, start=%v, end=%v, shop=%s",
						name, isActive, startDate, endDate, shopStr)
				}
			}
		}
	}

	return c.JSON(fiber.Map{"status": "success", "data": promotions})
}

// HandleSearchProductsForPOS handles searching for products in a specific shop's inventory.
func HandleSearchProductsForPOS(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	shopID := c.Query("shopId")
	searchTerm := c.Query("searchTerm")

	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId query parameter is required"})
	}

	// Ensure the requesting user has access to this shop
	var foundMerchantID string
	shopCheckQuery := "SELECT merchant_id FROM shops WHERE id = $1"
	if err := db.QueryRow(ctx, shopCheckQuery, shopID).Scan(&foundMerchantID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
	}
	if foundMerchantID != merchantID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access to shop denied"})
	}

	query := `
        SELECT
            i.id, i.merchant_id, i.name, i.description, i.sku,
            i.selling_price, i.original_price, i.low_stock_threshold,
            i.category, i.supplier_id, i.is_archived, i.created_at, i.updated_at,
            ss.quantity as stock_quantity
        FROM inventory_items i
        INNER JOIN shop_stock ss ON i.id = ss.inventory_item_id
        WHERE i.merchant_id = $1
          AND ss.shop_id = $2
          AND ss.quantity > 0
          AND i.is_archived = FALSE
          AND (i.name ILIKE $3 OR i.sku ILIKE $3)
        ORDER BY i.name ASC
        LIMIT 50
    `
	likeTerm := "%" + searchTerm + "%"

	rows, err := db.Query(ctx, query, merchantID, shopID, likeTerm)
	if err != nil {
		log.Printf("Error searching for POS products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database query failed"})
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var item models.InventoryItem
		var stockQuantity int
		if err := rows.Scan(
			&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU,
			&item.SellingPrice, &item.OriginalPrice, &item.LowStockThreshold,
			&item.Category, &item.SupplierID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt,
			&stockQuantity,
		); err != nil {
			log.Printf("Error scanning product item: %v", err)
			continue
		}

		// Include stock info in the response
		itemWithStock := fiber.Map{
			"id":                item.ID,
			"merchantId":        item.MerchantID,
			"name":              item.Name,
			"description":       item.Description,
			"sku":               item.SKU,
			"sellingPrice":      item.SellingPrice,
			"originalPrice":     item.OriginalPrice,
			"lowStockThreshold": item.LowStockThreshold,
			"category":          item.Category,
			"supplierId":        item.SupplierID,
			"isArchived":        item.IsArchived,
			"createdAt":         item.CreatedAt,
			"updatedAt":         item.UpdatedAt,
			"stockInfo": []fiber.Map{
				{
					"shopId":   shopID,
					"quantity": stockQuantity,
				},
			},
		}
		items = append(items, itemWithStock)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleCheckout processes a new sale in a transaction.
func HandleCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantID := claims.UserID

	var req models.CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	// If customer name is provided but no customer ID, create a new customer
	if req.CustomerName != nil && *req.CustomerName != "" && req.CustomerID == nil {
		customerID, err := createCustomerFromNameForMerchant(ctx, db, req.ShopID, merchantID, *req.CustomerName)
		if err != nil {
			log.Printf("Warning: Failed to create customer from name '%s': %v", *req.CustomerName, err)
			// Don't fail the checkout, just log the warning and continue without customer
		} else {
			req.CustomerID = &customerID
			log.Printf("Created new customer %s for name '%s'", customerID, *req.CustomerName)
		}
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not start database transaction"})
	}
	defer tx.Rollback(ctx)

	// Validate promotion if provided
	if req.AppliedPromotionID != nil && *req.AppliedPromotionID != "" {
		var promoActive bool
		var promoShopID *string
		var promoMerchantID string
		promoCheckQuery := `
			SELECT is_active, shop_id, merchant_id 
			FROM promotions 
			WHERE id = $1 
			AND (start_date IS NULL OR start_date <= NOW()) 
			AND (end_date IS NULL OR end_date >= NOW())
		`
		err := tx.QueryRow(ctx, promoCheckQuery, *req.AppliedPromotionID).Scan(&promoActive, &promoShopID, &promoMerchantID)
		if err != nil {
			log.Printf("Promotion %s not found or expired: %v", *req.AppliedPromotionID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid or expired promotion"})
		}
		if !promoActive {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Promotion is not active"})
		}
		if promoMerchantID != merchantID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Promotion does not belong to this merchant"})
		}
		// Promotion is valid if: it's a merchant-level promotion (shop_id IS NULL) OR it matches the shop
		if promoShopID != nil && *promoShopID != req.ShopID {
			log.Printf("Promotion shop validation failed: promo shop_id=%s, request shop_id=%s", *promoShopID, req.ShopID)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Promotion is not valid for this shop"})
		}
	}

	// Create the main Sale record with promotion support
	saleID := ""
	saleQuery := `
		INSERT INTO sales (shop_id, merchant_id, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, customer_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id
	`
	paymentStatus := "succeeded" // Assume success for now
	err = tx.QueryRow(ctx, saleQuery, req.ShopID, merchantID, req.TotalAmount, req.AppliedPromotionID, req.DiscountAmount, req.PaymentType, paymentStatus, req.StripePaymentIntentID, req.CustomerID).Scan(&saleID)
	if err != nil {
		log.Printf("Failed to create sale record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record sale"})
	}

	// Process each item in the sale
	for _, item := range req.Items {
		// 0. Fetch item details from inventory_items for denormalization
		var itemName string
		var itemSKU *string
		var originalPrice *float64
		itemQuery := `SELECT name, sku, original_price FROM inventory_items WHERE id = $1`
		err := tx.QueryRow(ctx, itemQuery, item.ProductID).Scan(&itemName, &itemSKU, &originalPrice)
		if err != nil {
			log.Printf("Failed to fetch inventory item details for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Product %s not found", item.ProductID)})
		}

		// 1. Decrement stock and check for sufficiency
		var newQuantity int
		stockUpdateQuery := `
			UPDATE shop_stock
			SET quantity = quantity - $1
			WHERE shop_id = $2 AND inventory_item_id = $3 AND quantity >= $1
			RETURNING quantity
		`
		err = tx.QueryRow(ctx, stockUpdateQuery, item.Quantity, req.ShopID, item.ProductID).Scan(&newQuantity)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Insufficient stock for product ID: %s", item.ProductID)})
			}
			log.Printf("Failed to update stock for item %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		// 2. Create the sale_items record
		saleItemQuery := `
			INSERT INTO sale_items (sale_id, inventory_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		subtotal := float64(item.Quantity) * item.SellingPriceAtSale
		_, err = tx.Exec(ctx, saleItemQuery, saleID, item.ProductID, itemName, itemSKU, item.Quantity, item.SellingPriceAtSale, originalPrice, subtotal)
		if err != nil {
			log.Printf("Failed to create sale_item record for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record sale item details"})
		}

		// 3. Create a stock movement record
		stockMovementQuery := `
			INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
			VALUES ($1, $2, $3, 'sale', $4, $5, $6)
		`
		reason := fmt.Sprintf("Sale #%s", saleID)
		_, err = tx.Exec(ctx, stockMovementQuery, item.ProductID, req.ShopID, merchantID, -item.Quantity, newQuantity, reason)
		if err != nil {
			log.Printf("Failed to create stock movement record for product %s: %v", item.ProductID, err)
			// This is a non-critical error for the customer, but we must log it.
		}
	}

	// Generate invoice number and create invoice (merchant POS should create an invoice)
	invoiceNumber, err := utils.GenerateInvoiceNumber(ctx, tx)
	if err != nil {
		log.Printf("Error generating invoice number (merchant POS): %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to generate invoice number"})
	}

	// Calculate invoice amounts
	discountAmount := req.DiscountAmount
	subtotal := req.TotalAmount + discountAmount
	taxAmount := 0.0

	invoiceQuery := `
		INSERT INTO invoices (
			sale_id, invoice_number, merchant_id, shop_id, customer_id,
			invoice_date, subtotal, discount_amount, tax_amount, total_amount, payment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.Exec(ctx, invoiceQuery,
		saleID, invoiceNumber, merchantID, req.ShopID, req.CustomerID,
		time.Now(), subtotal, discountAmount, taxAmount, req.TotalAmount, "paid",
	)
	if err != nil {
		log.Printf("Error creating invoice (merchant POS): %v; params: saleID=%s invoiceNumber=%s merchantID=%s shopID=%s customerID=%v subtotal=%.2f discount=%.2f tax=%.2f total=%.2f",
			err, saleID, invoiceNumber, merchantID, req.ShopID, req.CustomerID, subtotal, discountAmount, taxAmount, req.TotalAmount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create invoice"})
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to finalize sale"})
	}

	// Debug: note that this merchant POS checkout endpoint does not create an invoice
	log.Printf("📄 [MERCHANT POS] Checkout committed for saleID=%s shopID=%s total=%.2f. (This handler does not create an invoice.)",
		saleID, req.ShopID, req.TotalAmount)

	// Re-fetch the created sale with its items to return to the client
	createdSale, err := getSaleByID(ctx, db, saleID)
	if err != nil {
		log.Printf("Failed to fetch created sale %s: %v", saleID, err)
		// The sale was successful, so we return a success message even if re-fetch fails.
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "message": "Sale completed successfully"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": createdSale})
}

// getSaleByID is a helper function to fetch a sale and its items.
func getSaleByID(ctx context.Context, db *pgxpool.Pool, saleID string) (*models.Sale, error) {
	var sale models.Sale
	saleQuery := `SELECT id, shop_id, merchant_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at FROM sales WHERE id = $1`
	err := db.QueryRow(ctx, saleQuery, saleID).Scan(
		&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.SaleDate, &sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	itemsQuery := `SELECT id, sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, item_name, item_sku, created_at, updated_at FROM sale_items WHERE sale_id = $1`
	rows, err := db.Query(ctx, itemsQuery, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sale.Items = make([]models.SaleItem, 0)
	for rows.Next() {
		var item models.SaleItem
		if err := rows.Scan(&item.ID, &item.SaleID, &item.InventoryItemID, &item.QuantitySold, &item.SellingPriceAtSale, &item.OriginalPriceAtSale, &item.Subtotal, &item.ItemName, &item.ItemSKU, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}

	return &sale, nil
}

// Helper function to create a customer from a name for merchants
func createCustomerFromNameForMerchant(ctx context.Context, db *pgxpool.Pool, shopID, merchantID, name string) (string, error) {
	var customerID string
	query := `
		INSERT INTO shop_customers (merchant_id, shop_id, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := db.QueryRow(ctx, query, merchantID, shopID, name).Scan(&customerID)
	return customerID, err
}
