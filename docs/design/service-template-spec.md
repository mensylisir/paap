# PAAP 服务模板规范（ServiceTemplate Spec）

> **存储机制：** ServiceTemplate 和 EnvTemplate **不是 CRD**，存储在 PostgreSQL 数据库中。
> PAAP Server 负责模板的 CRUD 和渲染，渲染结果写入 CR（Environment/ServiceInstance/Component），
> Operator 负责管理 CR 对应的 K8s 资源。详见 [技术选型](tech-stack.md)。

## 1. 术语定义

| 术语 | 定义 | 关系 |
|------|------|------|
| **应用（Application）** | 业务应用，如订单服务 | 包含多个环境 |
| **环境（Environment）** | 应用的一个运行实例，如开发环境、测试环境 | 包含多个 namespace + 一套独立工具实例 |
| **Namespace** | K8s 资源隔离单位 | 环境内部的一个资源分组 |
| **服务模板（ServiceTemplate）** | 定义一个工具/基础设施如何部署、需要什么权限、怎么动态扩展 | 被环境模板引用，在环境内实例化 |
| **环境模板（EnvTemplate）** | 定义一个环境标配哪些服务 | 编排层，引用服务模板的 type |
| **服务实例（ServiceInstance）** | 某个环境内实际安装的服务 | 由服务模板渲染 + Operator 管理 |

**重要：环境 ≠ Namespace。一个环境 = {主namespace, ...可选附加namespaces} + {一组独立工具实例}。**

---

## 2. 服务模板（ServiceTemplate）

### 2.1 数据模型

```yaml
# ServiceTemplate 不是 CRD，存储在 PAAP Server 的数据库中
# 使用 YAML 格式定义，PAAP Server 负责解析和渲染
kind: ServiceTemplate
spec:
  type: deploy              # 唯一标识名，被环境模板引用
  name: 部署服务             # 对外展示名（可中文）
  category: tool            # tool | infra | middleware
spec:
  parameters:               # UI表单参数定义
    - key: version
      label: 版本
      type: select
      options: ["v2.9", "v2.10"]
      default: "v2.10"

  rbac:                       # 权限模型声明
    serviceAccountName: "{{ .appIdentifier }}-{{ .envIdentifier }}-{{ .type }}"
    serviceAccountNamespace: "{{ .primaryNamespace }}"

    # 工具Deployment需要的权限
    deploymentRole:
      rules:
        - apiGroups: ["argoproj.io"]
          resources: ["applications", "appprojects"]
          verbs: ["*"]

    # 每个环境namespace需要被授予的权限
    envRole:
      rules:
        - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
          resources: ["*"]
          verbs: ["*"]

  lifecycle:                  # 生命周期合约
    install:
      template: | ...        # 首次渲染安装

    onEnvNsAdded:             # 环境新增namespace时触发
      template: | ...

    onEnvNsRemoved:           # 环境删除namespace时触发
      template: | ...

    uninstall:
      template: | ...        # 卸载服务
```

### 2.2 生命周期合约详解

| 事件 | 触发时机 | 模板职责 |
|------|---------|---------|
| `install` | 用户在环境内安装此服务 | 创建工具组件（Deployment/Service/ConfigMap...） |
| `onEnvNsAdded` | 环境新增 namespace | 在新 namespace 创建 Role + RoleBinding |
| `onEnvNsRemoved` | 环境删除 namespace | 在被删 namespace 清理 Role + RoleBinding |
| `uninstall` | 用户卸载此服务 | 清理所有 namespace 的 Role/RoleBinding + 删工具组件 |

**安装时流程（平台调和器执行）：**
1. 渲染 `install` 模板 → 创建工具组件（Deployment/Service...），部署到 `primaryNamespace`
2. 在 `primaryNamespace` 创建 ServiceAccount（命名: `{appIdentifier}-{envIdentifier}-{type}`）+ Role + RoleBinding
3. 遍历该环境所有附加 namespace → 对每个 namespace 渲染 `onEnvNsAdded` → 创建 Role + RoleBinding（引用主 SA）

**环境新增 namespace 时：**
1. 遍历该环境已安装的所有服务
2. 读每个服务的 ServiceTemplate
3. 渲染 `onEnvNsAdded`（传入 `.newNamespace`）
4. 在新 namespace 创建 Role + RoleBinding（引用主 SA）

### 2.3 SA 和 RoleBinding 模型

**单一SA原则**：每个服务实例只有一个 SA，放在环境的 `primaryNamespace` 里。

```
环境「开发环境」
├── primaryNamespace: order-service-dev   ← SA 存在这里
│   ├── SA: order-service-dev-argocd
│   └── ArgoCD Deployment + Service...
│
├── namespace: order-service-dev-cache
│   └── RoleBinding → order-service-dev-argocd (SA 在 dev)
│
└── namespace: order-service-dev-db
    └── RoleBinding → order-service-dev-argocd (SA 在 dev)
```

**跨环境完全隔离**：`order-service-dev-argocd` 的 RoleBinding 只在开发环境的 namespaces 里，测试环境有自己的 `order-service-staging-argocd`。

---

## 3. 环境模板（EnvTemplate）

### 3.1 数据模型

```yaml
# EnvTemplate 不是 CRD，存储在 PAAP Server 的数据库中
# 使用 YAML 格式定义，平台管理员通过 UI 维护
kind: EnvTemplate
spec:
  name: quick-start                    # 标识（英文），用于 API 引用
  displayName: 快速开始                # 展示名（可中文）
  description: 标准开发环境，含完整CI/CD和监控
  # 预装服务列表（引用 ServiceTemplate.Type）
  services:
    - type: deploy
    - type: ci
    - type: monitor

  # 预装基础设施
  infra:
    - type: postgresql
    - type: redis

  # 环境资源配额
  resourceQuota:
    cpu: "4核"
    memory: "8GB"
    storage: "50GB"

  # 网络配置（可选）
  network:
    # 默认使用 NetworkPolicy 隔离
    isolation: NetworkPolicy
    # 高级选项：绑定 IP Pool（需要 Calico/MetalLB 支持）
    ipPool:
      enabled: false
      cidr: ""            # 该环境的 IP 段，如 "10.244.0.0/24"

  # 附加 namespace 定义（可选，主空间自动创建）
  additionalNamespaces:
    - suffix: app         # → {app}-{env}-app，工作负载空间
      purpose: workload
    - suffix: db          # → {app}-{env}-db，数据库空间
      purpose: database
```

### 3.2 与环境的关系

- 环境模板是**建议性的**。用户选择「快速开始」后，平台先把环境中的工具装好。
- 用户后续可在环境内**增删工具**（不受模板限制）。
- 不同环境可选不同模板：开发环境用「完整工具链」（deploy+ci+monitor+db），生产环境用「轻量版」（deploy+monitor）。
- 附加 namespace 可在模板中预定义，也可后续动态添加。
- IP Pool 为可选高级功能，默认不启用，使用 NetworkPolicy 实现环境间隔离。

---

## 4. 环境（Environment）

### 4.1 数据模型

```go
type Environment struct {
    ID             uint      // 环境ID
    ApplicationID  uint      // 所属应用
    Name           string    // 环境名：开发环境 / 测试环境 / 生产环境
    Identifier     string    // 环境标识：dev / staging / prod
    PrimaryNS      string    // 主namespace（工具SA放这里）：order-service-dev
    Namespaces     []string  // 该环境包含的所有namespaces：[order-service-dev, order-service-dev-cache]
    Services       []string  // 已安装的服务列表：[deploy, ci, monitor]
    Status         string    // Pending / Creating / Running / Deleting / Error
}
```

### 4.2 Namespace 结构示例

```
应用「订单服务」
├── 环境「开发环境」 (id=1)
│   ├── primaryNamespace: order-service-dev
│   ├── namespaces: [order-service-dev, order-service-dev-cache]
│   ├── 已安装服务: [deploy, ci, monitor, postgresql, redis]
│   │   └── ArgoCD (SA: order-service-dev-argocd) → 权限覆盖 dev, dev-cache
│   │   └── Prometheus (SA: order-service-dev-prometheus) → 权限覆盖 dev, dev-cache
│   │   └── Tekton (SA: order-service-dev-tekton) → 权限覆盖 dev, dev-cache
│   │   └── PostgreSQL → 只在 dev ns
│   │   └── Redis → 只在 dev ns
│
├── 环境「测试环境」 (id=2)
│   ├── primaryNamespace: order-service-staging
│   ├── namespaces: [order-service-staging, order-service-staging-cache]
│   ├── 已安装服务: [deploy, ci, monitor]
│   │   └── ArgoCD (SA: order-service-staging-argocd) → 权限覆盖 staging, staging-cache
│   │   └── Prometheus (SA: order-service-staging-prometheus) → 权限覆盖 staging, staging-cache
│   │   └── Tekton (SA: order-service-staging-tekton) → 权限覆盖 staging, staging-cache
│
└── 环境「生产环境」 (id=3)
    ├── primaryNamespace: order-service-prod
    ├── namespaces: [order-service-prod]
    ├── 已安装服务: [deploy, monitor]
    │   └── ArgoCD (SA: order-service-prod-argocd) → 权限覆盖 prod
    │   └── Prometheus (SA: order-service-prod-prometheus) → 权限覆盖 prod
```

---

## 5. 平台调和器

### 5.1 架构概览

平台采用 **PAAP Server + PAAP Operator** 双层架构，详细设计参见 [Operator 设计方案](operator-design.md)。

```
┌────────────────────────────────────────────────────────────┐
│  PAAP Server（业务层）                                       │
│                                                             │
│  职责：                                                     │
│    1. 接收用户请求（创建环境/安装服务/创建组件）             │
│    2. 渲染 ServiceTemplate / EnvTemplate                    │
│    3. 创建/更新 CR 资源（Environment, ServiceInstance, Component）│
│    4. 监听 CR status，同步到 PostgreSQL                     │
│    5. 通过 WebSocket 推送状态给前端                         │
└──────────────────────┬──────────────────────────────────────┘
                       │ 创建/更新 CR
                       ▼
┌────────────────────────────────────────────────────────────┐
│  PAAP Operator（控制层）                                     │
│                                                             │
│  职责：                                                     │
│    1. 监听 CR 变化，执行 reconcile                          │
│    2. 管理 K8s 原生资源（Namespace, SA, Role, Deployment...）│
│    3. 维护 OwnerReference 级联关系                          │
│    4. 汇报状态到 CR status                                  │
│    5. 漂移检测与自动修复                                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
              K8s 集群
```

### 5.2 事件 → CR → K8s 资源的映射

| 用户事件 | PAAP Server 操作 | Operator 操作 |
|----------|-----------------|--------------|
| 创建环境 | 创建 Environment CR + ServiceInstance CR × N | 创建 Namespace, NetworkPolicy, Quota, SA, Role, RoleBinding, 工具 Deployment |
| 安装服务 | 创建 ServiceInstance CR | 创建 SA, Role, RoleBinding, 工具 Deployment/Service/ConfigMap |
| 环境新增 namespace | 更新 Environment CR（加 namespace） | 创建新 Namespace, 为所有 ServiceInstance 创建 Role + RoleBinding |
| 环境删除 namespace | 更新 Environment CR（去 namespace） | 清理 RBAC, 删除 Namespace |
| 创建组件 | 创建 Component CR | 创建 Deployment, Service, ConfigMap |
| 卸载服务 | 删除 ServiceInstance CR | 级联删除 SA, Role, RoleBinding, 工具组件 |
| 删除环境 | 删除 Environment CR | 级联删除所有子 CR + Namespace |

### 5.2 模板引擎

**引擎：** Go `text/template` + [Sprig](http://masterminds.github.io/sprig/) 函数库

**内置函数：**
- `json` — 将值转为 JSON 字符串
- `join` — 连接字符串列表
- `Sprig 全部函数` — default, empty, quote, trim, etc.

**模板验证：**
- PAAP Server 在渲染时验证模板语法
- 未定义变量返回错误，不渲染
- 渲染结果存储到 ConfigMap（`manifestsRef`），不直接存入 CR

### 5.3 运行时变量（模板引擎自动注入）

| 变量 | 说明 | 示例 |
|------|------|------|
| `.appID` | 应用ID | `1` |
| `.appName` | 应用名称 | `订单服务` |
| `.appIdentifier` | 应用标识 | `order-service` |
| `.envID` | 环境ID | `1` |
| `.envName` | 环境名 | `开发环境` |
| `.envIdentifier` | 环境标识 | `dev` |
| `.primaryNamespace` | 环境主namespace | `order-service-dev` |
| `.namespaces` | 环境所有namespace列表 | `[order-service-dev, order-service-dev-cache]` |
| `.newNamespace` | `onEnvNsAdded` 时触发的新namespace | `order-service-dev-db` |
| `.removedNamespace` | `onEnvNsRemoved` 时触发的被删namespace | `order-service-dev-cache` |
| `.releaseName` | 服务实例发布名（自动生成） | `order-service-dev-argocd` |
| `.serviceAccountName` | SA 完整名 | `order-service-dev-argocd` |
| `.serviceAccountNamespace` | SA 所在namespace | `order-service-dev` |
| `.envRole` | ServiceTemplate.rbac.envRole（预注入） | - |
| `.parameters.*` | 用户提交的参数值 | - |

---

## 6. 完整示例

### 6.1 ArgoCD 服务模板（需要精细RBAC）

```yaml
apiVersion: paap.io/v1
kind: ServiceTemplate
metadata:
  type: deploy
  name: 部署服务
  category: tool
spec:
  parameters:
    - key: version
      label: ArgoCD版本
      type: select
      options: ["v2.9", "v2.10", "v2.11"]
      default: "v2.10"

  rbac:
    serviceAccountName: "{{ .appIdentifier }}-{{ .envIdentifier }}-argocd"
    serviceAccountNamespace: "{{ .primaryNamespace }}"
    deploymentRole:
      rules:
        - apiGroups: ["argoproj.io"]
          resources: ["applications", "appprojects"]
          verbs: ["*"]
    envRole:
      rules:
        - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
          resources: ["*"]
          verbs: ["*"]

  templates:
    install: |
      ---
      # ArgoCD Server
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: argocd-server
        namespace: "{{ .primaryNamespace }}"
      spec:
        selector:
          matchLabels:
            app: argocd-server
        template:
          metadata:
            labels:
              app: argocd-server
          spec:
            serviceAccountName: "{{ .serviceAccountName }}"
            containers:
              - name: server
                image: quay.io/argoproj/argocd:{{ .parameters.version }}
      ---
      # ArgoCD Application Controller
      apiVersion: apps/v1
      kind: Deployment
      ...
      ---
      # ConfigMap: 限定只管理本环境的namespaces
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: argocd-cm
        namespace: "{{ .primaryNamespace }}"
      data:
        application.namespaces: "{{ join .namespaces "," }}"

    onEnvNsAdded: |
      # 在新namespace创建Role + RoleBinding
      ---
      apiVersion: rbac.authorization.k8s.io/v1
      kind: Role
      metadata:
        name: argocd-manager
        namespace: "{{ .newNamespace }}"
      rules:
        {{ range .envRole.rules }}
        - apiGroups: {{ .apiGroups | json }}
          resources: {{ .resources | json }}
          verbs: {{ .verbs | json }}
        {{ end }}
      ---
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: argocd-manager
        namespace: "{{ .newNamespace }}"
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: Role
        name: argocd-manager
      subjects:
        - kind: ServiceAccount
          name: "{{ .serviceAccountName }}"
          namespace: "{{ .serviceAccountNamespace }}"

    uninstall: |
      # 平台会先遍历所有ns删Role/RoleBinding，然后执行此模板删组件
      # ... 删 Deployment/Service/ConfigMap/SA
```

### 6.2 PostgreSQL 服务模板（无跨namespace问题）

```yaml
apiVersion: paap.io/v1
kind: ServiceTemplate
metadata:
  type: postgresql
  name: PostgreSQL
  category: infra
spec:
  parameters:
    - key: storage
      label: 存储大小
      type: select
      options: ["10Gi", "20Gi", "50Gi"]
      default: "10Gi"
    - key: password
      label: 密码
      type: password
      required: true

  templates:
    install: |
      ---
      apiVersion: v1
      kind: Secret
      metadata:
        name: "{{ .releaseName }}-secret"
        namespace: "{{ .primaryNamespace }}"
      stringData:
        password: "{{ .parameters.password }}"
      ---
      apiVersion: apps/v1
      kind: StatefulSet
      metadata:
        name: "{{ .releaseName }}"
        namespace: "{{ .primaryNamespace }}"
      spec:
        serviceName: "{{ .releaseName }}"
        selector:
          matchLabels:
            app: postgresql
        template:
          metadata:
            labels:
              app: postgresql
          spec:
            containers:
              - name: postgresql
                image: postgres:15-alpine
                env:
                  - name: POSTGRES_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: "{{ .releaseName }}-secret"
                        key: password
                volumeMounts:
                  - name: data
                    mountPath: /var/lib/postgresql/data
            volumes:
              - name: data
                persistentVolumeClaim:
                  claimName: "{{ .releaseName }}-pvc"
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: "{{ .releaseName }}-pvc"
        namespace: "{{ .primaryNamespace }}"
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: "{{ .parameters.storage }}"
```

---

## 7. Helm 模式的支持

对于不敏感的基础设施，仍支持 Helm 模式。PAAP Server 执行 `helm template` 生成 raw YAML，存储到 ConfigMap，Operator 读取 ConfigMap 并 reconcile。

```yaml
kind: ServiceTemplate
spec:
  type: postgresql
  parameters:
    - key: storage
      ...

  # Helm 模式：PAAP Server 执行 helm template 生成 raw YAML
  # 存储到 manifestsRef 指定的 ConfigMap 中
  # Operator 读取 ConfigMap 并创建/管理 K8s 资源
  helm:
    repo: "https://charts.bitnami.com/bitnami"
    chart: "bitnami/postgresql"
    version: "12.12.10"
    userSetValues:
      auth.username: "appuser"
      auth.password: "{{ .parameters.password }}"
      primary.persistence.size: "{{ .parameters.storage }}"
```

**流程：**
1. PAAP Server 执行 `helm template` → 生成 raw YAML
2. PAAP Server 将 YAML 存入 ConfigMap（`manifestsRef`）
3. Operator 读取 ConfigMap，解析 YAML，创建/管理 K8s 资源
4. Operator 使用 server-side apply 管理资源生命周期
