package handlers

import (
	"app/database"
	"app/models"
	"app/utils"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleListMerchantSuppliers handles the request to list merchant suppliers with pagination.
func HandleListMerchantSuppliers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	nameQuery := c.Query("name")

	// Base query
	baseQuery := "FROM suppliers WHERE merchant_id = $1"
	args := []interface{}{merchantId}
	argCount := 2

	// Add name filter if provided
	if nameQuery != "" {
		baseQuery += fmt.Sprintf(" AND name ILIKE $%d", argCount)
		args = append(args, "%"+nameQuery+"%")
	}

	// Count total items
	var totalItems int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting suppliers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count suppliers"})
	}

	// Fetch paginated data
	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at
		%s
		ORDER BY name
		LIMIT $%d OFFSET $%d`, baseQuery, argCount, argCount+1)

	paginatedArgs := append(args, pageSize, offset)
	rows, err := db.Query(ctx, query, paginatedArgs...)
	if err != nil {
		log.Printf("Error fetching suppliers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch suppliers"})
	}
	defer rows.Close()

	suppliers := make([]models.Supplier, 0)
	for rows.Next() {
		var s models.Supplier
		var contactName, contactEmail, contactPhone, address, notes sql.NullString
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.Name, &contactName, &contactEmail, &contactPhone, &address, &notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			log.Printf("Error scanning supplier: %v", err)
			continue
		}
		s.ContactName = utils.NullStringToStringPtr(contactName)
		s.ContactEmail = utils.NullStringToStringPtr(contactEmail)
		s.ContactPhone = utils.NullStringToStringPtr(contactPhone)
		s.Address = utils.NullStringToStringPtr(address)
		s.Notes = utils.NullStringToStringPtr(notes)
		suppliers = append(suppliers, s)
	}

	// Create response
	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	return c.JSON(models.PaginatedSuppliersResponse{
		Data:       suppliers,
		Pagination: pagination,
	})
}

// HandleGetSupplierDetails retrieves a single supplier.
func HandleGetSupplierDetails(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	supplierId := c.Params("supplierId")

	var s models.Supplier
	var contactName, contactEmail, contactPhone, address, notes sql.NullString
	query := `SELECT id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at
	          FROM suppliers WHERE id = $1 AND merchant_id = $2`

	err := db.QueryRow(ctx, query, supplierId, merchantId).Scan(
		&s.ID, &s.MerchantID, &s.Name, &contactName, &contactEmail, &contactPhone, &address, &notes, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found"})
		}
		log.Printf("Error fetching supplier details: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	s.ContactName = utils.NullStringToStringPtr(contactName)
	s.ContactEmail = utils.NullStringToStringPtr(contactEmail)
	s.ContactPhone = utils.NullStringToStringPtr(contactPhone)
	s.Address = utils.NullStringToStringPtr(address)
	s.Notes = utils.NullStringToStringPtr(notes)

	return c.JSON(fiber.Map{"data": s})
}

// HandleCreateNewSupplier adds a new supplier for a merchant.
func HandleCreateNewSupplier(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	var input models.Supplier
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	input.MerchantID = merchantId

	query := `INSERT INTO suppliers (merchant_id, name, contact_name, contact_email, contact_phone, address, notes) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) 
	          RETURNING id, created_at, updated_at`

	err := db.QueryRow(ctx, query, input.MerchantID, input.Name, input.ContactName, input.ContactEmail, input.ContactPhone, input.Address, input.Notes).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "suppliers_merchant_id_name_key") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "A supplier with this name already exists for your account."})
		}
		log.Printf("Error creating supplier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create supplier"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": input})
}

// HandleUpdateExistingSupplier updates a supplier's details.
func HandleUpdateExistingSupplier(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	supplierId := c.Params("supplierId")

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON format"})
	}

	var setParts []string
	var args []interface{}
	argCount := 1

	for key, value := range updates {
		if key != "id" && key != "merchant_id" {
			setParts = append(setParts, fmt.Sprintf("%s = $%d", key, argCount))
			args = append(args, value)
			argCount++
		}
	}

	if len(setParts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No update fields provided"})
	}

	query := fmt.Sprintf(`UPDATE suppliers SET %s, updated_at = NOW() WHERE id = $%d AND merchant_id = $%d
	          RETURNING id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at`,
		strings.Join(setParts, ", "), argCount, argCount+1)

	args = append(args, supplierId, merchantId)

	var s models.Supplier
	var contactName, contactEmail, contactPhone, address, notes sql.NullString

	err := db.QueryRow(ctx, query, args...).Scan(
		&s.ID, &s.MerchantID, &s.Name, &contactName, &contactEmail, &contactPhone, &address, &notes, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found or you do not have permission to update it"})
		}
		log.Printf("Error updating supplier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update supplier"})
	}

	s.ContactName = utils.NullStringToStringPtr(contactName)
	s.ContactEmail = utils.NullStringToStringPtr(contactEmail)
	s.ContactPhone = utils.NullStringToStringPtr(contactPhone)
	s.Address = utils.NullStringToStringPtr(address)
	s.Notes = utils.NullStringToStringPtr(notes)

	return c.JSON(fiber.Map{"data": s})
}

// HandleDeleteExistingSupplier deletes a supplier.
func HandleDeleteExistingSupplier(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	supplierId := c.Params("supplierId")

	query := "DELETE FROM suppliers WHERE id = $1 AND merchant_id = $2"

	res, err := db.Exec(ctx, query, supplierId, merchantId)
	if err != nil {
		log.Printf("Error deleting supplier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete supplier"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Supplier not found or you do not have permission to delete it"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// HandleGetSuppliersForSelection fetches a list of suppliers for dropdowns.
func HandleGetSuppliersForSelection(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	query := "SELECT id, name FROM suppliers WHERE merchant_id = $1 ORDER BY name"

	rows, err := db.Query(ctx, query, merchantId)
	if err != nil {
		log.Printf("Error fetching suppliers for selection: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	defer rows.Close()

	var selectionItems []models.UserSelectionItem
	for rows.Next() {
		var item models.UserSelectionItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			log.Printf("Error scanning supplier selection item: %v", err)
			continue
		}
		selectionItems = append(selectionItems, item)
	}

	return c.JSON(fiber.Map{"data": selectionItems})
}
