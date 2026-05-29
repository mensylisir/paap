# 服务实例权限隔离设计方案

> 本文档描述权限隔离的设计方案。具体实现由 PAAP Operator 负责，详见 [Operator 设计方案](operator-design.md)。

## 一、问题背景

PAAP 平台为每个环境部署独立的 ArgoCD、Tekton/Jenkins、Prometheus 等工具实例。这些工具默认安装时拥有 `cluster-admin` 权限，可以操作集群中任意资源，存在安全风险。

**核心要求：每个环境的工具实例只能访问和服务自己环境内的资源（自己的 namespace），没有权限访问其他资源。**

**关键设计：每个环境都有自己的 ArgoCD、Jenkins、Prometheus，即使在同一集群上。不同环境的工具链可以不同（如开发环境有CI+CD，生产环境只有CD）。**

---

## 二、核心概念

### 名称与标识

所有资源都有**名称**（展示用，可中文）和**标识**（K8s 资源用，纯英文小写+连字符）：

| 资源 | 名称（展示） | 标识（K8s） | 说明 |
|------|-------------|-------------|------|
| 应用 | 订单服务 | order-service | 用于 namespace 前缀 |
| 环境 | 开发环境 | dev | 用于 namespace 后缀 |
| 工具 | 部署服务 | argocd | 用于 SA/Role 命名 |
| 工具 | CI服务 | tekton | 用于 SA/Role 命名 |
| 工具 | 监控服务 | prometheus | 用于 SA/Role 命名 |
| 组件 | 前端服务 | frontend | 用于 Deployment 命名 |
| 基础设施 | 缓存数据库 | redis | 用于 StatefulSet 命名 |

**规则：**
- 标识只能包含小写字母、数字和连字符，不能以连字符开头或结尾
- 标识在同类型资源内唯一（如两个应用不能都叫 `order-service`）
- K8s 资源名全部使用标识，UI 展示全部使用名称

### 环境 ≠ Namespace

| 概念 | 定义 | 说明 |
|------|------|------|
| **应用** | 业务应用，如订单服务 | 包含多个环境 |
| **环境** | 应用的一个运行实例 | = {主 namespace, ...可选附加 namespaces} + {一组独立工具实例} |
| **Namespace** | K8s 资源隔离单位 | 环境内部的一个资源分组 |

**一个应用可以有多个环境，每个环境有自己独立的 namespace 和工具实例。**

### 环境内 Namespace 的三种用途

一个环境可以包含多个 namespace，按用途分为三类：

| 类型 | 说明 | 示例 | 谁往里部署 |
|------|------|------|-----------|
| **主空间** | 放工具实例 + SA + 部分组件 | order-service-dev | 工具自身 + 用户手动创建组件 |
| **工作负载空间** | 放业务组件，由 ArgoCD 部署 | order-service-dev-app | ArgoCD 自动部署 |
| **专用空间** | 放特定类型基础设施 | order-service-dev-db, order-service-dev-cache | 用户手动或工具自动 |

**所有工具的 RBAC 必须覆盖该环境的全部 namespace**（无论哪种类型）：
- ArgoCD → 可以往任何环境内 namespace 部署 Deployment/Service 等
- Prometheus → 可以采集任何环境内 namespace 的 Pod/Service 指标
- Tekton → 可以在任何环境内 namespace 创建 Job/Pod
- 日志工具 → 可以收集任何环境内 namespace 的日志

**权限绝不外溢**：工具的 RoleBinding 只存在于本环境的 namespace 中，不能跨环境。

### 旧设计 vs 新设计

| 维度 | 旧设计（每应用一套工具） | 新设计（每环境一套工具） |
|------|--------------------------|--------------------------|
| ArgoCD 数量 | 每应用1个，管理所有环境 | 每个环境1个，互相隔离 |
| 权限范围 | 跨环境（dev/staging/prod） | 只在当前环境内 |
| 新增环境 | 给已有 ArgoCD 加新 ns 权限 | 新环境装新的 ArgoCD |
| 安全性 | 隔离了不同应用 | 还隔离了不同环境 |
| 资源消耗 | 低（一套ArgoCD） | 高（多套ArgoCD） |
| 工具链灵活性 | 所有环境相同 | 每个环境可以不同 |

---

## 三、整体架构

### 命名空间规划

每个应用在每个环境中拥有独立的 namespace。环境名由用户自定义，不限于 dev/staging/prod。

```
主空间命名规则:  {应用标识}-{环境标识}
附加空间命名规则: {应用标识}-{环境标识}-{用途}

示例：
  order-service-dev                  （订单服务 - 开发环境 - 主空间）
  order-service-dev-app              （订单服务 - 开发环境 - 工作负载）
  order-service-dev-db               （订单服务 - 开发环境 - 数据库）
  order-service-dev-cache            （订单服务 - 开发环境 - 缓存）
  order-service-staging              （订单服务 - 测试环境 - 主空间）
  order-service-staging-app          （订单服务 - 测试环境 - 工作负载）
  order-service-prod                 （订单服务 - 生产环境 - 主空间）
  user-center-dev                    （用户中心 - 开发环境 - 主空间）
```

### 工具部署模式

**每个环境独立一套工具实例**，部署在环境的主 namespace 中，通过 RBAC 授权访问该环境的所有 namespace（主空间 + 工作负载空间 + 专用空间）。不同环境的工具链可以不同。

```
应用「订单服务」（标识: order-service）
│
├── 环境「开发环境」（标识: dev）
│   ├── 主空间: order-service-dev
│   │   ├── ArgoCD 实例 (order-service-dev-argocd)
│   │   ├── Tekton 实例 (order-service-dev-tekton)
│   │   ├── Prometheus 实例 (order-service-dev-prometheus)
│   │   ├── Grafana 实例 (order-service-dev-grafana)
│   │   ├── 业务组件 (frontend, backend)
│   │   └── 基础设施 (PostgreSQL, Redis, RabbitMQ)
│   │
│   ├── 工作负载空间: order-service-dev-app
│   │   └── 业务组件（由 ArgoCD 自动部署）
│   │
│   └── 专用空间: order-service-dev-db, order-service-dev-cache
│       └── 数据库/缓存组件
│   │
│   └── RBAC: 所有工具 SA 覆盖 dev, dev-app, dev-db, dev-cache
│       ├── ArgoCD → 全部读写（部署/更新/删除）
│       ├── Prometheus → 全部只读（采集指标）
│       ├── Tekton → 全部读写（构建/部署）
│       └── 日志工具 → 全部只读（收集日志）
│
├── 环境「测试环境」（标识: staging）
│   ├── 主空间: order-service-staging
│   │   ├── ArgoCD 实例 (order-service-staging-argocd)   ← 另一套！
│   │   ├── Tekton 实例 (order-service-staging-tekton)   ← 另一套！
│   │   ├── Prometheus 实例 (order-service-staging-prom)  ← 另一套！
│   │   └── 业务组件 (frontend, backend)
│   │
│   └── 工作负载空间: order-service-staging-app
│       └── 业务组件（由 ArgoCD 自动部署）
│   │
│   └── RBAC: 所有工具 SA 覆盖 staging, staging-app
│
└── 环境「生产环境」（标识: prod）
    ├── 主空间: order-service-prod
    │   ├── ArgoCD 实例 (order-service-prod-argocd)      ← 另一套！
    │   ├── Prometheus 实例 (order-service-prod-prom)      ← 另一套！
    │   ├── 业务组件 (frontend, backend)
    │   └── 注意：生产环境没有 CI，只有 CD 和监控
    │
    └── 工作负载空间: order-service-prod-app
        └── 业务组件（由 ArgoCD 自动部署）
    │
    └── RBAC: 所有工具 SA 覆盖 prod, prod-app
```

**关键变化：**
- 开发环境的 ArgoCD (`order-service-dev-argocd`) 完全碰不到测试环境的任何资源
- 测试环境的 ArgoCD (`order-service-staging-argocd`) 是另一套独立的实例
- 生产环境可以只有 ArgoCD + Prometheus，没有 Tekton（不同环境工具链不同）

**为什么每环境独立一套？**
- 最强隔离：环境 A 的工具完全碰不到环境 B 的资源
- 工具链灵活：开发环境有CI+CD，生产环境可以只有CD
- 故障隔离：一个环境的工具出问题不影响其他环境
- 安全合规：生产环境的工具权限最小化

---

## 四、ArgoCD 权限隔离

### 4.1 部署方式

每个环境的 ArgoCD 实例部署在该环境的主 namespace 中，**不使用 cluster-admin**。

```bash
# 使用 namespace-scoped 安装
# 不创建 ClusterRole/ClusterRoleBinding
kubectl apply -n order-service-dev \
  -f argocd-namespace-install.yaml
```

### 4.2 RBAC 配置

**ArgoCD SA 在环境主 namespace 拥有完整权限：**

```yaml
# 环境主 namespace: order-service-dev
apiVersion: v1
kind: ServiceAccount
metadata:
  name: order-service-dev-argocd
  namespace: order-service-dev
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-manager
  namespace: order-service-dev
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
    resources: ["*"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: argocd-manager
  namespace: order-service-dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: argocd-manager
subjects:
  - kind: ServiceAccount
    name: order-service-dev-argocd
    namespace: order-service-dev
```

**ArgoCD SA 在环境其他 namespace 也拥有完整权限：**

```yaml
# 环境内附加 namespace: order-service-dev-cache
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-manager
  namespace: order-service-dev-cache
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
    resources: ["*"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: argocd-manager
  namespace: order-service-dev-cache
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: argocd-manager
subjects:
  - kind: ServiceAccount
    name: order-service-dev-argocd
    namespace: order-service-dev   # 引用环境主 namespace 的 SA
```

### 4.3 隔离效果

```
order-service-dev-argocd SA 的权限范围:
  ✓ order-service-dev           (环境主 namespace, 完整权限)
  ✓ order-service-dev-cache     (环境附加 namespace, 完整权限)
  ✗ order-service-staging       (无权限, 其他环境)
  ✗ order-service-prod          (无权限, 其他环境)
  ✗ user-center-dev             (无权限, 其他应用)
  ✗ kube-system                 (无权限, 系统 namespace)
```

---

## 五、Tekton/Jenkins 权限隔离

### 5.1 部署方式

每个环境的 Tekton 实例（或 Jenkins）部署在该环境的主 namespace 中。生产环境可以不安装 CI 工具。

### 5.2 RBAC 配置

```yaml
# CI 流水线使用的 SA
apiVersion: v1
kind: ServiceAccount
metadata:
  name: order-service-dev-tekton
  namespace: order-service-dev
---
# CI SA 在环境主 namespace 的权限
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-pipeline-role
  namespace: order-service-dev
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "services", "configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "create", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ci-pipeline-role-binding
  namespace: order-service-dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ci-pipeline-role
subjects:
  - kind: ServiceAccount
    name: order-service-dev-tekton
    namespace: order-service-dev
```

**环境附加 namespace（如 CI 流水线部署到缓存空间）：**

```yaml
# 在环境附加 namespace 创建 RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-pipeline-role
  namespace: order-service-dev-cache
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "services", "configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "create", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ci-pipeline-role-binding
  namespace: order-service-dev-cache
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ci-pipeline-role
subjects:
  - kind: ServiceAccount
    name: order-service-dev-tekton
    namespace: order-service-dev   # 引用环境主 namespace 的 SA
```

---

## 六、Prometheus 权限隔离

### 6.1 部署方式

每个环境的 Prometheus + Grafana 实例部署在该环境的主 namespace 中。

### 6.2 RBAC 配置

```yaml
# Prometheus SA — 只读权限
apiVersion: v1
kind: ServiceAccount
metadata:
  name: order-service-dev-prometheus
  namespace: order-service-dev
---
# 在环境主 namespace 的只读权限
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-reader
  namespace: order-service-dev
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
    # 注：nodes 是集群级资源，namespaced Role 无法授权
    # 如需 node 指标，由 Prometheus 自身的 ClusterRole 统一提供，不由环境 SA 管理
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prometheus-reader-binding
  namespace: order-service-dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prometheus-reader
subjects:
  - kind: ServiceAccount
    name: order-service-dev-prometheus
    namespace: order-service-dev
```

**在环境附加 namespace 的只读权限：**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-reader
  namespace: order-service-dev-cache
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prometheus-reader-binding
  namespace: order-service-dev-cache
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prometheus-reader
subjects:
  - kind: ServiceAccount
    name: order-service-dev-prometheus
    namespace: order-service-dev
```

### 6.3 采集配置

Prometheus 的 `kubernetes_sd_configs` 只发现该环境的**所有** namespace（主空间 + 工作负载空间 + 专用空间）：

```yaml
scrape_configs:
  - job_name: 'order-service-dev-pods'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - order-service-dev          # 主空间
            - order-service-dev-app      # 工作负载空间
            - order-service-dev-db       # 数据库空间
            - order-service-dev-cache    # 缓存空间
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

Grafana 数据源只指向自己的 Prometheus：

```yaml
datasources:
  - name: Prometheus
    type: prometheus
    url: http://order-service-dev-prometheus:9090
    access: proxy
```

### 6.4 监控与日志默认配置

**监控默认启用**：工具安装后，Prometheus 自动采集该环境所有 namespace 的指标，Grafana 预置仪表盘，无需额外配置即可查看：
- 容器资源使用（CPU/内存/磁盘）
- Pod 状态与重启次数
- Service/Endpoint 健康状态
- 网络流量

**日志默认启用**：日志工具安装后，自动收集该环境所有 namespace 的容器日志，支持按 namespace/Pod/容器筛选。

**权限不外溢**：监控和日志的采集范围严格限定在该环境的 namespace 列表内，新增 namespace 时自动扩展采集范围，删除时自动缩窄。

---

## 七、SA 命名规则

### 7.1 统一命名规则

```
SA 名称: {应用标识}-{环境标识}-{工具类型}

示例:
  order-service-dev-argocd          (订单服务-开发环境-ArgoCD)
  order-service-dev-tekton          (订单服务-开发环境-Tekton)
  order-service-dev-prometheus      (订单服务-开发环境-Prometheus)
  order-service-staging-argocd      (订单服务-测试环境-ArgoCD)
  order-service-staging-tekton      (订单服务-测试环境-Tekton)
  order-service-prod-argocd         (订单服务-生产环境-ArgoCD)
  order-service-prod-prometheus     (订单服务-生产环境-Prometheus)
```

### 7.2 SA 放置位置

SA 统一放在环境的主 namespace（primaryNamespace）中：

```
环境「开发环境」
├── order-service-dev (主 namespace)  ← SA 放这里
│   ├── SA: order-service-dev-argocd
│   ├── SA: order-service-dev-tekton
│   ├── SA: order-service-dev-prometheus
│   └── RoleBinding → 绑定各 SA
│
├── order-service-dev-cache (附加 namespace)
│   └── RoleBinding → 引用 order-service-dev 的 SA
│
└── order-service-dev-db (附加 namespace)
    └── RoleBinding → 引用 order-service-dev 的 SA
```

---

## 八、环境新增 namespace 时的权限自动分配

### 8.1 触发时机

当用户为环境新增 namespace 时，平台自动执行权限分配。

### 8.2 自动执行流程

```
用户为「开发环境」新增 namespace: "order-service-dev-db"
  ↓
1. 创建 namespace: order-service-dev-db
   打标签: paap.io/app=order-service, paap.io/env=dev
   ↓
2. 检查该环境已安装了哪些工具
   → 发现: ArgoCD, Tekton, Prometheus
   ↓
3. 为每个工具在新 namespace 创建 Role + RoleBinding:

   ArgoCD:
   - Role: argocd-manager (namespace: order-service-dev-db)
   - RoleBinding → SA: order-service-dev-argocd (namespace: order-service-dev)
   → ArgoCD 现在可以部署到 dev-db

   Tekton:
   - Role: ci-pipeline-role (namespace: order-service-dev-db)
   - RoleBinding → SA: order-service-dev-tekton (namespace: order-service-dev)
   → CI 流水线现在可以操作 dev-db

   Prometheus:
   - Role: prometheus-reader (namespace: order-service-dev-db)
   - RoleBinding → SA: order-service-dev-prometheus (namespace: order-service-dev)
   - 更新 Prometheus 采集配置，加入 order-service-dev-db
   → 监控服务现在可以采集 dev-db 的指标
   ↓
4. 创建 ResourceQuota（资源配额）
   ↓
5. 创建 NetworkPolicy（可选）
   ↓
6. 更新工具配置（如 ArgoCD AppProject destinations）
   ↓
7. 新 namespace 就绪
```

### 8.3 删除环境 namespace 时的清理

```
用户删除「开发环境」的 namespace "order-service-dev-db"
  ↓
1. 对该环境每个已安装工具:
   - 清理 order-service-dev-db 中的 Role + RoleBinding
   - 更新工具配置（如移除 ArgoCD AppProject destination）
   ↓
2. 更新 Prometheus 采集配置，移除 order-service-dev-db
   ↓
3. 删除 namespace: order-service-dev-db
   → K8s 自动清理该 namespace 下所有剩余资源
   ↓
4. 完成
```

---

## 九、安装工具时的权限分配

### 9.1 场景

用户在已有环境中安装新工具（如在开发环境中安装监控服务）。

### 9.2 自动执行流程

```
用户在「开发环境」中点击 [安装监控服务]
  ↓
1. 在环境主 namespace (order-service-dev) 部署 Prometheus + Grafana
   ↓
2. 创建 SA: order-service-dev-prometheus
   namespace: order-service-dev
   ↓
3. 在环境主 namespace 创建 Role + RoleBinding (只读)
   ↓
4. 检查该环境是否有其他附加 namespace
   → 发现: order-service-dev-cache
   ↓
5. 在每个附加 namespace 创建 Role + RoleBinding (只读)
   ↓
6. 配置 Prometheus 采集该环境所有 namespace 的指标
   ↓
7. 配置 Grafana 数据源指向自己的 Prometheus
   ↓
8. 安装完成，监控服务变为可用
```

---

## 十、创建新环境时的流程

### 10.1 场景：应用已有开发环境，用户创建测试环境

```
应用「订单服务」已有:
  - 开发环境 (order-service-dev) — 已装 ArgoCD + Tekton + Prometheus

用户创建新环境: 测试环境 (order-service-staging)
```

### 10.2 平台自动执行的流程

```
1. 创建 namespace: order-service-staging
   打标签: paap.io/app=order-service, paap.io/env=staging
   ↓
2. 如果从模板创建 → 根据模板安装工具:
   - 模板定义: [deploy, ci, monitor]
   - 为每个工具执行 install 流程:
     a. 创建 Deployment/Service/ConfigMap
     b. 创建 SA: order-service-staging-{type}
     c. 在主 namespace 创建 Role + RoleBinding
   ↓
3. 如果是空环境 → 不安装任何工具
   ↓
4. 创建 ResourceQuota（资源配额）
   ↓
5. 新环境就绪

注意：新环境的工具是全新独立的实例，与开发环境完全隔离。
```

### 10.3 如果新环境选择「空环境」

```
用户创建空环境: 预发布环境 (order-service-pre)
  ↓
1. 创建 namespace: order-service-pre
   ↓
2. 不安装任何工具
   ↓
3. 环境页面展示为「空画布」
   → 用户可以按需装工具、建组件
```

---

## 十一、安全边界总结

### 11.1 工具实例的权限范围

```
开发环境的工具只能操作:
  ✓ order-service-dev           (主空间)
  ✓ order-service-dev-app       (工作负载空间)
  ✓ order-service-dev-db        (专用空间)
  ✓ order-service-dev-cache     (专用空间)
  ✗ order-service-staging       (其他环境)
  ✗ order-service-prod          (其他环境)
  ✗ user-center-dev             (其他应用)
  ✗ kube-system                 (系统 namespace)

测试环境的工具只能操作:
  ✓ order-service-staging       (主空间)
  ✓ order-service-staging-app   (工作负载空间)
  ✗ order-service-dev           (其他环境)
  ✗ order-service-prod          (其他环境)
  ✗ user-center-dev             (其他应用)
```

### 11.2 各工具的权限粒度

| 工具 | 权限级别 | 说明 |
|------|----------|------|
| ArgoCD | 完整权限 (verbs: *) | 可以部署/更新/删除 Deployment, Service, ConfigMap 等 |
| Tekton | 有限权限 | 可以创建 Pod, Job, 部署 Deployment，但不能修改 RBAC |
| Prometheus | 只读权限 | 只能读取 Pod, Service, Endpoint 信息 |

### 11.3 纵深防御

```
┌─────────────────────────────────────────────────────┐
│  第1层: 每环境独立工具实例                            │
│  环境 A 的 ArgoCD 和环境 B 的 ArgoCD 完全独立        │
│  物理隔离，互不影响                                   │
├─────────────────────────────────────────────────────┤
│  第2层: Namespace 隔离                               │
│  每个环境独立 namespace，资源天然隔离                 │
├─────────────────────────────────────────────────────┤
│  第3层: RBAC - Role (非 ClusterRole)                 │
│  所有工具的 SA 只有 Role + RoleBinding               │
│  权限限定在自己环境的 namespace 内                    │
├─────────────────────────────────────────────────────┤
│  第4层: 工具自身配置                                  │
│  ArgoCD: AppProject destinations 限定目标 namespace   │
│  Prometheus: kubernetes_sd_configs 只发现自己的 ns    │
│  Tekton: serviceAccountName 使用限定 SA              │
├─────────────────────────────────────────────────────┤
│  第5层: NetworkPolicy (可选)                          │
│  限制 namespace 间的网络通信                          │
└─────────────────────────────────────────────────────┘
```

---

## 十二、IP Pool 绑定评估

### 12.1 是否需要为每个环境绑定 IP Pool？

| 维度 | 分析 |
|------|------|
| **网络隔离** | 不同环境需要网络隔离，IP Pool 可配合 NetworkPolicy 实现 L3 隔离 |
| **外部访问** | 组件需要 LoadBalancer/Ingress 时，绑定 IP Pool 可控制出口 IP |
| **合规要求** | 生产环境可能要求固定 IP 段，用于防火墙白名单 |
| **实施复杂度** | 需要 CNI 插件支持（Calico IPPool / MetalLB AddressPool / 云厂商 VPC） |
| **资源消耗** | 每个环境占用独立 IP 段，IP 地址空间消耗较大 |

### 12.2 结论：可选功能，不强制

**推荐方案：**
- **默认**：使用 NetworkPolicy 实现环境间网络隔离（不依赖 CNI 插件）
- **高级**：在环境模板中可选配置 IP Pool（需要集群支持）

**IP Pool 配置示例（环境模板高级选项）：**

```yaml
# 环境模板中的网络配置（可选）
network:
  # 默认使用 NetworkPolicy 隔离
  isolation: NetworkPolicy

  # 高级选项：绑定 IP Pool（需要 Calico/MetalLB 支持）
  ipPool:
    enabled: false
    cidr: "10.244.0.0/24"      # 该环境的 IP 段
    # Calico IPPool 配置
    calico:
      blockSize: 26
      natOutgoing: true
      disabled: false
    # MetalLB AddressPool 配置
    metallb:
      addresses:
        - "192.168.1.100-192.168.1.110"
```

**按环境差异化配置：**
- 开发环境：不需要 IP Pool，NetworkPolicy 足够
- 测试环境：不需要 IP Pool
- 生产环境：可选绑定 IP Pool，满足防火墙白名单/合规要求

### 12.3 NetworkPolicy 默认配置

即使不使用 IP Pool，每个环境也应默认创建 NetworkPolicy：

```yaml
# 默认策略：允许同环境通信 + 外部访问，拒绝跨环境通信
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-cross-env
  namespace: order-service-dev
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        # 允许同环境 namespace 的流量
        - namespaceSelector:
            matchLabels:
              paap.io/app: order-service
              paap.io/env: dev
        # 允许 Ingress Controller 所在 namespace 的流量
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
        # 允许 Prometheus 采集（如果监控服务在独立 namespace）
        - namespaceSelector:
            matchLabels:
              paap.io/app: order-service
              paap.io/env: dev
  egress:
    - to:
        # 允许同环境 namespace 的流量
        - namespaceSelector:
            matchLabels:
              paap.io/app: order-service
              paap.io/env: dev
    - to:
        # 允许 DNS 查询
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    - to:
        # 允许外部流量（镜像仓库、外部 API、云服务等）
        # 不限制目标 IP，仅限制跨环境 namespace
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              # 排除集群内部 Pod 网段（防止绕过 namespace 隔离）
              # 具体值根据集群配置调整
              - 10.244.0.0/16
```

**注意：**
- `except` 中的集群 Pod CIDR 需要根据实际集群配置调整
- 如果需要访问集群内部服务（如内部镜像仓库），需要额外添加 egress 规则
- Ingress Controller 的 namespace label 需要根据实际部署调整

---

## 十三、对用户界面的影响

用户在 PAAP 平台上操作时，**完全不需要感知这些隔离机制**：

| 用户操作 | 后台实际执行 |
|----------|-------------|
| 创建环境 | 创建 namespace + 根据模板安装工具（或空环境） |
| 安装工具（如 ArgoCD） | 部署工具实例 + 创建 SA + 在环境所有 namespace 创建 Role/RoleBinding |
| 新增环境 namespace | 创建 namespace + 为所有已安装工具分配权限 |
| 删除环境 namespace | 清理工具权限 + 删除 namespace |
| 安装基础设施（如 PostgreSQL） | 部署数据库实例 + 创建 SA + 配置连接信息 |
| 创建组件 | 创建 Deployment + Service |
| 右键配置部署 | 通过 ArgoCD 管理组件（权限已就绪） |
| 右键配置监控 | 通过 Prometheus 采集指标（权限已就绪） |
| 删除环境 | 删除 namespace（自动清理所有资源） |

用户看到的是「选择服务 → 选择环境 → 点确定」，背后的安全隔离完全自动化。
