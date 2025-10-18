package handlers

import (
	"app/database"
	"app/models"
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleSearchCustomers searches for customers.
// GET /api/v1/merchant/customers/search
func HandleSearchCustomers(c *fiber.Ctx) error {
	db := database.GetDB()
	query := c.Query("query")
	shopId := c.Query("shopId")

	if query == "" || shopId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Search query and shopId are required"})
	}

	searchQuery := `
		SELECT id, shop_id, name, phone, email, created_at
		FROM customers
		WHERE shop_id = $1 AND (name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2)
	`

	rows, err := db.Query(searchQuery, shopId, "%"+query+"%")
	if err != nil {
		log.Printf("Error searching for customers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}
	defer rows.Close()

	customers := make([]models.Customer, 0)
	for rows.Next() {
		var customer models.Customer
		if err := rows.Scan(&customer.ID, &customer.ShopID, &customer.Name, &customer.Phone, &customer.Email, &customer.CreatedAt); err != nil {
			log.Printf("Error scanning customer row: %v", err)
			continue
		}
		customers = append(customers, customer)
	}

	return c.JSON(fiber.Map{"success": true, "data": customers})
}

// HandleCreateCustomer creates a new customer.
// POST /api/v1/merchant/customers
func HandleCreateCustomer(c *fiber.Ctx) error {
	db := database.GetDB()
	var customer models.Customer

	if err := c.BodyParser(&customer); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid JSON"})
	}

	if customer.ShopID == "" || customer.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Shop ID and name are required"})
	}

	query := `
		INSERT INTO customers (shop_id, name, phone, email)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	err := db.QueryRow(query, customer.ShopID, customer.Name, customer.Phone, customer.Email).Scan(&customer.ID, &customer.CreatedAt)
	if err != nil {
		log.Printf("Error creating customer: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": customer})
}
