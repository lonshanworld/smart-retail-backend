package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// HandleSearchCustomers searches for customers in a specific shop.
func HandleSearchCustomers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	query := c.Query("query")
	shopId := c.Query("shopId")

	if query == "" || shopId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Search query and shopId are required"})
	}

	searchQuery := `
        SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at
        FROM shop_customers
        WHERE shop_id = $1 AND (name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2)
    `

	rows, err := db.Query(ctx, searchQuery, shopId, "%"+query+"%")
	if err != nil {
		log.Printf("Error searching for customers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}
	defer rows.Close()

	customers := make([]models.ShopCustomer, 0)
	for rows.Next() {
		var customer models.ShopCustomer
		var phone, email sql.NullString
		if err := rows.Scan(
			&customer.ID, &customer.ShopID, &customer.MerchantID, &customer.Name,
			&phone, &email, &customer.CreatedAt, &customer.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning customer row: %v", err)
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

	return c.JSON(fiber.Map{"success": true, "data": customers})
}

// HandleCreateCustomer creates a new customer for a merchant.
func HandleCreateCustomer(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var req models.ShopCustomer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid JSON"})
	}

	// Extract merchant ID from JWT claims
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	if req.ShopID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Shop ID and name are required"})
	}

	query := `
        INSERT INTO shop_customers (shop_id, merchant_id, name, phone, email)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at
    `

	err := db.QueryRow(ctx, query, req.ShopID, merchantId, req.Name, req.Phone, req.Email).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		log.Printf("Error creating customer: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}

	req.MerchantID = merchantId

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": req})
}
