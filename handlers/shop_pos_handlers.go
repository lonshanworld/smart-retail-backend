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

func resolveShopPOSScope(c *fiber.Ctx, db *pgxpool.Pool, requestedShopID string) (string, string, error) {
	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return "", "", err
	}
	ctx := context.Background()
	shopID := requestedShopID
	if claims.Role == "staff" {
		shopID, err = getShopIDFromStaffID(ctx, db, claims.UserID)
		if err != nil {
			return "", "", fiber.NewError(404, "Assigned shop not found")
		}
	} else if claims.Role == "merchant" {
		if shopID == "" {
			shopID = c.Query("shopId")
		}
		if shopID == "" {
			return "", "", fiber.NewError(400, "shopId is required")
		}
		var owner string
		if err = db.QueryRow(ctx, `SELECT merchant_id FROM shops WHERE id=$1`, shopID).Scan(&owner); err != nil {
			return "", "", fiber.NewError(404, "Shop not found")
		}
		if owner != claims.UserID {
			return "", "", fiber.NewError(403, "Shop access denied")
		}
	} else {
		return "", "", fiber.NewError(403, "Shop access denied")
	}
	merchantID, err := getMerchantIDFromShopID(ctx, db, shopID)
	if err != nil {
		return "", "", fiber.NewError(404, "Shop not found")
	}
	return shopID, merchantID, nil
}

// HandleSearchShopProducts searches for products in the current shop's inventory.
func HandleSearchShopProducts(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

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

	shopID, _, err := resolveShopPOSScope(c, db, c.Params("shopId"))
	if err != nil {
		return err
	}

	baseQuery := `
		SELECT si.id, ii.merchant_id, si.name, si.sku, COALESCE(pp.selling_price,0), COALESCE(pp.cost_price,0),
			   ii.quantity_on_hand, ii.shop_id, si.created_at, si.updated_at
		FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN products p ON p.id=si.product_id
		LEFT JOIN LATERAL(SELECT selling_price,cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE
		WHERE ii.shop_id = $1 AND ii.quantity_on_hand > 0 AND p.is_active=TRUE
		  AND (si.name ILIKE $2 OR si.sku ILIKE $2)
	`

	args := []interface{}{shopID, "%" + searchTerm + "%"}
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count shop products"})
	}

	finalQuery := baseQuery + fmt.Sprintf("\n        ORDER BY ii.created_at DESC, ii.id DESC\n        LIMIT %d OFFSET %d\n    ", pageSize, (page-1)*pageSize)

	rows, err := db.Query(ctx, finalQuery, args...)
	if err != nil {
		log.Printf("Error searching for products in shop %s: %v", shopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to search for products."})
	}
	defer rows.Close()

	items := make([]models.InventoryItem, 0)
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

	return c.JSON(fiber.Map{"status": "success", "success": true, "data": items, "pagination": fiber.Map{"totalItems": total, "totalPages": (total + pageSize - 1) / pageSize, "currentPage": page, "pageSize": pageSize, "hasNext": page*pageSize < total}})
}

// HandleShopCheckout processes a new sale transaction from a shop terminal.
func HandleShopCheckout(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	staffID := claims.UserID

	var req models.CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}
	clientSaleID := strings.TrimSpace(req.ClientSaleID)
	if clientSaleID == "" {
		clientSaleID = strings.TrimSpace(req.ID)
	}
	if len(req.Items) == 0 || len(req.Items) > 100 || req.TotalAmount < 0 || req.DiscountAmount < 0 || req.DeliveryCharge < 0 || req.DiscountAmount > req.TotalAmount {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Invalid sale totals or item count"})
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
	if calculatedTotal-req.DiscountAmount+req.DeliveryCharge < req.TotalAmount-0.01 || calculatedTotal-req.DiscountAmount+req.DeliveryCharge > req.TotalAmount+0.01 {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Sale total does not match item totals"})
	}
	if clientSaleID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "clientSaleId is required"})
	}

	shopID, merchantID, err := resolveShopPOSScope(c, db, c.Params("shopId"))
	if err != nil {
		return err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not start transaction"})
	}
	defer tx.Rollback(ctx)
	claimed, err := claimInventoryOperation(ctx, tx, clientSaleID, "shop_pos_checkout", staffID, &shopID)
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
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shop_customers WHERE id=$1 AND shop_id=$2 AND merchant_id=$3)`, *req.CustomerID, shopID, merchantID).Scan(&valid); err != nil || !valid {
			return c.Status(403).JSON(fiber.Map{"status": "error", "message": "Customer does not belong to this shop"})
		}
	}
	if req.CustomerName != nil && strings.TrimSpace(*req.CustomerName) != "" && req.CustomerID == nil {
		var customerID string
		if err = tx.QueryRow(ctx, `INSERT INTO shop_customers(merchant_id,shop_id,name) VALUES($1,$2,$3) RETURNING id`, merchantID, shopID, strings.TrimSpace(*req.CustomerName)).Scan(&customerID); err == nil {
			req.CustomerID = &customerID
		}
	}

	saleID, err := createSaleRecord(ctx, tx, shopID, merchantID, staffID, clientSaleID, req)
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

	// Generate invoice number and create invoice
	invoiceNumber, err := utils.GenerateInvoiceNumber(ctx, tx)
	if err != nil {
		log.Printf("Error generating invoice number: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to generate invoice number"})
	}

	// Calculate invoice amounts (req already has totalAmount)
	subtotal := req.TotalAmount + req.DiscountAmount - req.DeliveryCharge
	taxAmount := 0.0 // Implement tax calculation if needed

	// Debug logging: show invoice parameters before insert
	log.Printf("📄 [SHOP POS] Preparing invoice: invoiceNumber=%s, saleID=%s, shopID=%s, subtotal=%.2f, discount=%.2f, tax=%.2f, total=%.2f",
		invoiceNumber, saleID, shopID, subtotal, req.DiscountAmount, taxAmount, req.TotalAmount)

	invoiceQuery := `
		INSERT INTO invoices (
			sale_id, invoice_number, merchant_id, shop_id, customer_id,
			invoice_date, subtotal, discount_amount, tax_amount, delivery_charge, total_amount, payment_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.Exec(ctx, invoiceQuery,
		saleID, invoiceNumber, merchantID, shopID, req.CustomerID,
		time.Now(), subtotal, req.DiscountAmount, taxAmount, req.DeliveryCharge, req.TotalAmount, "paid",
	)
	if err != nil {
		log.Printf("Error creating invoice: %v; params: saleID=%s invoiceNumber=%s merchantID=%s shopID=%s customerID=%v subtotal=%.2f discount=%.2f tax=%.2f total=%.2f",
			err, saleID, invoiceNumber, merchantID, shopID, req.CustomerID, subtotal, req.DiscountAmount, taxAmount, req.TotalAmount)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create invoice"})
	}

	log.Printf("Created invoice %s for sale %s", invoiceNumber, saleID)

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not complete checkout"})
	}

	sale, err := getFullSaleDetails(ctx, db, saleID)
	if err != nil {
		log.Printf("Error retrieving final sale details: %v", err)
		return c.Status(201).JSON(fiber.Map{"status": "success", "success": true, "message": "Checkout successful, but failed to retrieve final details"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "success": true, "data": sale})
}

func getMerchantIDFromShopID(ctx context.Context, db *pgxpool.Pool, shopID string) (string, error) {
	var merchantID string
	query := "SELECT merchant_id FROM shops WHERE id = $1"
	err := db.QueryRow(ctx, query, shopID).Scan(&merchantID)
	return merchantID, err
}

func createSaleRecord(ctx context.Context, tx pgx.Tx, shopID, merchantID, staffID, clientSaleID string, req models.CheckoutRequest) (string, error) {
	var saleID string
	serverSaleID := generateUUID()
	query := `
	INSERT INTO sales (id, client_sale_id, shop_id, merchant_id, staff_id, total_amount, discount_amount, applied_promotion_id, delivery_charge, payment_type, customer_id, sale_date)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        RETURNING id
    `
	err := tx.QueryRow(ctx, query, serverSaleID, clientSaleID, shopID, merchantID, staffID, req.TotalAmount, req.DiscountAmount, req.AppliedPromotionID, req.DeliveryCharge, req.PaymentType, req.CustomerID, time.Now()).Scan(&saleID)
	return saleID, err
}

func processSaleItem(ctx context.Context, tx pgx.Tx, saleID, shopID, staffID string, item models.CheckoutItem) error {
	var current models.InventoryItem
	var inventoryID, productID, merchantID string
	lockQuery := `SELECT ii.id,si.product_id,s.name,si.sku,pp.cost_price,s.merchant_id FROM inventory_items ii JOIN stock_items si ON si.id=ii.stock_item_id JOIN shops s ON s.id=ii.shop_id LEFT JOIN LATERAL(SELECT cost_price FROM product_prices WHERE product_id=si.product_id AND shop_id IS NULL AND price_type='RETAIL' ORDER BY created_at DESC LIMIT 1)pp ON TRUE WHERE ii.shop_id=$1 AND ii.stock_item_id=$2 FOR UPDATE OF ii,si,s`
	err := tx.QueryRow(ctx, lockQuery, shopID, item.ProductID).Scan(&inventoryID, &productID, &current.Name, &current.SKU, &current.OriginalPrice, &merchantID)
	if err != nil {
		return fmt.Errorf("could not find or lock inventory item %s: %w", item.ProductID, err)
	}

	saleItemQuery := `
        INSERT INTO sale_items (sale_id, inventory_item_id, product_id, stock_item_id, item_name, item_sku, quantity_sold, selling_price_at_sale, original_price_at_sale, subtotal)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
	subtotal := float64(item.Quantity) * item.SellingPriceAtSale
	_, err = tx.Exec(ctx, saleItemQuery, saleID, inventoryID, productID, item.ProductID, current.Name, current.SKU, item.Quantity, item.SellingPriceAtSale, current.OriginalPrice, subtotal)
	if err != nil {
		return fmt.Errorf("could not create sale item record for %s: %w", item.ProductID, err)
	}

	stockUpdateQuery := `UPDATE inventory_items SET quantity_on_hand=quantity_on_hand-$1,updated_at=NOW() WHERE id=$2 AND quantity_on_hand >= $1 RETURNING quantity_on_hand`
	var newQuantity float64
	err = tx.QueryRow(ctx, stockUpdateQuery, item.Quantity, inventoryID).Scan(&newQuantity)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("insufficient stock for product %s", item.ProductID)
		}
		return fmt.Errorf("could not update stock for item %s: %w", item.ProductID, err)
	}

	movementQuery := `
        INSERT INTO inventory_movements (merchant_id,shop_id,inventory_item_id,product_id,stock_item_id,movement_type,quantity,base_quantity,reference_type,reference_id,event_key,notes)
        VALUES ($1,$2,$3,$4,$5,'OUT',$6,$6,'SALE',$7,$8,$9)
    `
	reason := fmt.Sprintf("Sale #%s", saleID)
	_, err = tx.Exec(ctx, movementQuery, merchantID, shopID, inventoryID, productID, item.ProductID, item.Quantity, saleID, fmt.Sprintf("%s:%s", saleID, item.ProductID), reason)
	if err != nil {
		return fmt.Errorf("could not log stock movement for item %s in sale %s: %w", item.ProductID, saleID, err)
	}

	return nil
}

func getFullSaleDetails(ctx context.Context, db *pgxpool.Pool, saleID string) (*models.Sale, error) {
	var sale models.Sale
	saleQuery := "SELECT id, shop_id, merchant_id, staff_id, customer_id, sale_date, total_amount, delivery_charge, payment_type, payment_status, created_at, updated_at FROM sales WHERE id = $1"
	err := db.QueryRow(ctx, saleQuery, saleID).Scan(
		&sale.ID, &sale.ShopID, &sale.MerchantID, &sale.StaffID, &sale.CustomerID, &sale.SaleDate, &sale.TotalAmount, &sale.DeliveryCharge, &sale.PaymentType, &sale.PaymentStatus, &sale.CreatedAt, &sale.UpdatedAt,
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

// createCustomerFromName creates a new customer record in shop_customers table with just a name
func createCustomerFromName(ctx context.Context, db *pgxpool.Pool, shopID, merchantID, name string) (string, error) {
	var customerID string
	query := `
		INSERT INTO shop_customers (merchant_id, shop_id, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	err := db.QueryRow(ctx, query, merchantID, shopID, name).Scan(&customerID)
	return customerID, err
}

// HandleGetActivePromotionsForShop godoc
// @Summary Get active promotions for shop
// @Description Fetches all active promotions that can be applied in the shop.
// @Tags Shop POS
// @Accept  json
// @Produce  json
// @Security ApiKeyAuth
// @Success 200 {array} models.Promotion
// @Failure 401 {object} fiber.Map{message=string}
// @Failure 500 {object} fiber.Map{message=string}
// @Router /api/v1/shop/pos/promotions [get]
func HandleGetActivePromotionsForShop(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := strings.TrimSpace(c.Query("search"))

	shopID, merchantID, err := resolveShopPOSScope(c, db, "")
	if err != nil {
		return err
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
		ORDER BY created_at DESC LIMIT $3 OFFSET $4
	`
	args := []interface{}{merchantID, shopID}
	where := " AND (shop_id IS NULL OR shop_id = $2) AND is_active = TRUE AND (start_date IS NULL OR start_date <= NOW()) AND (end_date IS NULL OR end_date >= NOW())"
	if search != "" {
		where += " AND (name ILIKE $3 OR description ILIKE $3)"
		args = append(args, "%"+search+"%")
	}
	countQuery := "SELECT COUNT(*) FROM promotions WHERE merchant_id=$1" + where
	var total int
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count promotions"})
	}
	query = "SELECT id, merchant_id, shop_id, name, description, promo_type, promo_value, min_spend, start_date, end_date, is_active, created_at, updated_at FROM promotions WHERE merchant_id=$1" + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
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
