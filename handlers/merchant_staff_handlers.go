package handlers

import (
	"app/database"
	"app/models"
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// HandleListMerchantStaff handles listing all staff for the current merchant.
func HandleListMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM users WHERE role = 'staff' AND merchant_id = $1"
	err := db.QueryRow(ctx, countQuery, merchantId).Scan(&totalCount)
	if err != nil {
		log.Printf("Error counting merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	// Get paginated staff
	query := `
		SELECT u.id, u.name, u.email, u.role, u.is_active, u.merchant_id, u.assigned_shop_id, u.created_at, u.updated_at
		FROM users u
		WHERE u.role = 'staff' AND u.merchant_id = $1
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := db.Query(ctx, query, merchantId, pageSize, offset)
	if err != nil {
		log.Printf("Error listing merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}
	defer rows.Close()

	staffList := make([]models.User, 0)
	for rows.Next() {
		var staff models.User
		if err := rows.Scan(&staff.ID, &staff.Name, &staff.Email, &staff.Role, &staff.IsActive, &staff.MerchantID, &staff.AssignedShopID, &staff.CreatedAt, &staff.UpdatedAt); err != nil {
			log.Printf("Error scanning staff row: %v", err)
			continue
		}
		staffList = append(staffList, staff)
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"users":       staffList,
			"totalCount":  totalCount,
			"currentPage": page,
			"pageSize":    pageSize,
			"totalPages":  totalPages,
		},
	})
}

// HandleCreateMerchantStaff handles creating a new staff member for the current merchant.
func HandleCreateMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)

	type CreateStaffRequest struct {
		Name           string  `json:"name"`
		Email          string  `json:"email"`
		Password       string  `json:"password"`
		AssignedShopID *string `json:"assignedShopId,omitempty"`
	}

	var req CreateStaffRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to hash password"})
	}

	query := `
		INSERT INTO users (name, email, password_hash, role, merchant_id, assigned_shop_id)
		VALUES ($1, $2, $3, 'staff', $4, $5)
		RETURNING id, name, email, role, is_active, merchant_id, assigned_shop_id, created_at, updated_at
	`

	var newStaff models.User
	err = db.QueryRow(ctx, query, req.Name, req.Email, string(hashedPassword), merchantId, req.AssignedShopID).Scan(
		&newStaff.ID, &newStaff.Name, &newStaff.Email, &newStaff.Role, &newStaff.IsActive, &newStaff.MerchantID, &newStaff.AssignedShopID, &newStaff.CreatedAt, &newStaff.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create staff member. Email may already be in use."})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": newStaff})
}

// HandleUpdateMerchantStaff handles updating a staff member for the current merchant.
func HandleUpdateMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	staffId := c.Params("staffId")

	type UpdateStaffRequest struct {
		Name           *string `json:"name,omitempty"`
		IsActive       *bool   `json:"isActive,omitempty"`
		AssignedShopID *string `json:"assignedShopId,omitempty"`
	}

	var req UpdateStaffRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	// Build query dynamically based on provided fields
	// (Implementation omitted for brevity, but would be similar to HandleUpdateMerchantProfile)

	query := `
		UPDATE users
		SET name = COALESCE($1, name), is_active = COALESCE($2, is_active), assigned_shop_id = $3, updated_at = NOW()
		WHERE id = $4 AND merchant_id = $5 AND role = 'staff'
		RETURNING id, name, email, role, is_active, merchant_id, assigned_shop_id, created_at, updated_at
	`

	var updatedStaff models.User
	err := db.QueryRow(ctx, query, req.Name, req.IsActive, req.AssignedShopID, staffId, merchantId).Scan(
		&updatedStaff.ID, &updatedStaff.Name, &updatedStaff.Email, &updatedStaff.Role, &updatedStaff.IsActive, &updatedStaff.MerchantID, &updatedStaff.AssignedShopID, &updatedStaff.CreatedAt, &updatedStaff.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error updating merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update staff member."})
	}

	return c.JSON(fiber.Map{"status": "success", "data": updatedStaff})
}

// HandleDeleteMerchantStaff handles deleting a staff member for the current merchant.
func HandleDeleteMerchantStaff(c *fiber.Ctx) error {
	db := database.GetDB()
	ctx := context.Background()

	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	merchantId := claims["userId"].(string)
	staffId := c.Params("staffId")

	query := "DELETE FROM users WHERE id = $1 AND merchant_id = $2 AND role = 'staff'"

	res, err := db.Exec(ctx, query, staffId, merchantId)
	if err != nil {
		log.Printf("Error deleting merchant staff: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if res.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Staff member not found or you do not have permission to delete it."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success"})
}
