# PAAP 自定义模板开发指南（BYO - Bring Your Own）

> **核心理念：** 模板 = Helm Chart（资源定义） + platform-manifest.yaml（平台元数据）

> **📌 适用范围：** 本文档描述 PAAP 平台的**标准模板格式**。
> 
> **当前状态：**
> - ✅ **用户上传模板**：已完全实现本文档描述的格式
> - ⚠️ **内置模板**：正在迁移到本文档描述的格式
> 
> **如果你要开发自定义模板，请完全按照本文档的规范进行。**
> 
> 如需了解平台模板系统的整体架构和迁移状态，请参考 [模板系统总览](template-system-overview.md)。

## 1. 概述

### 1.1 为什么需要"1+1"组合？

Helm Chart 本身存在**能力边界**，它只能管理包内资源，无法感知和操作包外的平台级能力。

| 功能需求 | 仅用 Helm Chart | 模板包 (Helm + Spec) |
|---------|----------------|---------------------|
| 安装 Pod/Service | ✅ 完美支持 | ✅ 包含其中 |
| 权限控制 (RBAC) | ❌ 只能写死，无法感知环境内动态增加的 Namespace | ✅ 在 Spec 中声明，由平台动态注入 RoleBinding |
| 监控面板接入 | ❌ 用户装完后需手动导入 JSON 到 Grafana | ✅ Spec 记录路径，平台安装时自动调 API 导入 |
| 动态权限同步 | ❌ 无法实现 | ✅ 平台根据 Spec 里的声明，监听环境变化并更新权限 |

### 1.2 类比：插卡游戏机

- **Helm Chart** = 游戏软件（决定了工具怎么运行）
- **platform-manifest.yaml** = 金手指或说明书（告诉游戏机这个软件需要什么权限、怎么接手柄、怎么存盘）
- **PAAP 平台** = 游戏机主机（负责读取说明书，分配内存和权限，运行软件）

---

## 2. 模板包结构

### 2.1 标准目录结构

用户需要上传一个 `.tar.gz` 或 `.zip` 压缩包，解压后的目录结构如下：

```
my-prometheus-template/
├── chart/                          # 标准的 Helm Chart 目录
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── templates/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   └── configmap.yaml
│   └── ...
├── dashboards/                     # 可选：预定义的 Grafana 面板
│   ├── main-metrics.json
│   └── detailed-view.json
├── preset-values.yaml              # 可选：预设值（禁用 Chart 内置 RBAC）
└── platform-manifest.yaml          # 必需：平台元数据声明
```

### 2.2 文件说明

| 文件/目录 | 必需 | 说明 |
|----------|------|------|
| `chart/` | ✅ | 标准 Helm Chart，包含所有 K8s 资源定义 |
| `platform-manifest.yaml` | ✅ | 平台元数据，声明权限、监控、变量映射 |
| `preset-values.yaml` | ⚠️ | 推荐：用于禁用 Chart 内置的 RBAC 创建 |
| `dashboards/` | ❌ | 可选：Grafana 面板 JSON 文件 |

---

## 3. platform-manifest.yaml 规范

### 3.1 完整示例

```yaml
# 基本信息
name: "CustomPrometheus"
version: "v1.0.0"
description: "自定义 Prometheus 监控服务，支持环境级权限动态同步"

# 1. 权限申请声明
permissions:
  # 工具自身 namespace 权限，安装和运行自身资源使用。
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "endpoints", "configmaps", "secrets", "serviceaccounts"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
      - apiGroups: ["apps"]
        resources: ["deployments", "statefulsets"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # 业务负载 namespace 权限。部署/CI 工具需要控制业务资源时使用。
  workloadNamespaces:
    rules:
      - apiGroups: ["*"]
        resources: ["*"]
        verbs: ["*"]

  # 同环境其它 namespace 权限。监控/日志工具需要看数据库、中间件、其它工具 namespace 时使用。
  environmentNamespaces:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "endpoints"]
        verbs: ["get", "list", "watch"]
      - apiGroups: ["apps"]
        resources: ["deployments", "statefulsets"]
        verbs: ["get", "list", "watch"]

# 2. 可观测性声明（可选）
observability:
  # Grafana 面板路径（相对于压缩包根目录）
  dashboard_path: "./dashboards/main-metrics.json"
  
  # Prometheus 指标配置
  metrics:
    port: 9090
    path: "/metrics"

# 3. 变量映射（可选）
# 将平台变量映射到 Helm values
variable_mapping:
  # 把平台当前环境的名字传给 Helm 里的 global.envName 变量
  - platform_var: "current_env_name"
    helm_var: "global.envName"
  
  # 把环境的所有 namespace 列表传给 Helm
  - platform_var: "env_namespaces"
    helm_var: "config.targetNamespaces"
```

### 3.2 字段详解

#### 3.2.1 基本信息

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 工具名称，用于标识和展示 |
| `version` | string | ✅ | 版本号，建议使用语义化版本 |
| `description` | string | ❌ | 工具描述 |

#### 3.2.2 permissions（权限声明）

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `toolNamespace.rules` | array | ✅ | 工具自身 namespace 内的 Role 规则 |
| `workloadNamespaces.rules` | array | ❌ | 投射到业务负载 namespace 的 Role 规则 |
| `environmentNamespaces.rules` | array | ❌ | 投射到同环境其它 namespace 的 Role 规则 |
| `clusterResources.rules` | array | ❌ | 集群级只读权限，例如 nodes、namespaces、storageclasses |

**权限类型说明：**

- **`toolNamespace`**: 工具只在自己的 namespace 内运行或管理自身资源
  - 示例：PostgreSQL、Redis、MinIO、Harbor 自身组件
  - 平台行为：只在工具所在 namespace 创建 Role + RoleBinding

- **`workloadNamespaces`**: 工具需要读写业务负载 namespace
  - 示例：ArgoCD、Jenkins
  - 平台行为：在当前环境的业务 namespace 创建 Role + RoleBinding

- **`environmentNamespaces`**: 工具需要访问同环境其它工具、中间件、数据库和业务 namespace
  - 示例：Prometheus、Loki
  - 平台行为：在当前环境的所有非自身 namespace 创建 Role + RoleBinding，并随 namespace 增删动态同步

**rules 格式：**

```yaml
rules:
  - apiGroups: [""]              # 空字符串代表 core API group
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
  
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
```

#### 3.2.3 observability（可观测性）

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `dashboard_path` | string | ❌ | Grafana 面板 JSON 文件路径 |
| `metrics.port` | int | ❌ | Prometheus 指标端口 |
| `metrics.path` | string | ❌ | Prometheus 指标路径 |

**平台行为：**
- 如果提供了 `dashboard_path`，平台会在安装时自动调用 Grafana API 导入面板
- 如果提供了 `metrics`，平台会自动配置 Prometheus ServiceMonitor

#### 3.2.4 variable_mapping（变量映射）

允许将平台上下文变量映射到 Helm values。

**可用的平台变量：**

| 平台变量 | 说明 | 示例值 |
|---------|------|--------|
| `current_env_name` | 当前环境名称 | `开发环境` |
| `current_env_identifier` | 当前环境标识 | `dev` |
| `primary_namespace` | 环境主 namespace | `order-service-dev` |
| `env_namespaces` | 环境所有 namespace（逗号分隔） | `order-service-dev,order-service-dev-cache` |
| `app_name` | 应用名称 | `订单服务` |
| `app_identifier` | 应用标识 | `order-service` |

**示例：**

```yaml
variable_mapping:
  # 将环境标识传给 Helm 的 global.env
  - platform_var: "current_env_identifier"
    helm_var: "global.env"
  
  # 将所有 namespace 传给 Helm 的 prometheus.targetNamespaces
  - platform_var: "env_namespaces"
    helm_var: "prometheus.targetNamespaces"
```

---

## 4. preset-values.yaml 规范

### 4.1 作用

大多数 Helm Chart 会内置创建 ServiceAccount、Role、RoleBinding。但在 PAAP 平台中，**权限由平台统一管理**，因此需要禁用 Chart 内置的 RBAC 创建。

### 4.2 示例

```yaml
# 禁用 Chart 内置的 RBAC 创建
rbac:
  create: false

serviceAccount:
  create: false
  # 平台会自动注入 ServiceAccount 名称
  name: ""  # 留空，平台会填充

# 禁用内置的 NetworkPolicy（平台统一管理）
networkPolicy:
  enabled: false

# 其他预设值
replicaCount: 1
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### 4.3 应用顺序

平台在安装时会按以下顺序合并 values：

1. **Chart 默认 values.yaml**
2. **preset-values.yaml**（你提供的预设值）
3. **平台注入的上下文变量**（如 ServiceAccount 名称、namespace 列表）
4. **用户在 UI 填写的参数**

---

## 5. Chart 开发规范

### 5.1 禁止事项

❌ **禁止在 Chart 中创建以下资源：**

- `ClusterRole` / `ClusterRoleBinding`
- `CustomResourceDefinition` (CRD)
- `MutatingWebhookConfiguration` / `ValidatingWebhookConfiguration`
- `PodSecurityPolicy`
- 任何集群级别的资源

**原因：** 这些资源会影响整个集群，违反多租户隔离原则。

### 5.2 推荐做法

✅ **推荐：**

1. **使用 namespace 级别的资源**
   - Deployment、StatefulSet、DaemonSet
   - Service、ConfigMap、Secret
   - PersistentVolumeClaim

2. **使用模板变量接收平台注入的值**

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "mychart.fullname" . }}
  namespace: {{ .Values.global.namespace }}  # 平台注入
spec:
  template:
    spec:
      serviceAccountName: {{ .Values.global.serviceAccountName }}  # 平台注入
      containers:
        - name: app
          image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
```

3. **在 values.yaml 中预留平台变量占位符**

```yaml
# values.yaml
global:
  # 以下字段由平台自动注入，无需用户填写
  namespace: ""
  serviceAccountName: ""
  envNamespaces: ""
  
# 用户可配置的参数
image:
  repository: prom/prometheus
  tag: v2.45.0

replicaCount: 1
```

### 5.3 安全审查

平台在接收模板包时会进行以下检查：

1. ✅ 解析 `chart/` 目录，验证是否为合法的 Helm Chart
2. ✅ 扫描 `templates/` 目录，检测是否包含违规资源（ClusterRole 等）
3. ✅ 验证 `platform-manifest.yaml` 格式和字段
4. ✅ 检查 `permissions.rules` 是否包含危险权限（如 `*/*` 全权限）

**如果检测到违规，平台会拒绝上传并返回错误信息。**

---

## 6. 平台处理流程

### 6.1 用户上传模板包

```
用户上传 my-template.tar.gz
         ↓
平台解压并验证结构
         ↓
安全审查（扫描违规资源）
         ↓
解析 platform-manifest.yaml
         ↓
存储到数据库 + 文件系统/S3
         ↓
模板可用
```

### 6.2 用户安装服务

```
用户点击"安装"
         ↓
平台读取 platform-manifest.yaml
         ↓
检查 permissions.toolNamespace / workloadNamespaces / environmentNamespaces
         ↓
┌─────────────────────────────────────┐
│ toolNamespace.rules                 │
│   → 在工具 namespace 创建 Role       │
└─────────────────────────────────────┘
┌─────────────────────────────────────┐
│ workloadNamespaces.rules            │
│   → 在业务 namespace 创建           │
│     Role + RoleBinding              │
└─────────────────────────────────────┘
┌─────────────────────────────────────┐
│ environmentNamespaces.rules         │
│   → 在同环境非自身 namespace 创建   │
│     Role + RoleBinding              │
└─────────────────────────────────────┘
         ↓
合并 values（preset + 平台变量 + 用户参数）
         ↓
执行 helm template 生成 YAML
         ↓
创建 ServiceInstance CR
         ↓
Operator 监听 CR 并创建 K8s 资源
         ↓
如果有 dashboard_path，调用 Grafana API 导入面板
         ↓
安装完成
```

### 6.3 环境新增 namespace 时

```
用户在环境中新增 namespace-03
         ↓
平台查询该环境已安装的服务
         ↓
发现 CustomPrometheus 声明了 environmentNamespaces.rules
         ↓
平台自动在 namespace-03 创建 Role + RoleBinding
         ↓
Prometheus 立即可以监控新 namespace
```

**关键：** 整个过程不需要重新安装 Helm Chart，权限也绝不溢出。

---

## 7. 完整示例

### 7.1 自定义 Prometheus 模板

#### 目录结构

```
custom-prometheus/
├── chart/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── configmap.yaml
├── dashboards/
│   └── prometheus-overview.json
├── preset-values.yaml
└── platform-manifest.yaml
```

#### platform-manifest.yaml

```yaml
name: "CustomPrometheus"
version: "v2.45.0"
description: "自定义 Prometheus 监控服务，支持环境级动态权限"

permissions:
  clusterResources:
    rules:
      - apiGroups: [""]
        resources: ["nodes"]
        verbs: ["get", "list", "watch"]
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "endpoints", "configmaps", "secrets", "serviceaccounts"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
      - apiGroups: ["apps"]
        resources: ["deployments", "statefulsets", "daemonsets"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  environmentNamespaces:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "endpoints"]
        verbs: ["get", "list", "watch"]
      - apiGroups: ["apps"]
        resources: ["deployments", "statefulsets", "daemonsets"]
        verbs: ["get", "list", "watch"]

observability:
  dashboard_path: "./dashboards/prometheus-overview.json"
  metrics:
    port: 9090
    path: "/metrics"

variable_mapping:
  - platform_var: "env_namespaces"
    helm_var: "prometheus.scrapeNamespaces"
```

#### preset-values.yaml

```yaml
rbac:
  create: false

serviceAccount:
  create: false
  name: ""

server:
  replicaCount: 1
  resources:
    limits:
      cpu: 1000m
      memory: 2Gi
    requests:
      cpu: 200m
      memory: 512Mi
```

#### chart/values.yaml

```yaml
global:
  # 平台注入的变量
  namespace: ""
  serviceAccountName: ""

prometheus:
  # 平台通过 variable_mapping 注入
  scrapeNamespaces: ""

server:
  image:
    repository: prom/prometheus
    tag: v2.45.0
  
  replicaCount: 1
  
  resources:
    limits:
      cpu: 1000m
      memory: 2Gi
    requests:
      cpu: 200m
      memory: 512Mi
```

#### chart/templates/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "prometheus.fullname" . }}
  namespace: {{ .Values.global.namespace }}
  labels:
    {{- include "prometheus.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.server.replicaCount }}
  selector:
    matchLabels:
      {{- include "prometheus.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "prometheus.selectorLabels" . | nindent 8 }}
    spec:
      serviceAccountName: {{ .Values.global.serviceAccountName }}
      containers:
        - name: prometheus
          image: "{{ .Values.server.image.repository }}:{{ .Values.server.image.tag }}"
          ports:
            - name: http
              containerPort: 9090
              protocol: TCP
          resources:
            {{- toYaml .Values.server.resources | nindent 12 }}
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
      volumes:
        - name: config
          configMap:
            name: {{ include "prometheus.fullname" . }}-config
```

#### chart/templates/configmap.yaml

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "prometheus.fullname" . }}-config
  namespace: {{ .Values.global.namespace }}
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    
    scrape_configs:
      # 动态抓取平台注入的 namespace 列表
      {{- $namespaces := splitList "," .Values.prometheus.scrapeNamespaces }}
      {{- range $namespaces }}
      - job_name: 'kubernetes-pods-{{ . }}'
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names:
                - {{ . }}
      {{- end }}
```

### 7.2 打包上传

```bash
# 打包
tar -czf custom-prometheus.tar.gz custom-prometheus/

# 上传到 PAAP 平台
# 通过 UI 或 API 上传 custom-prometheus.tar.gz
```

---

## 8. 常见问题

### 8.1 为什么要禁用 Chart 内置的 RBAC？

**答：** 平台需要统一管理权限，确保：
1. 权限不会溢出到其他环境
2. 环境新增 namespace 时自动同步权限
3. 卸载服务时自动清理所有权限

如果 Chart 自己创建 RBAC，会导致权限管理混乱。

### 8.2 如果我的 Chart 必须创建 CRD 怎么办？

**答：** 有两种方案：

1. **推荐：** 将 CRD 安装作为平台的前置步骤
   - 平台管理员预先安装 CRD（如 Prometheus Operator 的 CRD）
   - 用户的 Chart 只创建 CR（Custom Resource）

2. **备选：** 联系平台管理员，申请将 CRD 加入白名单
   - 平台会在隔离的环境中安装 CRD
   - 需要经过安全审查

### 8.3 如何调试我的模板？

**答：** 使用本地 Helm 测试：

```bash
# 1. 准备测试 values
cat > test-values.yaml <<EOF
global:
  namespace: test-ns
  serviceAccountName: test-sa

prometheus:
  scrapeNamespaces: "ns1,ns2,ns3"
EOF

# 2. 渲染模板
helm template my-release ./chart -f preset-values.yaml -f test-values.yaml

# 3. 检查输出的 YAML 是否符合预期
```

### 8.4 三类权限应该怎么选？

**答：** 根据工具的实际需求：

| 需求 | 权限块 | 示例 |
|------|--------|------|
| 工具安装和运行自身资源 | `toolNamespace` | PostgreSQL, Redis, MinIO |
| 控制业务负载 namespace | `workloadNamespaces` | ArgoCD, Jenkins |
| 观察或操作同环境工具/中间件/数据库 namespace | `environmentNamespaces` | Prometheus, Loki |
| 集群级只读发现 | `clusterResources` | nodes, namespaces, storageclasses |

**判断标准：** 工具要管理哪类 namespace，就只声明对应权限块；没有声明的块不会创建跨 namespace RBAC。

---

## 9. 最佳实践

### 9.1 权限最小化原则

只申请必需的权限，避免使用通配符：

```yaml
# ❌ 不推荐：过于宽泛
permissions:
  rules:
    - apiGroups: ["*"]
      resources: ["*"]
      verbs: ["*"]

# ✅ 推荐：精确指定
permissions:
  rules:
    - apiGroups: [""]
      resources: ["pods", "services"]
      verbs: ["get", "list", "watch"]
```

### 9.2 使用语义化版本

```yaml
# ✅ 推荐
name: "MyTool"
version: "v1.2.3"

# ❌ 不推荐
version: "latest"
version: "1.0"
```

### 9.3 提供清晰的描述

```yaml
# ✅ 推荐
description: "自定义 Prometheus 监控服务，支持环境级动态权限同步，自动抓取所有 namespace 的指标"

# ❌ 不推荐
description: "Prometheus"
```

### 9.4 测试多 namespace 场景

在本地测试时，模拟环境有多个 namespace 的情况：

```bash
# 测试 variable_mapping
helm template my-release ./chart \
  -f preset-values.yaml \
  --set global.namespace=test-ns \
  --set global.serviceAccountName=test-sa \
  --set prometheus.scrapeNamespaces="ns1,ns2,ns3"
```

---

## 10. 零改动转换第三方 Helm Chart

### 10.1 核心原则

**不要修改第三方 Chart 的代码**，而是通过"配置覆盖"和"平台外挂"来实现平台集成。

**为什么？**
- 如果每接入一个第三方工具都要修改 `templates/` 里的 YAML 文件，维护成本会爆炸
- 第三方 Chart 升级时，你的修改会丢失或冲突
- 保持对开源生态的最大兼容性

### 10.2 三个核心技巧

#### 技巧 1：利用 values.yaml 禁用内置 RBAC

绝大多数成熟的第三方 Helm Chart（如 Bitnami 系列、Prometheus 官方等）都提供了标准开关。

**操作方法：** 在 `preset-values.yaml` 中强制设置：

```yaml
# preset-values.yaml
rbac:
  create: false  # 告诉 Helm：别创建 ClusterRole

serviceAccount:
  create: false  # 告诉 Helm：别创建 SA，用平台分配的
  name: ""       # 留空，平台会注入

# 禁用 CRD 安装（CRD 应由集群管理员预先安装）
installCRDs: false

# 禁用内置的 NetworkPolicy
networkPolicy:
  enabled: false
```

**结果：** 第三方 Chart 里的"危险"权限资源不会被安装，权限控制权回到平台手上。

#### 技巧 2：参数映射（variable_mapping）

不需要改 Chart，只需要告诉平台怎么把环境信息"塞"给第三方 Chart。

**示例：转换 Bitnami Redis Chart**

```yaml
# platform-manifest.yaml
name: "Redis"
version: "17.11.3"
description: "Bitnami Redis 缓存服务"

permissions:
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "configmaps", "secrets", "persistentvolumeclaims", "serviceaccounts"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
      - apiGroups: ["apps"]
        resources: ["statefulsets", "replicasets"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# 参数映射：把平台变量翻译给第三方 Chart
variable_mapping:
  # 把平台生成的随机密码传给 Redis
  - platform_var: "generated_password"
    helm_var: "auth.password"
  
  # 把平台分配的 ServiceAccount 传给 Redis
  - platform_var: "service_account_name"
    helm_var: "master.serviceAccount.name"
  
  # 把环境标识传给 Redis
  - platform_var: "current_env_identifier"
    helm_var: "commonLabels.environment"
```

**平台会自动：**
1. 生成随机密码并注入到 `auth.password`
2. 创建 ServiceAccount 并注入到 `master.serviceAccount.name`
3. 将环境标识注入到 `commonLabels.environment`

#### 技巧 3：平台层的资源拦截与清洗（可选）

如果某些 Chart 写得不规范（没有 `rbac.create` 开关），平台后端可以在执行 `helm install` 前做预处理：

**流程：**

```
1. 渲染：helm template → 生成纯 YAML
         ↓
2. 清洗：扫描 YAML，检测违规资源
         ↓
3. 修正：ClusterRoleBinding → RoleBinding
         或直接丢弃违规资源
         ↓
4. 应用：kubectl apply -f cleaned.yaml
```

**实现示例（Go 伪代码）：**

```go
// 渲染 Helm Chart
manifests, err := helmTemplate(chartPath, values)

// 解析 YAML
resources := parseYAML(manifests)

// 清洗资源
cleanedResources := []Resource{}
for _, res := range resources {
    switch res.Kind {
    case "ClusterRole", "ClusterRoleBinding":
        // 拒绝集群级资源
        log.Warn("Rejected cluster-scoped resource: %s", res.Name)
        continue
    case "Role", "RoleBinding":
        // 检查是否在允许的 namespace 内
        if !isAllowedNamespace(res.Namespace) {
            log.Warn("Rejected resource in unauthorized namespace: %s", res.Namespace)
            continue
        }
    }
    cleanedResources = append(cleanedResources, res)
}

// 应用清洗后的资源
applyResources(cleanedResources)
```

**优点：** 100% 保证不会权限溢出，即使 Chart 写得不规范。

### 10.3 转换第三方 Chart 的标准步骤

以"官方 ArgoCD"为例，展示如何转换成平台模板。

#### 步骤 1：下载官方 Chart

```bash
# 下载官方 ArgoCD Helm Chart（不要修改任何文件）
helm repo add argo https://argoproj.github.io/argo-helm
helm pull argo/argo-cd --version 5.46.0 --untar -d ./chart
```

#### 步骤 2：编写 platform-manifest.yaml

```yaml
name: "ArgoCD"
version: "v2.9.3"
description: "GitOps 持续部署工具，支持环境级权限隔离"

# 声明权限需求
permissions:
  toolNamespace:
    rules:
      - apiGroups: ["argoproj.io"]
        resources: ["applications", "appprojects"]
        verbs: ["*"]
  workloadNamespaces:
    rules:
      # 管理应用资源
      - apiGroups: ["", "apps", "batch"]
        resources: ["*"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# 可观测性
observability:
  dashboard_path: "./dashboards/argocd-overview.json"
  metrics:
    port: 8082
    path: "/metrics"

# 变量映射
variable_mapping:
  # 把环境的所有 namespace 传给 ArgoCD
  - platform_var: "env_namespaces"
    helm_var: "configs.params.application.namespaces"
```

#### 步骤 3：编写 preset-values.yaml

```yaml
# preset-values.yaml
# 禁用官方 Chart 的 RBAC 创建
global:
  # 禁用 CRD 安装（由集群管理员预先安装）
  installCRDs: false

# 禁用内置 RBAC
crds:
  install: false

# Server 配置
server:
  # 禁用内置 ServiceAccount
  serviceAccount:
    create: false
    name: ""  # 平台会注入
  
  # 禁用 Ingress（平台统一管理）
  ingress:
    enabled: false
  
  # 开启 insecure 模式（平台内部访问）
  extraArgs:
    - --insecure

# Controller 配置
controller:
  serviceAccount:
    create: false
    name: ""  # 平台会注入

# Repo Server 配置
repoServer:
  serviceAccount:
    create: false
    name: ""  # 平台会注入

# 禁用 Redis HA（简化部署）
redis-ha:
  enabled: false

# 使用单实例 Redis
redis:
  enabled: true
```

#### 步骤 4：打包上传

```bash
# 目录结构
argocd-template/
├── chart/                      # 官方 ArgoCD Chart（未修改）
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── dashboards/
│   └── argocd-overview.json
├── preset-values.yaml          # 禁用内置 RBAC
└── platform-manifest.yaml      # 平台元数据

# 打包
tar -czf argocd-template.tar.gz argocd-template/

# 上传到 PAAP 平台
```

#### 步骤 5：平台处理流程

```
用户点击"安装 ArgoCD"
         ↓
平台读取 platform-manifest.yaml
         ↓
平台发现 workloadNamespaces.rules
         ↓
平台在业务负载 namespace 创建 Role + RoleBinding
         ↓
平台合并 values：
  1. chart/values.yaml（官方默认值）
  2. preset-values.yaml（禁用 RBAC）
  3. 平台注入的变量（ServiceAccount 名称、namespace 列表）
  4. 用户在 UI 填写的参数
         ↓
平台执行 helm install
         ↓
Operator 监听 ServiceInstance CR 并创建资源
         ↓
平台调用 Grafana API 导入面板
         ↓
安装完成
```

### 10.4 最终效果

**对于用户：**
- 点击安装，ArgoCD 跑起来了
- 权限刚好能管当前环境的所有 namespace
- 不能访问其他环境的资源
- Grafana 面板自动导入

**对于平台开发者：**
- 没有修改 ArgoCD 的任何一行代码
- 官方升级时，只需替换 `chart/` 目录
- `platform-manifest.yaml` 和 `preset-values.yaml` 几乎不需要改

### 10.5 常见第三方 Chart 的转换要点

#### Bitnami PostgreSQL

```yaml
# preset-values.yaml
auth:
  enablePostgresUser: true
  postgresPassword: ""  # 平台会注入

primary:
  serviceAccount:
    create: false
    name: ""

rbac:
  create: false

networkPolicy:
  enabled: false
```

#### Bitnami Redis

```yaml
# preset-values.yaml
auth:
  enabled: true
  password: ""  # 平台会注入

master:
  serviceAccount:
    create: false
    name: ""

replica:
  serviceAccount:
    create: false
    name: ""

rbac:
  create: false

networkPolicy:
  enabled: false
```

#### Prometheus (kube-prometheus-stack)

```yaml
# preset-values.yaml
prometheus:
  serviceAccount:
    create: false
    name: ""
  
  prometheusSpec:
    # 限制抓取的 namespace
    serviceMonitorNamespaceSelector:
      matchLabels:
        paap.io/environment: "{{ .Values.global.env }}"

grafana:
  enabled: false  # 平台统一提供 Grafana

alertmanager:
  enabled: false  # 简化部署

# 禁用 CRD 安装
crds:
  enabled: false
```

#### Tekton Pipelines

```yaml
# preset-values.yaml
rbac:
  create: false

serviceAccount:
  create: false
  name: ""

# 禁用 Webhook（平台环境可能不需要）
webhook:
  enabled: false
```

### 10.6 转换检查清单

在转换第三方 Chart 时，请确认：

- [ ] **未修改** `chart/` 目录下的任何文件
- [ ] `preset-values.yaml` 禁用了 `rbac.create`
- [ ] `preset-values.yaml` 禁用了 `serviceAccount.create`
- [ ] `preset-values.yaml` 禁用了 `installCRDs`（如果有）
- [ ] `platform-manifest.yaml` 正确声明了三类 namespace 权限
- [ ] `platform-manifest.yaml` 的所有 `rules` 遵循最小权限原则
- [ ] 使用 `variable_mapping` 将平台变量传递给 Chart
- [ ] 本地测试：`helm template` 渲染成功，无集群级资源

### 10.7 升级第三方 Chart

当官方 Chart 发布新版本时：

```bash
# 1. 下载新版本
helm pull argo/argo-cd --version 5.50.0 --untar -d ./chart-new

# 2. 替换 chart/ 目录
rm -rf argocd-template/chart
mv chart-new/argo-cd argocd-template/chart

# 3. 检查 preset-values.yaml 是否需要更新
# 查看新版本的 values.yaml，确认开关名称是否变化
diff chart-old/values.yaml chart-new/values.yaml

# 4. 本地测试
helm template test ./argocd-template/chart \
  -f argocd-template/preset-values.yaml \
  --set global.namespace=test-ns \
  --set global.serviceAccountName=test-sa

# 5. 重新打包上传
tar -czf argocd-template-v5.50.0.tar.gz argocd-template/
```

**关键：** 因为你没有修改 Chart 代码，升级只需要替换目录，风险极低。

---

## 11. 参考资料

- [Helm Chart 开发指南](https://helm.sh/docs/chart_template_guide/)
- [Kubernetes RBAC 文档](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [PAAP 服务模板规范](./service-template-spec.md)
- [PAAP Operator 设计](./operator-design.md)
- [Bitnami Helm Charts](https://github.com/bitnami/charts)
- [Artifact Hub](https://artifacthub.io/) - 查找第三方 Helm Charts

---

## 附录：Go 数据模型

平台使用以下 Go 结构体解析 `platform-manifest.yaml`：

```go
// PlatformManifest 是用户在自定义 Helm Chart 上传时包含的元数据文件
type PlatformManifest struct {
    Name        string              `yaml:"name"`
    Version     string              `yaml:"version"`
    Description string              `yaml:"description,omitempty"`
    Permissions PermissionsSpec     `yaml:"permissions"`
    Observability *ObservabilitySpec `yaml:"observability,omitempty"`
    VariableMapping []VariableMappingEntry `yaml:"variable_mapping,omitempty"`
}

// PermissionsSpec 声明工具需要的 RBAC 权限
type PermissionsSpec struct {
    ClusterResources ClusterResourcePermissionsSpec `yaml:"clusterResources,omitempty"`
    ToolNamespace NamespacePermissionsSpec `yaml:"toolNamespace,omitempty"`
    WorkloadNamespaces NamespacePermissionsSpec `yaml:"workloadNamespaces,omitempty"`
    EnvironmentNamespaces NamespacePermissionsSpec `yaml:"environmentNamespaces,omitempty"`
}

// PolicyRuleSpec 对应 K8s rbac.PolicyRule
type PolicyRuleSpec struct {
    APIGroups []string `yaml:"apiGroups"`
    Resources []string `yaml:"resources"`
    Verbs     []string `yaml:"verbs"`
}

// ObservabilitySpec 声明如何集成监控
type ObservabilitySpec struct {
    DashboardPath string       `yaml:"dashboard_path,omitempty"`
    Metrics       *MetricsSpec `yaml:"metrics,omitempty"`
}

// MetricsSpec 描述如何抓取 Prometheus 指标
type MetricsSpec struct {
    Port int    `yaml:"port"`
    Path string `yaml:"path"`
}

// VariableMappingEntry 映射平台变量到 Helm values
type VariableMappingEntry struct {
    PlatformVar string `yaml:"platform_var"`
    HelmVar     string `yaml:"helm_var"`
}
```
