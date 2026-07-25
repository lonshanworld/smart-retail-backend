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
)

// HandleListInvoices lists all invoices for a merchant with pagination
func HandleListInvoices(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	merchantID := claims.UserID
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	where := " WHERE i.merchant_id = $1"
	args := []interface{}{merchantID}
	if shopID := strings.TrimSpace(c.Query("shopId")); shopID != "" {
		where += fmt.Sprintf(" AND i.shop_id = $%d", len(args)+1)
		args = append(args, shopID)
	}
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
			` + where + fmt.Sprintf(" ORDER BY i.invoice_date DESC, i.id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, offset)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error listing invoices: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve invoices"})
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var invoice models.Invoice
		if err := rows.Scan(
			&invoice.ID, &invoice.SaleID, &invoice.InvoiceNumber, &invoice.MerchantID,
			&invoice.ShopID, &invoice.ShopName, &invoice.CheckoutTime, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
			&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount, &invoice.DeliveryCharge,
			&invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
			&invoice.CreatedAt, &invoice.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}

		// Fetch sale items for this invoice to include in response
		itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`
		itemRows, err := db.Query(ctx, itemsQuery, invoice.SaleID)
		if err != nil {
			log.Printf("Error querying sale items for invoice %s: %v", invoice.ID, err)
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
			invoice.Items = items
		}

		invoices = append(invoices, invoice)
	}

	// Count total invoices
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM invoices i" + where
	countArgs := args[:len(args)-2]

	if err := db.QueryRow(ctx, countQuery, countArgs...).Scan(&totalItems); err != nil {
		log.Printf("Error counting invoices: %v", err)
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

// HandleGetInvoiceByID retrieves a single invoice by its ID
func HandleGetInvoiceByID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	merchantID := claims.UserID
	invoiceID := c.Params("invoiceId")

	query := `
		SELECT i.id, i.sale_id, i.invoice_number, i.merchant_id, i.shop_id, s.name AS shop_name, i.invoice_date AS checkout_time, i.customer_id,
			   i.invoice_date, i.due_date, i.subtotal, i.discount_amount, i.tax_amount, i.delivery_charge,
			   i.total_amount, i.payment_status, i.notes, i.created_at, i.updated_at
		FROM invoices i
		JOIN shops s ON s.id = i.shop_id
		WHERE i.id = $1 AND i.merchant_id = $2
	`

	var invoice models.Invoice
	if err := db.QueryRow(ctx, query, invoiceID, merchantID).Scan(
		&invoice.ID, &invoice.SaleID, &invoice.InvoiceNumber, &invoice.MerchantID,
		&invoice.ShopID, &invoice.ShopName, &invoice.CheckoutTime, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
		&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount,
		&invoice.DeliveryCharge, &invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice by ID: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
	}

	// Fetch sale items for this invoice's sale
	itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`
	rows, err := db.Query(ctx, itemsQuery, invoice.SaleID)
	if err != nil {
		log.Printf("Error querying sale items for invoice %s: %v", invoice.ID, err)
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
		invoice.Items = items
	}

	return c.JSON(fiber.Map{"status": "success", "data": invoice})
}

// HandleGetInvoiceBySaleID retrieves an invoice by its sale ID
func HandleGetInvoiceBySaleID(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}

	merchantID := claims.UserID
	saleID := c.Params("saleId")

	query := `
		SELECT i.id, i.sale_id, i.invoice_number, i.merchant_id, i.shop_id, s.name AS shop_name, i.invoice_date AS checkout_time, i.customer_id,
			   i.invoice_date, i.due_date, i.subtotal, i.discount_amount, i.tax_amount, i.delivery_charge,
			   i.total_amount, i.payment_status, i.notes, i.created_at, i.updated_at
		FROM invoices i
		JOIN shops s ON s.id = i.shop_id
		WHERE sale_id = $1 AND merchant_id = $2
	`

	var invoice models.Invoice
	if err := db.QueryRow(ctx, query, saleID, merchantID).Scan(
		&invoice.ID, &invoice.SaleID, &invoice.InvoiceNumber, &invoice.MerchantID,
		&invoice.ShopID, &invoice.ShopName, &invoice.CheckoutTime, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
		&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount,
		&invoice.DeliveryCharge, &invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice by sale ID: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
	}

	// Fetch sale items for this sale
	itemsQuery := `
			SELECT si.id, si.sale_id, si.inventory_item_id, si.quantity_sold, si.selling_price_at_sale,
				   si.original_price_at_sale, si.subtotal, si.created_at, si.updated_at,
				   COALESCE(si.item_name, ii.name) as item_name,
				   COALESCE(si.item_sku, ii.sku) as item_sku
			FROM sale_items si
			LEFT JOIN stock_items ii ON si.stock_item_id = ii.id
			WHERE si.sale_id = $1
		`
	rows, err := db.Query(ctx, itemsQuery, invoice.SaleID)
	if err != nil {
		log.Printf("Error querying sale items for sale %s: %v", invoice.SaleID, err)
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
		invoice.Items = items
	}

	return c.JSON(fiber.Map{"status": "success", "data": invoice})
}
