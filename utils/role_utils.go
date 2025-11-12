package utils

import (
	"strings"
)

var ValidUserRoles = map[string]bool{
	"admin":    true,
	"merchant": true,
	"staff":    true,
}

// ValidateAndNormalizeRole validates and normalizes a role string.
// Returns the normalized role (lowercase) and a boolean indicating if it's valid.
func ValidateAndNormalizeRole(role string) (string, bool) {
	normalized := strings.ToLower(role)
	return normalized, ValidUserRoles[normalized]
}

// IsValidRole checks if a role is valid without normalizing it
func IsValidRole(role string) bool {
	return ValidUserRoles[strings.ToLower(role)]
}
