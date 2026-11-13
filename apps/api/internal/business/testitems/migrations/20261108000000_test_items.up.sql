CREATE TABLE product_test_items (
    id text PRIMARY KEY,
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    category text NOT NULL,
    method text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO product_test_items (id, code, name, category, method, description) VALUES
    ('ti-001', 'WATER-PH', '水质 pH 值', '水质检测', '电极法', '测定水样的酸碱度。'),
    ('ti-002', 'WATER-COD', '化学需氧量（COD）', '水质检测', '重铬酸盐法', '测定水样中可被氧化物质的氧当量。'),
    ('ti-003', 'SOIL-PH', '土壤 pH 值', '土壤检测', '电位法', '测定土壤浸提液的酸碱度。'),
    ('ti-004', 'AIR-PM25', '细颗粒物（PM2.5）', '环境空气检测', '重量法', '测定环境空气中空气动力学当量直径不大于 2.5 μm 的颗粒物。');
