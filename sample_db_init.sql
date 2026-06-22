-- ============================================================
-- Bytebase 样例数据库：模拟电商系统
-- 用于测试 SQL 审查、Schema 变更、数据管理等能力
-- ============================================================

-- ==================== SCHEMA: inventory (库存) ====================
CREATE SCHEMA IF NOT EXISTS inventory;

-- 商品分类
CREATE TABLE inventory.categories (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    parent_id   INT REFERENCES inventory.categories(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 商品
CREATE TABLE inventory.products (
    id             SERIAL PRIMARY KEY,
    name           VARCHAR(200) NOT NULL,
    description    TEXT,
    price          DECIMAL(10, 2) NOT NULL CHECK (price >= 0),
    category_id    INT NOT NULL REFERENCES inventory.categories(id),
    stock_quantity INT NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    sku            VARCHAR(50) NOT NULL UNIQUE,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    tags           TEXT[],
    metadata       JSONB DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_category ON inventory.products(category_id);
CREATE INDEX idx_products_name ON inventory.products(name);
CREATE INDEX idx_products_tags ON inventory.products USING GIN(tags);

-- 仓库
CREATE TABLE inventory.warehouses (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL UNIQUE,
    location   VARCHAR(200),
    capacity   INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 商品-仓库（多对多）
CREATE TABLE inventory.product_warehouses (
    product_id   INT NOT NULL REFERENCES inventory.products(id) ON DELETE CASCADE,
    warehouse_id INT NOT NULL REFERENCES inventory.warehouses(id) ON DELETE CASCADE,
    quantity     INT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    PRIMARY KEY (product_id, warehouse_id)
);

-- ==================== SCHEMA: users_schema (用户) ====================
CREATE SCHEMA IF NOT EXISTS users_schema;

-- 用户角色枚举
CREATE TYPE users_schema.user_role AS ENUM ('admin', 'manager', 'customer', 'viewer');
CREATE TYPE users_schema.user_status AS ENUM ('active', 'inactive', 'suspended');

-- 用户
CREATE TABLE users_schema.users (
    id            SERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    name          VARCHAR(100) NOT NULL,
    phone         VARCHAR(30),
    role          users_schema.user_role NOT NULL DEFAULT 'customer',
    status        users_schema.user_status NOT NULL DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users_schema.users(email);
CREATE INDEX idx_users_role ON users_schema.users(role);

-- 用户地址
CREATE TABLE users_schema.user_addresses (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users_schema.users(id) ON DELETE CASCADE,
    address_line VARCHAR(500) NOT NULL,
    city        VARCHAR(100) NOT NULL,
    state       VARCHAR(100),
    country     VARCHAR(100) NOT NULL DEFAULT 'China',
    postal_code VARCHAR(20),
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_addresses_user ON users_schema.user_addresses(user_id);

-- 用户画像（1:1）
CREATE TABLE users_schema.user_profiles (
    user_id      INT PRIMARY KEY REFERENCES users_schema.users(id) ON DELETE CASCADE,
    bio          TEXT,
    avatar_url   VARCHAR(500),
    birth_date   DATE,
    preferences  JSONB DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ==================== SCHEMA: orders_schema (订单) ====================
CREATE SCHEMA IF NOT EXISTS orders_schema;

CREATE TYPE orders_schema.order_status AS ENUM (
    'pending', 'confirmed', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded'
);
CREATE TYPE orders_schema.payment_method AS ENUM (
    'credit_card', 'debit_card', 'alipay', 'wechat_pay', 'bank_transfer'
);
CREATE TYPE orders_schema.payment_status AS ENUM (
    'pending', 'completed', 'failed', 'refunded'
);

-- 订单
CREATE TABLE orders_schema.orders (
    id              SERIAL PRIMARY KEY,
    order_number    VARCHAR(50) NOT NULL UNIQUE,
    user_id         INT NOT NULL REFERENCES users_schema.users(id),
    status          orders_schema.order_status NOT NULL DEFAULT 'pending',
    total_amount    DECIMAL(12, 2) NOT NULL CHECK (total_amount >= 0),
    discount_amount DECIMAL(12, 2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    shipping_address TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user ON orders_schema.orders(user_id);
CREATE INDEX idx_orders_status ON orders_schema.orders(status);
CREATE INDEX idx_orders_created ON orders_schema.orders(created_at);

-- 订单项
CREATE TABLE orders_schema.order_items (
    id         SERIAL PRIMARY KEY,
    order_id   INT NOT NULL REFERENCES orders_schema.orders(id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES inventory.products(id),
    quantity   INT NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(10, 2) NOT NULL CHECK (unit_price >= 0)
);

CREATE INDEX idx_order_items_order ON orders_schema.order_items(order_id);

-- 支付记录
CREATE TABLE orders_schema.payments (
    id             SERIAL PRIMARY KEY,
    order_id       INT NOT NULL REFERENCES orders_schema.orders(id) ON DELETE CASCADE,
    amount         DECIMAL(12, 2) NOT NULL CHECK (amount >= 0),
    method         orders_schema.payment_method NOT NULL,
    status         orders_schema.payment_status NOT NULL DEFAULT 'pending',
    transaction_id VARCHAR(100),
    paid_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order ON orders_schema.payments(order_id);

-- ==================== SCHEMA: public (混合) ====================
-- 审计日志表
CREATE TABLE public.audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    table_name  VARCHAR(100) NOT NULL,
    record_id   INT NOT NULL,
    action      VARCHAR(20) NOT NULL,
    old_data    JSONB,
    new_data    JSONB,
    changed_by  INT REFERENCES users_schema.users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_table ON public.audit_logs(table_name, record_id);
CREATE INDEX idx_audit_created ON public.audit_logs(created_at);

-- 系统配置表
CREATE TABLE public.system_configs (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  INT REFERENCES users_schema.users(id)
);

-- ==================== 视图 ====================

-- 库存概览视图
CREATE VIEW inventory.v_low_stock_products AS
SELECT 
    p.id,
    p.name AS product_name,
    c.name AS category_name,
    p.stock_quantity,
    p.price,
    p.is_active
FROM inventory.products p
JOIN inventory.categories c ON p.category_id = c.id
WHERE p.stock_quantity < 50 AND p.is_active = TRUE;

-- 订单汇总视图
CREATE VIEW orders_schema.v_order_summary AS
SELECT 
    o.id,
    o.order_number,
    u.name AS customer_name,
    o.status,
    o.total_amount,
    COUNT(oi.id) AS item_count,
    o.created_at
FROM orders_schema.orders o
JOIN users_schema.users u ON o.user_id = u.id
LEFT JOIN orders_schema.order_items oi ON o.id = oi.order_id
GROUP BY o.id, o.order_number, u.name, o.status, o.total_amount, o.created_at;

-- ==================== 函数 ====================

-- 计算订单实际支付金额
CREATE OR REPLACE FUNCTION orders_schema.get_paid_amount(order_id INT)
RETURNS DECIMAL(12,2) AS $$
DECLARE
    total DECIMAL(12,2);
BEGIN
    SELECT COALESCE(SUM(amount), 0) INTO total
    FROM orders_schema.payments
    WHERE orders_schema.payments.order_id = $1 AND status = 'completed';
    RETURN total;
END;
$$ LANGUAGE plpgsql;

-- 自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ==================== 触发器 ====================

-- products 更新时自动更新 updated_at
CREATE TRIGGER trg_products_updated_at
    BEFORE UPDATE ON inventory.products
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- orders 更新时自动更新 updated_at
CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders_schema.orders
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- users 更新时自动更新 updated_at
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users_schema.users
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
