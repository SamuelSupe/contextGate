\set ON_ERROR_STOP on
BEGIN;
CREATE SCHEMA support_demo;
CREATE TABLE support_demo.orders (
  id bigint PRIMARY KEY,
  customer_id bigint NOT NULL,
  status text NOT NULL,
  amount numeric(20,2) NOT NULL,
  currency text NOT NULL,
  placed_at timestamptz NOT NULL
);
CREATE TABLE support_demo.refunds (
  id bigint PRIMARY KEY,
  order_id bigint NOT NULL REFERENCES support_demo.orders,
  status text NOT NULL,
  amount numeric(20,2) NOT NULL,
  currency text NOT NULL
);
INSERT INTO support_demo.orders VALUES
  (101, 1, 'paid', 120.00, 'USD', '2026-09-01T10:00:00Z'),
  (102, 1, 'cancelled', 90.00, 'USD', '2026-09-02T10:00:00Z'),
  (103, 2, 'paid', 50.00, 'USD', '2026-09-03T10:00:00Z');
INSERT INTO support_demo.refunds VALUES (201, 101, 'settled', 10.00, 'USD');
COMMIT;
