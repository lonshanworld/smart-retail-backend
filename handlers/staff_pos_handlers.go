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

	// Optional filters
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

	baseQuery := `
		SELECT si.id, si.merchant_id, si.name, si.sku, COALESCE(pp.selling_price,0), COALESCE(pp.cost_price,0), si.created_at, si.updated_at
		FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id
		LEFT JOIN LATERAL(SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE
		WHERE ii.shop_id = $1 AND ii.quantity_on_hand > 0 AND p.is_active=TRUE
		AND (si.name ILIKE $2 OR si.sku ILIKE $2)
	`

	args := []interface{}{assignedShopID, "%" + searchTerm + "%"}
	argPos := 3
	if categoryId != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=si.product_id AND pc.category_id = $%d)", argPos)
		args = append(args, categoryId)
		argPos++
	}
	if subcategoryId != "" {
		baseQuery += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM product_categories pc2 WHERE pc2.product_id=si.product_id AND pc2.category_id=$%d)", argPos)
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count products"})
	}

	finalQuery := baseQuery + fmt.Sprintf(" ORDER BY ii.created_at DESC, ii.id DESC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)

	rows, err := db.Query(ctx, finalQuery, args...)
	if err != nil {
		log.Printf("Error searching products: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to search products"})
	}
	defer rows.Close()

	items := make([]models.InventoryItem, 0)
	for rows.Next() {
		var item models.InventoryItem
		if err := rows.Scan(&item.ID, &item.MerchantID, &item.Name, &item.SKU, &item.SellingPrice, &item.OriginalPrice, &item.CreatedAt, &item.UpdatedAt); err != nil {
			log.Printf("Error scanning product item: %v", err)
			continue
		}
		items = append(items, item)
	}

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize, "hasNext": page*pageSize < total}})
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
	clientSaleID := strings.TrimSpace(req.ClientSaleID)
	if clientSaleID == "" {
		clientSaleID = strings.TrimSpace(req.ID)
	}
	if len(req.Items) == 0 || len(req.Items) > 100 || req.TotalAmount < 0 || req.DeliveryCharge < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid sale totals or item count"})
	}
	var calculatedTotal float64
	seenProducts := make(map[string]struct{}, len(req.Items))
	for _, item := range req.Items {
		if strings.TrimSpace(item.ProductID) == "" || item.Quantity <= 0 || item.SellingPriceAtSale < 0 {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid sale item"})
		}
		if _, exists := seenProducts[item.ProductID]; exists {
			return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Duplicate product lines are not allowed"})
		}
		seenProducts[item.ProductID] = struct{}{}
		calculatedTotal += float64(item.Quantity) * item.SellingPriceAtSale
	}
	if calculatedTotal-req.DeliveryCharge < req.TotalAmount-0.01 || calculatedTotal-req.DeliveryCharge > req.TotalAmount+0.01 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Sale total does not match item totals"})
	}
	if clientSaleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientSaleId is required"})
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientSaleID, "staff_pos_checkout", userID, &assignedShopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to start operation"})
	}
	if !claimed {
		if existing, lookupErr := getSaleByClientSaleID(ctx, db, clientSaleID, merchantID); lookupErr == nil {
			if sale, detailErr := getFullSaleDetails(ctx, db, existing.ID); detailErr == nil {
				return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": sale})
			}
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "Operation already processed"})
	}
	if req.CustomerID != nil && *req.CustomerID != "" {
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shop_customers WHERE id=$1 AND shop_id=$2 AND merchant_id=$3)`, *req.CustomerID, assignedShopID, merchantID).Scan(&valid); err != nil || !valid {
			return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Customer does not belong to this shop"})
		}
	}
	if req.CustomerName != nil && strings.TrimSpace(*req.CustomerName) != "" && req.CustomerID == nil {
		var customerID string
		if err = tx.QueryRow(ctx, `INSERT INTO shop_customers(merchant_id,shop_id,name) VALUES($1,$2,$3) RETURNING id`, merchantID, assignedShopID, strings.TrimSpace(*req.CustomerName)).Scan(&customerID); err == nil {
			req.CustomerID = &customerID
		}
	}

	saleQuery := `
	INSERT INTO sales (id, client_sale_id, shop_id, merchant_id, staff_id, total_amount, discount_amount, applied_promotion_id, delivery_charge, payment_type, payment_status, customer_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'succeeded', $11)
        RETURNING id, sale_date, created_at, updated_at
    `
	var sale models.Sale
	sale.ShopID = assignedShopID
	sale.MerchantID = merchantID
	sale.StaffID = &userID
	sale.TotalAmount = req.TotalAmount
	sale.DiscountAmount = &req.DiscountAmount
	sale.AppliedPromotionID = req.AppliedPromotionID
	sale.PaymentType = req.PaymentType
	sale.PaymentStatus = "succeeded"
	sale.CustomerID = req.CustomerID
	sale.ID = generateUUID()

	err = tx.QueryRow(ctx, saleQuery, sale.ID, clientSaleID, sale.ShopID, sale.MerchantID, sale.StaffID, sale.TotalAmount, req.DiscountAmount, req.AppliedPromotionID, req.DeliveryCharge, sale.PaymentType, req.CustomerID).Scan(&sale.ID, &sale.SaleDate, &sale.CreatedAt, &sale.UpdatedAt)
	if err != nil {
		log.Printf("Error creating sale: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale"})
	}

	for _, item := range req.Items {
		// Resolve canonical stock item and shop balance.
		var itemName string
		var itemSKU *string
		var originalPrice *float64
		var inventoryID, productID string
		itemQuery := `SELECT ii.id,si.product_id,si.name,si.sku,pp.cost_price FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id LEFT JOIN LATERAL(SELECT cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE WHERE ii.shop_id=$1 AND ii.stock_item_id=$2 FOR UPDATE OF ii,si`
		err := tx.QueryRow(ctx, itemQuery, assignedShopID, item.ProductID).Scan(&inventoryID, &productID, &itemName, &itemSKU, &originalPrice)
		if err != nil {
			log.Printf("Failed to fetch inventory item details for product %s: %v", item.ProductID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Product %s not found", item.ProductID)})
		}

		subtotal := float64(item.Quantity) * item.SellingPriceAtSale
		saleItemQuery := `
            INSERT INTO sale_items (sale_id, inventory_item_id, product_id, stock_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        `
		_, err = tx.Exec(ctx, saleItemQuery, sale.ID, inventoryID, productID, item.ProductID, itemName, itemSKU, item.Quantity, item.SellingPriceAtSale, originalPrice, subtotal)
		if err != nil {
			log.Printf("Error creating sale item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create sale item"})
		}

		updateStockQuery := `UPDATE inventory_items SET quantity_on_hand=quantity_on_hand-$1,updated_at=NOW() WHERE id=$2 AND quantity_on_hand >= $1 RETURNING quantity_on_hand`
		var currentQuantity float64
		err = tx.QueryRow(ctx, updateStockQuery, item.Quantity, inventoryID).Scan(&currentQuantity)
		if err != nil {
			if err == pgx.ErrNoRows {
				log.Printf("Insufficient stock for product %s", item.ProductID)
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Insufficient stock for product %s", item.ProductID)})
			}
			log.Printf("Error updating stock: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update stock"})
		}

		stockMovementQuery := `
            INSERT INTO inventory_movements (merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes)
            VALUES ($1,$2,$3,$4,$5,'OUT',$6,$6,'SALE',$7,$8,'Sale')
        `
		_, err = tx.Exec(ctx, stockMovementQuery, merchantID, assignedShopID, inventoryID, productID, item.ProductID, item.Quantity, sale.ID, fmt.Sprintf("%s:%s", sale.ID, item.ProductID))
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
	subtotal := req.TotalAmount + discountAmount - req.DeliveryCharge
	taxAmount := 0.0

	invoiceQuery := `
		INSERT INTO invoices (
			sale_id, invoice_number, merchant_id, shop_id, customer_id,
			invoice_date, subtotal, discount_amount, tax_amount, delivery_charge, total_amount, payment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.Exec(ctx, invoiceQuery,
		sale.ID, invoiceNumber, merchantID, assignedShopID, req.CustomerID,
		time.Now(), subtotal, discountAmount, taxAmount, req.DeliveryCharge, req.TotalAmount, "paid",
	)
	if err != nil {
		log.Printf("Error creating invoice (staff POS): %v; params: saleID=%s invoiceNumber=%s merchantID=%s shopID=%s customerID=%v subtotal=%.2f discount=%.2f tax=%.2f total=%.2f",
			err, sale.ID, invoiceNumber, merchantID, assignedShopID, req.CustomerID, subtotal, discountAmount, taxAmount, req.TotalAmount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create invoice"})
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": sale})
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
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := strings.TrimSpace(c.Query("search"))

	var assignedShopID, merchantID string
	userQuery := `SELECT assigned_shop_id, merchant_id FROM users WHERE id = $1`
	if err = db.QueryRow(ctx, userQuery, userID).Scan(&assignedShopID, &merchantID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Assigned shop not found for this user"})
	}

	where := " WHERE merchant_id = $1 AND (shop_id IS NULL OR shop_id = $2) AND is_active = TRUE AND (start_date IS NULL OR start_date <= NOW()) AND (end_date IS NULL OR end_date >= NOW())"
	args := []interface{}{merchantID, assignedShopID}
	if search != "" {
		where += " AND (name ILIKE $3 OR description ILIKE $3)"
		args = append(args, "%"+search+"%")
	}
	var total int
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM promotions"+where, args...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count promotions"})
	}
	query := `SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend,
		       start_date, end_date, is_active, created_at, updated_at
		FROM promotions` + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := db.Query(ctx, query, args...)
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

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": promotions, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize, "hasNext": page*pageSize < total}})
}
