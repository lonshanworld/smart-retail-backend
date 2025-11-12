package handlers

import (
	"app/database"
	"app/middleware"
	"app/models"
	"context"
	"database/sql"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleSearchCustomers searches for customers in a specific shop.
func HandleSearchCustomers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()
	query := c.Query("query")
	shopId := c.Query("shopId")

	if shopId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "shopId is required"})
	}

	var searchQuery string

	customers := make([]models.ShopCustomer, 0)

	// If query is empty, return all customers for the shop
	if query == "" {
		searchQuery = `
			SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at
			FROM shop_customers
			WHERE shop_id = $1
			ORDER BY created_at DESC
		`
		rows, err := db.Query(ctx, searchQuery, shopId)
		if err != nil {
			log.Printf("Error fetching customers: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
		}
		defer rows.Close()

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
	} else {
		// If query is provided, search by name, email, or phone
		searchQuery = `
			SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at
			FROM shop_customers
			WHERE shop_id = $1 AND (name ILIKE $2 OR email ILIKE $2 OR phone ILIKE $2)
			ORDER BY created_at DESC
		`
		rows, err := db.Query(ctx, searchQuery, shopId, "%"+query+"%")
		if err != nil {
			log.Printf("Error searching customers: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
		}
		defer rows.Close()

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
	}

	return c.JSON(fiber.Map{"success": true, "data": customers})
}

// HandleCreateCustomer creates a new customer, ensuring no duplicates for the same shop, phone, and email.
func HandleCreateCustomer(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	var req models.ShopCustomer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid JSON"})
	}

	claims, err := middleware.ExtractClaims(c)
	if err != nil {
		return err
	}
	merchantId := claims.UserID

	if req.ShopID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Shop ID and name are required"})
	}

	// Check for existing customer
	var existingID string
	checkQuery := "SELECT id FROM shop_customers WHERE shop_id = $1 AND phone = $2 AND email = $3"
	err = db.QueryRow(ctx, checkQuery, req.ShopID, req.Phone, req.Email).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking for existing customer: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Database error"})
	}
	if existingID != "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "Customer with this phone and email already exists in this shop"})
	}

	// Create new customer
	createQuery := `
        INSERT INTO shop_customers (shop_id, merchant_id, name, phone, email)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at
    `
	err = db.QueryRow(ctx, createQuery, req.ShopID, merchantId, req.Name, req.Phone, req.Email).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		log.Printf("Error creating customer: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create customer"})
	}

	req.MerchantID = merchantId

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": req})
}

// HandleListCustomers retrieves a paginated list of customers for a specific shop.
func HandleListCustomers(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	shopID := c.Query("shopId")
	if shopID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "shopId is required"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// Count total items
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM shop_customers WHERE shop_id = $1"
	if err := db.QueryRow(ctx, countQuery, shopID).Scan(&totalItems); err != nil {
		log.Printf("Error counting customers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to count customers"})
	}

	// Fetch paginated items
	query := `
        SELECT id, shop_id, merchant_id, name, phone, email, created_at, updated_at
        FROM shop_customers
        WHERE shop_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := db.Query(ctx, query, shopID, pageSize, offset)
	if err != nil {
		log.Printf("Error fetching customers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch customers"})
	}
	defer rows.Close()

	customers := make([]models.ShopCustomer, 0)
	for rows.Next() {
		var customer models.ShopCustomer
		var phone, email sql.NullString
		if err := rows.Scan(&customer.ID, &customer.ShopID, &customer.MerchantID, &customer.Name, &phone, &email, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
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

	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(pageSize))),
		CurrentPage: page,
		PageSize:    pageSize,
	}

	return c.JSON(models.PaginatedShopCustomersResponse{
		Data:       customers,
		Pagination: pagination,
	})
}
