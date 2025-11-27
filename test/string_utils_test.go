package main

import (
	"database/sql"
	"testing"

	"app/utils"
)

func TestNullStringToStringPtr(t *testing.T) {
	ns := sql.NullString{String: "hello", Valid: true}
	p := utils.NullStringToStringPtr(ns)
	if p == nil || *p != "hello" {
		t.Fatalf("expected pointer to 'hello', got %v", p)
	}

	ns2 := sql.NullString{Valid: false}
	p2 := utils.NullStringToStringPtr(ns2)
	if p2 != nil {
		t.Fatalf("expected nil pointer, got %v", p2)
	}
}

func TestPointerToString(t *testing.T) {
	s := "world"
	if utils.PointerToString(&s) != "world" {
		t.Fatalf("expected 'world'")
	}
	if utils.PointerToString(nil) != "<nil>" {
		t.Fatalf("expected '<nil>' for nil pointer")
	}
}
