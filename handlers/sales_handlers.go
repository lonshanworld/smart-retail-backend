package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
)

// CreateSaleInput defines the expected input for creating a new sale.

type CreateSaleInput struct {
	ShopID      string            `json:"shopId"`
	PaymentType string            `json:"paymentType"`
	Items       []models.SaleItem `json:"items"`
}

// HandleCreateSale handles the creation of a new sale.
func HandleCreateSale(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var input CreateSaleInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Calculate total amount
	var totalAmount float64
	for _, item := range input.Items {
		totalAmount += item.Subtotal
	}

	// Create the sale
	saleQuery := `
		INSERT INTO sales (shop_id, merchant_id, total_amount, payment_type)
		VALUES ($1, (SELECT merchant_id FROM shops WHERE id = $1), $2, $3)
		RETURNING id, sale_date, payment_status, created_at, updated_at
	`
	var sale models.Sale
	sale.ShopID = input.ShopID
	sale.PaymentType = input.PaymentType
	sale.TotalAmount = totalAmount

	if err := tx.QueryRow(ctx, saleQuery, input.ShopID, totalAmount, input.PaymentType).Scan(&sale.ID, &sale.SaleDate, &sale.PaymentStatus, &sale.CreatedAt, &sale.UpdatedAt); err != nil {
		log.Printf("Error creating sale: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale"})
	}

	// Create sale items
	for _, item := range input.Items {
		saleItemQuery := `
			INSERT INTO sale_items (sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
		if _, err := tx.Exec(ctx, saleItemQuery, sale.ID, item.InventoryItemID, item.QuantitySold, item.SellingPriceAtSale, item.OriginalPriceAtSale, item.Subtotal); err != nil {
			log.Printf("Error creating sale item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale item"})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": sale})
}

// HandleListSalesForShop lists sales for a specific shop.
func HandleListSalesForShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	shopID := c.Params("shopId")
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	offset := (page - 1) * pageSize

	log.Printf("📥 [SALES HANDLER] Fetching sales for shopID: %s, page: %d, pageSize: %d", shopID, page, pageSize)

	query := `
		SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at
		FROM sales
		WHERE shop_id = $1
		LIMIT $2 OFFSET $3
	`
	rows, err := db.Query(ctx, query, shopID, pageSize, offset)
	if err != nil {
		log.Printf("❌ [SALES HANDLER] Error listing sales for shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales"})
	}
	defer rows.Close()

	var sales []models.Sale
	for rows.Next() {
		var sale models.Sale
		if err := rows.Scan(&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt); err != nil {
			log.Printf("❌ [SALES HANDLER] Error scanning sale: %v", err)
			continue
		}

		log.Printf("✅ [SALES HANDLER] Found sale ID: %s, ShopID: %s, TotalAmount: %.2f, SaleDate: %s", sale.ID, sale.ShopID, sale.TotalAmount, sale.SaleDate)

		// Fetch sale items for this sale
		itemsQuery := `
			SELECT id, sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal
			FROM sale_items
			WHERE sale_id = $1
		`
		itemRows, err := db.Query(ctx, itemsQuery, sale.ID)
		if err != nil {
			log.Printf("⚠️ [SALES HANDLER] Error fetching sale items for sale ID %s: %v", sale.ID, err)
			// Don't fail the entire request if items fail
			sale.Items = []models.SaleItem{}
		} else {
			defer itemRows.Close()
			var items []models.SaleItem
			for itemRows.Next() {
				var item models.SaleItem
				if err := itemRows.Scan(&item.ID, &item.SaleID, &item.InventoryItemID, &item.QuantitySold, &item.SellingPriceAtSale, &item.OriginalPriceAtSale, &item.Subtotal); err != nil {
					log.Printf("⚠️ [SALES HANDLER] Error scanning sale item: %v", err)
					continue
				}
				log.Printf("   📦 [SALES HANDLER] Item: %s, Qty: %d, SellingPrice: %.2f, OriginalPrice: %.2f, Subtotal: %.2f",
					item.InventoryItemID, item.QuantitySold, item.SellingPriceAtSale, item.OriginalPriceAtSale, item.Subtotal)
				items = append(items, item)
			}
			sale.Items = items
			log.Printf("   📋 [SALES HANDLER] Total items in sale: %d", len(items))
		}

		sales = append(sales, sale)
	}

	var totalItems int
	countQuery := "SELECT COUNT(*) FROM sales WHERE shop_id = $1"
	if err := db.QueryRow(ctx, countQuery, shopID).Scan(&totalItems); err != nil {
		log.Printf("❌ [SALES HANDLER] Error counting sales: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count sales"})
	}

	log.Printf("📊 [SALES HANDLER] Total sales count: %d, Returning: %d sales", totalItems, len(sales))

	response := models.PaginatedSalesResponse{
		Items: sales,
		Pagination: models.Pagination{
			TotalItems:  totalItems,
			CurrentPage: page,
			PageSize:    pageSize,
			TotalPages:  (totalItems + pageSize - 1) / pageSize,
		},
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}

// HandleGetSaleByID retrieves a single sale by its ID.
func HandleGetSaleByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	saleID := c.Params("saleId")

	query := `
		SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at
		FROM sales
		WHERE id = $1
	`
	var sale models.Sale
	if err := db.QueryRow(ctx, query, saleID).Scan(&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt); err != nil {
		log.Printf("Error getting sale by ID: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Sale not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": sale})
}

// HandleGetReceipt retrieves a receipt for a specific sale.
func HandleGetReceipt(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	saleID := c.Params("saleId")

	query := `
		SELECT 
			s.id, s.sale_date, sh.name, sh.address, m.name, 
			s.total_amount - s.discount_amount as final_total, s.discount_amount, s.total_amount, 
			s.payment_type, s.payment_status
		FROM sales s
		JOIN shops sh ON s.shop_id = sh.id
		JOIN users m ON s.merchant_id = m.id
		WHERE s.id = $1
	`
	var receipt models.Receipt
	if err := db.QueryRow(ctx, query, saleID).Scan(
		&receipt.SaleID, &receipt.SaleDate, &receipt.ShopName, &receipt.ShopAddress, &receipt.MerchantName,
		&receipt.FinalTotal, &receipt.DiscountAmount, &receipt.OriginalTotal,
		&receipt.PaymentType, &receipt.PaymentStatus,
	); err != nil {
		log.Printf("Error getting receipt: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Receipt not found"})
	}

	itemsQuery := `
		SELECT i.name, si.quantity_sold, si.selling_price_at_sale, si.subtotal
		FROM sale_items si
		JOIN inventory_items i ON si.inventory_item_id = i.id
		WHERE si.sale_id = $1
	`
	rows, err := db.Query(ctx, itemsQuery, saleID)
	if err != nil {
		log.Printf("Error getting receipt items: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve receipt items"})
	}
	defer rows.Close()

	for rows.Next() {
		var item models.ReceiptItem
		if err := rows.Scan(&item.ItemName, &item.Quantity, &item.UnitPrice, &item.Total); err != nil {
			log.Printf("Error scanning receipt item: %v", err)
			continue
		}
		receipt.Items = append(receipt.Items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": receipt})
}
