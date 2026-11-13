-- =============================================================================
-- ⚠️ 本文件已停用（仅供参考）：项目已从 PostgreSQL 迁移到 SQLite。
--    SQLite 下由 internal/store/db.go 的 AutoMigrate 自动建表，无需手动 SQL 迁移。
--    以下 PostgreSQL DDL 仅作为历史参考，保留用于表结构对照。
--
-- sing-box 企业级代理面板 — 初始迁移（PostgreSQL 版本，已停用）
-- 与 internal/store/db.go 的 AutoMigrate 等价（含外键约束与索引）。
--
-- 约定（system_design.md §9.7）：
--   * 主键 id 为 UUID v4，由应用层（google/uuid）生成，故此处不设 DB 默认值；
--   * 时间字段为 timestamptz（ISO8601 UTC）；
--   * 敏感字段：🔐=AES-256-GCM 加密；#=哈希（不可逆）。
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- group（分组/集群/区域）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "group" (
    id          uuid PRIMARY KEY,
    name        varchar(128) NOT NULL,
    desc        text,
    parent_id   uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uni_group_name ON "group" (name);
CREATE INDEX IF NOT EXISTS idx_group_parent_id ON "group" (parent_id);

-- -----------------------------------------------------------------------------
-- role（角色）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS role (
    id          uuid PRIMARY KEY,
    name        varchar(64) NOT NULL,
    permissions jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uni_role_name ON role (name);

-- -----------------------------------------------------------------------------
-- node（节点）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS node (
    id               uuid PRIMARY KEY,
    group_id         uuid NOT NULL REFERENCES "group" (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    name             varchar(128) NOT NULL,
    address          varchar(255),
    agent_token_hash varchar(64) NOT NULL,          -- # Agent 令牌 SHA-256
    agent_version    varchar(32),
    singbox_version  varchar(32),
    status           varchar(16) NOT NULL DEFAULT 'pending',
    last_heartbeat   timestamptz,
    tags             jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_node_group_id ON node (group_id);
CREATE INDEX IF NOT EXISTS idx_node_status ON node (status);
CREATE INDEX IF NOT EXISTS idx_node_last_heartbeat ON node (last_heartbeat);

-- -----------------------------------------------------------------------------
-- panel_user（面板用户）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS panel_user (
    id            uuid PRIMARY KEY,
    username      varchar(64) NOT NULL,
    password_hash varchar(128) NOT NULL,            -- # argon2id
    role_id       uuid NOT NULL REFERENCES role (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    status        varchar(16) NOT NULL DEFAULT 'enabled',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uni_panel_user_username ON panel_user (username);
CREATE INDEX IF NOT EXISTS idx_panel_user_role_id ON panel_user (role_id);

-- -----------------------------------------------------------------------------
-- inbound_config（Inbound 配置，中立模型）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS inbound_config (
    id              uuid PRIMARY KEY,
    group_id        uuid NOT NULL REFERENCES "group" (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    node_id         uuid REFERENCES node (id) ON UPDATE CASCADE ON DELETE CASCADE,  -- 空=分组模板
    name            varchar(64) NOT NULL,           -- 即 sing-box tag
    type            varchar(32) NOT NULL,
    listen          varchar(64),
    port            int NOT NULL DEFAULT 0,
    settings        jsonb,
    tls             jsonb,                          -- 🔐 Reality private_key 等私钥加密
    transport       jsonb,
    is_user_inbound bool NOT NULL DEFAULT false,
    status          varchar(16) NOT NULL DEFAULT 'enabled',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_inbound_group_id ON inbound_config (group_id);
CREATE INDEX IF NOT EXISTS idx_inbound_node_id ON inbound_config (node_id);
CREATE UNIQUE INDEX IF NOT EXISTS uni_inbound_scope_name ON inbound_config (group_id, node_id, name);

-- -----------------------------------------------------------------------------
-- outbound_config（Outbound 配置，中立模型）
-- 注意：包含 settings/tls/transport/status 四列（tls、transport 为 nullable JSONB）。
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS outbound_config (
    id         uuid PRIMARY KEY,
    group_id   uuid NOT NULL REFERENCES "group" (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    node_id    uuid REFERENCES node (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name       varchar(64) NOT NULL,                -- 即 sing-box tag
    type       varchar(32) NOT NULL,
    settings   jsonb,                               -- 🔐 服务器密码等敏感值加密
    tls        jsonb,                               -- 🔐 出站 TLS/Reality，私钥加密
    transport  jsonb,                               -- 出站传输层配置
    status     varchar(16) NOT NULL DEFAULT 'enabled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_outbound_group_id ON outbound_config (group_id);
CREATE INDEX IF NOT EXISTS idx_outbound_node_id ON outbound_config (node_id);
CREATE UNIQUE INDEX IF NOT EXISTS uni_outbound_scope_name ON outbound_config (group_id, node_id, name);

-- -----------------------------------------------------------------------------
-- routing_rule（路由规则）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS routing_rule (
    id            uuid PRIMARY KEY,
    group_id      uuid NOT NULL REFERENCES "group" (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    node_id       uuid REFERENCES node (id) ON UPDATE CASCADE ON DELETE CASCADE,
    name          varchar(64) NOT NULL,
    inbound_tags  jsonb,                            -- 入站 tag 数组
    outbound_tag  varchar(64),
    rule_type     varchar(16),
    value         text,
    priority      int NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_routing_group_id ON routing_rule (group_id);
CREATE INDEX IF NOT EXISTS idx_routing_node_id ON routing_rule (node_id);

-- -----------------------------------------------------------------------------
-- proxy_user（代理用户/终端用户）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS proxy_user (
    id             uuid PRIMARY KEY,
    group_id       uuid NOT NULL REFERENCES "group" (id) ON UPDATE CASCADE ON DELETE RESTRICT,
    uuid           text,                            -- 🔐 vless/vmess 用户 UUID，AES-GCM 加密
    password_hash  varchar(128),                    -- # 协议密码不可逆哈希（argon2id）
    password_enc   text,                            -- 🔐 协议明文密码可逆加密（AES-GCM，订阅/下发解密注入）
    remark         varchar(255),
    quota_bytes    bigint NOT NULL DEFAULT -1,      -- -1=无限
    used_bytes     bigint NOT NULL DEFAULT 0,
    expire_at      timestamptz,
    status         varchar(16) NOT NULL DEFAULT 'enabled',
    sub_token_hash varchar(64),                     -- # 订阅令牌 SHA-256
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_proxy_user_group_id ON proxy_user (group_id);
CREATE INDEX IF NOT EXISTS idx_proxy_user_status ON proxy_user (status);
CREATE INDEX IF NOT EXISTS idx_proxy_user_sub_token_hash ON proxy_user (sub_token_hash);

-- -----------------------------------------------------------------------------
-- config_version（配置版本，只追加）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS config_version (
    id          uuid PRIMARY KEY,
    node_id     uuid NOT NULL REFERENCES node (id) ON UPDATE CASCADE ON DELETE CASCADE,
    version     int NOT NULL,
    content_enc text,                               -- 🔐 完整 sing-box JSON 快照 AES-GCM 加密
    hash        varchar(64),                        -- 明文 SHA-256
    status      varchar(16) NOT NULL DEFAULT 'draft',
    created_by  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uni_config_version_node_version UNIQUE (node_id, version)
);
CREATE INDEX IF NOT EXISTS idx_config_version_hash ON config_version (hash);

-- -----------------------------------------------------------------------------
-- traffic_record（流量时序，TimescaleDB hypertable P1）
-- 组合主键，无 UUID；proxy_user_id 可为空（节点级聚合）。
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS traffic_record (
    ts            timestamptz NOT NULL,
    node_id       uuid NOT NULL,
    inbound_tag   varchar(64) NOT NULL,
    outbound_tag  varchar(64) NOT NULL,
    proxy_user_id uuid,
    up_bytes      bigint NOT NULL DEFAULT 0,
    down_bytes    bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (ts, node_id, inbound_tag, outbound_tag)
);
CREATE INDEX IF NOT EXISTS idx_traffic_node_id ON traffic_record (node_id);
CREATE INDEX IF NOT EXISTS idx_traffic_proxy_user_id ON traffic_record (proxy_user_id);

-- -----------------------------------------------------------------------------
-- audit_log（审计日志）
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_log (
    id          uuid PRIMARY KEY,
    operator_id uuid,
    action      varchar(64) NOT NULL,
    resource    varchar(255),
    detail      jsonb,                              -- 变更前后快照
    ip          varchar(64),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_operator_id ON audit_log (operator_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_log (created_at);

COMMIT;
