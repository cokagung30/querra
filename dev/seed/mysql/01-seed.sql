-- Seed data for Querra integration tests and manual testing (MySQL 8.4 and MariaDB 11).
-- Runs once, as root, against the `querra` database when the data volume is first created.

SET NAMES utf8mb4;

-- One column per common type, to test type mapping and cell encoding.
CREATE TABLE all_types (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    c_tinyint   TINYINT,
    c_smallint  SMALLINT,
    c_mediumint MEDIUMINT,
    c_int       INT,
    c_bigint    BIGINT,
    c_bigint_u  BIGINT UNSIGNED,
    c_decimal   DECIMAL(30, 10),
    c_float     FLOAT,
    c_double    DOUBLE,
    c_bit       BIT(8),
    c_bool      BOOLEAN,
    c_char      CHAR(10),
    c_varchar   VARCHAR(100),
    c_text      TEXT,
    c_binary    BINARY(4),
    c_varbinary VARBINARY(16),
    c_blob      BLOB,
    c_date      DATE,
    c_time      TIME(6),
    c_datetime  DATETIME(6),
    c_timestamp TIMESTAMP(6) NULL DEFAULT NULL,
    c_year      YEAR,
    c_enum      ENUM('sad', 'ok', 'happy'),
    c_set       SET('read', 'write', 'admin'),
    c_json      JSON
);

INSERT INTO all_types (
    c_tinyint, c_smallint, c_mediumint, c_int, c_bigint, c_bigint_u, c_decimal, c_float, c_double,
    c_bit, c_bool, c_char, c_varchar, c_text, c_binary, c_varbinary, c_blob, c_date, c_time,
    c_datetime, c_timestamp, c_year, c_enum, c_set, c_json
) VALUES
    -- Upper extremes, including values a JavaScript number can't hold exactly.
    (127, 32767, 8388607, 2147483647, 9223372036854775807, 18446744073709551615,
     12345678901234567890.0123456789, 3.4e38, 1.7976931348623157e308,
     b'10101010', TRUE, 'char', 'varchar',
     CONCAT('multi-line', CHAR(10), 'text with ''quotes'' and unicode: ünïcödé 日本語 🚀'),
     X'DEADBEEF', X'00FF', X'CAFEBABE', '2026-11-07', '23:59:59.999999',
     '2026-11-07 10:00:00.123456', '2026-11-07 10:00:00.123456', 2026, 'happy', 'read,write',
     '{"a": 1, "big": 12345678901234567890}'),
    -- Lower extremes and empty values (distinct from NULL).
    (-128, -32768, -8388608, -2147483648, -9223372036854775808, 0,
     -0.0000000001, -1.5, -2.5,
     b'0', FALSE, '', '', '',
     X'00000000', X'', X'', '1000-01-01', '-838:59:59',
     '1000-01-01 00:00:00', '1970-01-01 00:00:01', 1901, 'sad', '',
     '[]');

-- Every column NULL.
INSERT INTO all_types () VALUES ();

-- No primary key, with duplicate rows (FR-10).
CREATE TABLE no_pk (
    name  VARCHAR(50),
    value INT
);

INSERT INTO no_pk VALUES ('duplicate', 1), ('duplicate', 1), ('unique', 2);

-- 1,000,000 rows for pagination, sorting, filtering and scroll performance.
-- Generated from a cross join of digits so it works on MySQL and MariaDB alike.
CREATE TABLE big_table (
    id         BIGINT PRIMARY KEY,
    name       VARCHAR(50) NOT NULL,
    category   VARCHAR(20) NOT NULL,
    amount     DECIMAL(12, 2) NOT NULL,
    is_active  BOOLEAN NOT NULL,
    created_at DATETIME NOT NULL,
    INDEX big_table_category_idx (category)
);

CREATE TABLE seed_digits (d TINYINT PRIMARY KEY);
INSERT INTO seed_digits VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9);

INSERT INTO big_table (id, name, category, amount, is_active, created_at)
SELECT n,
       CONCAT('item ', n),
       ELT(1 + n % 4, 'alpha', 'beta', 'gamma', 'delta'),
       (n % 100000) / 100,
       n % 3 <> 0,
       TIMESTAMPADD(MINUTE, n, '2020-01-01 00:00:00')
FROM (
    SELECT 1 + d1.d + d2.d * 10 + d3.d * 100 + d4.d * 1000 + d5.d * 10000 + d6.d * 100000 AS n
    FROM seed_digits d1, seed_digits d2, seed_digits d3, seed_digits d4, seed_digits d5, seed_digits d6
) AS numbers;

DROP TABLE seed_digits;
ANALYZE TABLE big_table;

-- Identifiers that need quoting.
CREATE TABLE `Weird Table` (
    `ID`          INT AUTO_INCREMENT PRIMARY KEY,
    `Mixed Case`  TEXT,
    `select`      TEXT,
    `spasi kolom` TEXT,
    `ünïcödé`     TEXT,
    `dot.name`    TEXT
);

INSERT INTO `Weird Table` (`Mixed Case`, `select`, `spasi kolom`, `ünïcödé`, `dot.name`)
VALUES ('a', 'b', 'c', 'd', 'e');

-- Second database (MySQL's equivalent of a schema): foreign-key chain, composite key,
-- view, function.
CREATE DATABASE querra_sales;
GRANT ALL PRIVILEGES ON querra_sales.* TO 'querra'@'%';
USE querra_sales;

CREATE TABLE customers (
    id    INT AUTO_INCREMENT PRIMARY KEY,
    name  VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE
);

CREATE TABLE products (
    id    INT AUTO_INCREMENT PRIMARY KEY,
    sku   VARCHAR(20) NOT NULL UNIQUE,
    name  VARCHAR(100) NOT NULL,
    price DECIMAL(12, 2) NOT NULL,
    CHECK (price >= 0)
);

CREATE TABLE orders (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    ordered_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status      VARCHAR(20) NOT NULL DEFAULT 'new',
    FOREIGN KEY (customer_id) REFERENCES customers (id)
);

CREATE TABLE order_items (
    order_id   INT NOT NULL,
    product_id INT NOT NULL,
    quantity   INT NOT NULL,
    PRIMARY KEY (order_id, product_id),
    CHECK (quantity > 0),
    FOREIGN KEY (order_id) REFERENCES orders (id) ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES products (id)
);

INSERT INTO customers (name, email) VALUES
    ('Ayu', 'ayu@example.com'),
    ('Budi', 'budi@example.com'),
    ('Citra', NULL);

INSERT INTO products (sku, name, price) VALUES
    ('SKU-001', 'Keyboard', 450000.00),
    ('SKU-002', 'Mouse', 150000.00),
    ('SKU-003', 'Monitor', 2500000.00);

INSERT INTO orders (customer_id, ordered_at, status) VALUES
    (1, '2026-11-01 09:00:00', 'paid'),
    (1, '2026-11-02 14:30:00', 'new'),
    (2, '2026-11-03 08:15:00', 'shipped');

INSERT INTO order_items (order_id, product_id, quantity) VALUES
    (1, 1, 1),
    (1, 2, 2),
    (2, 3, 1),
    (3, 2, 1);

CREATE VIEW order_totals AS
SELECT o.id AS order_id, c.name AS customer, SUM(oi.quantity * p.price) AS total
FROM orders o
JOIN customers c ON c.id = o.customer_id
JOIN order_items oi ON oi.order_id = o.id
JOIN products p ON p.id = oi.product_id
GROUP BY o.id, c.name;

-- Single-statement body, so no DELIMITER is needed. READS SQL DATA satisfies
-- MySQL's binary-logging rule for stored functions.
CREATE FUNCTION order_count(p_customer_id INT) RETURNS INT
READS SQL DATA
RETURN (SELECT COUNT(*) FROM orders WHERE customer_id = p_customer_id);
