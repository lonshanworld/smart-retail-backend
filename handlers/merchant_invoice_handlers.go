package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"log"

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
	offset := (page - 1) * pageSize

	// Optional filter by shop
	shopID := c.Query("shopId")

	var query string
	var args []interface{}

	if shopID != "" {
		query = `
			SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
				   invoice_date, due_date, subtotal, discount_amount, tax_amount, 
				   total_amount, payment_status, notes, created_at, updated_at
			FROM invoices
			WHERE merchant_id = $1 AND shop_id = $2
			ORDER BY invoice_date DESC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{merchantID, shopID, pageSize, offset}
	} else {
		query = `
			SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
				   invoice_date, due_date, subtotal, discount_amount, tax_amount, 
				   total_amount, payment_status, notes, created_at, updated_at
			FROM invoices
			WHERE merchant_id = $1
			ORDER BY invoice_date DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{merchantID, pageSize, offset}
	}

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
			&invoice.ShopID, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
			&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount,
			&invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
			&invoice.CreatedAt, &invoice.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}
		invoices = append(invoices, invoice)
	}

	// Count total invoices
	var totalItems int
	var countQuery string
	var countArgs []interface{}

	if shopID != "" {
		countQuery = "SELECT COUNT(*) FROM invoices WHERE merchant_id = $1 AND shop_id = $2"
		countArgs = []interface{}{merchantID, shopID}
	} else {
		countQuery = "SELECT COUNT(*) FROM invoices WHERE merchant_id = $1"
		countArgs = []interface{}{merchantID}
	}

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
		SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
			   invoice_date, due_date, subtotal, discount_amount, tax_amount, 
			   total_amount, payment_status, notes, created_at, updated_at
		FROM invoices
		WHERE id = $1 AND merchant_id = $2
	`

	var invoice models.Invoice
	if err := db.QueryRow(ctx, query, invoiceID, merchantID).Scan(
		&invoice.ID, &invoice.SaleID, &invoice.InvoiceNumber, &invoice.MerchantID,
		&invoice.ShopID, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
		&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount,
		&invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice by ID: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
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
		SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
			   invoice_date, due_date, subtotal, discount_amount, tax_amount, 
			   total_amount, payment_status, notes, created_at, updated_at
		FROM invoices
		WHERE sale_id = $1 AND merchant_id = $2
	`

	var invoice models.Invoice
	if err := db.QueryRow(ctx, query, saleID, merchantID).Scan(
		&invoice.ID, &invoice.SaleID, &invoice.InvoiceNumber, &invoice.MerchantID,
		&invoice.ShopID, &invoice.CustomerID, &invoice.InvoiceDate, &invoice.DueDate,
		&invoice.Subtotal, &invoice.DiscountAmount, &invoice.TaxAmount,
		&invoice.TotalAmount, &invoice.PaymentStatus, &invoice.Notes,
		&invoice.CreatedAt, &invoice.UpdatedAt,
	); err != nil {
		log.Printf("Error getting invoice by sale ID: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Invoice not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": invoice})
}
