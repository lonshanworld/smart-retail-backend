package handlers

import (
	"app/database"
	"app/models"
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAllStaff retrieves all users with the 'staff' role.
// GET /api/v1/admin/staff
func HandleGetAllStaff(c *fiber.Ctx) error {
	db := database.GetDB()

	query := `
        SELECT u.id, u.name, u.email, u.role, u.is_active, u.created_at, u.merchant_id, m.name as merchant_name
        FROM users u
        LEFT JOIN merchants m ON u.merchant_id = m.id
        WHERE u.role = 'staff'
        ORDER BY u.created_at DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error querying staff users: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to retrieve staff data",
		})
	}
	defer rows.Close()

	var staffUsers []models.User
	for rows.Next() {
		var user models.User
		// Add MerchantName to the User struct if it doesn't exist
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.MerchantID, &user.MerchantName); err != nil {
			log.Printf("Error scanning staff user row: %v", err)
			continue // Or handle more gracefully
		}
		staffUsers = append(staffUsers, user)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating staff user rows: %v", err)
	}

	return c.JSON(fiber.Map{"status": "success", "data": staffUsers})
}
