package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
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

	if role == "merchant" {
		// verify merchant owns the shop
		var ownerId string
		if err := db.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", shopId).Scan(&ownerId); err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Shop not found"})
			}
			log.Printf("Error checking shop owner: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if ownerId != userId {
			log.Printf("Merchant %s attempted to access invoices for shop %s owned by %s", userId, shopId, ownerId)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Merchant not authorized for this shop"})
		}
	} else if role == "staff" {
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
	} else {
		// other roles (admin) can be allowed if needed — for now allow admin
		if role != "admin" {
			log.Printf("Access denied for role: %s", role)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Access denied"})
		}
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	offset := (page - 1) * pageSize

	query := `
        SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
               invoice_date, due_date, subtotal, discount_amount, tax_amount,
               total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
        WHERE shop_id = $1
        ORDER BY invoice_date DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := db.Query(ctx, query, shopId, pageSize, offset)
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
			&inv.ShopID, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
			&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount,
			&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}
		invoices = append(invoices, inv)
	}

	var totalItems int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM invoices WHERE shop_id = $1", shopId).Scan(&totalItems); err != nil {
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
        SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
               invoice_date, due_date, subtotal, discount_amount, tax_amount,
               total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
        WHERE id = $1
    `
	if err := db.QueryRow(ctx, query, invoiceId).Scan(
		&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
		&inv.ShopID, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
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

	// Authorization similar to list handler
	role := claims.Role
	userId := claims.UserID
	if role == "merchant" {
		var ownerId string
		if err := db.QueryRow(ctx, "SELECT merchant_id FROM shops WHERE id = $1", inv.ShopID).Scan(&ownerId); err != nil {
			log.Printf("Error checking shop owner: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if ownerId != userId {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Merchant not authorized for this shop"})
		}
	} else if role == "staff" {
		var assignedShopID *string
		if err := db.QueryRow(ctx, "SELECT assigned_shop_id FROM users WHERE id = $1", userId).Scan(&assignedShopID); err != nil {
			log.Printf("Error checking staff assigned shop: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Internal error"})
		}
		if assignedShopID == nil || *assignedShopID != inv.ShopID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "Staff not authorized for this shop"})
		}
	} else {
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
	offset := (page - 1) * pageSize

	query := `
        SELECT id, sale_id, invoice_number, merchant_id, shop_id, customer_id,
               invoice_date, due_date, subtotal, discount_amount, tax_amount,
               total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
        WHERE shop_id = $1
        ORDER BY invoice_date DESC
        LIMIT $2 OFFSET $3
    `

	rows, err := db.Query(ctx, query, shopId, pageSize, offset)
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
			&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount,
			&inv.TotalAmount, &inv.PaymentStatus, &inv.Notes,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning invoice: %v", err)
			continue
		}
		invoices = append(invoices, inv)
	}

	var totalItems int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM invoices WHERE shop_id = $1", shopId).Scan(&totalItems); err != nil {
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
               invoice_date, due_date, subtotal, discount_amount, tax_amount,
               total_amount, payment_status, notes, created_at, updated_at
        FROM invoices
        WHERE id = $1
    `
	if err := db.QueryRow(ctx, query, invoiceId).Scan(
		&inv.ID, &inv.SaleID, &inv.InvoiceNumber, &inv.MerchantID,
		&inv.ShopID, &inv.CustomerID, &inv.InvoiceDate, &inv.DueDate,
		&inv.Subtotal, &inv.DiscountAmount, &inv.TaxAmount,
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

	return c.JSON(fiber.Map{"status": "success", "data": inv})
}
