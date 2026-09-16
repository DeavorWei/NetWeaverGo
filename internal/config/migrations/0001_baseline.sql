-- 0001_baseline.sql: NetWeaverGo 基线数据表与结构版本追踪
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(64) PRIMARY KEY,
    description VARCHAR(255),
    applied_at DATETIME NOT NULL,
    checksum VARCHAR(64),
    success BOOLEAN DEFAULT 1
);

CREATE TABLE IF NOT EXISTS device_assets (
    id TEXT PRIMARY KEY,
    ip TEXT NOT NULL,
    port INTEGER DEFAULT 22,
    protocol TEXT DEFAULT 'ssh',
    auth_type TEXT DEFAULT 'password',
    username TEXT,
    password TEXT,
    enable_password TEXT,
    vendor TEXT,
    model TEXT,
    os_version TEXT,
    group_name TEXT,
    device_type TEXT,
    status TEXT DEFAULT 'unknown',
    description TEXT,
    login_status TEXT DEFAULT 'untested',
    last_login_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME
);
