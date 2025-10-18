package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"

	"app/models"
)

var JWTSecret []byte

// JWTMiddleware validates the JWT token provided in the Authorization header.
func JWTMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Missing or malformed JWT"})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Missing or malformed JWT"})
	}

	tokenStr := parts[1]
	claims := &models.JwtClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token signing method is what you expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.ErrUnauthorized
		}
		return JWTSecret, nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid or expired JWT"})
	}

	c.Locals("userID", claims.UserID)
	c.Locals("userRole", claims.Role)

	return c.Next()
}

// AdminRequired is a middleware function that checks if the user has an 'admin' role.
func AdminRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	if !ok || role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Admin access required"})
	}
	return c.Next()
}

// MerchantRequired is a middleware function that checks if the user has a 'merchant' role.
func MerchantRequired(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	if !ok || role != "merchant" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Merchant access required"})
	}
	return c.Next()
}
