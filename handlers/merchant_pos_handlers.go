package handlers

import (
	"app/database"
	"app/models"
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// HandleSearchProductsForPOS handles searching for products in a specific shop's inventory.
func HandleSearchProductsForPOS(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

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
            i.category, i.supplier_id, i.is_archived, i.created_at, i.updated_at
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

	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(
			&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU,
			&item.SellingPrice, &item.OriginalPrice, &item.LowStockThreshold,
			&item.Category, &item.SupplierID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning product item: %v", err)
			continue
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// --- Checkout Logic ---

// CheckoutItem represents a single item in the checkout request.
type CheckoutItem struct {
	ProductID          string  `json:"productId"`
	Quantity           int     `json:"quantity"`
	SellingPriceAtSale float64 `json:"sellingPriceAtSale"`
}

// CheckoutRequest is the full request body for the checkout endpoint.
type CheckoutRequest struct {
	ShopID                string         `json:"shopId"`
	Items                 []CheckoutItem `json:"items"`
	TotalAmount           float64        `json:"totalAmount"`
	PaymentType           string         `json:"paymentType"`
	StripePaymentIntentID *string        `json:"stripePaymentIntentId,omitempty"`
}

// HandleCheckout processes a new sale in a transaction.
func HandleCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)
	staffID := claims["sub"].(string) // The actual user performing the action

	var req CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not start database transaction"})
	}
	defer tx.Rollback(ctx)

	// Create the main Sale record
	saleID := ""
	saleQuery := `
		INSERT INTO sales (shop_id, merchant_id, total_amount, payment_type, payment_status, stripe_payment_intent_id)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
	`
	paymentStatus := "succeeded" // Assume success for now
	err = tx.QueryRow(ctx, saleQuery, req.ShopID, merchantID, req.TotalAmount, req.PaymentType, paymentStatus, req.StripePaymentIntentID).Scan(&saleID)
	if err != nil {
		log.Printf("Failed to create sale record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to record sale"})
	}

	// Process each item in the sale
	for _, item := range req.Items {
		// 1. Decrement stock and check for sufficiency
		var newQuantity int
		stockUpdateQuery := `
			UPDATE shop_stock
			SET quantity = quantity - $1
			WHERE shop_id = $2 AND inventory_item_id = $3 AND quantity >= $1
			RETURNING quantity
		`
		err := tx.QueryRow(ctx, stockUpdateQuery, item.Quantity, req.ShopID, item.ProductID).Scan(&newQuantity)
		if err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Insufficient stock for product ID: %s", item.ProductID)})
			} 
			log.Printf("Failed to update stock for item %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		// 2. Create the sale_items record
		saleItemQuery := `
			INSERT INTO sale_items (sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, subtotal)
			VALUES ($1, $2, $3, $4, $5)
		`
		subtotal := float64(item.Quantity) * item.SellingPriceAtSale
		_, err = tx.Exec(ctx, saleItemQuery, saleID, item.ProductID, item.Quantity, item.SellingPriceAtSale, subtotal)
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
		_, err = tx.Exec(ctx, stockMovementQuery, item.ProductID, req.ShopID, staffID, -item.Quantity, newQuantity, reason)
		if err != nil {
			log.Printf("Failed to create stock movement record for product %s: %v", item.ProductID, err)
			// This is a non-critical error for the customer, but we must log it.
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to finalize sale"})
	}

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

	itemsQuery := `SELECT id, sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, created_at, updated_at FROM sale_items WHERE sale_id = $1`
	rows, err := db.Query(ctx, itemsQuery, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sale.Items = make([]models.SaleItem, 0)
	for rows.Next() {
		var item models.SaleItem
		if err := rows.Scan(&item.ID, &item.SaleID, &item.InventoryItemID, &item.QuantitySold, &item.SellingPriceAtSale, &item.OriginalPriceAtSale, &item.Subtotal, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}

	return &sale, nil
}
