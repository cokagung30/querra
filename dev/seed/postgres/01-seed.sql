-- Seed data for Querra integration tests and manual testing (PostgreSQL).
-- Loaded into a fresh querra_test database by dev/testdb.sh (`wails3 task db:reset`).

CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');

-- One column per common type, to test type mapping and cell encoding.
CREATE TABLE all_types (
    id            bigserial PRIMARY KEY,
    c_smallint    smallint,
    c_integer     integer,
    c_bigint      bigint,
    c_numeric     numeric(30, 10),
    c_real        real,
    c_double      double precision,
    c_money       money,
    c_boolean     boolean,
    c_char        char(10),
    c_varchar     varchar(100),
    c_text        text,
    c_bytea       bytea,
    c_date        date,
    c_time        time,
    c_timetz      timetz,
    c_timestamp   timestamp,
    c_timestamptz timestamptz,
    c_interval    interval,
    c_uuid        uuid,
    c_json        json,
    c_jsonb       jsonb,
    c_inet        inet,
    c_cidr        cidr,
    c_macaddr     macaddr,
    c_int_array   integer[],
    c_text_array  text[],
    c_mood        mood,
    c_point       point,
    c_xml         xml,
    c_tsvector    tsvector
);

INSERT INTO all_types (
    c_smallint, c_integer, c_bigint, c_numeric, c_real, c_double, c_money, c_boolean,
    c_char, c_varchar, c_text, c_bytea, c_date, c_time, c_timetz, c_timestamp, c_timestamptz,
    c_interval, c_uuid, c_json, c_jsonb, c_inet, c_cidr, c_macaddr, c_int_array, c_text_array,
    c_mood, c_point, c_xml, c_tsvector
) VALUES
    -- Upper extremes, including values a JavaScript number can't hold exactly.
    (32767, 2147483647, 9223372036854775807, 12345678901234567890.0123456789, 3.4e38,
     1.7976931348623157e308, 1234567.89, true,
     'char', 'varchar', E'multi-line\ntext with ''quotes'' and unicode: ünïcödé 日本語 🚀',
     '\xdeadbeef', '2026-11-07', '23:59:59.999999', '12:00:00+07',
     '2026-11-07 10:00:00.123456', '2026-11-07 10:00:00.123456+07',
     '1 year 2 months 3 days 04:05:06', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
     '{"a": 1, "big": 12345678901234567890}', '{"nested": {"list": [1, 2, 3]}}',
     '192.168.1.1', '10.0.0.0/8', '08:00:2b:01:02:03', '{1,2,3}', '{"a","b c",NULL}',
     'happy', '(1.5,2.5)', '<root><child>x</child></root>', 'the quick brown fox'),
    -- Lower extremes and empty values (distinct from NULL).
    (-32768, -2147483648, -9223372036854775808, -0.0000000001, -1.5, -2.5, -1.00, false,
     '', '', '', '\x', '0001-01-01', '00:00:00', '00:00:00+00',
     '1970-01-01 00:00:00', '1970-01-01 00:00:00+00',
     '-1 day', '00000000-0000-0000-0000-000000000000', '[]', '{}',
     '::1', '::/0', '00:00:00:00:00:00', '{}', '{}',
     'sad', '(0,0)', '<empty/>', '');

-- Every column NULL.
INSERT INTO all_types DEFAULT VALUES;

-- No primary key, with duplicate rows (FR-10).
CREATE TABLE no_pk (
    name  text,
    value integer
);

INSERT INTO no_pk VALUES ('duplicate', 1), ('duplicate', 1), ('unique', 2);

-- 1,000,000 rows for pagination, sorting, filtering and scroll performance.
CREATE TABLE big_table (
    id         bigint PRIMARY KEY,
    name       text NOT NULL,
    category   text NOT NULL,
    amount     numeric(12, 2) NOT NULL,
    is_active  boolean NOT NULL,
    created_at timestamptz NOT NULL
);

INSERT INTO big_table (id, name, category, amount, is_active, created_at)
SELECT g,
       'item ' || g,
       (ARRAY['alpha', 'beta', 'gamma', 'delta'])[1 + g % 4],
       (g % 100000) / 100.0,
       g % 3 <> 0,
       timestamptz '2020-01-01 00:00:00+00' + g * interval '1 minute'
FROM generate_series(1, 1000000) AS g;

CREATE INDEX big_table_category_idx ON big_table (category);
ANALYZE big_table;

-- Identifiers that need quoting.
CREATE TABLE "Weird Table" (
    "ID"          serial PRIMARY KEY,
    "Mixed Case"  text,
    "select"      text,
    "spasi kolom" text,
    "ünïcödé"     text,
    "dot.name"    text
);

INSERT INTO "Weird Table" ("Mixed Case", "select", "spasi kolom", "ünïcödé", "dot.name")
VALUES ('a', 'b', 'c', 'd', 'e');

-- Second schema: foreign-key chain, composite key, view, materialized view, function.
CREATE SCHEMA sales;

CREATE TABLE sales.customers (
    id    serial PRIMARY KEY,
    name  text NOT NULL,
    email text UNIQUE
);

CREATE TABLE sales.products (
    id    serial PRIMARY KEY,
    sku   text NOT NULL UNIQUE,
    name  text NOT NULL,
    price numeric(12, 2) NOT NULL CHECK (price >= 0)
);

CREATE TABLE sales.orders (
    id          serial PRIMARY KEY,
    customer_id integer NOT NULL REFERENCES sales.customers (id),
    ordered_at  timestamptz NOT NULL DEFAULT now(),
    status      text NOT NULL DEFAULT 'new'
);

CREATE TABLE sales.order_items (
    order_id   integer NOT NULL REFERENCES sales.orders (id) ON DELETE CASCADE,
    product_id integer NOT NULL REFERENCES sales.products (id),
    quantity   integer NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, product_id)
);

INSERT INTO sales.customers (name, email) VALUES
    ('Ayu', 'ayu@example.com'),
    ('Budi', 'budi@example.com'),
    ('Citra', NULL);

INSERT INTO sales.products (sku, name, price) VALUES
    ('SKU-001', 'Keyboard', 450000.00),
    ('SKU-002', 'Mouse', 150000.00),
    ('SKU-003', 'Monitor', 2500000.00);

INSERT INTO sales.orders (customer_id, ordered_at, status) VALUES
    (1, '2026-11-01 09:00:00+07', 'paid'),
    (1, '2026-11-02 14:30:00+07', 'new'),
    (2, '2026-11-03 08:15:00+07', 'shipped');

INSERT INTO sales.order_items (order_id, product_id, quantity) VALUES
    (1, 1, 1),
    (1, 2, 2),
    (2, 3, 1),
    (3, 2, 1);

CREATE VIEW sales.order_totals AS
SELECT o.id AS order_id, c.name AS customer, sum(oi.quantity * p.price) AS total
FROM sales.orders o
JOIN sales.customers c ON c.id = o.customer_id
JOIN sales.order_items oi ON oi.order_id = o.id
JOIN sales.products p ON p.id = oi.product_id
GROUP BY o.id, c.name;

CREATE MATERIALIZED VIEW sales.customer_order_counts AS
SELECT c.id AS customer_id, c.name, count(o.id) AS order_count
FROM sales.customers c
LEFT JOIN sales.orders o ON o.customer_id = c.id
GROUP BY c.id, c.name;

CREATE FUNCTION sales.order_count(p_customer_id integer) RETURNS bigint
LANGUAGE sql STABLE
AS $$
    SELECT count(*) FROM sales.orders WHERE customer_id = p_customer_id;
$$;
