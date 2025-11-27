package main

import (
	"testing"

	"app/utils"
)

func TestValidateAndNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"Admin", "admin", true},
		{"merchant", "merchant", true},
		{"STAFF", "staff", true},
		{"unknown", "unknown", false},
	}

	for _, c := range cases {
		got, ok := utils.ValidateAndNormalizeRole(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("ValidateAndNormalizeRole(%q) = (%q, %v); want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsValidRole(t *testing.T) {
	if !utils.IsValidRole("admin") {
		t.Fatalf("expected admin to be valid")
	}
	if utils.IsValidRole("not-a-role") {
		t.Fatalf("expected not-a-role to be invalid")
	}
}
