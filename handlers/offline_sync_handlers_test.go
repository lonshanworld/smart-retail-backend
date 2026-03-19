package handlers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRow implements DBRow for tests.
type fakeRow struct {
	scanFunc func(dest ...interface{}) error
}

func (r fakeRow) Scan(dest ...interface{}) error { return r.scanFunc(dest...) }

// fakeTx is a simple DBTx fake that returns pre-programmed responses.
type fakeTx struct {
	duplicateExistingID string
	priceLookupError    bool
}

func (f fakeTx) QueryRow(ctx context.Context, sql string, args ...interface{}) DBRow {
	// Shop ownership check
	if len(args) == 1 {
		if shopID, ok := args[0].(string); ok && shopID != "" {
			return fakeRow{scanFunc: func(dest ...interface{}) error {
				if len(dest) > 0 {
					if p, ok := dest[0].(*string); ok {
						*p = "merchant-1"
					}
				}
				return nil
			}}
		}
	}

	// Duplicate check
	if len(args) >= 2 {
		if _, ok := args[0].(string); ok {
			if f.duplicateExistingID != "" {
				return fakeRow{scanFunc: func(dest ...interface{}) error {
					if len(dest) > 0 {
						if p, ok := dest[0].(*string); ok {
							*p = f.duplicateExistingID
						}
					}
					return nil
				}}
			}
			return fakeRow{scanFunc: func(dest ...interface{}) error { return errors.New("no rows in result set") }}
		}
	}

	// Price lookup
	if len(args) >= 2 {
		if _, ok := args[0].(string); ok {
			if f.priceLookupError {
				return fakeRow{scanFunc: func(dest ...interface{}) error { return errors.New("no rows in result set") }}
			}
			return fakeRow{scanFunc: func(dest ...interface{}) error {
				if len(dest) > 0 {
					if p, ok := dest[0].(*float64); ok {
						*p = 9.99
					}
				}
				return nil
			}}
		}
	}

	return fakeRow{scanFunc: func(dest ...interface{}) error { return errors.New("no rows in result set") }}
}

func (f fakeTx) Exec(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	return 1, nil
}

func TestProcessSaleSync_DuplicateDetection(t *testing.T) {
	ctx := context.Background()
	tx := fakeTx{duplicateExistingID: "existing-123"}
	offline := OfflineSaleData{
		ID:          "local-1",
		ShopID:      "shop-1",
		TotalAmount: 10.0,
		Items:       []OfflineSaleItem{},
		PaymentType: "cash",
		Timestamp:   time.Now(),
	}

	res := processSaleSyncWithDB(ctx, tx, "merchant-1", offline)
	if res.Status != "synced" {
		t.Fatalf("expected synced, got %s", res.Status)
	}
	if res.ServerID == nil || *res.ServerID != "existing-123" {
		t.Fatalf("expected server id existing-123, got %v", res.ServerID)
	}
}

func TestProcessSaleSync_ItemNotFound(t *testing.T) {
	ctx := context.Background()
	tx := fakeTx{priceLookupError: true}
	offline := OfflineSaleData{
		ID:          "local-2",
		ShopID:      "shop-1",
		TotalAmount: 10.0,
		Items: []OfflineSaleItem{{
			ProductID:          "prod-1",
			Quantity:           1,
			SellingPriceAtSale: 9.99,
		}},
		PaymentType: "cash",
		Timestamp:   time.Now(),
	}

	res := processSaleSyncWithDB(ctx, tx, "merchant-1", offline)
	if res.Status != "failed" {
		t.Fatalf("expected failed, got %s", res.Status)
	}
	if res.Error == nil {
		t.Fatalf("expected error message for missing item")
	}
}
