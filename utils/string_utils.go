package utils

import "database/sql"

// NullStringToStringPtr converts a sql.NullString to a *string.
func NullStringToStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// PointerToString converts a string pointer to string, returning "<nil>" if the pointer is nil
func PointerToString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
