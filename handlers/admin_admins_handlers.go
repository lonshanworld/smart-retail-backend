package handlers

import (
	"app/database"
	"app/models"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAdmins fetches a paginated list of admin users.
// GET /api/v1/admin/admins
func HandleGetAdmins(c *fiber.Ctx) error {
	// Parse query parameters for pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	db := database.GetDB()

	// Query to get the total count of admin users
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'admin'"
	if err := db.QueryRow(countQuery).Scan(&totalCount); err != nil {
		log.Printf("Error counting admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to retrieve admin user count",
		})
	}

	// Query to get the paginated list of admin users
	query := `
		SELECT id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
		FROM users
		WHERE role = 'admin'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		log.Printf("Error querying for admin users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to retrieve admin users",
		})
	}
	defer rows.Close()

	// Process the query results
	users := []models.User{}
	for rows.Next() {
		var user models.User
		var phone, assignedShopID, merchantID sql.NullString

		if err := rows.Scan(
			&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, 
			&phone, &assignedShopID, &merchantID, 
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning admin user row: %v", err)
			continue
		}
		
		if phone.Valid {
			user.Phone = &phone.String
		}
		if assignedShopID.Valid {
			user.AssignedShopID = &assignedShopID.String
		}
		if merchantID.Valid {
			user.MerchantID = &merchantID.String
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
        log.Printf("Error after iterating over admin user rows: %v", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "status":  "error",
            "message": "Error processing admin user data",
        })
    }

	// Construct the response
	response := fiber.Map{
		"data": users,
		"pagination": fiber.Map{
			"totalItems":  totalCount,
			"totalPages":  int(math.Ceil(float64(totalCount) / float64(limit))),
			"currentPage": page,
		},
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
