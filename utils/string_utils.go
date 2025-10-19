package utils

import "database/sql"

// NullStringToStringPtr converts a sql.NullString to a *string.
func NullStringToStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
