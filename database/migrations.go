package database

import (
	"context"
)

// EnsureSyncSchema applies compatibility changes required by the offline POS.
// The complete clean schema is defined in schema.sql; these statements remain
// idempotent for deployments that start against an already-created database.
func EnsureSyncSchema(ctx context.Context) error {
	db := GetDB()
	if db == nil {
		return nil
	}

	statements := []string{
		`DROP TABLE IF EXISTS user_roles CASCADE`,
		`DROP TABLE IF EXISTS roles CASCADE`,
		`DROP TABLE IF EXISTS shop_staff CASCADE`,
		`DROP TABLE IF EXISTS product_attributes CASCADE`,
		`DROP TABLE IF EXISTS testimonials CASCADE`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_product_variants_merchant_product_id ON product_variants (merchant_id, product_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_product_variants_product_id ON product_variants (product_id, id)`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_stock_items_product_variant_same_product') THEN ALTER TABLE stock_items ADD CONSTRAINT fk_stock_items_product_variant_same_product FOREIGN KEY (merchant_id, product_id, variant_id) REFERENCES product_variants (merchant_id, product_id, id); END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_product_prices_product_variant_same_product') THEN ALTER TABLE product_prices ADD CONSTRAINT fk_product_prices_product_variant_same_product FOREIGN KEY (merchant_id, product_id, variant_id) REFERENCES product_variants (merchant_id, product_id, id) ON DELETE CASCADE; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_inventory_items_product_variant_same_product') THEN ALTER TABLE inventory_items ADD CONSTRAINT fk_inventory_items_product_variant_same_product FOREIGN KEY (merchant_id, product_id, variant_id) REFERENCES product_variants (merchant_id, product_id, id); END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_sale_items_product_variant_same_product') THEN ALTER TABLE sale_items ADD CONSTRAINT fk_sale_items_product_variant_same_product FOREIGN KEY (product_id, variant_id) REFERENCES product_variants (product_id, id); END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_product_attribute_assignments_product_variant_same_product') THEN ALTER TABLE product_attribute_assignments ADD CONSTRAINT fk_product_attribute_assignments_product_variant_same_product FOREIGN KEY (product_id, variant_id) REFERENCES product_variants (product_id, id); END IF; END $$`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS source_type VARCHAR(30) NOT NULL DEFAULT 'URL'`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS original_url TEXT`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS storage_provider VARCHAR(30)`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS storage_public_id VARCHAR(512)`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS storage_object_name VARCHAR(512)`,
		`ALTER TABLE product_images ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		`ALTER TABLE sales ADD COLUMN IF NOT EXISTS client_sale_id TEXT`,
		`ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_client_sale_id_key`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uq_sales_merchant_client_sale') THEN ALTER TABLE sales ADD CONSTRAINT uq_sales_merchant_client_sale UNIQUE (merchant_id, client_sale_id); END IF; END $$`,
		`ALTER TABLE sales ADD COLUMN IF NOT EXISTS delivery_charge NUMERIC(10, 2) DEFAULT 0.00`,
		`ALTER TABLE sales ALTER COLUMN delivery_charge SET DEFAULT 0.00`,
		`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS delivery_charge NUMERIC(10, 2) DEFAULT 0.00`,
		`ALTER TABLE invoices ALTER COLUMN delivery_charge SET DEFAULT 0.00`,
		`ALTER TABLE product_categories ADD COLUMN IF NOT EXISTS merchant_id UUID`,
		`UPDATE product_categories pc SET merchant_id = p.merchant_id FROM products p WHERE p.id = pc.product_id AND pc.merchant_id IS NULL`,
		`ALTER TABLE product_categories ALTER COLUMN merchant_id SET NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_products_merchant_id_id ON products (merchant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_categories_merchant_id_id ON categories (merchant_id, id)`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_product_categories_product_same_merchant') THEN ALTER TABLE product_categories ADD CONSTRAINT fk_product_categories_product_same_merchant FOREIGN KEY (merchant_id, product_id) REFERENCES products(merchant_id, id) ON DELETE CASCADE; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_product_categories_category_same_merchant') THEN ALTER TABLE product_categories ADD CONSTRAINT fk_product_categories_category_same_merchant FOREIGN KEY (merchant_id, category_id) REFERENCES categories(merchant_id, id) ON DELETE CASCADE; END IF; END $$`,
		`ALTER TABLE promotion_products ADD COLUMN IF NOT EXISTS merchant_id UUID`,
		`UPDATE promotion_products pp SET merchant_id = p.merchant_id FROM promotions p WHERE p.id = pp.promotion_id AND pp.merchant_id IS NULL`,
		`ALTER TABLE promotion_products ALTER COLUMN merchant_id SET NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_promotions_merchant_id_id ON promotions (merchant_id, id)`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_promotion_products_promotion_same_merchant') THEN ALTER TABLE promotion_products ADD CONSTRAINT fk_promotion_products_promotion_same_merchant FOREIGN KEY (merchant_id, promotion_id) REFERENCES promotions(merchant_id, id) ON DELETE CASCADE; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='fk_promotion_products_product_same_merchant') THEN ALTER TABLE promotion_products ADD CONSTRAINT fk_promotion_products_product_same_merchant FOREIGN KEY (merchant_id, product_id) REFERENCES products(merchant_id, id) ON DELETE CASCADE; END IF; END $$`,
		`CREATE INDEX IF NOT EXISTS idx_product_categories_merchant ON product_categories (merchant_id, category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_promotion_products_product ON promotion_products (merchant_id, product_id)`,
		`CREATE TABLE IF NOT EXISTS inventory_operations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			client_operation_id TEXT UNIQUE NOT NULL,
			operation_type VARCHAR(80) NOT NULL,
			actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
			shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_id VARCHAR(255),
			total_sales INTEGER NOT NULL DEFAULT 0,
			successful_syncs INTEGER NOT NULL DEFAULT 0,
			failed_syncs INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_pos_sessions_one_open_per_terminal ON pos_sessions (terminal_id) WHERE status = 'OPEN' AND terminal_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_stock_items_merchant_name ON stock_items (merchant_id, name)`,
		`CREATE TABLE IF NOT EXISTS measurement_groups (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			code VARCHAR(100) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS unit_definitions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			measurement_group_id UUID REFERENCES measurement_groups(id) ON DELETE SET NULL,
			code VARCHAR(100) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			symbol VARCHAR(50),
			allows_decimal BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stock_item_configurations (
			stock_item_id UUID PRIMARY KEY REFERENCES stock_items(id) ON DELETE CASCADE,
			track_batches BOOLEAN NOT NULL DEFAULT FALSE,
			track_expiry BOOLEAN NOT NULL DEFAULT FALSE,
			track_unique_assets BOOLEAN NOT NULL DEFAULT FALSE,
			track_reservations BOOLEAN NOT NULL DEFAULT FALSE,
			allow_unit_conversions BOOLEAN NOT NULL DEFAULT FALSE,
			allow_pack_breaking BOOLEAN NOT NULL DEFAULT FALSE,
			allow_multiple_barcodes BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stock_item_units (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			stock_item_id UUID NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			unit_id UUID NOT NULL REFERENCES unit_definitions(id) ON DELETE RESTRICT,
			conversion_to_base NUMERIC(20,8) NOT NULL CHECK (conversion_to_base > 0),
			is_base_unit BOOLEAN NOT NULL DEFAULT FALSE,
			is_sales_unit BOOLEAN NOT NULL DEFAULT FALSE,
			is_purchase_unit BOOLEAN NOT NULL DEFAULT FALSE,
			allows_fractional BOOLEAN NOT NULL DEFAULT FALSE,
			position INTEGER NOT NULL DEFAULT 0,
			UNIQUE (stock_item_id, unit_id)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_item_unit_conversions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			stock_item_id UUID NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			from_unit_id UUID NOT NULL REFERENCES unit_definitions(id) ON DELETE RESTRICT,
			to_unit_id UUID NOT NULL REFERENCES unit_definitions(id) ON DELETE RESTRICT,
			factor NUMERIC(20,8) NOT NULL CHECK (factor > 0),
			UNIQUE (stock_item_id, from_unit_id, to_unit_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_client_merchant ON sales (merchant_id, client_sale_id)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_orders_merchant_created ON purchase_orders (merchant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_merchant_created ON suppliers (merchant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_promotions_merchant_active ON promotions (merchant_id, is_active, start_date, end_date)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_shop_date ON invoices (shop_id, invoice_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_support_tickets_shop_status ON support_tickets (shop_id, status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_support_messages_ticket_created ON support_messages (ticket_id, created_at ASC)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}
