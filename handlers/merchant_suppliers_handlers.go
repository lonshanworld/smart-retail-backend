package handlers

import (
	"app/database"
	"app/models"
	"app/utils"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HandleListMerchantSuppliers handles the request to list merchant suppliers.
func HandleListMerchantSuppliers(c *fiber.Ctx) error {
	db := database.GetDB()

	merchantId, ok := c.Locals("merchantId").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Merchant not authenticated"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	nameQuery := c.Query("name")

	var suppliers []models.Supplier
	var totalItems int64

	query := db.Model(&models.Supplier{}).Where("merchant_id = ?", merchantId)
	if nameQuery != "" {
		query = query.Where("name ILIKE ?", "%"+nameQuery+"%")
	}

	if err := query.Count(&totalItems).Error; err != nil {
		log.Printf("Error counting suppliers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to count suppliers"})
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&suppliers).Error; err != nil {
		log.Printf("Error fetching suppliers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch suppliers"})
	}

	pagination := utils.CreatePagination(int(totalItems), page, pageSize)

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"data":       suppliers,
			"pagination": pagination,
		},
	})
}

// HandleGetSupplierDetails handles the request to get supplier details.
func HandleGetSupplierDetails(c *fiber.Ctx) error {
	db := database.GetDB()
	supplierId := c.Params("supplierId")
	merchantId, ok := c.Locals("merchantId").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Merchant not authenticated"})
	}

	var supplier models.Supplier
	if err := db.Where("id = ? AND merchant_id = ?", supplierId, merchantId).First(&supplier).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Supplier not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": supplier})
}

// HandleCreateNewSupplier handles the request to create a new supplier.
func HandleCreateNewSupplier(c *fiber.Ctx) error {
	db := database.GetDB()
	merchantId, ok := c.Locals("merchantId").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Merchant not authenticated"})
	}

	var input models.Supplier
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	input.MerchantID = merchantId

	query := `INSERT INTO suppliers (merchant_id, name, contact_name, contact_email, contact_phone, address, notes) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at`

	err := db.QueryRow(context.Background(), query, input.MerchantID, input.Name, input.ContactName, input.ContactEmail, input.ContactPhone, input.Address, input.Notes).Scan(&input.ID, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		log.Printf("Error creating supplier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create supplier"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": input})
}

// HandleUpdateExistingSupplier handles the request to update an existing supplier.
func HandleUpdateExistingSupplier(c *fiber.Ctx) error {
	// TODO: Implement this handler.
	return c.SendStatus(fiber.StatusNotImplemented)
}

// HandleDeleteExistingSupplier handles the request to delete an existing supplier.
func HandleDeleteExistingSupplier(c *fiber.Ctx) error {
	// TODO: Implement this handler.
	return c.SendStatus(fiber.StatusNotImplemented)
}
