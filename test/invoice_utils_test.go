package main

import (
	"context"
	"testing"

	"app/utils"
)

func TestGenerateInvoiceNumberUnsupportedDB(t *testing.T) {
	_, err := utils.GenerateInvoiceNumber(context.Background(), struct{}{})
	if err == nil {
		t.Fatalf("expected error for unsupported DB type")
	}
}
