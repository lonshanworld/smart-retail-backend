-- Unified User and Shop Schema for Smart Retail Backend
-- This schema uses a single 'users' table with UUIDs and roles.

-- Extension for UUID functions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Unified table for all user types: Admins, Merchants, and Staff
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'merchant', 'staff')),
    is_active BOOLEAN DEFAULT TRUE,
    merchant_id UUID REFERENCES users(id) ON DELETE SET NULL, -- For staff, points to their merchant
    assigned_shop_id UUID, -- Forward reference, see ALTER TABLE below
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Shops owned by merchants
CREATE TABLE IF NOT EXISTS shops (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address TEXT,
    phone VARCHAR(50),
    tax_rate NUMERIC(5,2) NOT NULL DEFAULT 5.00 CHECK (tax_rate >= 0 AND tax_rate <= 100),
    is_active BOOLEAN DEFAULT TRUE,
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create partial unique index for primary shop constraint
CREATE UNIQUE INDEX IF NOT EXISTS idx_shops_merchant_primary 
ON shops (merchant_id) 
WHERE is_primary = TRUE;

-- Now that shops table exists, add the foreign key constraint to users
ALTER TABLE users ADD CONSTRAINT fk_assigned_shop_id FOREIGN KEY (assigned_shop_id) REFERENCES shops(id) ON DELETE SET NULL;

-- Staff contracts table
CREATE TABLE IF NOT EXISTS staff_contracts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    salary NUMERIC(10, 2) NOT NULL,
    pay_frequency VARCHAR(50) NOT NULL, -- 'monthly', 'bi-weekly', etc.
    start_date DATE NOT NULL,
    end_date DATE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Suppliers created by merchants
CREATE TABLE IF NOT EXISTS suppliers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    contact_name VARCHAR(255),
    contact_email VARCHAR(255),
    contact_phone VARCHAR(50),
    address TEXT,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Categories and Brands
CREATE TABLE IF NOT EXISTS brands (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    image_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(merchant_id, name)
);

-- Two-level category model: categories (level 1) and subcategories (level 2)
CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(merchant_id, name)
);

CREATE TABLE IF NOT EXISTS subcategories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(category_id, name)
);


-- Master inventory of items for a merchant
CREATE TABLE IF NOT EXISTS inventory_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    sku VARCHAR(100),
    selling_price NUMERIC(10, 2) NOT NULL,
    original_price NUMERIC(10, 2),
    low_stock_threshold INTEGER,
    category VARCHAR(100),
    -- New normalized category/brand relationships (nullable for migration)
    category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    subcategory_id UUID REFERENCES subcategories(id) ON DELETE SET NULL,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    supplier_id UUID REFERENCES suppliers(id) ON DELETE SET NULL,
    is_archived BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(merchant_id, sku)
);

-- Stock levels for items in specific shops
CREATE TABLE IF NOT EXISTS shop_stock (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL,
    last_stocked_in_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(shop_id, inventory_item_id)
);

-- Log of all stock changes
CREATE TABLE IF NOT EXISTS stock_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    movement_type VARCHAR(50) NOT NULL, -- 'stock_in', 'sale', 'return', 'adjustment'
    quantity_changed INTEGER NOT NULL, -- Positive for in, negative for out
    new_quantity INTEGER NOT NULL,
    reason VARCHAR(255), -- For adjustments like 'damage', 'theft'
    movement_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

-- Client-generated inventory operation IDs used for idempotent stock mutations
CREATE TABLE IF NOT EXISTS inventory_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_operation_id TEXT UNIQUE NOT NULL,
    operation_type VARCHAR(80) NOT NULL,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Customers associated with a specific shop
CREATE TABLE IF NOT EXISTS shop_customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(shop_id, email)
);

-- Promotions created by merchants
CREATE TABLE IF NOT EXISTS promotions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    promo_type VARCHAR(50) NOT NULL, -- 'percentage', 'fixed_amount', 'bogo'
    promo_value NUMERIC(10, 2) NOT NULL,
    min_spend NUMERIC(10, 2) DEFAULT 0.00,
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Link table for many-to-many relationship between promotions and products
CREATE TABLE IF NOT EXISTS promotion_products (
    promotion_id UUID NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    PRIMARY KEY (promotion_id, inventory_item_id)
);

-- Sales transactions
CREATE TABLE IF NOT EXISTS sales (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_sale_id TEXT UNIQUE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    staff_id UUID REFERENCES users(id) ON DELETE SET NULL,
    customer_id UUID REFERENCES shop_customers(id) ON DELETE SET NULL,
    sale_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    total_amount NUMERIC(10, 2) NOT NULL,
    delivery_charge NUMERIC(10, 2) DEFAULT 0.00,
    applied_promotion_id UUID REFERENCES promotions(id) ON DELETE SET NULL,
    discount_amount NUMERIC(10, 2) DEFAULT 0.00,
    payment_type VARCHAR(50) NOT NULL,
    payment_status VARCHAR(50) NOT NULL DEFAULT 'succeeded',
    stripe_payment_intent_id VARCHAR(255) UNIQUE,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Individual items included in a sale
CREATE TABLE IF NOT EXISTS sale_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,
	item_name VARCHAR(255) NOT NULL, -- Denormalized for historical accuracy
    item_sku VARCHAR(100),         -- Denormalized for historical accuracy
    quantity_sold INTEGER NOT NULL,
    selling_price_at_sale NUMERIC(10, 2) NOT NULL,
    original_price_at_sale NUMERIC(10, 2),
    subtotal NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Invoices generated for sales
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sale_id UUID NOT NULL UNIQUE REFERENCES sales(id) ON DELETE CASCADE,
    invoice_number VARCHAR(50) NOT NULL UNIQUE,
    merchant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    customer_id UUID REFERENCES shop_customers(id) ON DELETE SET NULL,
    invoice_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    due_date TIMESTAMP WITH TIME ZONE,
    subtotal NUMERIC(10, 2) NOT NULL,
    discount_amount NUMERIC(10, 2) DEFAULT 0.00,
    tax_amount NUMERIC(10, 2) DEFAULT 0.00,
    delivery_charge NUMERIC(10, 2) DEFAULT 0.00,
    total_amount NUMERIC(10, 2) NOT NULL,
    payment_status VARCHAR(50) NOT NULL DEFAULT 'paid', -- 'paid', 'pending', 'overdue', 'cancelled'
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Salary payments made to staff
CREATE TABLE IF NOT EXISTS salary_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_date TIMESTAMP WITH TIME ZONE NOT NULL,
    amount_paid NUMERIC(10, 2) NOT NULL,
    payment_period_start DATE NOT NULL,
    payment_period_end DATE NOT NULL,
    payment_method VARCHAR(50),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- In-app notifications for users
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    notification_type VARCHAR(50), -- e.g., 'low_stock', 'new_sale', 'system_update'
    related_entity_type VARCHAR(50), -- e.g., 'inventory_item', 'sale'
    related_entity_id UUID,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- Migration helper: populate categories/subcategories
-- This block is idempotent and attempts to migrate existing
-- `inventory_items.category` textual values into normalized tables.
-- It treats a '>' in the text as a separator: "Category > Subcategory".
-- Review results before dropping the original `category` column.
-- =====================================================

BEGIN;

-- Create top-level categories from distinct inventory_items.category values
INSERT INTO categories (merchant_id, name)
SELECT DISTINCT merchant_id, trim(split_part(category, '>', 1))
FROM inventory_items
WHERE category IS NOT NULL AND trim(category) <> ''
ON CONFLICT (merchant_id, name) DO NOTHING;

-- Create subcategories when a second part exists
INSERT INTO subcategories (category_id, name)
SELECT c.id, trim(split_part(i.category, '>', 2))
FROM inventory_items i
JOIN categories c ON c.merchant_id = i.merchant_id AND c.name = trim(split_part(i.category, '>', 1))
WHERE split_part(i.category, '>', 2) IS NOT NULL AND trim(split_part(i.category, '>', 2)) <> ''
ON CONFLICT (category_id, name) DO NOTHING;

-- Link inventory_items to created categories and subcategories
UPDATE inventory_items i
SET category_id = c.id
FROM categories c
WHERE c.merchant_id = i.merchant_id AND c.name = trim(split_part(i.category, '>', 1));

UPDATE inventory_items i
SET subcategory_id = s.id
FROM subcategories s
JOIN categories c ON s.category_id = c.id
WHERE c.merchant_id = i.merchant_id
    AND c.name = trim(split_part(i.category, '>', 1))
    AND s.name = trim(split_part(i.category, '>', 2));

COMMIT;

-- =====================================================
-- Additive shop-generic schema extensions for multi-business support
-- These are additive and shop-scoped to support many business types
-- =====================================================

-- Add flexible business type and JSON settings to shops
ALTER TABLE shops
ADD COLUMN IF NOT EXISTS business_type VARCHAR(100) DEFAULT 'retail',
ADD COLUMN IF NOT EXISTS settings JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS opening_hours JSONB DEFAULT NULL,
ADD COLUMN IF NOT EXISTS supports_delivery BOOLEAN DEFAULT FALSE;

-- Shop-level payment settings (site can also use merchant-level settings)
CREATE TABLE IF NOT EXISTS payment_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    qr_image_url TEXT DEFAULT '',
    tax NUMERIC(5,2) DEFAULT 0,
    service_charge NUMERIC(10,2) DEFAULT 0,
    delivery_charge NUMERIC(10,2) DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (shop_id)
);

-- Generic testimonials for a shop or merchant (suitable for any business type)
CREATE TABLE IF NOT EXISTS testimonials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(255),
    content TEXT NOT NULL,
    rating SMALLINT DEFAULT 5 CHECK (rating >= 1 AND rating <= 5),
    avatar TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Support tickets and messages (shop-scoped, works for retail, pharmacy, services, etc.)
CREATE TABLE IF NOT EXISTS support_tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES users(id) ON DELETE CASCADE,
    shop_id UUID REFERENCES shops(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'OPEN',
    priority VARCHAR(50) DEFAULT 'MEDIUM',
    customer_name VARCHAR(255),
    customer_email VARCHAR(255),
    customer_phone VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS support_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_role VARCHAR(50) DEFAULT 'CUSTOMER', -- CUSTOMER, ADMIN, STAFF
    content TEXT NOT NULL,
    is_admin_reply BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- External ID mapping table to correlate external systems (e.g., pharmacy Prisma) to Smart Retail UUIDs
CREATE TABLE IF NOT EXISTS external_id_map (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(100) NOT NULL,         -- e.g. 'pharmacy-prisma'
    source_id TEXT NOT NULL,              -- e.g. cuid() value from external system
    target_table VARCHAR(100) NOT NULL,   -- e.g. 'inventory_items'
    target_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source, source_id, target_table)
);

-- Migration audit table for storing raw values during ETL (optional, helpful for debugging)
CREATE TABLE IF NOT EXISTS migration_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(100) NOT NULL,
    source_id TEXT,
    target_table VARCHAR(100),
    payload JSONB,
    imported_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Convenience view: shop_settings flattened for quick reads
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

-- End of additive schema extensions

ALTER TABLE brands ADD COLUMN IF NOT EXISTS image_url TEXT;