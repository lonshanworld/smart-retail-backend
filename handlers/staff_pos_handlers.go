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
)

// HandleSearchProductsForStaff godoc
// @Summary Search for products in the staff member's assigned shop
// @Description Searches for products available for sale in the staff member's assigned shop.
// @Tags Staff POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Param searchTerm query string false "Search term"
// @Success 200 {array} models.InventoryItem
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/pos/products [get]
func HandleSearchProductsForStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	var assignedShopID string
	userQuery := `SELECT assigned_shop_id FROM users WHERE id = $1`
	if err := db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	searchTerm := c.Query("searchTerm")

	query := `
        SELECT ii.id, ii.merchant_id, ii.name, ii.sku, ii.selling_price, ii.original_price, ii.created_at, ii.updated_at
        FROM inventory_items ii
        INNER JOIN shop_stock ss ON ii.id = ss.inventory_item_id
        WHERE ss.shop_id = $1
        AND (ii.name ILIKE $2 OR ii.sku ILIKE $2)
    `

	rows, err := db.Query(ctx, query, assignedShopID, "%"+searchTerm+"%")
	if err != nil {
		log.Printf("Error searching products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to search products"})
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &item.SKU, &item.SellingPrice, &item.OriginalPrice, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Printf("Error scanning product item: %v", err)
			continue
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleStaffCheckout godoc
// @Summary Process a new sale for the staff member's assigned shop
// @Description Processes a new sale (checkout) for the staff member's assigned shop.
// @Tags Staff POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Param sale body models.StaffCheckoutRequest true "Sale data"
// @Success 201 {object} models.Sale
// @Failure 400 {object} fiber.Map{message=string}
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/pos/checkout [post]
func HandleStaffCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	var assignedShopID, merchantID string
	userQuery := `SELECT assigned_shop_id, merchant_id FROM users WHERE id = $1`
	if err = db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID, &merchantID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	var req models.StaffCheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	// If customer name is provided but no customer ID, create a new customer
	if req.CustomerName != nil && *req.CustomerName != "" && req.CustomerID == nil {
		customerID, err := createCustomerFromName(ctx, db, assignedShopID, merchantID, *req.CustomerName)
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	saleQuery := `
        INSERT INTO sales (shop_id, merchant_id, staff_id, total_amount, payment_type, payment_status, customer_id)
        VALUES ($1, $2, $3, $4, $5, 'succeeded', $6)
        RETURNING id, sale_date, created_at, updated_at
    `
	var sale models.Sale
	sale.ShopID = assignedShopID
	sale.MerchantID = merchantID
	sale.StaffID = &userID
	sale.TotalAmount = req.TotalAmount
	sale.PaymentType = req.PaymentType
	sale.PaymentStatus = "succeeded"
	sale.CustomerID = req.CustomerID

	err = tx.QueryRow(ctx, saleQuery, sale.ShopID, sale.MerchantID, sale.StaffID, sale.TotalAmount, sale.PaymentType, req.CustomerID).Scan(&sale.ID, &sale.SaleDate, &sale.CreatedAt, &sale.UpdatedAt)
	if err != nil {
		log.Printf("Error creating sale: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale"})
	}

	for _, item := range req.Items {
		// Fetch item details from inventory_items for denormalization
		var itemName string
		var itemSKU *string
		var originalPrice *float64
		itemQuery := `SELECT name, sku, original_price FROM inventory_items WHERE id = $1`
		err := tx.QueryRow(ctx, itemQuery, item.ProductID).Scan(&itemName, &itemSKU, &originalPrice)
		if err != nil {
			log.Printf("Failed to fetch inventory item details for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Product %s not found", item.ProductID)})
		}

		subtotal := float64(item.Quantity) * item.SellingPriceAtSale
		saleItemQuery := `
            INSERT INTO sale_items (sale_id, inventory_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        `
		_, err = tx.Exec(ctx, saleItemQuery, sale.ID, item.ProductID, itemName, itemSKU, item.Quantity, item.SellingPriceAtSale, originalPrice, subtotal)
		if err != nil {
			log.Printf("Error creating sale item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale item"})
		}

		updateStockQuery := `
			UPDATE shop_stock
			SET quantity = quantity - $1
			WHERE shop_id = $2 AND inventory_item_id = $3 AND quantity >= $1
			RETURNING quantity
		`
		var currentQuantity int
		err = tx.QueryRow(ctx, updateStockQuery, item.Quantity, assignedShopID, item.ProductID).Scan(&currentQuantity)
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Printf("Insufficient stock for product %s", item.ProductID)
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Insufficient stock for product %s", item.ProductID)})
			}
			log.Printf("Error updating stock: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		stockMovementQuery := `
            INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
            VALUES ($1, $2, $3, 'sale', $4, $5, 'Sale')
        `
		_, err = tx.Exec(ctx, stockMovementQuery, item.ProductID, assignedShopID, userID, -item.Quantity, currentQuantity)
		if err != nil {
			log.Printf("Error creating stock movement: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create stock movement"})
		}
	}

	// Generate invoice number and create invoice for staff POS
	invoiceNumber, err := utils.GenerateInvoiceNumber(ctx, tx)
	if err != nil {
		log.Printf("Error generating invoice number (staff POS): %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to generate invoice number"})
	}

	// Staff checkout doesn't include discount field; set to 0
	discountAmount := 0.0
	subtotal := req.TotalAmount + discountAmount
	taxAmount := 0.0

	invoiceQuery := `
		INSERT INTO invoices (
			sale_id, invoice_number, merchant_id, shop_id, customer_id,
			invoice_date, subtotal, discount_amount, tax_amount, total_amount, payment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = tx.Exec(ctx, invoiceQuery,
		sale.ID, invoiceNumber, merchantID, assignedShopID, req.CustomerID,
		time.Now(), subtotal, discountAmount, taxAmount, req.TotalAmount, "paid",
	)
	if err != nil {
		log.Printf("Error creating invoice (staff POS): %v; params: saleID=%s invoiceNumber=%s merchantID=%s shopID=%s customerID=%v subtotal=%.2f discount=%.2f tax=%.2f total=%.2f",
			err, sale.ID, invoiceNumber, merchantID, assignedShopID, req.CustomerID, subtotal, discountAmount, taxAmount, req.TotalAmount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create invoice"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": sale})
}

// HandleGetActivePromotionsForStaff godoc
// @Summary Get active promotions for staff's assigned shop
// @Description Fetches all active promotions that can be applied in the staff member's assigned shop.
// @Tags Staff POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {array} models.Promotion
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/staff/pos/promotions [get]
func HandleGetActivePromotionsForStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	var assignedShopID, merchantID string
	userQuery := `SELECT assigned_shop_id, merchant_id FROM users WHERE id = $1`
	if err = db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID, &merchantID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	query := `
		SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, 
		       start_date, end_date, is_active, created_at, updated_at
		FROM promotions
		WHERE merchant_id = $1
		  AND (shop_id IS NULL OR shop_id = $2)
		  AND is_active = TRUE
		  AND (start_date IS NULL OR start_date <= NOW())
		  AND (end_date IS NULL OR end_date >= NOW())
		ORDER BY created_at DESC
	`

	rows, err := db.Query(ctx, query, merchantID, assignedShopID)
	if err != nil {
		log.Printf("Error fetching promotions: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch promotions"})
	}
	defer rows.Close()

	promotions := []models.Promotion{} // Initialize as empty slice instead of nil
	for rows.Next() {
		var promo models.Promotion
		if err := rows.Scan(&promo.ID, &promo.MerchantID, &promo.ShopID, &promo.Name, &promo.Description,
			&promo.PromoType, &promo.PromoValue, &promo.MinSpend,
			&promo.StartDate, &promo.EndDate, &promo.IsActive, &promo.CreatedAt, &promo.UpdatedAt); err != nil {
			log.Printf("Error scanning promotion: %v", err)
			continue
		}
		promotions = append(promotions, promo)
	}

	return c.JSON(fiber.Map{"status": "success", "data": promotions})
}
