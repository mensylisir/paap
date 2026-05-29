# PAAP Operator 设计方案

## 一、为什么需要 Operator

### 1.1 当前问题

PAAP 服务端直接通过 `kubectl apply` 管理 K8s 资源，存在以下问题：

```
创建一个「开发环境」+ 安装 ArgoCD + Tekton + Prometheus，需要创建：

Namespace × N        （主空间 + 工作负载空间 + 专用空间）
ServiceAccount × 3   （argocd, tekton, prometheus）
Role × N×3           （每个 namespace × 每个工具）
RoleBinding × N×3
Deployment × 5+      （argocd-server, argocd-controller, tekton, prometheus, grafana...）
Service × 5+
ConfigMap × N+
NetworkPolicy × N
ResourceQuota × N
...

总计：30-50 个 K8s 资源，分散在多个 namespace
```

**没有 Operator 的问题：**

| 问题 | 说明 |
|------|------|
| 资源散落 | 30+ 资源分散在多个 namespace，没有统一归属 |
| 删除困难 | 删除环境需要手动清理所有资源，遗漏会导致资源泄漏 |
| 状态不透明 | 创建成功/失败/进行中，需要自己轮询 |
| 漂移无法修复 | 手动改了资源后，平台不知道也无法修复 |
| 代码散乱 | kubectl 调用散落在各 service 代码中 |

### 1.2 Operator 带来的好处

| 维度 | 直接 kubectl | Operator |
|------|-------------|----------|
| 资源归属 | 散落，无主 | CRD 是 owner，级联删除 |
| 状态管理 | 需要自己轮询 | CRD status 自动汇报 |
| 漂移修复 | 手动 | 自动 reconcile |
| 删除清理 | 手动逐个删 | OwnerReference 级联 GC |
| 代码组织 | 散落在各 service | 集中在 controller |
| 可观测性 | 看日志 | `kubectl get` 看状态 |
| 事件追踪 | 无 | K8s Event 自动记录 |

---

## 二、CRD 设计

### 2.1 四个核心 CRD

**与 ArgoCD Application 的区别：**

| | PAAP Application | ArgoCD Application |
|--|-----------------|-------------------|
| API Group | `paap.io/v1` | `argoproj.io/v1alpha1` |
| 含义 | 业务应用，包含多个环境 | 部署单元，管理一组 K8s 资源 |
| 位置 | `paap-app-{identifier}` namespace | 环境主 namespace |
| 生命周期 | 跨环境 | 环境内部 |
| 关系 | 包含多个 Environment CR | 被 ServiceInstance（deploy 类型）管理 |

**不冲突**：API Group 不同，且 ArgoCD Application 是环境内部的工具资源，由 PAAP 的 ServiceInstance 管理。

```
┌─────────────────────────────────────────────────────────┐
│                    PAAP 平台层                            │
│  PAAP Server（Go）                                       │
│  ├── 管理应用/环境/用户等业务数据（PostgreSQL）           │
│  ├── 创建/更新/删除 CR 资源                              │
│  └── 监听 CR status 同步到数据库                         │
└──────────────────────┬──────────────────────────────────┘
                       │ 创建/更新 CR
                       ▼
┌─────────────────────────────────────────────────────────┐
│                    K8s 集群                               │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐                     │
│  │ Application  │  │ Environment  │                     │
│  │     CRD      │  │     CRD      │                     │
│  └──────┬───────┘  └──────┬───────┘                     │
│         │                 │                              │
│         │ 1:N             │ 1:N                          │
│         ▼                 ▼                              │
│  ┌──────────────┐  ┌──────────────┐                     │
│  │ServiceInstance│ │  Component   │                     │
│  │     CRD      │  │     CRD      │                     │
│  └──────┬───────┘  └──────┬───────┘                     │
│         │                 │                              │
│         ▼                 ▼                              │
│  ┌─────────────────────────────────────────────────┐    │
│  │              PAAP Controller                     │    │
│  │  ├── Application Reconciler                     │    │
│  │  │   └── 管理 paap-app-{id} namespace            │    │
│  │  ├── Environment Reconciler                     │    │
│  │  │   └── 管理业务 Namespace, NetworkPolicy, Quota│    │
│  │  ├── ServiceInstance Reconciler                  │    │
│  │  │   └── 管理 SA, Role, RoleBinding, 工具部署    │    │
│  │  └── Component Reconciler                       │    │
│  │      └── 管理 Deployment, Service, ConfigMap    │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### CR 存放策略

```
Namespace: paap-system                        ← 平台系统空间
│
├── Application CR: order-service             ← 应用级 CR
├── Application CR: user-center
│
Namespace: paap-app-order-service             ← 每个应用一个 CR namespace
│                                               命名: paap-app-{应用标识}
├── Environment CR: dev                       ← 环境 CR（名=环境标识）
├── Environment CR: staging
├── Environment CR: prod
│
├── ServiceInstance CR: dev-argocd            ← 工具实例 CR
├── ServiceInstance CR: dev-tekton
├── ServiceInstance CR: dev-prometheus
├── ServiceInstance CR: staging-argocd
├── ServiceInstance CR: prod-argocd
├── ServiceInstance CR: prod-prometheus
│
├── Component CR: dev-frontend                ← 组件 CR
├── Component CR: dev-backend
├── Component CR: staging-frontend
├── Component CR: prod-frontend
│
Namespace: paap-app-user-center               ← 另一个应用的 CR namespace
├── Environment CR: dev                       ← 同名不冲突！
├── Environment CR: prod
├── ...
│
Namespace: order-service-dev                  ← 业务 namespace（由 Environment CR 创建）
├── ArgoCD 实例 (Deployment, Service...)
├── Tekton 实例
├── Prometheus 实例
├── SA, Role, RoleBinding
├── NetworkPolicy
├── ResourceQuota
│
Namespace: order-service-dev-app              ← 业务 namespace（由 Environment CR 创建）
├── 组件 Deployment (由 ArgoCD 部署)
│
Namespace: user-center-dev                    ← 另一个应用的业务 namespace
├── ...
```

**为什么用 `paap-app-{identifier}` namespace 存 CR？**
- 不同应用的同名环境 CR 不冲突（`dev` 在 `paap-app-order-service` 和 `paap-app-user-center` 各有一个）
- CR 和业务 namespace 分离，不会混淆
- Application CR 放 `paap-system`，作为全局入口

### 2.2 Application CRD

代表一个业务应用，是所有环境的顶层归属。

```yaml
apiVersion: paap.io/v1
kind: Application
metadata:
  name: order-service
  namespace: paap-system          # Application CR 统一放在 paap-system
spec:
  # 应用信息
  name: 订单服务
  identifier: order-service       # 用于生成 paap-app-{identifier} namespace
  description: 订单管理服务

  # 应用管理员
  owners:
    - user: admin@example.com
      role: admin

status:
  phase: Active                   # Active / Deleting
  environments:                   # 自动汇总所有 Environment CR 的状态
    - name: dev
      namespace: paap-app-order-service
      phase: Running
    - name: staging
      namespace: paap-app-order-service
      phase: Running
    - name: prod
      namespace: paap-app-order-service
      phase: Running
  environmentCount: 3
  conditions:
    - type: CRNamespaceReady
      status: "True"
      message: "paap-app-order-service namespace is ready"
```

**Application Reconciler 职责：**
1. 创建/管理 `paap-app-{identifier}` namespace（存放该应用的所有 CR）
2. 汇总所有 Environment CR 的状态到 Application status
3. 删除 Application 时级联删除 `paap-app-{identifier}` namespace（触发所有子资源级联删除）

---

### 2.3 Environment CRD

管理一个环境的全部 namespace 和网络策略。存放在 `paap-app-{应用标识}` namespace 中。

```yaml
apiVersion: paap.io/v1
kind: Environment
metadata:
  name: dev                       # CR 名 = 环境标识（在 paap-app-{app} 内唯一）
  namespace: paap-app-order-service  # 存放在应用的 CR namespace
  labels:
    paap.io/app: order-service
    paap.io/env: dev
spec:
  # 环境信息
  name: 开发环境
  identifier: dev

  # 主 namespace（自动创建）
  primaryNamespace: order-service-dev

  # 附加 namespace（自动创建）
  # PAAP Server 渲染时: name = {appIdentifier}-{envIdentifier}-{suffix}
  additionalNamespaces:
    - suffix: app
      purpose: workload
    - suffix: db
      purpose: database
    - suffix: cache
      purpose: cache

  # 网络配置
  network:
    isolation: NetworkPolicy        # NetworkPolicy / None
    ipPool:
      enabled: false
      cidr: ""                      # 如 "10.244.0.0/24"
      provider: ""                  # calico / metallb / none
      # provider-specific 配置放在 annotations 中
      # 如 paap.io/calico-blocksize: "26"

  # 资源配额
  resourceQuota:
    cpu: "8"
    memory: "16Gi"
    storage: "100Gi"

status:
  phase: Running            # Pending / Creating / Running / Deleting / Error
  namespaces:               # 实际创建的 namespace 列表
    - name: order-service-dev
      phase: Active
    - name: order-service-dev-app
      phase: Active
    - name: order-service-dev-db
      phase: Active
  conditions:
    - type: NamespacesReady
      status: "True"
    - type: NetworkPolicyReady
      status: "True"
    - type: ResourceQuotaReady
      status: "True"
  observedGeneration: 1
  lastTransitionTime: "2026-05-29T10:00:00Z"
```

### 2.4 ServiceInstance CRD

管理一个环境内的工具实例（ArgoCD/Tekton/Prometheus 等）。

```yaml
apiVersion: paap.io/v1
kind: ServiceInstance
metadata:
  name: dev-argocd                # {envIdentifier}-{type}（在 paap-app-{app} 内唯一）
  namespace: paap-app-order-service  # 存放在应用的 CR namespace
  labels:
    paap.io/app: order-service
    paap.io/env: dev
    paap.io/type: deploy
spec:
  # 关联的环境
  environmentRef:
    name: dev                     # 引用同 namespace 下的 Environment CR

  # 服务模板类型
  type: deploy                    # deploy / ci / monitor / postgresql / redis / ...

  # 服务模板参数
  parameters:
    version: "v2.10"

  # SA 配置（自动生成）
  serviceAccount:
    name: order-service-dev-argocd
    namespace: order-service-dev   # 环境主 namespace

  # 工具自身 Deployment 需要的权限（在 primaryNamespace 创建 Role）
  deploymentRole:
    rules:
      - apiGroups: ["argoproj.io"]
        resources: ["applications", "appprojects"]
        verbs: ["*"]

  # 权限声明（来自 ServiceTemplate.rbac.envRole）
  envRole:
    rules:
      - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
        resources: ["*"]
        verbs: ["*"]

status:
  phase: Running            # Pending / Installing / Running / Upgrading / Deleting / Error
  # SA 状态
  serviceAccount:
    name: order-service-dev-argocd
    namespace: order-service-dev
    created: true
  # 权限覆盖的 namespace
  rbacNamespaces:
    - namespace: order-service-dev
      roleCreated: true
      roleBindingCreated: true
    - namespace: order-service-dev-app
      roleCreated: true
      roleBindingCreated: true
    - namespace: order-service-dev-db
      roleCreated: true
      roleBindingCreated: true
    - namespace: order-service-dev-cache
      roleCreated: true
      roleBindingCreated: true
  # 工具组件状态
  components:
    - name: argocd-server
      kind: Deployment
      ready: true
      replicas: 1/1
    - name: argocd-application-controller
      kind: StatefulSet
      ready: true
      replicas: 1/1
    - name: argocd-repo-server
      kind: Deployment
      ready: true
      replicas: 1/1
  conditions:
    - type: SAAvailable
      status: "True"
    - type: RBACReady
      status: "True"
    - type: ComponentsReady
      status: "True"
  observedGeneration: 1
```

### 2.5 Component CRD

管理环境内的业务组件（Deployment + Service）。

```yaml
apiVersion: paap.io/v1
kind: Component
metadata:
  name: dev-frontend              # {envIdentifier}-{componentIdentifier}
  namespace: paap-app-order-service  # 存放在应用的 CR namespace
  labels:
    paap.io/app: order-service
    paap.io/env: dev
spec:
  # 关联的环境
  environmentRef:
    name: dev                     # 引用同 namespace 下的 Environment CR

  # 组件信息
  name: 前端服务
  identifier: frontend
  type: frontend                  # frontend / backend / custom

  # 部署配置
  deployment:
    # 默认部署到工作负载空间（{app}-{env}-app）
    # 用户可在 UI 中选择其他 namespace（主空间或专用空间）
    namespace: order-service-dev-app
    image: registry.internal.com/order-service/frontend
    tag: v1.3
    replicas: 2
    resources:
      cpu: "0.5"
      memory: "512Mi"
    env:
      - name: DB_HOST
        valueFrom:
          secretKeyRef:
            name: order-service-dev-postgresql-secret
            key: host

  # 服务配置
  service:
    port: 80
    targetPort: 8080
    type: ClusterIP

  # 访问配置（可选）
  ingress:
    enabled: false
    host: ""
    path: ""

status:
  phase: Running            # Pending / Creating / Running / Scaling / Deleting / Error
  deployment:
    name: order-service-dev-frontend
    namespace: order-service-dev
    readyReplicas: 2
    replicas: 2
    updatedReplicas: 2
  service:
    name: order-service-dev-frontend
    namespace: order-service-dev
    clusterIP: 10.96.0.100
  conditions:
    - type: DeploymentReady
      status: "True"
    - type: ServiceReady
      status: "True"
```

---

## 三、Controller 设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                 PAAP Controller Manager                   │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Application Reconciler                           │    │
│  │                                                   │    │
│  │  Watch: Application CR (in paap-system)           │    │
│  │  Manage:                                          │    │
│  │    ├── paap-app-{identifier} namespace（创建/删除）│    │
│  │    └── 汇总 Environment 状态到 Application status  │    │
│  │                                                   │    │
│  │  Status: 更新 phase + environments 列表            │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Environment Reconciler                           │    │
│  │                                                   │    │
│  │  Watch: Environment CR (in paap-app-{app})        │    │
│  │  Manage:                                          │    │
│  │    ├── 业务 Namespaces（创建/删除）                │    │
│  │    ├── NetworkPolicy（跨环境隔离）                 │    │
│  │    ├── ResourceQuota                              │    │
│  │    └── Labels（paap.io/app, paap.io/env）         │    │
│  │                                                   │    │
│  │  Status: 更新 phase + namespaces 列表              │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │  ServiceInstance Reconciler                       │    │
│  │                                                   │    │
│  │  Watch: ServiceInstance CR (in paap-app-{app})    │    │
│  │  Manage:                                          │    │
│  │    ├── ServiceAccount（在 primaryNamespace）       │    │
│  │    ├── Role + RoleBinding（在每个 env namespace）  │    │
│  │    ├── 工具 Deployment/Service/ConfigMap           │    │
│  │    └── 工具内部配置（ArgoCD AppProject 等）        │    │
│  │                                                   │    │
│  │  Status: 更新 phase + RBAC 覆盖情况 + 组件状态     │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Component Reconciler                             │    │
│  │                                                   │    │
│  │  Watch: Component CR (in paap-app-{app})          │    │
│  │  Manage:                                          │    │
│  │    ├── Deployment（在指定 namespace）              │    │
│  │    ├── Service                                    │    │
│  │    ├── ConfigMap/Secret（环境变量）                │    │
│  │    └── Ingress（可选）                             │    │
│  │                                                   │    │
│  │  Status: 更新 phase + 副本状态                     │    │
│  └─────────────────────────────────────────────────┘    │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### 3.1.1 Component 与 ArgoCD 的所有权协调

当用户对组件执行「配置部署」时，组件的 Deployment 管理权从 Component Reconciler 转交给 ArgoCD。

**两个阶段：**

```
阶段1：组件由 Component Reconciler 直接管理
  Component CR spec:
    managedBy: operator       # 默认值
  → Operator 创建/管理 Deployment, Service

阶段2：用户「配置部署」后，转交给 ArgoCD
  Component CR spec:
    managedBy: argocd         # 转交给 ArgoCD
    argocdAppRef:             # ArgoCD Application 引用
      name: order-service-dev-frontend
  → Operator 不再管理 Deployment（保留 Service/ConfigMap）
  → ArgoCD ServiceInstance 创建 ArgoCD Application CR
  → ArgoCD 接管 Deployment 的生命周期
```

**Component Reconciler 逻辑：**

```go
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    comp := &paapv1.Component{}
    if err := r.Get(ctx, req.NamespacedName, comp); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    switch comp.Spec.ManagedBy {
    case "operator":
        // 直接管理 Deployment, Service, ConfigMap
        r.ensureDeployment(ctx, comp)
        r.ensureService(ctx, comp)
        r.ensureConfigMap(ctx, comp)

    case "argocd":
        // 只管理 Service, ConfigMap（Deployment 由 ArgoCD 管理）
        r.ensureService(ctx, comp)
        r.ensureConfigMap(ctx, comp)
        // 创建/更新 ArgoCD Application CR（由 ArgoCD ServiceInstance 管理）
        r.ensureArgoCDApplication(ctx, comp)
        // 监控 ArgoCD 部署状态，同步到 Component status
        r.syncArgoCDStatus(ctx, comp)
    }

    return ctrl.Result{}, r.Status().Update(ctx, comp)
}
```

**为什么不冲突：**
- Component Reconciler 和 ArgoCD 不会同时管理同一个 Deployment
- 切换时 Component Reconciler 先停止管理，再由 ArgoCD 接管
- ArgoCD Application CR 由 Component Reconciler 创建，ArgoCD Controller 消费

```go
func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    env := &paapv1.Environment{}
    if err := r.Get(ctx, req.NamespacedName, env); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 1. 收集期望的 namespace 列表
    expectedNS := []string{env.Spec.PrimaryNamespace}
    for _, ns := range env.Spec.AdditionalNamespaces {
        expectedNS = append(expectedNS, ns.Name)
    }

    // 2. 创建缺失的 namespace
    for _, nsName := range expectedNS {
        if err := r.ensureNamespace(ctx, env, nsName); err != nil {
            return ctrl.Result{}, err
        }
    }

    // 3. 删除多余的 namespace
    for _, nsName := range env.Status.Namespaces {
        if !contains(expectedNS, nsName.Name) {
            if err := r.deleteNamespace(ctx, nsName.Name); err != nil {
                return ctrl.Result{}, err
            }
        }
    }

    // 4. 创建/更新 NetworkPolicy
    for _, nsName := range expectedNS {
        if err := r.ensureNetworkPolicy(ctx, env, nsName); err != nil {
            return ctrl.Result{}, err
        }
    }

    // 5. 创建/更新 ResourceQuota
    if err := r.ensureResourceQuota(ctx, env); err != nil {
        return ctrl.Result{}, err
    }

    // 6. 更新 status
    env.Status.Phase = "Running"
    env.Status.Namespaces = r.buildNamespaceStatus(expectedNS)
    return ctrl.Result{}, r.Status().Update(ctx, env)
}
```

### 3.3 ServiceInstance Reconciler

```go
func (r *ServiceInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    svc := &paapv1.ServiceInstance{}
    if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 处理删除（Finalizer）
    if !svc.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, svc)
    }

    // 添加 Finalizer
    if !contains(svc.Finalizers, paapFinalizer) {
        svc.Finalizers = append(svc.Finalizers, paapFinalizer)
        r.Update(ctx, svc)
    }

    // 1. 获取关联的 Environment，拿到 namespace 列表
    env := &paapv1.Environment{}
    envKey := types.NamespacedName{
        Name:      svc.Spec.EnvironmentRef.Name,
        Namespace: svc.Namespace,  // 同一个 paap-app-{app} namespace
    }
    if err := r.Get(ctx, envKey, env); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 2. 检查 Environment 是否就绪（解决 3.9 并发问题）
    if env.Status.Phase != "Running" {
        // Environment 还没准备好，等待后重试
        return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
    }
    allNamespaces := env.Status.Namespaces

    // 2. 确保 SA 存在
    if err := r.ensureServiceAccount(ctx, svc); err != nil {
        return ctrl.Result{}, err
    }

    // 3. 遍历所有 namespace，确保 Role + RoleBinding
    for _, ns := range allNamespaces {
        if err := r.ensureRBAC(ctx, svc, ns.Name); err != nil {
            return ctrl.Result{}, err
        }
    }

    // 4. 清理已删除 namespace 的 RBAC
    for _, ns := range svc.Status.RBACNamespaces {
        if !containsNamespace(allNamespaces, ns.Namespace) {
            if err := r.cleanupRBAC(ctx, svc, ns.Namespace); err != nil {
                return ctrl.Result{}, err
            }
        }
    }

    // 5. 确保工具组件（Deployment/Service/ConfigMap）
    if err := r.ensureComponents(ctx, svc); err != nil {
        return ctrl.Result{}, err
    }

    // 6. 更新 status
    svc.Status.Phase = "Running"
    svc.Status.RBACNamespaces = r.buildRBACStatus(allNamespaces)
    svc.Status.Components = r.buildComponentStatus(ctx, svc)
    return ctrl.Result{}, r.Status().Update(ctx, svc)
}
```

### 3.4 资源归属与级联删除（Finalizer 机制）

> **K8s OwnerReference 限制：**
> 1. namespaced CR 不能 own 集群资源（如 Namespace）
> 2. OwnerReference 不能跨 namespace
>
> 因此，**不能用 OwnerReference 实现跨 namespace 级联删除**，必须使用 **Finalizer**。

**资源归属关系（逻辑关系，非 OwnerReference）：**

```
Application CR: order-service (in paap-system)
│
│   逻辑拥有（通过 Finalizer 实现级联删除）
│
├── Namespace: paap-app-order-service            ← 由 Application Reconciler 创建
│
├── Environment CR: dev (in paap-app-order-service)
│   │
│   │   逻辑拥有
│   │
│   ├── Namespace: order-service-dev             ← 由 Environment Reconciler 创建
│   ├── Namespace: order-service-dev-app
│   ├── Namespace: order-service-dev-db
│   ├── NetworkPolicy (in each ns)
│   └── ResourceQuota (in each ns)
│
├── ServiceInstance CR: dev-argocd (in paap-app-order-service)
│   │
│   │   逻辑拥有（跨 namespace 资源用 Finalizer）
│   │
│   ├── SA: order-service-dev-argocd             ← 跨 namespace
│   ├── Role: argocd-manager (in each ns)
│   ├── RoleBinding: argocd-manager (in each ns)
│   ├── Deployment: argocd-server
│   ├── Service: argocd-server
│   └── ConfigMap: argocd-cm
│
├── Component CR: dev-frontend (in paap-app-order-service)
│   ├── Deployment: order-service-dev-frontend   ← 跨 namespace
│   ├── Service: order-service-dev-frontend
│   └── ConfigMap: order-service-dev-frontend-env
│
└── Environment CR: staging (in paap-app-order-service)
    └── ...
```

**Finalizer 机制：**

```go
const paapFinalizer = "paap.io/finalizer"

func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    env := &paapv1.Environment{}
    if err := r.Get(ctx, req.NamespacedName, env); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 处理删除
    if !env.DeletionTimestamp.IsZero() {
        if contains(env.Finalizers, paapFinalizer) {
            // 1. 先删除子资源
            r.deleteChildServiceInstances(ctx, env)
            r.deleteChildComponents(ctx, env)
            // 2. 等待子 CR 全部删除完成
            if r.hasChildren(ctx, env) {
                return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
            }
            // 3. 删除业务 namespace（K8s 自动清理 ns 下所有资源）
            for _, ns := range env.Status.Namespaces {
                r.deleteNamespace(ctx, ns.Name)
            }
            // 4. 移除 Finalizer
            env.Finalizers = remove(env.Finalizers, paapFinalizer)
            r.Update(ctx, env)
        }
        return ctrl.Result{}, nil
    }

    // 正常 reconcile：添加 Finalizer
    if !contains(env.Finalizers, paapFinalizer) {
        env.Finalizers = append(env.Finalizers, paapFinalizer)
        r.Update(ctx, env)
    }
    // ... 正常 reconcile 逻辑
}
```

**删除级联流程（以删除 Application 为例）：**

```
用户删除 Application CR: order-service
  ↓
1. K8s 设置 DeletionTimestamp（不立即删除，因为有 Finalizer）
  ↓
2. Application Reconciler 检测到删除:
   a. 列出 paap-app-order-service 下所有 Environment CR
   b. 逐个删除 Environment CR
   c. 等待所有 Environment CR 被删除完成（Requeue 轮询）
   d. 删除 paap-app-order-service namespace
   e. 移除 Finalizer
   ↓
3. Environment Reconciler 检测到 dev 被删除:
   a. 列出同 namespace 下引用 dev 的 ServiceInstance CR → 逐个删除
   b. 列出同 namespace 下引用 dev 的 Component CR → 逐个删除
   c. 等待所有子 CR 被删除完成
   d. 删除业务 namespace: order-service-dev, order-service-dev-app, ...
   e. 移除 Finalizer
   ↓
4. ServiceInstance Reconciler 检测到 dev-argocd 被删除:
   a. 删除 SA, Role, RoleBinding (在各业务 ns)
   b. 删除 Deployment, Service, ConfigMap
   c. 移除 Finalizer
   ↓
5. Component Reconciler 检测到 dev-frontend 被删除:
   a. 删除 Deployment, Service, ConfigMap
   b. 移除 Finalizer
   ↓
6. 所有 Finalizer 移除后，K8s 真正删除 CR
```

**同 namespace 内可用 OwnerReference：**

```go
// 同 namespace 内：用 OwnerReference（如 ServiceInstance CR → 同 ns 的 ConfigMap）
cm := &corev1.ConfigMap{
    ObjectMeta: metav1.ObjectMeta{
        Name:      "argocd-cm-template",
        Namespace: "paap-app-order-service",  // 和 ServiceInstance CR 同 ns
        OwnerReferences: []metav1.OwnerReference{
            *metav1.NewControllerRef(svcInstance, paapv1.GroupVersion.WithKind("ServiceInstance")),
        },
    },
}

// 跨 namespace：用 Finalizer + 手动删除
// 在 Reconciler 的 DeletionTimestamp 处理逻辑中
```

---

### 3.5 错误恢复与重试策略

**Operator 重试机制：**
- controller-runtime 内置指数退避重试（默认初始 1s，最大 1000s）
- Reconcile 返回 `error` 时自动重试
- 返回 `ctrl.Result{RequeueAfter: N}` 时延迟重试

**Error 状态触发条件：**
- 连续 3 次 Reconcile 失败且无法自动恢复
- 依赖资源不存在（如引用的 Environment CR 被删除）
- K8s API 持续不可用

**部分失败处理：**
```go
// Reconciler 使用 status.conditions 记录每一步的状态
status:
  conditions:
    - type: SACreated
      status: "True"
    - type: RBACReady
      status: "False"
      reason: "RoleCreationFailed"
      message: "namespace order-service-dev-db not found"
    - type: ComponentsReady
      status: "Unknown"
  phase: Error
```

**用户触发重试：**
- UI 显示「重试」按钮，点击后 PAAP Server 更新 CR 的 annotation 触发 Reconcile
- 或删除 CR 重建

### 3.6 工具升级流程

```
用户在 UI 选择 ArgoCD 版本 v2.10 → v2.11
  ↓
1. PAAP Server 更新 ServiceInstance CR:
   spec.parameters.version: "v2.11"
   ↓
2. Operator 检测到 CR 变化:
   a. 更新 status.phase = "Upgrading"
   b. 读取 manifestsRef ConfigMap
   c. 重新渲染部署清单（新版本镜像）
   d. 执行 rolling update（K8s 自动处理）
   e. 等待所有 Pod Ready
   f. 更新 status.phase = "Running"
   ↓
3. 如果升级失败:
   a. status.phase = "Error"
   b. K8s 保留旧 Pod（rolling update 策略）
   c. 用户可点击「回滚」→ PAAP Server 改回旧版本参数
```

### 3.7 ResourceQuota 作用域

**方案：每个 namespace 独立 Quota，总配额分摊。**

```
Environment CR:
  resourceQuota:
    cpu: "8"
    memory: "16Gi"
    storage: "100Gi"

主 namespace: order-service-dev
  → ResourceQuota: cpu=4, memory=8Gi, storage=50Gi（50%）

工作负载 namespace: order-service-dev-app
  → ResourceQuota: cpu=3, memory=6Gi, storage=40Gi（40%）

专用 namespace: order-service-dev-db
  → ResourceQuota: cpu=1, memory=2Gi, storage=10Gi（10%）
```

**分摊规则：**
- 主空间：50%（工具 + 部分组件）
- 工作负载空间：40%（主要业务组件）
- 专用空间：10%（数据库/缓存）
- 比例可在 EnvTemplate 中自定义

---

## 四、PAAP Server 与 Operator 的职责划分

### 4.1 职责边界

```
┌─────────────────────────────────────────────────────────┐
│  PAAP Server（业务层）                                    │
│                                                          │
│  职责：                                                   │
│  ├── 用户认证与权限（JWT, RBAC）                         │
│  ├── 应用/环境/服务的 CRUD（写 PostgreSQL）              │
│  ├── 创建/更新/删除 CR 资源（写 K8s）                    │
│  ├── 监听 CR status 变化，同步到数据库                   │
│  ├── 业务逻辑（模板渲染、参数校验）                      │
│  └── API 接口（REST/WebSocket）                          │
│                                                          │
│  不做：                                                   │
│  ├── 不直接创建 Deployment/Service/RBAC                  │
│  ├── 不轮询 K8s 资源状态                                 │
│  └── 不管理 K8s 资源的生命周期                            │
└──────────────────────┬──────────────────────────────────┘
                       │ 创建/更新 CR
                       ▼
┌─────────────────────────────────────────────────────────┐
│  PAAP Operator（控制层）                                  │
│                                                          │
│  职责：                                                   │
│  ├── 监听 CR 变化，执行 reconcile                        │
│  ├── 管理 K8s 原生资源的生命周期                          │
│  ├── 维护 OwnerReference 级联关系                        │
│  ├── 汇报状态到 CR status                                │
│  ├── 发出 K8s Event 记录关键操作                         │
│  └── 漂移检测与自动修复                                   │
│                                                          │
│  不做：                                                   │
│  ├── 不处理业务逻辑（模板、参数）                        │
│  ├── 不直接与用户交互                                    │
│  └── 不管理业务数据（PostgreSQL）                        │
└──────────────────────────────────────────────────────────┘
```

### 4.2 交互流程示例

**用户创建应用：**

```
1. 用户 → PAAP API: POST /apps {name: "订单服务", identifier: "order-service"}
   ↓
2. PAAP Server:
   a. 写 PostgreSQL: 创建 Application 记录
   b. 创建 Application CR (in paap-system)
   ↓
3. PAAP Operator (Application Reconciler):
   a. 监听到 Application CR 创建
   b. 创建 namespace: paap-app-order-service
   c. 更新 Application CR status
   ↓
4. PAAP Server:
   a. 监听到 status 变化
   b. 更新 PostgreSQL
   c. 通知前端: 应用创建成功
```

**用户创建环境：**

```
1. 用户 → PAAP API: POST /apps/order-service/environments {name: "开发环境", template: "dev-standard"}
   ↓
2. PAAP Server:
   a. 写 PostgreSQL: 创建 Environment 记录
   b. 渲染环境模板，确定 services + namespaces
   c. 创建 Environment CR (in paap-app-order-service)
   d. 为模板中的每个 service 创建 ServiceInstance CR (in paap-app-order-service)
   ↓
3. PAAP Operator (Environment Reconciler):
   a. 监听到 Environment CR 创建
   b. 创建业务 namespace: order-service-dev, order-service-dev-app, ...
   c. 创建 NetworkPolicy
   d. 创建 ResourceQuota
   e. 更新 Environment CR status: phase=Running
   ↓
4. PAAP Operator (ServiceInstance Reconciler):
   a. 监听到 ServiceInstance CR 创建（如 dev-argocd）
   b. 创建 SA: order-service-dev-argocd (in order-service-dev)
   c. 遍历所有业务 namespace 创建 Role + RoleBinding
   d. 根据 ServiceTemplate 渲染部署清单，创建 Deployment/Service/ConfigMap
   e. 更新 ServiceInstance CR status: phase=Running
   ↓
5. PAAP Server:
   a. 监听到 CR status 变化
   b. 更新 PostgreSQL 中的状态
   c. 通过 WebSocket 推送给前端
   ↓
6. 前端: 显示「开发环境」已就绪
```

**用户删除环境：**

```
1. 用户 → PAAP API: DELETE /environments/order-service/dev
   ↓
2. PAAP Server:
   a. 删除 Environment CR: dev (in paap-app-order-service)
   b. 标记 PostgreSQL 记录为 deleting
   ↓
3. PAAP Operator:
   a. K8s GC 根据 OwnerReference 级联删除:
      - ServiceInstance CR × N (dev-argocd, dev-tekton, dev-prometheus)
        - SA, Role, RoleBinding, Deployment, Service, ConfigMap
      - Component CR × N (dev-frontend, dev-backend)
        - Deployment, Service, ConfigMap
      - Namespace × N (order-service-dev, order-service-dev-app, ...)
        - 该 ns 下所有资源被 K8s 自动清理
   b. Environment CR 被删除
   ↓
4. PAAP Server:
   a. 监听到 CR 被删除
   b. 删除 PostgreSQL 记录
   c. 通过 WebSocket 通知前端
```

**用户删除应用：**

```
1. 用户 → PAAP API: DELETE /apps/order-service
   ↓
2. PAAP Server:
   a. 删除 Application CR: order-service (in paap-system)
   b. 标记 PostgreSQL 记录为 deleting
   ↓
3. PAAP Operator:
   a. K8s GC 根据 OwnerReference 级联删除:
      - Namespace: paap-app-order-service
        - Environment CR × N (dev, staging, prod)
          - 业务 Namespace × N
          - ServiceInstance CR × N
          - Component CR × N
          - SA, Role, RoleBinding, Deployment, Service...
   b. Application CR 被删除
   ↓
4. PAAP Server:
   a. 监听到 CR 被删除
   b. 删除 PostgreSQL 记录
   c. 通过 WebSocket 通知前端
```
```

---

## 五、项目结构

```
paap/
├── cmd/
│   ├── server/                  # PAAP 服务端
│   │   └── main.go
│   └── operator/                # PAAP Operator
│       └── main.go
│
├── api/
│   └── v1/
│       ├── application_types.go      # Application CRD Go struct
│       ├── environment_types.go      # Environment CRD Go struct
│       ├── serviceinstance_types.go  # ServiceInstance CRD Go struct
│       ├── component_types.go        # Component CRD Go struct
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
│
├── internal/
│   ├── controller/
│   │   ├── application_controller.go
│   │   ├── environment_controller.go
│   │   ├── serviceinstance_controller.go
│   │   ├── component_controller.go
│   │   └── suite_test.go
│   │
│   ├── server/                  # PAAP 服务端业务逻辑
│   │   ├── handler/
│   │   ├── service/
│   │   ├── model/
│   │   └── repository/
│   │
│   └── reconciler/              # CR 创建/更新逻辑（被 server 调用）
│       ├── application.go       # 创建 Application CR
│       ├── environment.go       # 创建 Environment CR
│       ├── serviceinstance.go   # 创建 ServiceInstance CR
│       └── component.go         # 创建 Component CR
│
├── config/
│   ├── crd/
│   │   ├── bases/
│   │   │   ├── paap.io_applications.yaml
│   │   │   ├── paap.io_environments.yaml
│   │   │   ├── paap.io_serviceinstances.yaml
│   │   │   └── paap.io_components.yaml
│   │   ├── kustomization.yaml
│   │   └── patches/
│   ├── rbac/
│   │   ├── role.yaml
│   │   └── role_binding.yaml
│   ├── manager/
│   │   └── manager.yaml
│   └── samples/
│       ├── application_sample.yaml
│       ├── environment_sample.yaml
│       ├── serviceinstance_sample.yaml
│       └── component_sample.yaml
│
├── deploy/
│   └── k8s/
│       ├── paap-server.yaml
│       ├── paap-operator.yaml
│       ├── paap-crd.yaml
│       └── postgres.yaml
│
└── Makefile                     # 增加 operator 相关 target
```

---

## 六、部署架构

```
┌─────────────────────────────────────────────────────────┐
│  K8s 集群                                                │
│                                                          │
│  Namespace: paap-system                                  │
│  ├── PAAP Server Deployment                              │
│  │   └── paap-server:latest (REST API + 业务逻辑)        │
│  ├── PAAP Operator Deployment                            │
│  │   └── paap-operator:latest (Controller Manager)       │
│  ├── PostgreSQL Deployment                               │
│  ├── CRD 定义 (4 个)                                     │
│  ├── Application CR: order-service                       │
│  └── Application CR: user-center                         │
│                                                          │
│  Namespace: paap-app-order-service                       │
│  ├── Environment CR: dev                                 │
│  ├── Environment CR: staging                             │
│  ├── Environment CR: prod                                │
│  ├── ServiceInstance CR: dev-argocd, dev-tekton, ...     │
│  ├── ServiceInstance CR: staging-argocd, ...             │
│  ├── Component CR: dev-frontend, dev-backend, ...        │
│  └── ...                                                 │
│                                                          │
│  Namespace: paap-app-user-center                         │
│  ├── Environment CR: dev                                 │
│  ├── Environment CR: prod                                │
│  └── ...                                                 │
│                                                          │
│  Namespace: order-service-dev                            │
│  ├── ArgoCD 实例                                         │
│  ├── Tekton 实例                                         │
│  ├── Prometheus 实例                                     │
│  └── 业务组件                                            │
│                                                          │
│  Namespace: order-service-dev-app                        │
│  └── 业务组件（由 ArgoCD 部署）                          │
│                                                          │
│  Namespace: order-service-staging                        │
│  ├── ArgoCD 实例（另一套）                               │
│  └── ...                                                 │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

---

## 七、与现有 ServiceTemplate 的关系

ServiceTemplate 仍然是模板定义，Operator 负责执行。

```
ServiceTemplate（模板定义）
    ↓ PAAP Server 渲染
ServiceInstance CR（期望状态）
    ↓ Operator reconcile
K8s 原生资源（实际状态）
```

**ServiceTemplate 不变**，仍然定义 parameters、rbac、lifecycle。PAAP Server 渲染模板后，将结果写入 ServiceInstance CR 的 spec 中，Operator 根据 spec 创建实际资源。

```yaml
# ServiceInstance CR 的 spec 包含渲染后的结果
spec:
  type: deploy
  parameters:
    version: "v2.10"
  serviceAccount:
    name: order-service-dev-argocd
    namespace: order-service-dev
  envRole:
    rules:
      - apiGroups: ["", "apps", "batch"]
        resources: ["*"]
        verbs: ["*"]
  # 渲染后的部署清单（存储在 ConfigMap 中，避免超过 etcd 1.5MB 限制）
  manifestsRef:
    name: dev-argocd-manifests        # ConfigMap 名称
    namespace: paap-app-order-service  # 和 ServiceInstance CR 同 namespace
    # ConfigMap data 中包含渲染后的 YAML 清单
    # Operator reconcile 时读取 ConfigMap 并 apply
```

---

## 八、补充设计

### 8.1 Operator RBAC 权限

Operator 需要集群级权限管理所有 namespace 的资源：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: paap-operator-manager
rules:
  # 管理 CRD 实例
  - apiGroups: ["paap.io"]
    resources: ["applications", "environments", "serviceinstances", "components"]
    verbs: ["*"]
  - apiGroups: ["paap.io"]
    resources: ["*/status", "*/finalizers"]
    verbs: ["*"]
  # 管理 Namespace
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  # 管理 RBAC
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["*"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings"]
    verbs: ["*"]
  # 管理工作负载
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets"]
    verbs: ["*"]
  - apiGroups: [""]
    resources: ["services", "configmaps", "secrets", "persistentvolumeclaims"]
    verbs: ["*"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["*"]
  # 管理网络
  - apiGroups: ["networking.k8s.io"]
    resources: ["networkpolicies", "ingresses"]
    verbs: ["*"]
  # 管理配额
  - apiGroups: [""]
    resources: ["resourcequotas", "limitranges"]
    verbs: ["*"]
  # 事件记录
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
```

**注意：** Operator 需要 ClusterRole（集群级权限），这是不可避免的，因为它需要跨 namespace 管理资源。但 Operator 只操作带有 `paap.io/app` label 的 namespace，不会碰其他 namespace。

### 8.2 Admission Webhook

使用 kubebuilder 标记自动生成 CRD 验证规则：

```go
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*[a-z0-9]$`
type EnvironmentSpec struct {
    Identifier string `json:"identifier"`
    // ...
}
```

额外需要的 Webhook：
- **验证 Webhook**：检查 Environment CR 引用的 primaryNamespace 格式正确
- **变更 Webhook**：自动填充默认值（如 ResourceQuota 分摊比例）

### 8.3 灾备与恢复

| 场景 | 恢复策略 |
|------|---------|
| PAAP Server 数据库丢失 | 从 K8s 集群中的 CR 反向重建数据库（Operator status 包含完整状态） |
| Operator 长时间宕机 | CR 保持在 etcd 中，Operator 恢复后自动 reconcile 到期望状态 |
| CRD schema 升级 | 使用 conversion webhook 支持多版本 CRD（v1alpha1 → v1） |
| 集群重建 | 备份 CRD + CR（Velero 或 etcd 快照），恢复后 Operator 重建所有资源 |

### 8.4 缺失的模板示例

**Prometheus 部署模板**和 **onEnvNsRemoved/uninstall 模板** 在 ServiceTemplate 规范中未提供完整示例。实现时需要补充，参考 ArgoCD 模板的结构。

---

## 九、总结

| 层 | 职责 | 技术 |
|----|------|------|
| **PAAP Server** | 业务逻辑、API、模板渲染、CR 创建 | Go + Gin + PostgreSQL |
| **PAAP Operator** | 资源管理、状态汇报、漂移修复 | Go + controller-runtime |
| **K8s 集群** | 运行容器、存储状态 | K8s + CRD |

**四个 CRD：**

| CRD | 存放位置 | 职责 |
|-----|---------|------|
| Application | `paap-system` | 应用入口，管理 `paap-app-{id}` namespace |
| Environment | `paap-app-{app}` | 管理业务 namespace、NetworkPolicy、Quota |
| ServiceInstance | `paap-app-{app}` | 管理工具实例（SA, Role, 工具 Deployment） |
| Component | `paap-app-{app}` | 管理业务组件（Deployment, Service） |

**与 ArgoCD 的关系：** PAAP 的 Application（`paap.io/v1`）是业务概念，包含多个环境。ArgoCD 的 Application（`argoproj.io/v1alpha1`）是部署单元，是环境内部的工具资源，由 PAAP 的 ServiceInstance 管理。两者 API Group 不同，不冲突。

**核心原则：PAAP Server 管"想要什么"（CR），Operator 管"怎么实现"（K8s 资源）。**
