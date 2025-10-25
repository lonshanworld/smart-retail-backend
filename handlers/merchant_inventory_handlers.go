package handlers

import (
	"app/database"
	"app/models"
	"context"
    "database/sql"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleAdjustStock adjusts the stock for a given item in a shop.
func HandleAdjustStock(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	userID := claims["userId"].(string)

	shopID := c.Params("shopId")
	itemID := c.Params("itemId")

	var req struct {
		Quantity int    `json:"quantity"`
		Reason   string `json:"reason"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

    // Validation check
    var exists bool
    checkQuery := "SELECT EXISTS(SELECT 1 FROM shop_stock WHERE shop_id = $1 AND inventory_item_id = $2)"
    err := db.QueryRow(ctx, checkQuery, shopID, itemID).Scan(&exists)
    if err != nil {
        log.Printf("Error checking shop_stock link: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error during validation"})
    }
    if !exists {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop's stock"})
    }

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	// Update stock quantity
	var newQuantity int
	updateQuery := `
		UPDATE shop_stock 
		SET quantity = quantity + $1 
		WHERE shop_id = $2 AND inventory_item_id = $3
		RETURNING quantity
	`
	if err := tx.QueryRow(ctx, updateQuery, req.Quantity, shopID, itemID).Scan(&newQuantity); err != nil {
        if err == sql.ErrNoRows {
            return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Item not found in this shop for stock adjustment"})
        }
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to adjust stock"})
	}

	// Log the movement
	logQuery := `
		INSERT INTO stock_movements (shop_id, inventory_item_id, user_id, movement_type, quantity_changed, new_quantity, reason, movement_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.Exec(ctx, logQuery, shopID, itemID, userID, "adjustment", req.Quantity, newQuantity, req.Reason, time.Now())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to log stock movement"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}


// HandleGetStockMovementHistory retrieves the stock movement history for an item.
func HandleGetStockMovementHistory(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	shopID := c.Params("shopId")
	itemID := c.Params("itemId")

	query := `
		SELECT id, shop_id, inventory_item_id, user_id, movement_type, quantity_changed, new_quantity, reason, notes, movement_date
		FROM stock_movements 
		WHERE shop_id = $1 AND inventory_item_id = $2
		ORDER BY movement_date DESC
	`
	rows, err := db.Query(ctx, query, shopID, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve stock movement history"})
	}
	defer rows.Close()

	history := make([]models.StockMovement, 0)
	for rows.Next() {
		var h models.StockMovement
		if err := rows.Scan(&h.ID, &h.ShopID, &h.InventoryItemID, &h.UserID, &h.MovementType, &h.QuantityChanged, &h.NewQuantity, &h.Reason, &h.Notes, &h.MovementDate); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan history data"})
		}
		history = append(history, h)
	}

	return c.JSON(fiber.Map{"status": "success", "data": history})
}

func HandleListInventoryItems(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	query := `SELECT id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at FROM inventory_items WHERE merchant_id = $1`
	rows, err := db.Query(ctx, query, merchantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve inventory items"})
	}
	defer rows.Close()

	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU, &item.SellingPrice, &item.OriginalPrice, &item.LowStockThreshold, &item.Category, &item.SupplierID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to scan inventory item data"})
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

func HandleCreateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := `
		INSERT INTO inventory_items (merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	err := db.QueryRow(ctx, query, merchantID, req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.SupplierID, req.IsArchived).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create inventory item"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": req})
}

func HandleGetInventoryItemByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")

	query := `SELECT id, merchant_id, name, description, sku, selling_price, original_price, low_stock_threshold, category, supplier_id, is_archived, created_at, updated_at FROM inventory_items WHERE id = $1`
	var item models.InventoryItem
	err := db.QueryRow(ctx, query, itemID).Scan(&item.ID, &item.MerchantID, &item.Name, &item.Description, &item.SKU, &item.SellingPrice, &item.OriginalPrice, &item.LowStockThreshold, &item.Category, &item.SupplierID, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Inventory item not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": item})
}

func HandleUpdateInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")
	var req models.InventoryItem
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := `
		UPDATE inventory_items
		SET name = $1, description = $2, sku = $3, selling_price = $4, original_price = $5, low_stock_threshold = $6, category = $7, supplier_id = $8, is_archived = $9
		WHERE id = $10
		RETURNING updated_at
	`
	err := db.QueryRow(ctx, query, req.Name, req.Description, req.SKU, req.SellingPrice, req.OriginalPrice, req.LowStockThreshold, req.Category, req.SupplierID, req.IsArchived, itemID).Scan(&req.UpdatedAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update inventory item"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": req})
}

func HandleDeleteInventoryItem(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")

	query := "DELETE FROM inventory_items WHERE id = $1"
	_, err := db.Exec(ctx, query, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete inventory item"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func HandleArchiveInventoryItem(c *fiber.Ctx) error {
	return handleArchiveStatus(c, true)
}

func HandleUnarchiveInventoryItem(c *fiber.Ctx) error {
	return handleArchiveStatus(c, false)
}

func handleArchiveStatus(c *fiber.Ctx, isArchived bool) error {
	db := database.GetDB()
	ctx := context.Background()

	itemID := c.Params("itemId")

	query := "UPDATE inventory_items SET is_archived = $1 WHERE id = $2"
	_, err := db.Exec(ctx, query, isArchived, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update archive status"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func HandleListInventoryForShop(c *fiber.Ctx) error {
    db := database.GetDB()
    ctx := context.Background()
    shopID := c.Params("shopId")

    // Parse query parameters for pagination
    page, _ := strconv.Atoi(c.Query("page", "1"))
    pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
    offset := (page - 1) * pageSize

    // Query to get paginated inventory items
    query := `
        SELECT 
            i.id, i.name, i.sku, i.selling_price, 
            s.quantity, s.last_stocked_in_at
        FROM inventory_items i
        JOIN shop_stock s ON i.id = s.inventory_item_id
        WHERE s.shop_id = $1
        LIMIT $2 OFFSET $3
    `

    rows, err := db.Query(ctx, query, shopID, pageSize, offset)
    if err != nil {
        log.Printf("Error querying shop inventory: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "status":  "error",
            "message": "Failed to retrieve shop inventory",
        })
    }
    defer rows.Close()

    var items []models.ShopStockItem
    for rows.Next() {
        var item models.ShopStockItem
        if err := rows.Scan(&item.ID, &item.ItemName, &item.ItemSku, &item.ItemUnitPrice, &item.Quantity, &item.LastStockedInAt); err != nil {
            log.Printf("Error scanning shop inventory item: %v", err)
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "status":  "error",
                "message": "Error processing shop inventory data",
            })
        }
        items = append(items, item)
    }

    // Query to get the total count of items for pagination metadata
    countQuery := "SELECT COUNT(*) FROM shop_stock WHERE shop_id = $1"
    var totalItems int
    if err := db.QueryRow(ctx, countQuery, shopID).Scan(&totalItems); err != nil {
        log.Printf("Error counting shop inventory: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "status":  "error",
            "message": "Failed to count shop inventory",
        })
    }

    totalPages := (totalItems + pageSize - 1) / pageSize

    return c.JSON(models.PaginatedShopStockResponse{
        Items:      items,
        TotalItems:  totalItems,
        CurrentPage: page,
        PageSize:    pageSize,
        TotalPages:  totalPages,
    })
}


func HandleAdjustStockItem(c *fiber.Ctx) error {
    return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
        "status":  "error",
        "message": "Endpoint not implemented yet",
    })
}

func HandleStockInItem(c *fiber.Ctx) error {
    return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
        "status":  "error",
        "message": "Endpoint not implemented yet",
    })
}
