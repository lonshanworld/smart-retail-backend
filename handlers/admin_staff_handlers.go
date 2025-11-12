package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetAllStaff handles the GET /api/admin/staff endpoint
// StaffResponse represents the staff member data in the response
type StaffResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MerchantID   string    `json:"merchant_id,omitempty"`
	MerchantName string    `json:"merchant_name,omitempty"`
}

// PaginatedStaffResponse represents a paginated response containing staff members
type PaginatedStaffResponse struct {
	Data       []StaffResponse   `json:"data"`
	Pagination models.Pagination `json:"pagination"`
}

func HandleGetAllStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset := (page - 1) * limit

	// Get total count
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'staff'"
	if err := db.QueryRow(ctx, countQuery).Scan(&totalItems); err != nil {
		log.Printf("Error counting staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count staff"})
	}

	// Get paginated data with merchant info directly from users table
	query := `
		SELECT u.id, u.name, u.email, u.role, u.is_active, u.created_at, u.updated_at,
			   m.id as merchant_id, m.name as merchant_name
		FROM users u
		LEFT JOIN users m ON u.merchant_id = m.id
		WHERE u.role = 'staff'
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := db.Query(ctx, query, limit, offset)
	if err != nil {
		log.Printf("Error fetching staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve staff"})
	}
	defer rows.Close()

	var staff []StaffResponse
	for rows.Next() {
		var staffMember StaffResponse
		var merchantID, merchantName sql.NullString

		if err := rows.Scan(
			&staffMember.ID,
			&staffMember.Name,
			&staffMember.Email,
			&staffMember.Role,
			&staffMember.IsActive,
			&staffMember.CreatedAt,
			&staffMember.UpdatedAt,
			&merchantID,
			&merchantName,
		); err != nil {
			log.Printf("Error scanning staff row: %v", err)
			continue
		}

		// Log the raw data we get from database
		log.Printf("Raw staff data - ID: %s, Name: %s, Email: %s, Role: %s, IsActive: %v",
			staffMember.ID, staffMember.Name, staffMember.Email, staffMember.Role, staffMember.IsActive)
		log.Printf("Raw merchant data - MerchantID: %v (valid: %v), MerchantName: %v (valid: %v)",
			merchantID.String, merchantID.Valid, merchantName.String, merchantName.Valid)

		if merchantID.Valid && merchantName.Valid {
			staffMember.MerchantID = merchantID.String
			staffMember.MerchantName = merchantName.String
			log.Printf("Setting merchant info for staff %s - MerchantID: %s, MerchantName: %s",
				staffMember.Name, staffMember.MerchantID, staffMember.MerchantName)
		} else {
			log.Printf("No valid merchant info found for staff %s", staffMember.Name)
		}

		staff = append(staff, staffMember)
	}

	pagination := models.Pagination{
		TotalItems:  totalItems,
		TotalPages:  int(math.Ceil(float64(totalItems) / float64(limit))),
		CurrentPage: page,
		PageSize:    limit,
	}

	return c.JSON(PaginatedStaffResponse{
		Data:       staff,
		Pagination: pagination,
	})
}
