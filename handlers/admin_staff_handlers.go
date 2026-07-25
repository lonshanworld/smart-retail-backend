package handlers

import (
	"app/database"
	"app/models"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
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
	limit, _ := strconv.Atoi(c.Query("pageSize", c.Query("limit", "20")))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	search := strings.TrimSpace(c.Query("search"))
	where := " WHERE u.role = 'staff'"
	args := []interface{}{}
	if search != "" {
		where += " AND (u.name ILIKE $1 OR u.email ILIKE $1)"
		args = append(args, "%"+search+"%")
	}
	if merchantID := c.Query("merchantId"); merchantID != "" {
		where += " AND u.merchant_id = $" + strconv.Itoa(len(args)+1)
		args = append(args, merchantID)
	}

	// Get total count
	var totalItems int
	countQuery := "SELECT COUNT(*) FROM users u" + where
	if err := db.QueryRow(ctx, countQuery, args...).Scan(&totalItems); err != nil {
		log.Printf("Error counting staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count staff"})
	}

	// Get paginated data with merchant info directly from users table
	query := `
		SELECT u.id, u.name, u.email, u.role, u.is_active, u.created_at, u.updated_at,
			   m.id as merchant_id, m.name as merchant_name
		FROM users u
		LEFT JOIN users m ON u.merchant_id = m.id
		%s
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $%d OFFSET $%d
	`
	query = fmt.Sprintf(query, where, len(args)+1, len(args)+2)
	args = append(args, limit, offset)
	rows, err := db.Query(ctx, query, args...)
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

		if merchantID.Valid && merchantName.Valid {
			staffMember.MerchantID = merchantID.String
			staffMember.MerchantName = merchantName.String
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
