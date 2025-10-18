package handlers

import (
	"context"
	"fmt"
	"time"

	"app/database"
	"app/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// HandleLogin authenticates a user and returns a JWT token along with the user object.
func HandleLogin(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot parse JSON"})
	}

	var id int
	var passwordHash, role, tableName string
	var query string

	switch req.UserType {
	case "admin":
		query = "SELECT id, password_hash, 'admin' as role FROM admins WHERE email = $1"
		tableName = "admins"
	case "merchant":
		query = "SELECT id, password_hash, 'merchant' as role FROM merchants WHERE email = $1"
		tableName = "merchants"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid user type"})
	}

	err := database.DB.QueryRow(context.Background(), query, req.Email).Scan(&id, &passwordHash, &role)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid credentials"})
	}

	userIDStr := fmt.Sprintf("%d", id)
	token, err := createJWT(userIDStr, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Could not generate token"})
	}

	user, err := fetchUser(userIDStr, tableName, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch user details"})
	}

	return c.JSON(fiber.Map{"accessToken": token, "user": user})
}

// HandleShopLogin authenticates a staff member and returns a JWT, user, and shop object.
func HandleShopLogin(c *fiber.Ctx) error {
	var req models.ShopLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot parse JSON"})
	}

	var staffID int
	var passwordHash string
	query := "SELECT id, password_hash FROM staff WHERE email = $1 AND shop_id = $2 AND is_active = TRUE"

	err := database.DB.QueryRow(context.Background(), query, req.Email, req.ShopID).Scan(&staffID, &passwordHash)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid credentials or not an active staff for this shop"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid credentials"})
	}

	staffIDStr := fmt.Sprintf("%d", staffID)
	token, err := createJWT(staffIDStr, "staff")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Could not generate token"})
	}

	user, err := fetchUser(staffIDStr, "staff", "staff")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch user details"})
	}

	shop, err := fetchShop(req.ShopID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch shop details"})
	}

	return c.JSON(fiber.Map{"accessToken": token, "user": user, "shop": shop})
}

// fetchShop fetches the full shop details.
func fetchShop(shopID string) (*models.Shop, error) {
	var shop models.Shop
	query := `SELECT id, name, merchant_id, is_active, created_at, updated_at FROM shops WHERE id = $1`
	err := database.DB.QueryRow(context.Background(), query, shopID).Scan(
		&shop.ID, &shop.Name, &shop.MerchantID, &shop.IsActive, &shop.CreatedAt, &shop.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

// The rest of the file (HandleCreateUser, createJWT, fetchUser) remains the same...

func HandleCreateUser(c *fiber.Ctx) error {
	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot parse JSON"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Could not hash password"})
	}

	var query string
	var params []interface{}
	var tableName string

	switch req.Role {
	case "admin":
		query = "INSERT INTO admins (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id"
		params = []interface{}{req.Name, req.Email, string(hashedPassword)}
		tableName = "admins"
	case "merchant":
		query = "INSERT INTO merchants (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id"
		params = []interface{}{req.Name, req.Email, string(hashedPassword)}
		tableName = "merchants"
	case "staff":
		if req.MerchantID == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "MerchantID is required for staff"})
		}
		query = "INSERT INTO staff (name, email, password_hash, merchant_id) VALUES ($1, $2, $3, $4) RETURNING id"
		params = []interface{}{req.Name, req.Email, string(hashedPassword), *req.MerchantID}
		tableName = "staff"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid role specified"})
	}

	var userID int
	err = database.DB.QueryRow(context.Background(), query, params...).Scan(&userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Could not create user: %v", err)})
	}

	createdUser, err := fetchUser(fmt.Sprintf("%d", userID), tableName, req.Role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Could not fetch created user: %v", err)})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": createdUser})
}

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

func fetchUser(userID, tableName, role string) (*models.User, error) {
	var user models.User
	user.Role = role

	query := fmt.Sprintf(`SELECT id, name, email, phone, is_active, created_at, updated_at FROM %s WHERE id = $1`, tableName)
	err := database.DB.QueryRow(context.Background(), query, userID).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}
