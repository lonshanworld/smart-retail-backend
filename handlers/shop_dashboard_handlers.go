package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleDashboardListSales lists sales for the authenticated user's assigned shop (dashboard use).
func HandleDashboardListSales(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	// Resolve the assigned shop id for the user (works for staff)
	shopID, err := getShopIDFromStaffID(ctx, db, userID)
	if err != nil || shopID == "" {
		log.Printf("Could not resolve assigned shop for user %s: %v", userID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Could not determine assigned shop for this user"})
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	offset := (page - 1) * pageSize

	query := `
        SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at
        FROM sales
        WHERE shop_id = $1
        LIMIT $2 OFFSET $3
    `

	rows, err := db.Query(ctx, query, shopID, pageSize, offset)
	if err != nil {
		log.Printf("Error listing dashboard sales for shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales"})
	}
	defer rows.Close()

	var sales []models.Sale
	for rows.Next() {
		var sale models.Sale
		if err := rows.Scan(&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt); err != nil {
			log.Printf("Error scanning sale row: %v", err)
			continue
		}

		// fetch items
		itemsQuery := `SELECT id, sale_id, inventory_item_id, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal, item_name, item_sku FROM sale_items WHERE sale_id = $1`
		itemRows, err := db.Query(ctx, itemsQuery, sale.ID)
		if err == nil {
			var items []models.SaleItem
			for itemRows.Next() {
				var item models.SaleItem
				if err := itemRows.Scan(&item.ID, &item.SaleID, &item.InventoryItemID, &item.QuantitySold, &item.SellingPriceAtSale, &item.OriginalPriceAtSale, &item.Subtotal, &item.ItemName, &item.ItemSKU); err != nil {
					log.Printf("Error scanning sale item: %v", err)
					continue
				}
				items = append(items, item)
			}
			itemRows.Close()
			sale.Items = items
		} else {
			sale.Items = []models.SaleItem{}
		}

		sales = append(sales, sale)
	}

	var totalItems int
	countQuery := "SELECT COUNT(*) FROM sales WHERE shop_id = $1"
	if err := db.QueryRow(ctx, countQuery, shopID).Scan(&totalItems); err != nil {
		log.Printf("Error counting dashboard sales: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count sales"})
	}

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

// HandleDashboardGetItems returns inventory items for the authenticated user's assigned shop.
func HandleDashboardGetItems(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	shopID, err := getShopIDFromStaffID(ctx, db, userID)
	if err != nil || shopID == "" {
		log.Printf("Could not resolve assigned shop for user %s: %v", userID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Could not determine assigned shop for this user"})
	}

	query := `
        SELECT ss.inventory_item_id, ii.merchant_id, ii.name, ii.sku, ii.selling_price, ii.original_price, ss.quantity, ss.shop_id, ii.created_at, ii.updated_at
        FROM shop_stock ss
        JOIN inventory_items ii ON ss.inventory_item_id = ii.id
        WHERE ss.shop_id = $1
        ORDER BY ii.name ASC
    `
	rows, err := db.Query(ctx, query, shopID)
	if err != nil {
		log.Printf("Error querying dashboard shop items for shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve shop inventory."})
	}
	defer rows.Close()

	var items []models.InventoryItem
	for rows.Next() {
		var item models.InventoryItem
		var stock models.ShopStock
		var sku sql.NullString
		var originalPrice sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &sku, &item.SellingPrice, &originalPrice, &stock.Quantity, &stock.ShopID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Printf("Error scanning dashboard shop item row: %v", err)
			continue
		}
		if sku.Valid {
			s := sku.String
			item.SKU = &s
		}
		if originalPrice.Valid {
			op := originalPrice.Float64
			item.OriginalPrice = &op
		}
		stock.InventoryItemID = item.ID
		item.Stock = &stock
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "data": items})
}

// HandleDashboardSearchCustomers searches customers for the authenticated user's assigned shop.
func HandleDashboardSearchCustomers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	queryText := c.Query("query")

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID

	shopID, err := getShopIDFromStaffID(ctx, db, userID)
	if err != nil || shopID == "" {
		log.Printf("Could not resolve assigned shop for user %s: %v", userID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Could not determine assigned shop for this user"})
	}

	customers := make([]models.ShopCustomer, 0)
	if queryText == "" {
		searchQuery := `SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at FROM shop_customers WHERE shop_id = $1 ORDER BY created_at DESC`
		rows, err := db.Query(ctx, searchQuery, shopID)
		if err != nil {
			log.Printf("Error fetching dashboard customers: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
		}
		defer rows.Close()
		for rows.Next() {
			var customer models.ShopCustomer
			var phone, email sql.NullString
			if err := rows.Scan(&customer.ID, &customer.ShopID, &customer.MerchantID, &customer.Name, &phone, &email, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
				log.Printf("Error scanning customer row: %v", err)
				continue
			}
			if phone.Valid {
				p := phone.String
				customer.Phone = &p
			}
			if email.Valid {
				e := email.String
				customer.Email = &e
			}
			customers = append(customers, customer)
		}
	} else {
		searchQuery := `SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at FROM shop_customers WHERE shop_id = $1 AND (name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2) ORDER BY created_at DESC`
		rows, err := db.Query(ctx, searchQuery, shopID, "%"+queryText+"%")
		if err != nil {
			log.Printf("Error searching dashboard customers: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
		}
		defer rows.Close()
		for rows.Next() {
			var customer models.ShopCustomer
			var phone, email sql.NullString
			if err := rows.Scan(&customer.ID, &customer.ShopID, &customer.MerchantID, &customer.Name, &phone, &email, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
				log.Printf("Error scanning customer row: %v", err)
				continue
			}
			if phone.Valid {
				p := phone.String
				customer.Phone = &p
			}
			if email.Valid {
				e := email.String
				customer.Email = &e
			}
			customers = append(customers, customer)
		}
	}

	return c.JSON(fiber.Map{"success": true, "data": customers})
}

// HandleGetShopDashboardSummary retrieves a summary for the shop dashboard.
// Accessible by both merchants (with shopId query param) and staff (using assigned_shop_id).
func HandleGetShopDashboardSummary(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	userID := claims.UserID
	userRole := claims.Role

	var shopID string

	// Both merchant and staff should provide shopId query parameter
	shopID = c.Query("shopId")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId query parameter is required"})
	}

	if userRole == "merchant" {
		// Verify merchant owns the shop
		var merchantID string
		shopCheckQuery := "SELECT merchant_id FROM shops WHERE id = $1"
		if err := db.QueryRow(ctx, shopCheckQuery, shopID).Scan(&merchantID); err != nil {
			log.Printf("Error verifying shop ownership: %v", err)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
		}
		if merchantID != userID {
			log.Printf("Access denied: Shop %s belongs to merchant %s, not %s", shopID, merchantID, userID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied to this shop"})
		}
	} else if userRole == "staff" {
		// Verify staff is assigned to this shop
		var assignedShopID *string
		staffQuery := "SELECT assigned_shop_id FROM users WHERE id = $1"
		if err := db.QueryRow(ctx, staffQuery, userID).Scan(&assignedShopID); err != nil {
			log.Printf("Error getting staff's assigned shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve staff details"})
		}
		if assignedShopID == nil || *assignedShopID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Staff member has no assigned shop"})
		}
		if *assignedShopID != shopID {
			log.Printf("Access denied: Staff %s is assigned to shop %s, not %s", userID, *assignedShopID, shopID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied to this shop"})
		}
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied"})
	}

	// Get sales and transactions for today
	var salesToday float64
	var transactionsToday int
	salesQuery := `
		SELECT COALESCE(SUM(total_amount), 0), COUNT(id)
		FROM sales
		WHERE shop_id = $1 AND sale_date >= $2
	`
	today := time.Now().Truncate(24 * time.Hour)
	if err := db.QueryRow(ctx, salesQuery, shopID, today).Scan(&salesToday, &transactionsToday); err != nil {
		log.Printf("Error getting shop sales summary: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales summary"})
	}

	// Get count of low stock items
	var lowStockItems int
	lowStockQuery := `
		SELECT count(*)
		FROM shop_stock ss
		JOIN inventory_items ii ON ss.inventory_item_id = ii.id
		WHERE ss.shop_id = $1 AND ii.low_stock_threshold IS NOT NULL AND ss.quantity <= ii.low_stock_threshold
	`
	if err := db.QueryRow(ctx, lowStockQuery, shopID).Scan(&lowStockItems); err != nil {
		log.Printf("Error getting low stock items count: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve low stock items count"})
	}

	summary := models.ShopDashboardSummary{
		SalesToday:        salesToday,
		TransactionsToday: transactionsToday,
		LowStockItems:     lowStockItems,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}
