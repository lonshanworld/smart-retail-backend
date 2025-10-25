package handlers

import (
	"app/database"
	"app/models"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// HandleSearchShopProducts searches for products in the current shop's inventory.
func HandleSearchShopProducts(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)
	searchTerm := c.Query("searchTerm")

	shopID, err := getShopIDFromStaffID(ctx, db, staffID)
	if err != nil {
		log.Printf("Error finding shop for staff ID %s: %v", staffID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Could not find an assigned shop for this staff member."})
	}

	query := `
        SELECT ii.id, ii.merchant_id, ii.name, ii.sku, ii.selling_price, ii.original_price,
               ss.quantity, ss.shop_id, ii.created_at, ii.updated_at
        FROM inventory_items ii
        JOIN shop_stock ss ON ii.id = ss.inventory_item_id
        WHERE ss.shop_id = $1
          AND (ii.name ILIKE $2 OR ii.sku ILIKE $2)
          AND ii.is_archived = FALSE
        ORDER BY ii.name ASC
        LIMIT 50
    `

	rows, err := db.Query(ctx, query, shopID, "%"+searchTerm+"%")
	if err != nil {
		log.Printf("Error searching for products in shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to search for products."})
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		var stock models.ShopStock
		err := rows.Scan(
			&item.ID, &item.MerchantID, &item.Name, &item.SKU, &item.SellingPrice, &item.OriginalPrice,
			&stock.Quantity, &stock.ShopID, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning product row: %v", err)
			continue
		}
		item.Stock = &stock
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleShopCheckout processes a new sale transaction from a shop terminal.
func HandleShopCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	staffID := claims["userId"].(string)

	var req models.CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	shopID, err := getShopIDFromStaffID(ctx, db, staffID)
	if err != nil {
		log.Printf("Error finding shop for staff ID %s: %v", staffID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Could not find an assigned shop for this staff member."})
	}

	merchantID, err := getMerchantIDFromShopID(ctx, db, shopID)
	if err != nil {
		log.Printf("Error finding merchant for shop ID %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal server error."})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not start transaction"})
	}
	defer tx.Rollback(ctx)

	saleID, err := createSaleRecord(ctx, tx, shopID, merchantID, staffID, req)
	if err != nil {
		log.Printf("Error creating sale record: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not create sale record"})
	}

	for _, item := range req.Items {
		if err := processSaleItem(ctx, tx, saleID, shopID, staffID, item); err != nil {
			log.Printf("Error processing sale item %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Error processing item %s", item.ProductID)})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not complete checkout"})
	}

	sale, err := getFullSaleDetails(ctx, db, saleID)
	if err != nil {
		log.Printf("Error retrieving final sale details: %v", err)
		return c.Status(201).JSON(fiber.Map{"status": "success", "message": "Checkout successful, but failed to retrieve final details"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": sale})
}

func getMerchantIDFromShopID(ctx context.Context, db *pgxpool.Pool, shopID string) (string, error) {
	var merchantID string
	query := "SELECT merchant_id FROM shops WHERE id = $1"
	err := db.QueryRow(ctx, query, shopID).Scan(&merchantID)
	return merchantID, err
}

func createSaleRecord(ctx context.Context, tx pgx.Tx, shopID, merchantID, staffID string, req models.CheckoutRequest) (string, error) {
	var saleID string
	query := `
        INSERT INTO sales (shop_id, merchant_id, staff_id, total_amount, payment_type, customer_id, sale_date)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id
    `
	err := tx.QueryRow(ctx, query, shopID, merchantID, staffID, req.TotalAmount, req.PaymentType, req.CustomerID, time.Now()).Scan(&saleID)
	return saleID, err
}

func processSaleItem(ctx context.Context, tx pgx.Tx, saleID, shopID, staffID string, item models.CheckoutItem) error {
	var current models.InventoryItem
	lockQuery := "SELECT name, sku, original_price FROM inventory_items WHERE id = $1 FOR UPDATE"
	err := tx.QueryRow(ctx, lockQuery, item.ProductID).Scan(&current.Name, &current.SKU, &current.OriginalPrice)
	if err != nil {
		return fmt.Errorf("could not find or lock inventory item %s: %w", item.ProductID, err)
	}

	saleItemQuery := `
        INSERT INTO sale_items (sale_id, inventory_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `
	subtotal := float64(item.Quantity) * item.SellingPriceAtSale
	_, err = tx.Exec(ctx, saleItemQuery, saleID, item.ProductID, current.Name, current.SKU, item.Quantity, item.SellingPriceAtSale, current.OriginalPrice, subtotal)
	if err != nil {
		return fmt.Errorf("could not create sale item record for %s: %w", item.ProductID, err)
	}

	stockUpdateQuery := `
        UPDATE shop_stock
        SET quantity = quantity - $1
        WHERE shop_id = $2 AND inventory_item_id = $3
        RETURNING quantity
    `
	var newQuantity int
	err = tx.QueryRow(ctx, stockUpdateQuery, item.Quantity, shopID, item.ProductID).Scan(&newQuantity)
	if err != nil {
		return fmt.Errorf("could not update stock for item %s: %w", item.ProductID, err)
	}

	movementQuery := `
        INSERT INTO stock_movements (inventory_item_id, shop_id, user_id, movement_type, quantity_changed, new_quantity, reason)
        VALUES ($1, $2, $3, 'sale', $4, $5, $6)
    `
	reason := fmt.Sprintf("Sale #%s", saleID)
	_, err = tx.Exec(ctx, movementQuery, item.ProductID, shopID, staffID, -item.Quantity, newQuantity, reason)
	if err != nil {
		log.Printf("Warning: could not log stock movement for item %s in sale %s: %v", item.ProductID, saleID, err)
	}

	return nil
}

func getFullSaleDetails(ctx context.Context, db *pgxpool.Pool, saleID string) (*models.Sale, error) {
	var sale models.Sale
	saleQuery := "SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, payment_type, payment_status, created_at, updated_at FROM sales WHERE id = $1"
	err := db.QueryRow(ctx, saleQuery, saleID).Scan(
		&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.CreatedAt, &sale.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	itemsQuery := "SELECT id, sale_id, inventory_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, created_at, updated_at FROM sale_items WHERE sale_id = $1"
	rows, err := db.Query(ctx, itemsQuery, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.SaleItem
		err := rows.Scan(
			&item.ID, &item.SaleID, &item.InventoryItemID, &item.ItemName, &item.ItemSKU, &item.QuantitySold, &item.SellingPriceAtSale, &item.OriginalPriceAtSale, &item.Subtotal, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sale.Items = append(sale.Items, item)
	}

	return &sale, nil
}
