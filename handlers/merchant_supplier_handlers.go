package handlers

// import (
// 	"app/database"
// 	"app/models"
// 	"context"
// 	"log"

// 	"github.com/gofiber/fiber/v2"
// 	"github.com/golang-jwt/jwt/v4"
// )

/*
// HandleListMerchantSuppliers fetches all suppliers for the logged-in merchant.
func HandleListMerchantSuppliers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	query := `SELECT id, merchant_id, name, contact_name, contact_email, contact_phone, address, notes, created_at, updated_at FROM suppliers WHERE merchant_id = $1`
	rows, err := db.Query(ctx, query, merchantID)
	if err != nil {
		log.Printf("Error querying suppliers for merchant %s: %v", merchantID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to retrieve suppliers"})
	}
	defer rows.Close()

	var suppliers []models.Supplier
	for rows.Next() {
		var s models.Supplier
		if err := rows.Scan(&s.ID, &s.MerchantID, &s.Name, &s.ContactName, &s.ContactEmail, &s.ContactPhone, &s.Address, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			log.Printf("Error scanning supplier row: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Error processing supplier data"})
		}
		suppliers = append(suppliers, s)
	}

	return c.JSON(fiber.Map{"status": "success", "data": suppliers})
}

// HandleCreateNewSupplier creates a new supplier for the logged-in merchant.
func HandleCreateNewSupplier(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantID := claims["userId"].(string)

	var supplier models.Supplier
	if err := c.BodyParser(&supplier); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	query := `INSERT INTO suppliers (merchant_id, name, contact_name, contact_email, contact_phone, address, notes) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at`
	err := db.QueryRow(ctx, query, merchantID, supplier.Name, supplier.ContactName, supplier.ContactEmail, supplier.ContactPhone, supplier.Address, supplier.Notes).Scan(&supplier.ID, &supplier.CreatedAt, &supplier.UpdatedAt)
	if err != nil {
		log.Printf("Error creating supplier for merchant %s: %v", merchantID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create supplier"})
	}

	supplier.MerchantID = merchantID
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": supplier})
}
*/
