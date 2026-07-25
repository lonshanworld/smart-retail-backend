-- Smart Retail PostgreSQL schema
--
-- This is a clean multi-shop POS schema. It intentionally does not include
-- e-commerce-only concepts such as carts, guest checkout, or web orders.
-- Run this against a new database, or migrate existing data into these tables
-- before removing the previous database.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ================================================================
-- Identity, tenancy, shops, and security
-- ================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'merchant', 'staff')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    merchant_id UUID REFERENCES users(id) ON DELETE SET NULL,
    assigned_shop_id UUID,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    before_data JSONB,
    after_data JSONB,
    metadata JSONB,
    ip_address VARCHAR(45),
    user_agent VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE shops (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    phone VARCHAR(50),
    business_type VARCHAR(100) NOT NULL DEFAULT 'retail',
    tax_rate NUMERIC(5,2) NOT NULL DEFAULT 5.00 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    opening_hours JSONB,
    supports_delivery BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, name),
    UNIQUE (merchant_id, id)
);

ALTER TABLE users ADD CONSTRAINT fk_users_assigned_shop
    FOREIGN KEY (assigned_shop_id) REFERENCES shops(id) ON DELETE SET NULL;

CREATE TABLE shop_staff (
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (shop_id, user_id)
);

-- ================================================================
-- Catalog
-- ================================================================

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    image_url TEXT,
    path VARCHAR(2048),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, slug),
    UNIQUE (merchant_id, id)
);

CREATE TABLE brands (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, name),
    UNIQUE (merchant_id, id)
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand_id UUID,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    description TEXT,
    short_description VARCHAR(500),
    product_type VARCHAR(30) NOT NULL DEFAULT 'PHYSICAL'
        CHECK (product_type IN ('PHYSICAL', 'DIGITAL', 'SERVICE', 'FOOD')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    is_stock_tracked BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, slug),
    UNIQUE (merchant_id, id),
    FOREIGN KEY (merchant_id, brand_id) REFERENCES brands(merchant_id, id) ON DELETE SET NULL
);

CREATE TABLE product_categories (
	merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (product_id, category_id),
    CONSTRAINT fk_product_categories_product_same_merchant FOREIGN KEY (merchant_id, product_id) REFERENCES products(merchant_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_product_categories_category_same_merchant FOREIGN KEY (merchant_id, category_id) REFERENCES categories(merchant_id, id) ON DELETE CASCADE
);

CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sku VARCHAR(100) NOT NULL,
    barcode VARCHAR(100),
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (product_id, sku),
    UNIQUE (merchant_id, id)
);

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE product_attributes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    attribute_key VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    UNIQUE (product_id, attribute_key)
);

CREATE TABLE product_prices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID REFERENCES product_variants(id) ON DELETE CASCADE,
    price_type VARCHAR(30) NOT NULL CHECK (price_type IN ('COST', 'RETAIL', 'WHOLESALE', 'MEMBER', 'PROMOTION')),
    cost_price NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (cost_price >= 0),
    selling_price NUMERIC(15,2) NOT NULL CHECK (selling_price >= 0),
    wholesale_price NUMERIC(15,2) CHECK (wholesale_price >= 0),
    member_price NUMERIC(15,2) CHECK (member_price >= 0),
    promotion_price NUMERIC(15,2) CHECK (promotion_price >= 0),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE attribute_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    value_type VARCHAR(20) NOT NULL CHECK (value_type IN ('TEXT', 'NUMBER', 'BOOLEAN', 'SELECT', 'DATE', 'JSON')),
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, code)
);

CREATE TABLE attribute_definition_options (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    value VARCHAR(255) NOT NULL,
    label VARCHAR(255) NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    UNIQUE (definition_id, value)
);

CREATE TABLE product_attribute_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    definition_id UUID NOT NULL REFERENCES attribute_definitions(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID REFERENCES product_variants(id) ON DELETE CASCADE,
    value_text TEXT,
    value_number NUMERIC(18,6),
    value_boolean BOOLEAN,
    value_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ================================================================
-- Units, stock items, and inventory
-- ================================================================

CREATE TABLE measurement_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE unit_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    measurement_group_id UUID REFERENCES measurement_groups(id) ON DELETE SET NULL,
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    symbol VARCHAR(50),
    allows_decimal BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stock_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    sku VARCHAR(100),
    track_inventory BOOLEAN NOT NULL DEFAULT TRUE,
    tracking_mode VARCHAR(20) NOT NULL DEFAULT 'SIMPLE'
        CHECK (tracking_mode IN ('SIMPLE', 'BATCH', 'ASSET', 'SERIAL')),
    base_unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, sku),
    UNIQUE (merchant_id, id)
);

CREATE TABLE stock_item_configurations (
    stock_item_id UUID PRIMARY KEY REFERENCES stock_items(id) ON DELETE CASCADE,
    track_batches BOOLEAN NOT NULL DEFAULT FALSE,
    track_expiry BOOLEAN NOT NULL DEFAULT FALSE,
    track_unique_assets BOOLEAN NOT NULL DEFAULT FALSE,
    track_reservations BOOLEAN NOT NULL DEFAULT FALSE,
    allow_unit_conversions BOOLEAN NOT NULL DEFAULT FALSE,
    allow_pack_breaking BOOLEAN NOT NULL DEFAULT FALSE,
    allow_multiple_barcodes BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stock_item_units (
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
);

CREATE TABLE stock_item_unit_conversions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stock_item_id UUID NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
    from_unit_id UUID NOT NULL REFERENCES unit_definitions(id) ON DELETE RESTRICT,
    to_unit_id UUID NOT NULL REFERENCES unit_definitions(id) ON DELETE RESTRICT,
    factor NUMERIC(20,8) NOT NULL CHECK (factor > 0),
    UNIQUE (stock_item_id, from_unit_id, to_unit_id)
);

-- One inventory balance exists for each shop and stock item. Quantities are
-- changed through inventory_movements, not by arbitrary application updates.
CREATE TABLE inventory_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID NOT NULL REFERENCES stock_items(id) ON DELETE RESTRICT,
    variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    quantity_on_hand NUMERIC(15,3) NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    reserved_quantity NUMERIC(15,3) NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    low_stock_threshold NUMERIC(15,3) CHECK (low_stock_threshold >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id, stock_item_id)
);

CREATE TABLE inventory_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    batch_code VARCHAR(100) NOT NULL,
    quantity_received NUMERIC(15,3) NOT NULL CHECK (quantity_received >= 0),
    quantity_remaining NUMERIC(15,3) NOT NULL CHECK (quantity_remaining >= 0),
    unit_cost NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    manufacture_date DATE,
    expiry_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id, stock_item_id, batch_code)
);

CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    movement_type VARCHAR(20) NOT NULL CHECK (movement_type IN ('IN', 'OUT', 'ADJUSTMENT', 'RETURN', 'TRANSFER')),
    quantity NUMERIC(15,3) NOT NULL CHECK (quantity > 0),
    base_quantity NUMERIC(20,8),
    unit_cost NUMERIC(15,2),
    reference_type VARCHAR(30),
    reference_id UUID,
    event_key TEXT UNIQUE,
    movement_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

CREATE TABLE inventory_reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    reference_id UUID,
    reservation_key TEXT NOT NULL UNIQUE,
    quantity NUMERIC(15,3) NOT NULL CHECK (quantity > 0),
    base_quantity NUMERIC(20,8),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'RELEASED')),
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inventory_serials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    serial_number VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'SOLD', 'RETURNED')),
    reference_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE barcode_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(255) NOT NULL,
    normalized_code VARCHAR(255) NOT NULL,
    owner_type VARCHAR(30) NOT NULL CHECK (owner_type IN ('PRODUCT', 'VARIANT', 'STOCK_ITEM', 'UNIT', 'ASSET', 'BATCH')),
    owner_id UUID NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    is_generated BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, normalized_code)
);

CREATE TABLE inventory_identifier_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    validation_regex TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, code)
);

CREATE TABLE inventory_assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    batch_id UUID REFERENCES inventory_batches(id) ON DELETE SET NULL,
    asset_tag VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'AVAILABLE'
        CHECK (status IN ('AVAILABLE', 'RESERVED', 'SOLD', 'RETURNED', 'INACTIVE')),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, asset_tag)
);

CREATE TABLE inventory_asset_identifiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    asset_id UUID NOT NULL REFERENCES inventory_assets(id) ON DELETE CASCADE,
    identifier_type_id UUID NOT NULL REFERENCES inventory_identifier_types(id) ON DELETE RESTRICT,
    value VARCHAR(255) NOT NULL,
    normalized_value VARCHAR(255) NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (identifier_type_id, normalized_value)
);

CREATE TABLE inventory_transformations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE RESTRICT,
    transformation_type VARCHAR(20) NOT NULL
        CHECK (transformation_type IN ('PACK_BREAK', 'REPACK', 'ASSEMBLY', 'ADJUSTMENT')),
    reference_id UUID,
    notes TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inventory_transformation_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transformation_id UUID NOT NULL REFERENCES inventory_transformations(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('IN', 'OUT')),
    quantity NUMERIC(15,3) NOT NULL CHECK (quantity > 0),
    base_quantity NUMERIC(20,8),
    unit_cost NUMERIC(15,2)
);

CREATE TABLE inventory_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_operation_id TEXT NOT NULL UNIQUE,
    operation_type VARCHAR(80) NOT NULL,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inventory_reconciliation_exceptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    exception_key VARCHAR(255) NOT NULL UNIQUE,
    exception_type VARCHAR(80) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'RESOLVED')),
    entity_type VARCHAR(80) NOT NULL,
    entity_id UUID,
    payload JSONB,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ================================================================
-- Customers, promotions, sales, POS, and payments
-- ================================================================

CREATE TABLE shop_customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(50),
    customer_type VARCHAR(20) NOT NULL DEFAULT 'RETAIL' CHECK (customer_type IN ('RETAIL', 'WHOLESALE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id, email)
);

CREATE TABLE customer_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(30),
    UNIQUE (merchant_id, name)
);

CREATE TABLE customer_tag_map (
    customer_id UUID NOT NULL REFERENCES shop_customers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES customer_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (customer_id, tag_id)
);

CREATE TABLE customer_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES shop_customers(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE customer_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES shop_customers(id) ON DELETE CASCADE,
    event_key VARCHAR(255) UNIQUE,
    activity_type VARCHAR(100) NOT NULL,
    description TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE promotions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    promo_type VARCHAR(50) NOT NULL CHECK (promo_type IN ('PERCENTAGE', 'FIXED_AMOUNT', 'BOGO')),
    promo_value NUMERIC(15,2) NOT NULL CHECK (promo_value >= 0),
    min_spend NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (min_spend >= 0),
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_promotions_merchant_id_id UNIQUE (merchant_id, id)
);

CREATE TABLE promotion_products (
	merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    promotion_id UUID NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    PRIMARY KEY (promotion_id, product_id),
    CONSTRAINT fk_promotion_products_promotion_same_merchant FOREIGN KEY (merchant_id, promotion_id) REFERENCES promotions(merchant_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_promotion_products_product_same_merchant FOREIGN KEY (merchant_id, product_id) REFERENCES products(merchant_id, id) ON DELETE CASCADE
);

CREATE TABLE sales (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_sale_id TEXT,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    staff_id UUID REFERENCES users(id) ON DELETE SET NULL,
    customer_id UUID REFERENCES shop_customers(id) ON DELETE SET NULL,
    sale_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    total_amount NUMERIC(15,2) NOT NULL CHECK (total_amount >= 0),
    delivery_charge NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (delivery_charge >= 0),
    applied_promotion_id UUID REFERENCES promotions(id) ON DELETE SET NULL,
    discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    payment_type VARCHAR(50) NOT NULL,
    payment_status VARCHAR(50) NOT NULL DEFAULT 'succeeded',
    stripe_payment_intent_id VARCHAR(255) UNIQUE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_sales_merchant_client_sale UNIQUE (merchant_id, client_sale_id)
);

CREATE TABLE sale_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    product_id UUID REFERENCES products(id) ON DELETE RESTRICT,
    variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    item_name VARCHAR(255) NOT NULL,
    item_sku VARCHAR(100),
    quantity_sold NUMERIC(15,3) NOT NULL CHECK (quantity_sold > 0),
    selling_price_at_sale NUMERIC(15,2) NOT NULL CHECK (selling_price_at_sale >= 0),
    original_price_at_sale NUMERIC(15,2),
    subtotal NUMERIC(15,2) NOT NULL CHECK (subtotal >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE pos_terminals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    device_identifier VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id, name)
);

CREATE TABLE pos_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE RESTRICT,
    terminal_id UUID REFERENCES pos_terminals(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMPTZ,
    cash_in_hand NUMERIC(15,2) NOT NULL DEFAULT 0,
    expected_cash NUMERIC(15,2),
    counted_cash NUMERIC(15,2),
    variance NUMERIC(15,2),
    reconciled_at TIMESTAMPTZ,
    reconciled_by UUID REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED'))
);

CREATE TABLE pos_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES pos_sessions(id) ON DELETE RESTRICT,
    sale_id UUID NOT NULL UNIQUE REFERENCES sales(id) ON DELETE RESTRICT,
    total NUMERIC(15,2) NOT NULL CHECK (total >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    method VARCHAR(30) NOT NULL CHECK (method IN ('CASH', 'CARD', 'TRANSFER', 'ONLINE', 'QR_MANUAL')),
    amount NUMERIC(15,2) NOT NULL CHECK (amount >= 0),
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SUCCESS', 'FAILED', 'REFUNDED')),
    reference VARCHAR(255) UNIQUE,
    idempotency_key VARCHAR(255) UNIQUE,
    refund_of_payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE merchant_payment_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    provider_name VARCHAR(50) NOT NULL,
    account_name VARCHAR(255),
    account_number VARCHAR(255),
    qr_image_url TEXT,
    provider_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id, provider_name)
);

CREATE TABLE payment_proofs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    original_filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    storage_provider VARCHAR(30) NOT NULL DEFAULT 'MINIO',
    storage_public_id VARCHAR(512),
    public_url TEXT,
    size BIGINT NOT NULL CHECK (size >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    rejection_reason VARCHAR(500),
    uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_provider_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_session_id VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'CONFIRMED', 'CANCELLED', 'FAILED', 'EXPIRED')),
    payment_url TEXT,
    callback_data JSONB,
    confirmed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id UUID NOT NULL UNIQUE REFERENCES sales(id) ON DELETE CASCADE,
    invoice_number VARCHAR(50) NOT NULL UNIQUE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    customer_id UUID REFERENCES shop_customers(id) ON DELETE SET NULL,
    invoice_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    due_date TIMESTAMPTZ,
    subtotal NUMERIC(15,2) NOT NULL,
    discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    tax_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    delivery_charge NUMERIC(15,2) NOT NULL DEFAULT 0,
    total_amount NUMERIC(15,2) NOT NULL,
    payment_status VARCHAR(30) NOT NULL DEFAULT 'paid',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ================================================================
-- Purchasing
-- ================================================================

CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    contact_name VARCHAR(255),
    contact_email VARCHAR(255),
    contact_phone VARCHAR(50),
    address TEXT,
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, name)
);

CREATE TABLE purchase_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE RESTRICT,
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    status VARCHAR(30) NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'APPROVED', 'PARTIALLY_RECEIVED', 'RECEIVED', 'CANCELLED')),
    subtotal NUMERIC(15,2) NOT NULL DEFAULT 0,
    tax NUMERIC(15,2) NOT NULL DEFAULT 0,
    total NUMERIC(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchase_order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    quantity NUMERIC(15,3) NOT NULL CHECK (quantity > 0),
    base_quantity NUMERIC(20,8),
    received_quantity NUMERIC(15,3) NOT NULL DEFAULT 0,
    unit_cost NUMERIC(15,2) NOT NULL CHECK (unit_cost >= 0),
    total_cost NUMERIC(15,2) NOT NULL CHECK (total_cost >= 0)
);

CREATE TABLE goods_receipts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_key VARCHAR(255) UNIQUE,
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE RESTRICT,
    received_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE goods_receipt_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    goods_receipt_id UUID NOT NULL REFERENCES goods_receipts(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    stock_item_id UUID REFERENCES stock_items(id) ON DELETE SET NULL,
    unit_id UUID REFERENCES unit_definitions(id) ON DELETE SET NULL,
    quantity NUMERIC(15,3) NOT NULL CHECK (quantity > 0),
    base_quantity NUMERIC(20,8),
    unit_cost NUMERIC(15,2) NOT NULL CHECK (unit_cost >= 0),
    batch_code VARCHAR(100),
    expiry_date DATE
);

CREATE TABLE supplier_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE RESTRICT,
    total_amount NUMERIC(15,2) NOT NULL CHECK (total_amount >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'UNPAID' CHECK (status IN ('UNPAID', 'PARTIAL', 'PAID')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE supplier_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_invoice_id UUID NOT NULL REFERENCES supplier_invoices(id) ON DELETE RESTRICT,
    amount NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    payment_method VARCHAR(30) NOT NULL,
    payment_date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ================================================================
-- Accounting
-- ================================================================

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL,
    account_type VARCHAR(20) NOT NULL CHECK (account_type IN ('ASSET', 'LIABILITY', 'EQUITY', 'INCOME', 'EXPENSE')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, code)
);

CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    reference_type VARCHAR(30) NOT NULL,
    reference_id UUID,
    event_key VARCHAR(255) UNIQUE,
    description VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE journal_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    debit NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (credit >= 0),
    CHECK (NOT (debit > 0 AND credit > 0)),
    CHECK (debit > 0 OR credit > 0)
);

CREATE TABLE ledger_balances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    period DATE NOT NULL,
    balance NUMERIC(15,2) NOT NULL DEFAULT 0,
    UNIQUE (account_id, period)
);

-- ================================================================
-- Staff, notifications, support, files, and migration support
-- ================================================================

CREATE TABLE staff_contracts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    salary NUMERIC(15,2) NOT NULL CHECK (salary >= 0),
    pay_frequency VARCHAR(50) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE salary_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_date DATE NOT NULL,
    amount_paid NUMERIC(15,2) NOT NULL CHECK (amount_paid >= 0),
    payment_period_start DATE NOT NULL,
    payment_period_end DATE NOT NULL,
    payment_method VARCHAR(50),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    notification_type VARCHAR(50),
    related_entity_type VARCHAR(50),
    related_entity_id UUID,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    qr_image_url TEXT,
    tax NUMERIC(5,2) NOT NULL DEFAULT 0,
    service_charge NUMERIC(15,2) NOT NULL DEFAULT 0,
    delivery_charge NUMERIC(15,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id)
);

CREATE TABLE support_tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    subject VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'OPEN',
    priority VARCHAR(30) NOT NULL DEFAULT 'MEDIUM',
    customer_name VARCHAR(255),
    customer_email VARCHAR(255),
    customer_phone VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE support_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_role VARCHAR(50) NOT NULL DEFAULT 'CUSTOMER',
    content TEXT NOT NULL,
    is_admin_reply BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE testimonials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(255),
    content TEXT NOT NULL,
    rating SMALLINT NOT NULL DEFAULT 5 CHECK (rating >= 1 AND rating <= 5),
    avatar TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE file_objects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bucket VARCHAR(255) NOT NULL,
    object_name VARCHAR(512) NOT NULL UNIQUE,
    original_filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    storage_provider VARCHAR(30) NOT NULL DEFAULT 'MINIO',
    storage_public_id VARCHAR(512),
    public_url TEXT,
    uploaded_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE system_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    setting_key VARCHAR(255) NOT NULL UNIQUE,
    value_json JSONB NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ai_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    provider_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (merchant_id, name)
);

CREATE TABLE ai_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ai_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID REFERENCES ai_sessions(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES ai_providers(id) ON DELETE SET NULL,
    module_source VARCHAR(50) NOT NULL,
    input_json JSONB NOT NULL,
    output_json JSONB,
    tokens_used INTEGER,
    status VARCHAR(20) NOT NULL CHECK (status IN ('SUCCESS', 'FAILED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE external_id_map (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(100) NOT NULL,
    source_id TEXT NOT NULL,
    target_table VARCHAR(100) NOT NULL,
    target_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source, source_id, target_table)
);

CREATE TABLE migration_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(100) NOT NULL,
    source_id TEXT,
    target_table VARCHAR(100),
    payload JSONB,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sync_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255),
    total_sales INTEGER NOT NULL DEFAULT 0 CHECK (total_sales >= 0),
    successful_syncs INTEGER NOT NULL DEFAULT 0 CHECK (successful_syncs >= 0),
    failed_syncs INTEGER NOT NULL DEFAULT 0 CHECK (failed_syncs >= 0),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Tenant-integrity constraints.  The application also scopes every query,
-- while these composite foreign keys prevent cross-merchant references when
-- data is written through a future integration or direct SQL client.
ALTER TABLE users ADD CONSTRAINT uq_users_merchant_id_id UNIQUE (merchant_id, id);
ALTER TABLE categories ADD CONSTRAINT fk_categories_parent_same_merchant
    FOREIGN KEY (merchant_id, parent_id) REFERENCES categories (merchant_id, id);
ALTER TABLE stock_items ADD CONSTRAINT fk_stock_items_product_same_merchant
    FOREIGN KEY (merchant_id, product_id) REFERENCES products (merchant_id, id);
ALTER TABLE product_variants ADD CONSTRAINT fk_product_variants_product_same_merchant
    FOREIGN KEY (merchant_id, product_id) REFERENCES products (merchant_id, id);
ALTER TABLE stock_items ADD CONSTRAINT fk_stock_items_variant_same_merchant
    FOREIGN KEY (merchant_id, variant_id) REFERENCES product_variants (merchant_id, id);
ALTER TABLE product_prices ADD CONSTRAINT fk_product_prices_product_same_merchant
    FOREIGN KEY (merchant_id, product_id) REFERENCES products (merchant_id, id);
ALTER TABLE product_prices ADD CONSTRAINT fk_product_prices_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE product_prices ADD CONSTRAINT fk_product_prices_variant_same_merchant
    FOREIGN KEY (merchant_id, variant_id) REFERENCES product_variants (merchant_id, id);
ALTER TABLE inventory_items ADD CONSTRAINT fk_inventory_items_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE inventory_items ADD CONSTRAINT fk_inventory_items_product_same_merchant
    FOREIGN KEY (merchant_id, product_id) REFERENCES products (merchant_id, id);
ALTER TABLE inventory_items ADD CONSTRAINT fk_inventory_items_stock_same_merchant
    FOREIGN KEY (merchant_id, stock_item_id) REFERENCES stock_items (merchant_id, id);
ALTER TABLE inventory_items ADD CONSTRAINT fk_inventory_items_variant_same_merchant
    FOREIGN KEY (merchant_id, variant_id) REFERENCES product_variants (merchant_id, id);
ALTER TABLE shop_customers ADD CONSTRAINT uq_shop_customers_merchant_id UNIQUE (merchant_id, id);
ALTER TABLE shop_customers ADD CONSTRAINT fk_shop_customers_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE promotions ADD CONSTRAINT fk_promotions_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE sales ADD CONSTRAINT fk_sales_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE sales ADD CONSTRAINT fk_sales_staff_same_merchant
    FOREIGN KEY (merchant_id, staff_id) REFERENCES users (merchant_id, id);
ALTER TABLE sales ADD CONSTRAINT fk_sales_customer_same_merchant
    FOREIGN KEY (merchant_id, customer_id) REFERENCES shop_customers (merchant_id, id);
ALTER TABLE invoices ADD CONSTRAINT fk_invoices_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE invoices ADD CONSTRAINT fk_invoices_customer_same_merchant
    FOREIGN KEY (merchant_id, customer_id) REFERENCES shop_customers (merchant_id, id);
ALTER TABLE suppliers ADD CONSTRAINT uq_suppliers_merchant_id UNIQUE (merchant_id, id);
ALTER TABLE purchase_orders ADD CONSTRAINT fk_purchase_orders_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);
ALTER TABLE purchase_orders ADD CONSTRAINT fk_purchase_orders_supplier_same_merchant
    FOREIGN KEY (merchant_id, supplier_id) REFERENCES suppliers (merchant_id, id);
ALTER TABLE accounts ADD CONSTRAINT fk_accounts_shop_same_merchant
    FOREIGN KEY (merchant_id, shop_id) REFERENCES shops (merchant_id, id);

-- ================================================================
-- Indexes
-- ================================================================

CREATE UNIQUE INDEX idx_shops_one_primary_per_merchant
    ON shops (merchant_id) WHERE is_primary = TRUE;
CREATE INDEX idx_users_merchant ON users (merchant_id);
CREATE INDEX idx_users_locked_until ON users (locked_until);
CREATE INDEX idx_sync_logs_merchant_created ON sync_logs (merchant_id, created_at DESC);
CREATE INDEX idx_user_roles_role ON user_roles (role_id);
CREATE INDEX idx_refresh_tokens_user_expiry ON refresh_tokens (user_id, expires_at);
CREATE INDEX idx_audit_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_shops_merchant_active ON shops (merchant_id, is_active);
CREATE INDEX idx_categories_tree ON categories (merchant_id, parent_id);
CREATE INDEX idx_categories_path ON categories (merchant_id, path);
CREATE INDEX idx_product_categories_category ON product_categories (category_id);
CREATE INDEX idx_product_categories_merchant ON product_categories (merchant_id, category_id);
CREATE INDEX idx_promotion_products_product ON promotion_products (merchant_id, product_id);
CREATE INDEX idx_products_merchant_active ON products (merchant_id, is_active);
CREATE INDEX idx_product_variants_product ON product_variants (product_id);
CREATE INDEX idx_product_variants_merchant ON product_variants (merchant_id, product_id);
CREATE INDEX idx_product_variants_barcode ON product_variants (barcode);
CREATE INDEX idx_product_images_product ON product_images (product_id, position);
CREATE INDEX idx_product_prices_lookup ON product_prices (merchant_id, shop_id, product_id, variant_id, price_type);
CREATE INDEX idx_attribute_definitions_merchant ON attribute_definitions (merchant_id, code);
CREATE INDEX idx_attribute_options_definition ON attribute_definition_options (definition_id, position);
CREATE INDEX idx_product_attribute_assignments ON product_attribute_assignments (product_id, variant_id);
CREATE INDEX idx_stock_items_product ON stock_items (merchant_id, product_id);
CREATE INDEX idx_stock_items_merchant_name ON stock_items (merchant_id, name);
CREATE INDEX idx_inventory_items_shop ON inventory_items (merchant_id, shop_id, is_active);
CREATE INDEX idx_batches_lookup ON inventory_batches (shop_id, stock_item_id, expiry_date);
CREATE INDEX idx_inventory_movements_report ON inventory_movements (merchant_id, shop_id, movement_date);
CREATE INDEX idx_inventory_reservations_active ON inventory_reservations (shop_id, status);
CREATE INDEX idx_barcode_lookup ON barcode_registry (merchant_id, normalized_code);
CREATE INDEX idx_inventory_reconciliation ON inventory_reconciliation_exceptions (merchant_id, shop_id, status);
CREATE INDEX idx_inventory_assets_shop_status ON inventory_assets (shop_id, status);
CREATE INDEX idx_inventory_asset_identifiers_lookup ON inventory_asset_identifiers (identifier_type_id, normalized_value);
CREATE INDEX idx_inventory_transformations_shop_date ON inventory_transformations (shop_id, created_at);
CREATE INDEX idx_customer_tags_merchant ON customer_tags (merchant_id, name);
CREATE INDEX idx_customer_notes_customer ON customer_notes (customer_id, created_at);
CREATE INDEX idx_customer_activities_customer ON customer_activities (customer_id, created_at);
CREATE INDEX idx_sales_report ON sales (merchant_id, shop_id, sale_date);
CREATE INDEX idx_sales_client_merchant ON sales (merchant_id, client_sale_id);
CREATE INDEX idx_sale_items_sale ON sale_items (sale_id);
CREATE INDEX idx_pos_terminals_shop ON pos_terminals (shop_id, is_active);
CREATE INDEX idx_pos_sessions_shop_status ON pos_sessions (shop_id, status);
CREATE UNIQUE INDEX idx_pos_sessions_one_open_per_terminal
    ON pos_sessions (terminal_id) WHERE status = 'OPEN' AND terminal_id IS NOT NULL;
CREATE INDEX idx_pos_transactions_session ON pos_transactions (session_id);
CREATE INDEX idx_payments_sale ON payments (sale_id, status);
CREATE INDEX idx_payment_proofs_payment_status ON payment_proofs (payment_id, status);
CREATE INDEX idx_payment_sessions_payment_status ON payment_provider_sessions (payment_id, status);
CREATE INDEX idx_purchase_orders_shop_status ON purchase_orders (shop_id, status);
CREATE INDEX idx_purchase_orders_merchant_created ON purchase_orders (merchant_id, created_at DESC);
CREATE INDEX idx_goods_receipts_purchase_order ON goods_receipts (purchase_order_id);
CREATE INDEX idx_journal_entries_shop_date ON journal_entries (merchant_id, shop_id, created_at);
CREATE INDEX idx_journal_lines_account ON journal_lines (account_id);
CREATE INDEX idx_notifications_recipient ON notifications (recipient_user_id, is_read, created_at);
CREATE INDEX idx_suppliers_merchant_created ON suppliers (merchant_id, created_at DESC);
CREATE INDEX idx_promotions_merchant_active ON promotions (merchant_id, is_active, start_date, end_date);
CREATE INDEX idx_invoices_shop_date ON invoices (shop_id, invoice_date DESC);
CREATE INDEX idx_support_tickets_shop_status ON support_tickets (shop_id, status, created_at DESC);
CREATE INDEX idx_support_messages_ticket_created ON support_messages (ticket_id, created_at ASC);
CREATE INDEX idx_system_settings_updated ON system_settings (updated_at);
CREATE INDEX idx_ai_sessions_merchant ON ai_sessions (merchant_id, updated_at);
CREATE INDEX idx_ai_requests_merchant_status ON ai_requests (merchant_id, status, created_at);

CREATE OR REPLACE VIEW shop_settings_view AS
SELECT s.id AS shop_id,
       s.merchant_id,
       s.name AS shop_name,
       s.business_type,
       COALESCE(ps.tax, 0) AS tax,
       COALESCE(ps.service_charge, 0) AS service_charge,
       COALESCE(ps.delivery_charge, 0) AS delivery_charge,
       s.settings AS shop_settings,
       s.opening_hours
FROM shops s
LEFT JOIN payment_settings ps ON ps.shop_id = s.id;
