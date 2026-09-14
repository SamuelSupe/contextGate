-- Run as the fixture owner in an isolated PostgreSQL database.
-- An existing schema is an error; this script never replaces existing data.
BEGIN;
CREATE SCHEMA retail_example;

CREATE TABLE retail_example.customers (
    id integer PRIMARY KEY,
    display_name text NOT NULL,
    segment text NOT NULL CHECK (segment IN ('consumer', 'business')),
    region text NOT NULL CHECK (region IN ('APAC', 'EMEA', 'AMER')),
    joined_at timestamptz NOT NULL
);
CREATE TABLE retail_example.products (
    id integer PRIMARY KEY,
    sku text NOT NULL UNIQUE,
    name text NOT NULL,
    category text NOT NULL CHECK (category IN ('stationery', 'lighting', 'drinkware')),
    active boolean NOT NULL
);
CREATE TABLE retail_example.orders (
    id integer PRIMARY KEY,
    customer_id integer NOT NULL REFERENCES retail_example.customers(id),
    placed_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'paid', 'cancelled')),
    currency text NOT NULL CHECK (currency = 'USD'),
    gross_amount numeric(12,2) NOT NULL CHECK (gross_amount >= 0)
);
CREATE TABLE retail_example.order_lines (
    order_id integer NOT NULL REFERENCES retail_example.orders(id),
    line_number integer NOT NULL CHECK (line_number > 0),
    product_id integer NOT NULL REFERENCES retail_example.products(id),
    quantity integer NOT NULL CHECK (quantity > 0),
    unit_price numeric(12,2) NOT NULL CHECK (unit_price >= 0),
    line_amount numeric(12,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    PRIMARY KEY (order_id, line_number)
);

INSERT INTO retail_example.customers VALUES
    (1, 'Acme Retail (fictional)', 'business', 'APAC', '2026-08-01T00:00:00Z'),
    (2, 'River Studio (fictional)', 'business', 'EMEA', '2026-08-10T00:00:00Z'),
    (3, 'Lena Chen (fictional)', 'consumer', 'APAC', '2026-08-20T00:00:00Z');
INSERT INTO retail_example.products VALUES
    (101, 'NOTEBOOK-A5', 'A5 notebook', 'stationery', true),
    (102, 'LAMP-DESK', 'Desk lamp', 'lighting', true),
    (103, 'MUG-COFFEE', 'Coffee mug', 'drinkware', false);
INSERT INTO retail_example.orders VALUES
    (1001, 1, '2026-09-01T09:00:00Z', 'paid', 'USD', 70.00),
    (1002, 1, '2026-09-02T10:00:00Z', 'pending', 'USD', 18.00),
    (1003, 2, '2026-09-03T11:00:00Z', 'paid', 'USD', 90.00);
INSERT INTO retail_example.order_lines(order_id, line_number, product_id, quantity, unit_price) VALUES
    (1001, 1, 101, 2, 12.50),
    (1001, 2, 102, 1, 45.00),
    (1002, 1, 103, 1, 18.00),
    (1003, 1, 102, 2, 45.00);
COMMIT;
