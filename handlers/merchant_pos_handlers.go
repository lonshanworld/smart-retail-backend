package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"app/utils"
	"context"
	"fmt"
	"log"
	"strings"
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
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
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
	search := strings.TrimSpace(c.Query("search"))
	where := " WHERE merchant_id = $1 AND (shop_id IS NULL OR shop_id = $2) AND is_active = TRUE AND (start_date IS NULL OR start_date <= NOW()) AND (end_date IS NULL OR end_date >= NOW())"
	args := []interface{}{merchantID, shopID}
	if search != "" {
		where += " AND (name ILIKE $3 OR description ILIKE $3)"
		args = append(args, "%"+search+"%")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions"+where, args...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count promotions"})
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
		LIMIT $3 OFFSET $4
	`

	query = "SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, start_date, end_date, is_active, created_at, updated_at FROM promotions" + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := db.Query(ctx, query, args...)
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
			debugQuery := `SELECT id, name, is_active, start_date, end_date, shop_id FROM promotions WHERE merchant_id = $1 ORDER BY created_at DESC LIMIT 100`
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

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": promotions, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize, "hasNext": page*pageSize < total}})
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

	// optional filters
	categoryId := c.Query("categoryId")
	subcategoryId := c.Query("subcategoryId")
	brandId := c.Query("brandId")
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

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

	// Build dynamic query with optional filters
	baseQuery := `
		SELECT si.id, si.merchant_id, si.name, p.description, si.sku,
			COALESCE(pp.selling_price, 0), COALESCE(pp.cost_price, 0),
			NULL, NULL, p.brand_id, NOT p.is_active, si.created_at, si.updated_at,
			ii.quantity_on_hand
		FROM inventory_items ii
		JOIN stock_items si ON si.id = ii.stock_item_id
		JOIN products p ON p.id = si.product_id
		LEFT JOIN LATERAL (SELECT selling_price, cost_price FROM product_prices
			WHERE product_id = si.product_id AND shop_id IS NULL AND price_type = 'RETAIL'
			ORDER BY created_at DESC LIMIT 1) pp ON TRUE
		WHERE ii.merchant_id = $1 AND ii.shop_id = $2 AND ii.quantity_on_hand > 0
		  AND p.is_active = TRUE AND (si.name ILIKE $3 OR si.sku ILIKE $3)
	`

	args := []interface{}{merchantID, shopID, "%" + searchTerm + "%"}
	argPos := 4
	if categoryId != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM product_categories pc WHERE pc.product_id = si.product_id AND pc.category_id = $%d)", argPos)
		args = append(args, categoryId)
		argPos++
	}
	if subcategoryId != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM product_categories pc2 WHERE pc2.product_id=si.product_id AND pc2.category_id=$%d)", argPos)
		args = append(args, subcategoryId)
		argPos++
	}
	if brandId != "" {
		baseQuery += fmt.Sprintf(" AND p.brand_id = $%d", argPos)
		args = append(args, brandId)
		argPos++
	}
	countQuery := "SELECT COUNT(*) " + baseQuery[strings.Index(baseQuery, "FROM"):]
	var total int
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count POS products"})
	}

	finalQuery := baseQuery + fmt.Sprintf("\n        ORDER BY si.created_at DESC, si.id DESC\n        LIMIT %d OFFSET %d\n    ", pageSize, (page-1)*pageSize)

	rows, err := db.Query(ctx, finalQuery, args...)
	if err != nil {
		log.Printf("Error searching for POS products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database query failed"})
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var item models.InventoryItem
		var stockQuantity float64
		if err := rows.Scan(
			&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU,
			&item.SellingPrice, &item.OriginalPrice, &item.LowStockThreshold,
			&item.CategoryID, &item.BrandID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt,
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

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize, "hasNext": page*pageSize < total}})
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
	if req.ShopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	if err := authorizeShopAccess(c, req.ShopID); err != nil {
		return err
	}
	clientSaleID := strings.TrimSpace(req.ClientSaleID)
	if clientSaleID == "" {
		clientSaleID = strings.TrimSpace(req.ID)
	}
	if clientSaleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientSaleId is required"})
	}
	if len(req.Items) == 0 || len(req.Items) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Between 1 and 100 sale items are required"})
	}
	if req.TotalAmount < 0 || req.DiscountAmount < 0 || req.TaxAmount < 0 || req.DeliveryCharge < 0 || req.DiscountAmount > req.TotalAmount {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid sale totals"})
	}
	var calculatedSubtotal float64
	for _, item := range req.Items {
		if strings.TrimSpace(item.ProductID) == "" || item.Quantity <= 0 || item.SellingPriceAtSale < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid sale item"})
		}
		calculatedSubtotal += float64(item.Quantity) * item.SellingPriceAtSale
	}
	calculatedTotal := calculatedSubtotal - req.DiscountAmount + req.TaxAmount + req.DeliveryCharge
	if calculatedTotal < req.TotalAmount-0.01 || calculatedTotal > req.TotalAmount+0.01 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Sale total does not match item totals"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not start database transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientSaleID, "merchant_pos_checkout", merchantID, &req.ShopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		sale, err := getSaleByClientSaleID(ctx, db, clientSaleID, merchantID)
		if err == nil {
			return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": sale})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}
	if req.CustomerID != nil && *req.CustomerID != "" {
		var customerExists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shop_customers WHERE id=$1 AND shop_id=$2 AND merchant_id=$3)`, *req.CustomerID, req.ShopID, merchantID).Scan(&customerExists); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to verify customer"})
		}
		if !customerExists {
			return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Customer does not belong to this shop"})
		}
	}
	if req.CustomerName != nil && strings.TrimSpace(*req.CustomerName) != "" && req.CustomerID == nil {
		var customerID string
		if err = tx.QueryRow(ctx, `INSERT INTO shop_customers(merchant_id,shop_id,name) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING id`, merchantID, req.ShopID, strings.TrimSpace(*req.CustomerName)).Scan(&customerID); err == nil {
			req.CustomerID = &customerID
		}
	}

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
	saleID := generateUUID()
	saleQuery := `
		INSERT INTO sales (id, client_sale_id, shop_id, merchant_id, total_amount, delivery_charge, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, customer_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id
	`
	paymentStatus := "succeeded" // Assume success for now
	err = tx.QueryRow(ctx, saleQuery, saleID, clientSaleID, req.ShopID, merchantID, req.TotalAmount, req.DeliveryCharge, req.AppliedPromotionID, req.DiscountAmount, req.PaymentType, paymentStatus, req.StripePaymentIntentID, req.CustomerID).Scan(&saleID)
	if err != nil {
		log.Printf("Failed to create sale record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record sale"})
	}

	// Process each item in the sale
	for _, item := range req.Items {
		// Resolve the merchant stock item to the shop-specific inventory balance.
		var itemName string
		var itemSKU *string
		var originalPrice *float64
		var inventoryID, productID, stockItemID string
		itemQuery := `
			SELECT ii.id, si.product_id, si.id, si.name, si.sku, pp.cost_price
			FROM inventory_items ii
			JOIN stock_items si ON si.id = ii.stock_item_id
			LEFT JOIN LATERAL (SELECT cost_price FROM product_prices
				WHERE product_id = si.product_id AND shop_id IS NULL AND price_type = 'RETAIL'
				ORDER BY created_at DESC LIMIT 1) pp ON TRUE
			WHERE ii.shop_id = $1 AND (ii.stock_item_id = $2 OR ii.product_id = $2) AND ii.merchant_id = $3
			FOR UPDATE OF ii, si`
		err := tx.QueryRow(ctx, itemQuery, req.ShopID, item.ProductID, merchantID).Scan(&inventoryID, &productID, &stockItemID, &itemName, &itemSKU, &originalPrice)
		if err != nil {
			log.Printf("Failed to fetch inventory item details for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Product %s not found", item.ProductID)})
		}

		// 1. Decrement stock and check for sufficiency
		var newQuantity float64
		stockUpdateQuery := `
			UPDATE inventory_items
			SET quantity_on_hand = quantity_on_hand - $1, updated_at = NOW()
			WHERE id = $2 AND quantity_on_hand >= $1
			RETURNING quantity_on_hand
		`
		err = tx.QueryRow(ctx, stockUpdateQuery, item.Quantity, inventoryID).Scan(&newQuantity)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Insufficient stock for product ID: %s", item.ProductID)})
			}
			log.Printf("Failed to update stock for item %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		// 2. Create the sale_items record
		saleItemQuery := `
			INSERT INTO sale_items (sale_id, inventory_item_id, product_id, stock_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		subtotal := float64(item.Quantity) * item.SellingPriceAtSale
		_, err = tx.Exec(ctx, saleItemQuery, saleID, inventoryID, productID, stockItemID, itemName, itemSKU, item.Quantity, item.SellingPriceAtSale, originalPrice, subtotal)
		if err != nil {
			log.Printf("Failed to create sale_item record for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record sale item details"})
		}

		// 3. Create a stock movement record
		stockMovementQuery := `
			INSERT INTO inventory_movements (merchant_id, shop_id, inventory_item_id, product_id, stock_item_id, movement_type, quantity, base_quantity, reference_type, reference_id, event_key, notes)
			VALUES ($1, $2, $3, $4, $5, 'OUT', $6, $6, 'SALE', $7, $8, $9)
		`
		reason := fmt.Sprintf("Sale #%s", saleID)
		_, err = tx.Exec(ctx, stockMovementQuery, merchantID, req.ShopID, inventoryID, productID, stockItemID, item.Quantity, saleID, fmt.Sprintf("%s:%s", saleID, stockItemID), reason)
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
	subtotal := req.TotalAmount + discountAmount - req.TaxAmount - req.DeliveryCharge
	taxAmount := req.TaxAmount

	invoiceQuery := `
			INSERT INTO invoices (
				sale_id, invoice_number, merchant_id, shop_id, customer_id,
				invoice_date, subtotal, discount_amount, tax_amount, delivery_charge, total_amount, payment_status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`
	_, err = tx.Exec(ctx, invoiceQuery,
		saleID, invoiceNumber, merchantID, req.ShopID, req.CustomerID,
		time.Now(), subtotal, discountAmount, taxAmount, req.DeliveryCharge, req.TotalAmount, "paid",
	)
	if err != nil {
		log.Printf("Error creating invoice (merchant POS): %v; params: saleID=%s invoiceNumber=%s merchantID=%s shopID=%s customerID=%v subtotal=%.2f discount=%.2f tax=%.2f total=%.2f",
			err, saleID, invoiceNumber, merchantID, req.ShopID, req.CustomerID, subtotal, discountAmount, taxAmount, req.TotalAmount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create invoice"})
	}
	if req.POSSessionID != nil && *req.POSSessionID != "" {
		result, execErr := tx.Exec(ctx, `INSERT INTO pos_transactions (session_id,sale_id,total) SELECT $1,$2,$3 WHERE EXISTS (SELECT 1 FROM pos_sessions WHERE id=$1 AND shop_id=$4 AND status='OPEN')`, *req.POSSessionID, saleID, req.TotalAmount, req.ShopID)
		if execErr != nil || result.RowsAffected() == 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Invalid or closed POS session"})
		}
	}
	method := strings.ToUpper(strings.TrimSpace(req.PaymentType))
	if method == "" {
		method = "CASH"
	}
	if method != "CASH" && method != "CARD" && method != "TRANSFER" && method != "ONLINE" && method != "QR_MANUAL" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Unsupported payment type"})
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments (sale_id,method,amount,status) VALUES ($1,$2,$3,'SUCCESS')`, saleID, method, req.TotalAmount); err != nil {
		log.Printf("Failed to create payment record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record payment"})
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
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "message": "Sale completed successfully"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": createdSale})
}

// getSaleByID is a helper function to fetch a sale and its items.
func getSaleByID(ctx context.Context, db *pgxpool.Pool, saleID string) (*models.Sale, error) {
	var sale models.Sale
	saleQuery := `SELECT id, shop_id, merchant_id, sale_date, total_amount, delivery_charge, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at FROM sales WHERE id = $1`
	err := db.QueryRow(ctx, saleQuery, saleID).Scan(
		&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.SaleDate, &sale.TotalAmount, &sale.DeliveryCharge, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt,
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

func getSaleByClientSaleID(ctx context.Context, db *pgxpool.Pool, clientSaleID, merchantID string) (*models.Sale, error) {
	var saleID string
	if err := db.QueryRow(ctx, `SELECT id FROM sales WHERE client_sale_id=$1 AND merchant_id=$2`, clientSaleID, merchantID).Scan(&saleID); err != nil {
		return nil, err
	}
	return getSaleByID(ctx, db, saleID)
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
