# 环境交互与工具权限方案（修订版）

> 本文档描述环境交互和工具权限的设计方案。具体实现由 PAAP Operator 负责，详见 [Operator 设计方案](operator-design.md)。

## 一、核心设计变更

### 原方案 vs 新方案

| 维度 | 原方案 | 新方案 |
|------|--------|--------|
| 工具部署模式 | 共享实例 + AppProject 隔离 | **每环境独立实例** |
| ArgoCD | 1个集群1个，通过 AppProject 限制 | 每个环境1个 ArgoCD |
| Jenkins/Tekton | 共享，通过 SA 隔离 | 每个环境1个 Jenkins/Tekton |
| Prometheus | 共享或每应用1个 | 每个环境1个 Prometheus |
| 环境创建 | 创建时预装所有服务 | **支持空环境，按需装工具** |
| 用户交互 | 固定模板 | **交互式画布，右键配置** |

---

## 二、环境生命周期

### 2.1 两种创建模式

```
创建环境
├── 从模板创建（快速开始）
│   → 创建环境 namespace，安装模板中定义的工具和基础设施
│
└── 创建空环境（空白画布）
    → 只创建环境 namespace，工具按需安装
    → 用户自己选择装什么工具、建什么组件
```

注意：环境 ≠ namespace。每个环境创建一个主 namespace，工具安装在该主 namespace 内。用户后续可根据需要为环境附加更多 namespace（工作负载空间、专用空间等）。所有工具的 RBAC 自动覆盖该环境的全部 namespace。

**名称与标识：** 所有资源都有名称（展示用，可中文）和标识（K8s 资源用，纯英文）。例如：应用名称「订单服务」→ 标识 `order-service`，环境名称「开发环境」→ 标识 `dev`，工具名称「部署服务」→ 标识 `argocd`。Namespace 命名规则：`{应用标识}-{环境标识}`（主空间），`{应用标识}-{环境标识}-{用途}`（附加空间）。

### 2.2 空环境的状态

```
┌─────────────────────────────────────────────────────┐
│  订单服务 - 开发环境（空环境）                        │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌─ 工具 ──────────────────────────────────────────┐│
│  │                                                  ││
│  │  [部署服务]  [CI服务]  [监控服务]  [+ 添加更多]  ││
│  │                                                  ││
│  │  当前未安装任何工具，点击上方按钮安装             ││
│  └──────────────────────────────────────────────────┘│
│                                                      │
│  ┌─ 组件 ──────────────────────────────────────────┐│
│  │                                                  ││
│  │  [+ 创建组件]                                    ││
│  │                                                  ││
│  │  当前没有组件，点击上方创建                       ││
│  └──────────────────────────────────────────────────┘│
│                                                      │
└─────────────────────────────────────────────────────┘
```

### 2.3 装好工具、组件和基础设施后的状态

环境名字由用户自定义（如"开发环境"、"联调环境"、"UAT验收"），不限于 dev/staging/prod。

```
┌─────────────────────────────────────────────────────┐
│  订单服务 > 开发环境                                 │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌─ 工具 ──────────────────────────────────────────┐│
│  │                                                  ││
│  │  ● 部署服务   ● CI服务   ● 监控服务   [+ 添加]  ││
│  │                                                  ││
│  └──────────────────────────────────────────────────┘│
│                                                      │
│  ┌─ 组件 ──────────────────────────────────────────┐│
│  │                                                  ││
│  │  ┌──────────┐  ┌──────────┐                     ││
│  │  │ 前端服务  │  │ 后端服务  │                     ││
│  │  │ ● 运行中 │  │ ● 运行中 │                     ││
│  │  │ v1.3     │  │ v1.3     │                     ││
│  │  │ 2副本    │  │ 2副本    │                     ││
│  │  │ 右键菜单▼│  │ 右键菜单▼│                     ││
│  │  └──────────┘  └──────────┘                     ││
│  │                                                  ││
│  │  [+ 创建组件]                                    ││
│  └──────────────────────────────────────────────────┘│
│                                                      │
│  ┌─ 基础设施 ──────────────────────────────────────┐│
│  │                                                  ││
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐      ││
│  │  │PostgreSQL│  │  Redis   │  │ RabbitMQ │      ││
│  │  │ ● 运行中 │  │ ● 运行中 │  │ ● 运行中 │      ││
│  │  │ v14.2    │  │ v7.0     │  │ v3.12    │      ││
│  │  │ 存储:20G │  │ 内存:2G  │  │ 队列:5个 │      ││
│  │  │ 右键菜单▼│  │ 右键菜单▼│  │ 右键菜单▼│      ││
│  │  └──────────┘  └──────────┘  └──────────┘      ││
│  │                                                  ││
│  │  [+ 添加基础设施]                                ││
│  └──────────────────────────────────────────────────┘│
│                                                      │
└─────────────────────────────────────────────────────┘
```

**三类资源说明：**
- **工具**：管理类服务（ArgoCD/Jenkins/Prometheus），帮助用户管理部署、构建、监控
- **组件**：用户自己的业务服务（前端/后端等），是应用的核心
- **基础设施**：为组件提供运行所需的底层能力（数据库/缓存/消息队列等），连接信息自动注入组件环境变量

---

## 三、组件右键菜单

### 3.1 右键菜单设计

```
右键点击组件「后端服务」
┌─────────────────────────┐
│  查看详情               │
│  查看日志               │
│  ─────────────────────  │
│  配置部署               │
│  配置监控               │
│  配置访问地址           │
│  配置环境变量           │
│  ─────────────────────  │
│  扩缩容                 │
│  重启                   │
│  停止                   │
│  ─────────────────────  │
│  删除组件               │
└─────────────────────────┘
```

### 3.2 各菜单项说明

| 菜单项 | 功能 | 前置条件 |
|--------|------|----------|
| 查看详情 | 显示组件的版本、副本、资源使用、运行状态 | 无 |
| 查看日志 | 显示组件各实例的运行日志 | 已安装部署服务或监控服务 |
| 配置部署 | 配置镜像来源、副本数、资源配额、自动部署规则 | 已安装部署服务 |
| 配置监控 | 为该组件启用/配置监控指标采集 | 已安装监控服务 |
| 配置访问地址 | 配置 Ingress/域名/端口映射 | 无 |
| 配置环境变量 | 管理组件的环境变量和配置项 | 无 |
| 扩缩容 | 调整副本数 | 已安装部署服务 |
| 重启 | 滚动重启组件 | 已安装部署服务 |
| 停止 | 将副本数设为0 | 已安装部署服务 |
| 删除组件 | 删除组件及其所有资源 | 无 |

### 3.3 配置监控的交互流程

```
右键 → 配置监控
┌─────────────────────────────────────┐
│  配置监控 - 后端服务                 │
│                                     │
│  采集指标:                          │
│  ☑ CPU / 内存使用率                │
│  ☑ 请求量（QPS）                   │
│  ☑ 响应延迟（P50/P95/P99）         │
│  ☑ 错误率                          │
│  ☐ 自定义指标 [指标名________]      │
│                                     │
│  告警规则:                          │
│  ☑ CPU > 80% 持续5分钟 → 通知      │
│  ☑ 内存 > 90% 持续5分钟 → 通知     │
│  ☑ 错误率 > 5% 持续3分钟 → 通知    │
│  ☐ 自定义规则 [添加规则]            │
│                                     │
│  [保存配置 →]                       │
└─────────────────────────────────────┘
```

### 3.4 配置部署的交互流程

```
右键 → 配置部署
┌─────────────────────────────────────┐
│  配置部署 - 后端服务                 │
│                                     │
│  镜像来源:                          │
│  仓库地址  [registry.internal.com]  │
│  镜像名称  [order-service/backend]  │
│  版本策略:                          │
│  ○ 固定版本  [v1.3 ▾]              │
│  ○ 始终最新（latest）               │
│  ○ Git标签触发                      │
│                                     │
│  副本数量:  [2]  [+][-]             │
│                                     │
│  资源配置:                          │
│  CPU    [0.5核 ▾]                   │
│  内存   [512MB ▾]                   │
│                                     │
│  自动部署:                          │
│  ☑ 镜像更新时自动部署               │
│  ☐ 仅在工作时间段部署               │
│                                     │
│  [保存配置 →]                       │
└─────────────────────────────────────┘
```

---

## 四、工具安装与权限自动分配

### 4.1 工具安装流程

用户在环境内点击「添加工具」：

```
点击 [+ 添加工具]
┌─────────────────────────────────────┐
│  选择要安装的工具                    │
│                                     │
│  ┌──────────────┐ ┌──────────────┐ │
│  │ 部署服务     │ │ CI服务       │ │
│  │              │ │              │ │
│  │ 管理应用的   │ │ 自动构建和   │ │
│  │ 部署、版本、 │ │ 测试你的代码 │ │
│  │ 回滚         │ │              │ │
│  │              │ │              │ │
│  │ [安装]       │ │ [安装]       │ │
│  └──────────────┘ └──────────────┘ │
│                                     │
│  ┌──────────────┐ ┌──────────────┐ │
│  │ 监控服务     │ │ 日志服务     │ │
│  │              │ │              │ │
│  │ 监控资源使用 │ │ 收集和查询   │ │
│  │ 和应用健康   │ │ 应用日志     │ │
│  │              │ │              │ │
│  │ [安装]       │ │ [安装]       │ │
│  └──────────────┘ └──────────────┘ │
└─────────────────────────────────────┘
```

### 4.2 安装「部署服务」后台自动执行

用户点击「安装」后，平台后台执行以下操作（用户不可见）：

```
用户点击「安装部署服务」（在「开发环境」内）
  ↓
1. 在环境主 namespace 创建 ArgoCD 实例
   - Deployment: argocd-server（仅在本环境可见）
   - StatefulSet: argocd-application-controller
   - ConfigMap: argocd-cm, argocd-rbac-cm
   - Service: argocd-server, argocd-repo-server
  ↓
2. 创建 ServiceAccount: order-service-dev-argocd
   - namespace: order-service-dev（环境主 namespace）
   - SA 命名规则: {应用标识}-{环境标识}-{工具类型}
  ↓
3. 在环境主 namespace 创建 Role + RoleBinding
   - Role: argocd-manager (full access)
   - 绑定到 order-service-dev-argocd SA
  ↓
4. 遍历该环境所有附加 namespace（如 dev-cache）
   - 在每个附加 namespace 创建 Role + RoleBinding
   - 将主 namespace 的 order-service-dev-argocd SA 授权过去
  ↓
5. 配置 ArgoCD 只管理当前环境的 namespace
   - argocd-cm: application.namespaces = [order-service-dev, order-service-dev-cache]
   - AppProject: destinations 限定为当前环境的 namespace 列表
  ↓
6. 环境页面刷新，「部署服务」变为可用状态

注意：每个环境的 ArgoCD 是完全独立的实例。
开发环境的 ArgoCD（order-service-dev-argocd）和测试环境的 ArgoCD（order-service-staging-argocd）
是两套完全独立的实例，互不干扰。
```

### 4.3 安装「CI服务」后台自动执行

```
用户点击「安装CI服务」（在「开发环境」内）
  ↓
1. 在环境主 namespace 创建 Tekton/Jenkins 实例
   - 如果 Tekton: 安装 Tekton Pipelines + Triggers
   - 如果 Jenkins: Deployment + Service
  ↓
2. 创建 ServiceAccount: order-service-dev-tekton
   - namespace: order-service-dev（环境主 namespace）
   - SA 命名规则: {应用标识}-{环境标识}-{工具类型}
  ↓
3. 在环境主 namespace 创建 Role + RoleBinding
   - 授予 pods/deployments/services/configmaps/secrets 等权限
  ↓
4. 遍历该环境所有附加 namespace
   - 在每个附加 namespace 创建 Role + RoleBinding 引用该 SA
  ↓
5. 环境页面刷新，「CI服务」变为可用状态
```

### 4.4 安装「监控服务」后台自动执行

```
用户点击「安装监控服务」（在「开发环境」内）
  ↓
1. 在环境主 namespace 创建 Prometheus + Grafana 实例
   - Prometheus: 只配置采集该环境所有 namespace 的指标
   - Grafana: 数据源只指向本环境的 Prometheus
  ↓
2. 创建 ServiceAccount: order-service-dev-prometheus
   - namespace: order-service-dev（环境主 namespace）
   - SA 命名规则: {应用标识}-{环境标识}-{工具类型}
  ↓
3. 在环境主 namespace 创建 Role + RoleBinding
   - 只读权限: pods/services/endpoints/endpointslices
  ↓
4. 遍历该环境所有附加 namespace
   - 在每个附加 namespace 创建 Role + RoleBinding（只读）
   - 更新 Prometheus 采集配置，加入那些 namespace
  ↓
5. 环境页面刷新，「监控服务」变为可用状态
```

---

## 五、创建新环境时的权限自动分配

### 5.1 场景：用户为应用创建新环境

```
应用「订单服务」已有:
  - 开发环境 (order-service-dev) — 已装 ArgoCD + Tekton + Prometheus

用户创建新环境: 测试环境 (order-service-staging)
```

### 5.2 平台自动执行的流程

```
1. 创建 namespace: order-service-staging
   打标签: paap.io/app=order-service, paap.io/env=staging
   ↓
2. 根据选择的环境模板安装工具（每环境独立实例）:
   - 模板定义: [deploy, ci, monitor]
   - 为每个工具执行 install 流程:

   ArgoCD（全新实例）:
   - 部署 ArgoCD 到 order-service-staging
   - 创建 SA: order-service-staging-argocd (namespace: order-service-staging)
   - 在 order-service-staging 创建 Role + RoleBinding
   → 结果: 测试环境有自己独立的 ArgoCD

   Tekton（全新实例）:
   - 部署 Tekton 到 order-service-staging
   - 创建 SA: order-service-staging-tekton (namespace: order-service-staging)
   - 在 order-service-staging 创建 Role + RoleBinding
   → 结果: 测试环境有自己独立的 Tekton

   Prometheus（全新实例）:
   - 部署 Prometheus 到 order-service-staging
   - 创建 SA: order-service-staging-prometheus (namespace: order-service-staging)
   - 在 order-service-staging 创建 Role + RoleBinding
   → 结果: 测试环境有自己独立的 Prometheus
   ↓
3. 创建 ResourceQuota（资源配额）
   ↓
4. 创建 NetworkPolicy（可选）
   ↓
5. 新环境就绪
   - 每个工具都是全新独立的实例，与开发环境完全隔离
   - 工具权限只覆盖新环境自己的 namespace
```

### 5.3 如果新环境选择「空环境」

```
用户创建空环境: 预发布环境 (order-service-pre)
  ↓
1. 创建 namespace: order-service-pre
   ↓
2. 不安装任何工具（空环境）
   ↓
3. 环境页面展示为「空画布」
   → 用户可以按需装工具、建组件
   → 安装工具时，自动创建 SA + Role + RoleBinding
```

---

## 六、工具实例架构

### 6.1 每环境独立实例的资源开销

```
订单服务 - 开发环境 (含 dev + dev-cache 两个 namespace)
├── ArgoCD 实例 (在 dev namespace 中)
│   ├── argocd-server: 1 Pod (0.5 CPU, 512Mi)
│   ├── argocd-application-controller: 1 Pod (0.5 CPU, 512Mi)
│   ├── argocd-repo-server: 1 Pod (0.5 CPU, 512Mi)
│   └── argocd-redis: 1 Pod (0.25 CPU, 256Mi)
│   小计: ~2 CPU, 2Gi 内存
│
├── Tekton 实例 (在 dev namespace 中)
│   ├── tekton-pipelines-controller: 1 Pod (共享集群级)
│   └── tekton-triggers-controller: 1 Pod (共享集群级)
│   小计: 控制器共享，每个 PipelineRun 动态创建 ~0.5 CPU
│
└── Prometheus + Grafana (在 dev namespace 中)
    ├── prometheus: 1 Pod (0.5 CPU, 1Gi)
    └── grafana: 1 Pod (0.25 CPU, 256Mi)
    小计: ~0.75 CPU, 1.25Gi 内存

单个环境总计: ~3-4 CPU, 4-5Gi 内存（可接受）

注：生产环境如果没有 CI，会更轻量（省掉 Tekton）
```

### 6.2 工具实例命名规则

```
{应用标识}-{环境标识}-{工具类型}

示例:
  order-service-dev-argocd          (订单服务-开发环境-ArgoCD)
  order-service-dev-tekton          (订单服务-开发环境-Tekton)
  order-service-dev-prometheus      (订单服务-开发环境-Prometheus)
  order-service-dev-grafana         (订单服务-开发环境-Grafana)
  order-service-staging-argocd      (订单服务-测试环境-ArgoCD)
  order-service-prod-argocd         (订单服务-生产环境-ArgoCD)
  order-service-prod-prometheus     (订单服务-生产环境-Prometheus)
```

### 6.3 工具实例所在 namespace

工具实例部署在该环境的**主 namespace** 中，通过 RBAC 管理该环境的所有 namespace：

```
order-service-dev namespace (开发环境主 namespace):
├── 工具实例
│   ├── order-service-dev-argocd
│   ├── order-service-dev-tekton
│   └── order-service-dev-prometheus
├── 业务组件
│   ├── frontend
│   └── backend
├── 基础设施
│   └── PostgreSQL
└── RBAC
    ├── Role: argocd-manager (本 namespace)
    ├── Role: ci-pipeline-role (本 namespace)
    └── Role: prometheus-reader (本 namespace)

order-service-dev-cache namespace (开发环境附加 namespace):
├── 缓存组件
└── RBAC
    ├── Role: argocd-manager → RoleBinding → SA: order-service-dev-argocd (dev ns)
    ├── Role: ci-pipeline-role → RoleBinding → SA: order-service-dev-tekton (dev ns)
    └── Role: prometheus-reader → RoleBinding → SA: order-service-dev-prometheus (dev ns)

order-service-staging namespace (测试环境主 namespace):
├── 工具实例（另一套，互不干扰）
│   ├── order-service-staging-argocd
│   ├── order-service-staging-tekton
│   └── order-service-staging-prometheus
├── 业务组件
└── RBAC (只在 staging 域内)

order-service-prod namespace (生产环境主 namespace):
├── 工具实例（只有 CD 和监控，没有 CI）
│   ├── order-service-prod-argocd
│   └── order-service-prod-prometheus
├── 业务组件
└── RBAC (只在 prod 域内)
```

---

## 七、RBAC 模板（每个工具的标准权限集）

### 7.1 ArgoCD SA 权限

```yaml
# 每个环境的 ArgoCD 使用的 Role
# 在该环境的每个 namespace 中都创建一份
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: argocd-manager
  namespace: {namespace}
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
    resources: ["*"]
    verbs: ["*"]
  # 禁止操作 RBAC 和 NetworkPolicy
  # (通过 ClusterRole 的 deny list 或 Kyverno 策略强制执行)
```

### 7.2 Tekton/Jenkins CI SA 权限

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-pipeline-role
  namespace: {namespace}
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
    verbs: ["get", "list"]  # 只读，不能修改 Secret
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["get", "list"]  # 只读
```

### 7.3 Prometheus SA 权限

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-reader
  namespace: {namespace}
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    # 注：nodes 是集群级资源，namespaced Role 无法授权，需用 ClusterRole
    verbs: ["get", "list", "watch"]
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch"]  # 只读
```

---

## 八、关键交互流程汇总

### 8.1 从空环境到完整应用

```
1. 创建应用「订单服务」
   → 创建 namespace: order-service-dev
   → 进入空环境画布

2. 点击「添加工具」→ 选择「部署服务」→ 安装
   → 后台创建 ArgoCD 实例 + SA + Role/RoleBinding
   → 部署服务变为可用

3. 点击「添加工具」→ 选择「CI服务」→ 安装
   → 后台创建 Tekton 实例 + SA + Role/RoleBinding
   → CI服务变为可用

4. 点击「添加工具」→ 选择「监控服务」→ 安装
   → 后台创建 Prometheus/Grafana + SA + Role/RoleBinding
   → 监控服务变为可用

5. 点击「创建组件」→ 输入名称「前端服务」
   → 创建 Deployment + Service
   → 组件卡片出现

6. 右键「前端服务」→ 配置部署
   → 设置镜像、副本数、资源
   → ArgoCD 开始管理该组件

7. 右键「前端服务」→ 配置监控
   → 启用指标采集、告警规则
   → Prometheus 开始采集该组件指标

8. 重复 5-7 创建「后端服务」「数据库」等组件

9. 创建新环境「测试环境」
   → 平台根据模板为新环境安装独立的工具实例（ArgoCD/Tekton/Prometheus）
   → 每个工具的权限只覆盖新环境自己的 namespace
   → 用户可在新环境复用相同的组件配置
```

### 8.2 日常操作

```
开发者推送代码
  ↓
Tekton 流水线自动触发（已配置好）
  ↓
构建成功 → ArgoCD 自动部署到开发环境
  ↓
应用管理员在监控页面看到开发环境运行正常
  ↓
手动触发「部署到测试环境」
  ↓
ArgoCD 部署到 order-service-staging
  ↓
在监控页面切换到测试环境，查看运行状态
```

---

## 九、权限分配的自动触发时机

| 触发事件 | 平台自动执行 |
|----------|-------------|
| 创建应用 | 初始化应用元数据 |
| 创建环境 | 创建 namespace + 根据模板安装工具实例（或空环境） |
| 安装工具（如 ArgoCD） | 在环境主 namespace 创建工具实例 + SA + Role/RoleBinding，遍历附加 namespace 创建 Role/RoleBinding |
| 环境新增 namespace | 为该环境所有已安装工具在新 namespace 创建 Role + RoleBinding |
| 环境删除 namespace | 清理工具权限 + 删除 namespace（K8s 自动清理剩余 RBAC） |
| 卸载工具 | 删除工具实例 + SA + 遍历所有 namespace 删除 Role/RoleBinding |

---

## 十、安全边界总结

```
开发环境的工具只能操作:
  ✓ order-service-dev
  ✓ order-service-dev-cache
  ✗ order-service-staging       (其他环境)
  ✗ order-service-prod          (其他环境)
  ✗ user-center-dev             (其他应用)
  ✗ kube-system                 (系统 namespace)

测试环境的工具只能操作:
  ✓ order-service-staging
  ✓ order-service-staging-cache
  ✗ order-service-dev           (其他环境)
  ✗ order-service-prod          (其他环境)
  ✗ user-center-dev             (其他应用)
  ✗ kube-system                 (系统 namespace)

每种工具的权限范围（以开发环境为例）:
  ArgoCD   → 可以在 dev, dev-cache 部署/更新/删除 Deployment, Service, ConfigMap 等
  Tekton   → 可以在 dev, dev-cache 创建 Pod, Job, 部署 Deployment
  Prometheus → 只能在 dev, dev-cache 读取 Pod, Service, Endpoint 信息
```

这就是「每环境独立实例 + 自动权限分配」的完整方案。核心思路：**用户管业务，平台管权限，工具只碰自己环境的地盘。**
