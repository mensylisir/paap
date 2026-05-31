# PAAP 模板系统总览

本文档提供 PAAP 平台模板系统的整体架构说明，帮助你理解平台如何管理和部署服务模板。

---

## 1. 模板系统架构

### 1.1 当前实现状态

PAAP 平台正在向统一的模板格式迁移：

**✅ 用户上传模板（已实现标准格式）：**
```
用户上传模板包结构：
├── chart/                      # Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── platform-manifest.yaml      # 平台元数据（必需）
├── preset-values.yaml          # 配置覆盖（推荐）
└── dashboards/                 # Grafana 面板（可选）
```

**⚠️ 内置模板（迁移中）：**
- 当前使用简化方式：
  - 部分使用 `raw-yaml`（硬编码 YAML 模板）：ArgoCD、Tekton、Prometheus、Loki
  - 部分直接引用 Helm 仓库：PostgreSQL、MySQL、Redis、RabbitMQ 等
- 计划迁移到标准格式（与用户上传模板相同）

**🎯 目标架构：**
所有模板（内置 + 用户上传）最终都将使用统一的标准格式。

### 1.2 标准模板格式（目标）

```
标准模板包结构：
├── chart/                      # Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── platform-manifest.yaml      # 平台元数据（必需）
├── preset-values.yaml          # 配置覆盖（推荐）
└── dashboards/                 # Grafana 面板（可选）
```

**适用范围：**
- ✅ 用户上传模板：已完全实现
- ⚠️ 内置模板：迁移中

---

## 2. 核心概念

### 2.1 模板包结构

```
my-template/
├── chart/                      # 标准 Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── platform-manifest.yaml      # 平台元数据（必需）
├── preset-values.yaml          # 配置覆盖（推荐）
└── dashboards/                 # Grafana 面板（可选）
```

### 2.2 platform-manifest.yaml

平台元数据文件，声明模板的权限需求、监控配置、变量映射等。

**核心字段：**

```yaml
name: "MyTool"
version: "v1.0.0"

permissions:
  scope: "tool-only"  # 或 "environment-wide"
  rules: [...]        # 仅当 scope=environment-wide 时需要

observability:        # 可选
  dashboard_path: "./dashboards/main.json"
  metrics:
    port: 9090
    path: "/metrics"

variable_mapping:     # 可选
  - platform_var: "env_namespaces"
    helm_var: "config.namespaces"
```

---

## 3. 权限模型

### 3.1 两种权限范围

| scope | 说明 | 适用场景 | 示例 |
|-------|------|---------|------|
| `tool-only` | 工具只需要访问自己所在的 namespace | 数据库、缓存、消息队列 | PostgreSQL, Redis, RabbitMQ |
| `environment-wide` | 工具需要访问环境内所有 namespace | 监控、CI/CD、日志收集 | Prometheus, ArgoCD, Loki |

### 3.2 权限同步机制

**scope: tool-only**
```
平台行为：
1. 在工具所在的 namespace 创建 ServiceAccount
2. 在工具所在的 namespace 创建 Role（基本权限）
3. 创建 RoleBinding（SA → Role）
```

**scope: environment-wide**
```
平台行为：
1. 在工具所在的 namespace 创建 ServiceAccount
2. 在环境的每个 namespace 创建 Role（使用 permissions.rules）
3. 在环境的每个 namespace 创建 RoleBinding（SA → Role）

动态同步：
- 环境新增 namespace → 自动创建 Role + RoleBinding
- 环境删除 namespace → 自动清理 Role + RoleBinding
```

---

## 4. 模板类型对比

### 4.1 内置模板 vs 用户上传模板

| 特性 | 内置模板 | 用户上传模板 |
|------|---------|-------------|
| 创建方式 | 平台管理员通过代码预定义 | 用户通过 UI/API 上传 tar.gz |
| 存储位置 | 数据库 + 文件系统/S3 | 数据库 + 文件系统/S3 |
| 格式 | ⚠️ 迁移中（目标：platform-manifest.yaml） | ✅ platform-manifest.yaml |
| 权限模型 | ⚠️ 部分使用旧方式 | ✅ 使用 permissions.scope + rules |
| 可见性 | 所有用户可见 | 仅上传者可见（或全局共享） |
| 升级方式 | 平台升级时更新 | 用户重新上传 |

**说明：**
- **用户上传模板**：已完全实现标准格式
- **内置模板**：正在迁移到标准格式，当前使用简化方式

### 4.2 自定义 Chart vs 第三方 Chart

| 特性 | 自定义 Chart | 第三方 Chart（零改动） |
|------|-------------|---------------------|
| 开发成本 | 高（从零编写） | 低（只写配置） |
| 维护成本 | 高（持续维护） | 低（跟随官方升级） |
| 功能完整性 | 取决于开发能力 | 通常很高 |
| 适用场景 | 内部工具、简单服务 | 成熟中间件、复杂工具 |

详见：[自定义 vs 第三方 Chart](custom-vs-third-party.md)

---

## 5. 平台处理流程

### 5.1 模板上传流程

```
用户上传 my-template.tar.gz
         ↓
平台解压并验证结构
         ↓
安全审查（扫描违规资源）
  - 检查是否包含 ClusterRole
  - 检查是否包含 CRD
  - 检查权限是否过大
         ↓
解析 platform-manifest.yaml
  - 验证格式
  - 验证 permissions.scope
  - 验证 permissions.rules
         ↓
存储到数据库 + 文件系统/S3
  - ServiceTemplate 记录
  - PlatformManifestJSON 字段
  - ChartArchivePath 或 S3Key
         ↓
模板可用
```

### 5.2 服务安装流程

```
用户点击"安装服务"
         ↓
平台读取 ServiceTemplate
         ↓
解析 PlatformManifestJSON
         ↓
检查 permissions.scope
         ↓
┌─────────────────────────────────────┐
│ scope = tool-only                   │
│   → 只在工具 namespace 创建 SA + Role│
└─────────────────────────────────────┘
┌─────────────────────────────────────┐
│ scope = environment-wide            │
│   → 在环境所有 namespace 创建       │
│     Role + RoleBinding              │
└─────────────────────────────────────┘
         ↓
合并 Helm values
  1. chart/values.yaml（Chart 默认值）
  2. preset-values.yaml（配置覆盖）
  3. 平台注入的变量（SA 名称、namespace 列表）
  4. 用户在 UI 填写的参数
         ↓
创建 ServiceInstance CR
  - ServiceAccount 配置
  - WorkloadRole（对应 permissions.rules）
  - ManifestsRef（指向 ConfigMap）
         ↓
Operator 监听 CR 并创建 K8s 资源
  - 创建 ServiceAccount
  - 创建 Role + RoleBinding
  - 执行 helm template 生成 YAML
  - 应用 YAML 到集群
         ↓
如果有 observability.dashboard_path
  → 调用 Grafana API 导入面板
         ↓
安装完成
```

### 5.3 环境新增 namespace 时

```
用户在环境中新增 namespace-03
         ↓
Operator 监听 Environment CR 变化
         ↓
查询该环境已安装的所有 ServiceInstance
         ↓
遍历每个 ServiceInstance
         ↓
检查 WorkloadRole 是否为空
         ↓
如果不为空（说明是 environment-wide）
  → 在 namespace-03 创建 Role + RoleBinding
         ↓
完成（工具立即可以访问新 namespace）
```

---

## 6. 数据模型

### 6.1 ServiceTemplate（数据库）

```go
type ServiceTemplate struct {
    ID          uint
    Type        string  // 唯一标识，如 "redis", "argocd"
    Name        string  // 展示名称，如 "Redis", "ArgoCD"
    Category    string  // "tool" | "infra" | "middleware"
    
    // Helm Chart 配置
    Installer    string  // "helm"
    ChartRepo    string  // Helm 仓库 URL（内置模板）
    ChartName    string  // Chart 名称（内置模板）
    ChartVersion string  // Chart 版本（内置模板）
    
    // 自定义模板（BYO）
    IsCustom             bool    // 是否为用户上传
    PlatformManifestJSON string  // platform-manifest.yaml 的 JSON
    ChartArchivePath     string  // 本地文件路径
    S3Bucket             string  // S3 存储桶（可选）
    S3Key                string  // S3 对象键（可选）
    PresetValues         string  // preset-values.yaml 内容
}
```

### 6.2 ServiceInstance CR（Kubernetes）

```yaml
apiVersion: paap.io/v1
kind: ServiceInstance
metadata:
  name: order-service-dev-redis
  namespace: paap-app-1
spec:
  # 工具配置
  toolNamespace: order-service-dev
  releaseName: order-service-dev-redis
  
  # ServiceAccount 配置
  serviceAccount:
    name: order-service-dev-redis
    namespace: order-service-dev
  
  # 工具需要对负载 namespace 的操作权限
  # 对应 platform-manifest.yaml 的 permissions.rules
  workloadRole:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services"]
        verbs: ["get", "list", "watch"]
  
  # 渲染后的部署清单（存储在 ConfigMap 中）
  manifestsRef:
    name: order-service-dev-redis-manifests
```

---

## 7. 文档导航

### 7.1 开发模板

- **[自定义模板开发指南](custom-template-guide.md)** - 完整的模板开发指南
  - platform-manifest.yaml 规范
  - Chart 开发规范
  - 安全审查要求
  - 零改动转换第三方 Chart

- **[自定义 vs 第三方 Chart](custom-vs-third-party.md)** - 如何选择
  - 决策树
  - 详细对比
  - 实际案例

- **[快速参考卡片](../QUICK-REFERENCE.md)** - 一页纸快速参考

### 7.2 示例模板

- **[自定义 Prometheus 模板](../examples/custom-prometheus-template/)** - 从零编写
- **[Bitnami Redis 模板](../examples/bitnami-redis-template/)** - 零改动转换

### 7.3 设计文档

- **[服务模板规范](service-template-spec.md)** - 早期设计方案（未实现）
- **[Operator 设计](operator-design.md)** - Operator 架构
- **[权限隔离设计](service-isolation-design.md)** - 权限模型详解

---

## 8. 常见问题

### Q1: 内置模板和用户上传模板有什么区别？

**A:** 来源不同，但目标格式相同：

- **用户上传模板**：
  - ✅ 已完全实现标准格式：`Helm Chart + platform-manifest.yaml + preset-values.yaml`
  - 用户通过 UI/API 上传 tar.gz 包
  - 完整的权限控制和安全审查

- **内置模板**：
  - ⚠️ 当前使用简化方式（`raw-yaml` 或直接引用 Helm 仓库）
  - 平台管理员通过代码预定义
  - 正在迁移到标准格式

**未来：** 所有模板都将使用相同的标准格式。

### Q2: 为什么不使用 lifecycle 钩子？

**A:** 早期设计考虑过 `lifecycle.install` / `onEnvNsAdded` 等钩子，但实际实现采用了更简洁的方案：
- 使用标准 Helm Chart 管理资源（无需自定义钩子）
- 由 Operator 自动处理权限同步（无需手动编写钩子逻辑）

这样降低了模板开发的复杂度，同时保持了灵活性。

### Q3: 如何升级第三方 Chart？

**A:** 零改动转换的优势：
```bash
# 1. 下载新版本
helm pull bitnami/redis --version 18.20.0 --untar -d ./chart-new

# 2. 替换 chart/ 目录
rm -rf chart/redis && mv chart-new/redis chart/

# 3. 重新打包上传
tar -czf redis-template.tar.gz redis-template/
```

因为没有修改 Chart 代码，升级只需要替换目录。

### Q4: permissions.scope 应该选哪个？

**A:** 根据工具的实际需求：
- **tool-only**: 工具只在自己的 namespace 内运行（数据库、缓存）
- **environment-wide**: 工具需要访问环境内所有 namespace（监控、CI/CD）

详见：[自定义模板开发指南 - permissions.scope](custom-template-guide.md#324-variable_mapping变量映射)

---

## 9. 总结

PAAP 平台的模板系统设计理念：

1. **统一格式**：内置模板和用户上传模板使用相同的格式
2. **标准化**：基于 Helm Chart，兼容开源生态
3. **零改动**：第三方 Chart 无需修改代码即可集成
4. **自动化**：权限同步、面板导入等由平台自动处理
5. **安全性**：严格的权限控制和安全审查

这种设计在保持灵活性的同时，大幅降低了模板开发和维护的成本。
