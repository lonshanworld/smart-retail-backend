package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"app/database"
	"app/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret is a temporary secret key. In production, this should be loaded from a secure configuration.
var JWTSecret = []byte("your-super-secret-key")

// HandleCreateUser creates a new user in the system (admin, merchant, or staff).
// POST /api/v1/auth/users (also used by POST /api/v1/admin/users)
func HandleCreateUser(c *fiber.Ctx) error {
	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Error parsing user creation request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.Name == "" || req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Missing required fields (name, email, password, role)"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not process password"})
	}

	query := `
        INSERT INTO users (name, email, password_hash, role, merchant_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, name, email, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
    `

	var createdUser models.User
	var phone, assignedShopID, merchantID sql.NullString

	err = database.GetDB().QueryRow(query, req.Name, req.Email, string(hashedPassword), req.Role, req.MerchantID).Scan(
		&createdUser.ID,
		&createdUser.Name,
		&createdUser.Email,
		&createdUser.Role,
		&createdUser.IsActive,
		&phone,
		&assignedShopID,
		&merchantID,
		&createdUser.CreatedAt,
		&createdUser.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating user in database: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("Could not create user: %v", err)})
	}

	if phone.Valid {
		createdUser.Phone = &phone.String
	}
	if assignedShopID.Valid {
		createdUser.AssignedShopID = &assignedShopID.String
	}
	if merchantID.Valid {
		createdUser.MerchantID = &merchantID.String
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": createdUser})
}

// HandleLogin authenticates a user and returns a JWT token.
func HandleLogin(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	var user models.User
	var passwordHash string
	var phone, assignedShopID, merchantID sql.NullString

	query := `
		SELECT id, name, email, password_hash, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
		FROM users
		WHERE email = $1 AND role = $2`

	err := database.GetDB().QueryRow(query, req.Email, req.UserType).Scan(
		&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials or user role"})
		}
		log.Printf("Database error during login for email %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "User account is inactive"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials"})
	}

	token, err := createJWT(user.ID, user.Role)
	if err != nil {
		log.Printf("Error creating JWT for user %s: %v", user.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not sign token"})
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

	return c.JSON(fiber.Map{"accessToken": token, "user": user})
}

// HandleShopLogin authenticates a staff member for a specific shop.
func HandleShopLogin(c *fiber.Ctx) error {
	var req models.ShopLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Cannot parse JSON"})
	}

	var user models.User
	var passwordHash string
	var phone, merchantID, assignedShopID sql.NullString

	query := `
		SELECT id, name, email, password_hash, role, is_active, phone, assigned_shop_id, merchant_id, created_at, updated_at
		FROM users
		WHERE email = $1 AND assigned_shop_id = $2 AND role = 'staff'`

	err := database.GetDB().QueryRow(query, req.Email, req.ShopID).Scan(
		&user.ID, &user.Name, &user.Email, &passwordHash, &user.Role, &user.IsActive,
		&phone, &assignedShopID, &merchantID,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials or not a staff for this shop"})
		}
		log.Printf("Database error during shop login for email %s: %v", req.Email, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Database error"})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Staff account is inactive"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "Invalid credentials"})
	}

	token, err := createJWT(user.ID, user.Role)
	if err != nil {
		log.Printf("Error creating JWT for staff %s: %v", user.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not sign token"})
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

	shop, err := fetchShop(req.ShopID)
	if err != nil {
		log.Printf("Error fetching shop details for shop %s: %v", req.ShopID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to fetch shop details"})
	}

	return c.JSON(fiber.Map{"accessToken": token, "user": user, "shop": shop})
}

// --- Helper Functions ---

func createJWT(userID, role string) (string, error) {
	claims := models.JwtClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func fetchShop(shopID string) (*models.Shop, error) {
	var shop models.Shop
    var address, phone sql.NullString
	query := `SELECT id, name, merchant_id, address, phone, is_active, is_primary, created_at, updated_at FROM shops WHERE id = $1`
	err := database.GetDB().QueryRow(query, shopID).Scan(
		&shop.ID, &shop.Name, &shop.MerchantID, &address, &phone, &shop.IsActive, &shop.IsPrimary, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
    if address.Valid {
        shop.Address = &address.String
    }
    if phone.Valid {
        shop.Phone = &phone.String
    }
	return &shop, nil
}
