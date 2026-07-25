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

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

// HandleListShopInvoices lists invoices for a given shop (accessible to merchant owners and assigned staff)
func HandleListShopInvoices(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	// shopId from path param
	shopId := c.Params("shopId")
	if shopId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "shopId is required"})
	}

	// Authorization: allow merchant owner or staff assigned to the shop
	role := claims.Role
	userId := claims.UserID

	switch role {
	case "merchant":
		// verify merchant owns the shop
		var ownerId string
		if err := db.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", shopId).Scan(&ownerId); err != nil {
			if err == pgx.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
			}
			log.Printf("Error checking shop owner: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if ownerId != userId {
			log.Printf("Merchant %s attempted to access invoices for shop %s owned by %s", userId, shopId, ownerId)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Merchant not authorized for this shop"})
		}
	case "staff":
		// verify staff assigned to this shop
		var assignedShopID *string
		if err := db.QueryRow(ctx, "SELECT assigned_shop_id FROM users WHERE id = $1", userId).Scan(&assignedShopID); err != nil {
			log.Printf("Error checking staff assigned shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if assignedShopID == nil || *assignedShopID != shopId {
			log.Printf("Staff %s attempted to access invoices for shop %s but is assigned to %v", userId, shopId, assignedShopID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff not authorized for this shop"})
		}
	default:
		// other roles (admin) can be allowed if needed — for now allow admin
		if role != "admin" {
			log.Printf("Access denied for role: %s", role)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied"})
		}
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

	where := " WHERE i.shop_id = $1"
	args := []interface{}{shopId}
	if status := strings.TrimSpace(c.Query("paymentStatus")); status != "" {
		where += fmt.Sprintf(" AND i.payment_status = $%d", len(args)+1)
		args = append(args, status)
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		where += fmt.Sprintf(" AND i.invoice_date >= $%d", len(args)+1)
		args = append(args, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		where += fmt.Sprintf(" AND i.invoice_date < ($%d::date + INTERVAL '1 day')", len(args)+1)
		args = append(args, to)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += fmt.Sprintf(" AND (i.invoice_number ILIKE $%d OR COALESCE(i.notes,'') ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	query := `
		 SELECT i.id, i.sale_id, i.invoice_number, i.merchant_id, i.shop_id, s.name AS shop_name, i.invoice_date AS checkout_time, i.customer_id,
			 i.invoice_date, i.due_date, i.subtotal, i.discount_amount, i.tax_amount, i.delivery_charge,
			 i.total_amount, i.payment_status, i.notes, i.created_at, i.updated_at
        FROM invoices i
		 JOIN shops s ON s.id = i.shop_id
		` + where + fmt.Sprintf(" ORDER BY i.invoice_date DESC, i.id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2) + `
    `

	args = append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error listing shop invoices: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve invoices"})
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
			&inv.ShopID, &inv.ShopName, &inv.CheckoutTime, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
			&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount, &inv.DeliveryCharge,
			&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}

		// Fetch sale items for this invoice to include in response (denormalized name/sku if available)
		itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`

		itemRows, err := db.Query(ctx, itemsQuery, inv.SaleID)
		if err != nil {
			log.Printf("Error querying sale items for invoice %s: %v", inv.ID, err)
			inv.Items = []models.SaleItem{}
		} else {
			var items []models.SaleItem
			for itemRows.Next() {
				var si models.SaleItem
				var original sql.NullFloat64
				if err := itemRows.Scan(
					&si.ID, &si.SaleID, &si.InventoryItemID, &si.QuantitySold, &si.SellingPriceAtSale,
					&original, &si.Subtotal, &si.CreatedAt, &si.UpdatedAt, &si.ItemName, &si.ItemSKU,
				); err != nil {
					log.Printf("Error scanning sale item: %v", err)
					continue
				}
				if original.Valid {
					v := original.Float64
					si.OriginalPriceAtSale = &v
				}
				items = append(items, si)
			}
			itemRows.Close()
			inv.Items = items
		}

		invoices = append(invoices, inv)
	}

	var totalItems int
	countQuery := "SELECT COUNT(*) FROM invoices i" + where
	countArgs := args[:len(args)-2]
	if err := db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalItems); err != nil {
		log.Printf("Error counting shop invoices: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count invoices"})
	}

	response := models.PaginatedInvoicesResponse{
		Items: invoices,
		Pagination: models.Pagination{
			TotalItems:  totalItems,
			CurrentPage: page,
			PageSize:    pageSize,
			TotalPages:  (totalItems + pageSize - 1) / pageSize,
		},
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}

// HandleGetShopInvoiceByID retrieves a single invoice for a shop (with same authorization checks)
func HandleGetShopInvoiceByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	invoiceId := c.Params("invoiceId")
	shopId := c.Params("shopId")
	if invoiceId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invoiceId is required"})
	}

	// get invoice's shop_id and merchant_id
	var inv models.Invoice
	query := `
		 SELECT i.id, i.sale_id, i.invoice_number, i.merchant_id, i.shop_id, s.name AS shop_name, i.invoice_date AS checkout_time, i.customer_id,
			 i.invoice_date, i.due_date, i.subtotal, i.discount_amount, i.tax_amount, i.delivery_charge,
			 i.total_amount, i.payment_status, i.notes, i.created_at, i.updated_at
        FROM invoices i
		 JOIN shops s ON s.id = i.shop_id
		WHERE i.id = $1
    `
	if err := db.QueryRow(ctx, query, invoiceId).Scan(
		&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
		&inv.ShopID, &inv.ShopName, &inv.CheckoutTime, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
		&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount,
		&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
	}

	// If shopId was provided in path, ensure it matches invoice
	if shopId != "" && inv.ShopID != shopId {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invoice does not belong to shop"})
	}

	// Fetch sale items for this invoice
	itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`
	rows, err := db.Query(ctx, itemsQuery, inv.SaleID)
	if err != nil {
		log.Printf("Error querying sale items for invoice %s: %v", inv.ID, err)
	} else {
		defer rows.Close()
		var items []models.SaleItem
		for rows.Next() {
			var si models.SaleItem
			var original sql.NullFloat64
			if err := rows.Scan(
				&si.ID, &si.SaleID, &si.InventoryItemID, &si.QuantitySold, &si.SellingPriceAtSale,
				&original, &si.Subtotal, &si.CreatedAt, &si.UpdatedAt, &si.ItemName, &si.ItemSKU,
			); err != nil {
				log.Printf("Error scanning sale item: %v", err)
				continue
			}
			if original.Valid {
				v := original.Float64
				si.OriginalPriceAtSale = &v
			}
			items = append(items, si)
		}
		inv.Items = items
	}

	// Authorization similar to list handler
	role := claims.Role
	userId := claims.UserID
	switch role {
	case "merchant":
		var ownerId string
		if err := db.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", inv.ShopID).Scan(&ownerId); err != nil {
			log.Printf("Error checking shop owner: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if ownerId != userId {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Merchant not authorized for this shop"})
		}
	case "staff":
		var assignedShopID *string
		if err := db.QueryRow(ctx, "SELECT assigned_shop_id FROM users WHERE id = $1", userId).Scan(&assignedShopID); err != nil {
			log.Printf("Error checking staff assigned shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if assignedShopID == nil || *assignedShopID != inv.ShopID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff not authorized for this shop"})
		}
	default:
		if role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied"})
		}
	}

	return c.JSON(fiber.Map{"status": "success", "data": inv})
}

// HandleListStaffInvoices lists invoices for the staff's assigned shop
func HandleListStaffInvoices(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	// Ensure role is staff
	if claims.Role != "staff" {
		log.Printf("Non-staff role attempted staff invoices: %s", claims.Role)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff access required"})
	}

	userId := claims.UserID

	// Get assigned shop for staff
	var assignedShopID *string
	if err := db.QueryRow(ctx, "SELECT assigned_shop_id FROM users WHERE id = $1", userId).Scan(&assignedShopID); err != nil {
		log.Printf("Error fetching assigned shop for staff %s: %v", userId, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
	}

	if assignedShopID == nil || *assignedShopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Staff has no assigned shop"})
	}

	shopId := *assignedShopID

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
	args := []interface{}{shopId}
	if status := strings.TrimSpace(c.Query("paymentStatus")); status != "" {
		where += fmt.Sprintf(" AND payment_status = $%d", len(args)+1)
		args = append(args, status)
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		where += fmt.Sprintf(" AND invoice_date >= $%d", len(args)+1)
		args = append(args, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		where += fmt.Sprintf(" AND invoice_date < ($%d::date + INTERVAL '1 day')", len(args)+1)
		args = append(args, to)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		where += fmt.Sprintf(" AND (invoice_number ILIKE $%d OR COALESCE(notes,'') ILIKE $%d)", len(args)+1, len(args)+1)
		args = append(args, "%"+search+"%")
	}
	query := `
        SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
               invoice_date, due_date, subtotal, discount_amount, tax_amount,
               delivery_charge, total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
		` + where + fmt.Sprintf(" ORDER BY invoice_date DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2) + `
    `

	args = append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error listing staff invoices: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve invoices"})
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
			&inv.ShopID, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
			&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount, &inv.DeliveryCharge,
			&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}
		invoices = append(invoices, inv)
	}

	var totalItems int
	countArgs := args[:len(args)-2]
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM invoices"+where, countArgs...).Scan(&totalItems); err != nil {
		log.Printf("Error counting staff invoices: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count invoices"})
	}

	response := models.PaginatedInvoicesResponse{
		Items: invoices,
		Pagination: models.Pagination{
			TotalItems:  totalItems,
			CurrentPage: page,
			PageSize:    pageSize,
			TotalPages:  (totalItems + pageSize - 1) / pageSize,
		},
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}

// HandleGetStaffInvoiceByID retrieves a single invoice for the staff's assigned shop
func HandleGetStaffInvoiceByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	if claims.Role != "staff" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff access required"})
	}

	userId := claims.UserID
	invoiceId := c.Params("invoiceId")
	if invoiceId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invoiceId is required"})
	}

	// fetch invoice
	var inv models.Invoice
	query := `
		 SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
			 invoice_date, due_date, subtotal, discount_amount, tax_amount, delivery_charge,
			 total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
        WHERE id = $1
    `
	if err := db.QueryRow(ctx, query, invoiceId).Scan(
		&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
		&inv.ShopID, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
		&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount, &inv.DeliveryCharge,
		&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
	}

	// verify staff assignment
	var assignedShopID *string
	if err := db.QueryRow(ctx, "SELECT assigned_shop_id FROM users WHERE id = $1", userId).Scan(&assignedShopID); err != nil {
		log.Printf("Error checking staff assigned shop: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
	}
	if assignedShopID == nil || *assignedShopID != inv.ShopID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff not authorized for this shop"})
	}

	// Fetch sale items for this invoice
	itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`
	rows, err := db.Query(ctx, itemsQuery, inv.SaleID)
	if err != nil {
		log.Printf("Error querying sale items for invoice %s: %v", inv.ID, err)
	} else {
		defer rows.Close()
		var items []models.SaleItem
		for rows.Next() {
			var si models.SaleItem
			var original sql.NullFloat64
			if err := rows.Scan(
				&si.ID, &si.SaleID, &si.InventoryItemID, &si.QuantitySold, &si.SellingPriceAtSale,
				&original, &si.Subtotal, &si.CreatedAt, &si.UpdatedAt, &si.ItemName, &si.ItemSKU,
			); err != nil {
				log.Printf("Error scanning sale item: %v", err)
				continue
			}
			if original.Valid {
				v := original.Float64
				si.OriginalPriceAtSale = &v
			}
			items = append(items, si)
		}
		inv.Items = items
	}

	return c.JSON(fiber.Map{"status": "success", "data": inv})
}
