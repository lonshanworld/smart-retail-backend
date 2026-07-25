package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
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
	shopID := c.Query("shopId")
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, userID)
	} else if claims.Role != "merchant" {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	where := " WHERE shop_id = $1"
	args := []interface{}{shopID}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		where += fmt.Sprintf(" AND sale_date >= $%d", len(args)+1)
		args = append(args, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		where += fmt.Sprintf(" AND sale_date < ($%d::date + INTERVAL '1 day')", len(args)+1)
		args = append(args, to)
	}
	if status := strings.TrimSpace(c.Query("paymentStatus")); status != "" {
		where += fmt.Sprintf(" AND payment_status = $%d", len(args)+1)
		args = append(args, status)
	}
	query := `
	SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, delivery_charge, applied_promotion_id, discount_amount, payment_type, payment_status, stripe_payment_intent_id, notes, created_at, updated_at
	        FROM sales
		` + where + fmt.Sprintf(" ORDER BY sale_date DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2) + `
    `

	args = append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error listing dashboard sales for shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve sales"})
	}
	defer rows.Close()

	var sales []models.Sale
	for rows.Next() {
		var sale models.Sale
		if err := rows.Scan(&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.DeliveryCharge, &sale.AppliedPromotionID, &sale.DiscountAmount, &sale.PaymentType, &sale.PaymentStatus, &sale.StripePaymentIntentID, &sale.Notes, &sale.CreatedAt, &sale.UpdatedAt); err != nil {
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
	countArgs := args[:len(args)-2]
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM sales"+where, countArgs...).Scan(&totalItems); err != nil {
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
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	shopID := c.Query("shopId")
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, userID)
	} else if claims.Role != "merchant" {
		return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Shop access denied"})
	}
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}

	where := " WHERE ii.shop_id = $1"
	args := []interface{}{shopID}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR COALESCE(si.sku,'') ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	if categoryID := strings.TrimSpace(c.Query("categoryId")); categoryID != "" {
		where += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=si.product_id AND pc.category_id=$%d)", len(args)+1)
		args = append(args, categoryID)
	}
	if brandID := strings.TrimSpace(c.Query("brandId")); brandID != "" {
		where += fmt.Sprintf(" AND p.brand_id=$%d", len(args)+1)
		args = append(args, brandID)
	}
	query := `
        SELECT si.id, ii.merchant_id, si.name, si.sku, COALESCE(pp.selling_price,0), COALESCE(pp.cost_price,0), ii.quantity_on_hand, ii.shop_id, si.created_at, si.updated_at
		FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id
        LEFT JOIN LATERAL(SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE
		` + where + fmt.Sprintf(" ORDER BY si.created_at DESC, si.id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2) + `
	`
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id`+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Failed to count shop inventory"})
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := db.Query(ctx, query, args...)
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

	return c.JSON(fiber.Map{"status": "success", "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize}})
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

	shopID := c.Query("shopId")
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, userID)
	} else if claims.Role != "merchant" {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "Shop access denied"})
	}
	if shopID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "shopId is required"})
	}
	if err := authorizeShopAccess(c, shopID); err != nil {
		return err
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := " WHERE shop_id=$1"
	args := []interface{}{shopID}
	if queryText != "" {
		where += " AND (name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2)"
		args = append(args, "%"+queryText+"%")
	}
	var total int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM shop_customers"+where, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Database error"})
	}
	query := fmt.Sprintf("SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at FROM shop_customers%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, len(args)+1, len(args)+2)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Database error"})
	}
	defer rows.Close()
	customers := make([]models.ShopCustomer, 0)
	for rows.Next() {
		var customer models.ShopCustomer
		var phone, email sql.NullString
		if err := rows.Scan(&customer.ID, &customer.ShopID, &customer.MerchantID, &customer.Name, &phone, &email, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			continue
		}
		if phone.Valid {
			customer.Phone = &phone.String
		}
		if email.Valid {
			customer.Email = &email.String
		}
		customers = append(customers, customer)
	}
	return c.JSON(fiber.Map{"success": true, "data": customers, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize}})
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

	switch userRole {
	case "merchant":
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
	case "staff":
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
	default:
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
		FROM inventory_items ii
		WHERE ii.shop_id = $1 AND ii.low_stock_threshold IS NOT NULL AND ii.quantity_on_hand <= ii.low_stock_threshold
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
