-- 0001_baseline.sql: NetWeaverGo 基线数据表与结构版本追踪
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(64) PRIMARY KEY,
    description VARCHAR(255),
    applied_at DATETIME NOT NULL,
    checksum VARCHAR(64),
    success BOOLEAN DEFAULT 1
);

CREATE TABLE IF NOT EXISTS device_assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip TEXT NOT NULL,
    port INTEGER DEFAULT 22,
    username TEXT,
    password TEXT,
    protocol TEXT DEFAULT 'ssh',
    group_name TEXT,
    display_name TEXT,
    vendor TEXT,
    role TEXT,
    site TEXT,
    description TEXT,
    tags TEXT,
    model TEXT,
    model_series TEXT,
    version TEXT,
    patch_version TEXT,
    esn TEXT,
    form_factor TEXT DEFAULT 'hardware',
    connect_mode TEXT DEFAULT 'direct',
    jump_host_id INTEGER,
    charset TEXT,
    last_seen DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_device_assets_ip ON device_assets(ip);
CREATE INDEX IF NOT EXISTS idx_device_assets_jump_host_id ON device_assets(jump_host_id);

