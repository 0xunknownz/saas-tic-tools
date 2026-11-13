ALTER TABLE product_test_items ADD COLUMN revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
