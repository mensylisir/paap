CREATE TABLE IF NOT EXISTS permissions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    scope_type VARCHAR(20) NOT NULL,
    resource VARCHAR(60) NOT NULL,
    action VARCHAR(60) NOT NULL,
    group_name VARCHAR(80) NOT NULL DEFAULT '',
    risk_level VARCHAR(20) NOT NULL DEFAULT 'normal',
    builtin BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_permissions_scope_type ON permissions (scope_type);
CREATE INDEX IF NOT EXISTS idx_permissions_builtin ON permissions (builtin);
CREATE INDEX IF NOT EXISTS idx_permissions_enabled ON permissions (enabled);
CREATE INDEX IF NOT EXISTS idx_permissions_deleted_at ON permissions (deleted_at);

CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    code VARCHAR(80) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    scope_type VARCHAR(20) NOT NULL,
    builtin BOOLEAN NOT NULL DEFAULT FALSE,
    editable BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT idx_roles_code_scope UNIQUE (code, scope_type)
);

CREATE INDEX IF NOT EXISTS idx_roles_scope_type ON roles (scope_type);
CREATE INDEX IF NOT EXISTS idx_roles_builtin ON roles (builtin);
CREATE INDEX IF NOT EXISTS idx_roles_enabled ON roles (enabled);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles (deleted_at);

CREATE TABLE IF NOT EXISTS role_permissions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    CONSTRAINT idx_role_permissions_role_permission UNIQUE (role_id, permission_id)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_deleted_at ON role_permissions (deleted_at);

CREATE TABLE IF NOT EXISTS role_bindings (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    scope_type VARCHAR(20) NOT NULL,
    scope_id BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT idx_role_bindings_user_role_scope UNIQUE (user_id, role_id, scope_type, scope_id)
);

CREATE INDEX IF NOT EXISTS idx_role_bindings_user_id ON role_bindings (user_id);
CREATE INDEX IF NOT EXISTS idx_role_bindings_role_id ON role_bindings (role_id);
CREATE INDEX IF NOT EXISTS idx_role_bindings_scope_type ON role_bindings (scope_type);
CREATE INDEX IF NOT EXISTS idx_role_bindings_scope_id ON role_bindings (scope_id);
CREATE INDEX IF NOT EXISTS idx_role_bindings_deleted_at ON role_bindings (deleted_at);

INSERT INTO permissions (code, name, description, scope_type, resource, action, group_name, risk_level, builtin, enabled)
VALUES
('system.user.read', '查看用户', '查看平台用户列表与基础信息', 'system', 'user', 'read', '用户管理', 'normal', TRUE, TRUE),
('system.user.manage', '管理用户', '调整平台级用户角色', 'system', 'user', 'manage', '用户管理', 'high', TRUE, TRUE),
('system.role.manage', '管理角色', '创建、更新、删除自定义角色并维护角色权限点', 'system', 'role', 'manage', '角色权限', 'high', TRUE, TRUE),
('system.template.manage', '管理模板', '创建、更新、删除服务模板和组件配置模板', 'system', 'template', 'manage', '模板管理', 'high', TRUE, TRUE),
('system.shared_pool.manage', '管理共享资源池', '管理平台共享资源池与共享资源', 'system', 'shared_pool', 'manage', '共享资源池', 'high', TRUE, TRUE),
('app.create', '创建应用', '创建新的应用', 'system', 'app', 'create', '应用管理', 'normal', TRUE, TRUE),
('app.read', '查看应用', '查看应用基础信息', 'app', 'app', 'read', '应用管理', 'normal', TRUE, TRUE),
('app.update', '编辑应用', '编辑应用名称和描述', 'app', 'app', 'update', '应用管理', 'normal', TRUE, TRUE),
('app.delete', '删除应用', '删除应用及其环境资源', 'app', 'app', 'delete', '应用管理', 'danger', TRUE, TRUE),
('app.member.read', '查看成员', '查看应用成员', 'app', 'member', 'read', '成员管理', 'normal', TRUE, TRUE),
('app.member.manage', '管理成员', '邀请、修改、移除应用成员', 'app', 'member', 'manage', '成员管理', 'high', TRUE, TRUE),
('env.create', '创建环境', '在应用中创建环境', 'app', 'env', 'create', '环境管理', 'normal', TRUE, TRUE),
('env.read', '查看环境', '查看环境、画布、资源状态', 'env', 'env', 'read', '环境管理', 'normal', TRUE, TRUE),
('env.manage', '管理环境', '修改环境画布、能力和基础配置', 'env', 'env', 'manage', '环境管理', 'normal', TRUE, TRUE),
('env.delete', '删除环境', '删除环境及其资源', 'env', 'env', 'delete', '环境管理', 'danger', TRUE, TRUE),
('service.read', '查看服务', '查看工具和中间件实例', 'env', 'service', 'read', '服务管理', 'normal', TRUE, TRUE),
('service.install', '安装服务', '安装工具或中间件实例', 'env', 'service', 'install', '服务管理', 'high', TRUE, TRUE),
('service.manage', '管理服务', '更新、卸载、执行服务工作区动作', 'env', 'service', 'manage', '服务管理', 'high', TRUE, TRUE),
('component.read', '查看组件', '查看业务组件、日志、指标和控制台', 'env', 'component', 'read', '组件管理', 'normal', TRUE, TRUE),
('component.create', '创建组件', '创建或接入业务组件', 'env', 'component', 'create', '组件管理', 'normal', TRUE, TRUE),
('component.deploy', '部署组件', '部署、重启、调整组件外部访问', 'env', 'component', 'deploy', '组件管理', 'high', TRUE, TRUE),
('component.manage', '管理组件', '更新或删除业务组件', 'env', 'component', 'manage', '组件管理', 'high', TRUE, TRUE)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    scope_type = EXCLUDED.scope_type,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    group_name = EXCLUDED.group_name,
    risk_level = EXCLUDED.risk_level,
    builtin = TRUE,
    enabled = TRUE;

INSERT INTO roles (code, name, description, scope_type, builtin, editable, enabled)
VALUES
('platform_admin', '平台管理员', '平台全局管理员，拥有所有平台、应用和环境权限', 'system', TRUE, FALSE, TRUE),
('app_admin', '应用管理员', '平台级应用管理员，可以创建应用', 'system', TRUE, FALSE, TRUE),
('user', '普通用户', '平台普通用户，只能访问被授权的应用', 'system', TRUE, FALSE, TRUE),
('admin', '应用管理员', '应用级管理员，管理当前应用、成员和环境', 'app', TRUE, FALSE, TRUE),
('member', '应用成员', '应用级成员，开发和部署当前应用资源', 'app', TRUE, FALSE, TRUE),
('viewer', '只读成员', '应用级只读成员，只能查看当前应用资源', 'app', TRUE, FALSE, TRUE)
ON CONFLICT (code, scope_type) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    builtin = TRUE,
    editable = FALSE,
    enabled = TRUE;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'system.user.read', 'system.user.manage', 'system.role.manage', 'system.template.manage', 'system.shared_pool.manage', 'app.create',
    'app.read', 'app.update', 'app.delete', 'app.member.read', 'app.member.manage', 'env.create',
    'env.read', 'env.manage', 'env.delete', 'service.read', 'service.install', 'service.manage',
    'component.read', 'component.create', 'component.deploy', 'component.manage'
)
WHERE r.code = 'platform_admin' AND r.scope_type = 'system'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('app.create')
WHERE r.code = 'app_admin' AND r.scope_type = 'system'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'app.read', 'app.update', 'app.delete', 'app.member.read', 'app.member.manage', 'env.create',
    'env.read', 'env.manage', 'env.delete', 'service.read', 'service.install', 'service.manage',
    'component.read', 'component.create', 'component.deploy', 'component.manage'
)
WHERE r.code = 'admin' AND r.scope_type = 'app'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN (
    'app.read', 'app.member.read', 'env.create',
    'env.read', 'env.manage', 'service.read', 'service.install', 'service.manage',
    'component.read', 'component.create', 'component.deploy', 'component.manage'
)
WHERE r.code = 'member' AND r.scope_type = 'app'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('app.read', 'app.member.read', 'env.read', 'service.read', 'component.read')
WHERE r.code = 'viewer' AND r.scope_type = 'app'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_bindings (user_id, role_id, scope_type, scope_id, created_by)
SELECT u.id, r.id, 'system', 0, 0
FROM users u
JOIN roles r ON r.scope_type = 'system' AND r.code IN ('platform_admin', 'app_admin')
WHERE u.username = 'admin'
ON CONFLICT (user_id, role_id, scope_type, scope_id) DO NOTHING;

INSERT INTO role_bindings (user_id, role_id, scope_type, scope_id, created_by)
SELECT u.id, r.id, 'system', 0, 0
FROM users u
JOIN roles r ON r.scope_type = 'system' AND r.code = 'user'
WHERE u.username = 'user'
ON CONFLICT (user_id, role_id, scope_type, scope_id) DO NOTHING;

INSERT INTO role_bindings (user_id, role_id, scope_type, scope_id, created_by)
SELECT am.user_id, r.id, 'app', am.application_id, 0
FROM app_members am
JOIN roles r ON r.code = am.role AND r.scope_type = 'app'
WHERE am.deleted_at IS NULL
ON CONFLICT (user_id, role_id, scope_type, scope_id) DO NOTHING;

DROP TABLE IF EXISTS user_roles;
