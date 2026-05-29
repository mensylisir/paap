# PAAP 技术选型

## 一、整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户浏览器                                │
└──────────────────────────┬──────────────────────────────────────┘
                           │ HTTPS
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Nginx / Ingress Controller                    │
│                    (反向代理 + 静态资源)                          │
└──────────┬──────────────────────────────────┬───────────────────┘
           │ /api/*                           │ /*
           ▼                                  ▼
┌─────────────────────┐            ┌─────────────────────┐
│   PAAP Server       │            │   PAAP Frontend     │
│   (Go + Gin)        │            │   (Vue 3 + Vite)    │
│   Port: 9090        │            │   Port: 5173 (dev)  │
└──────────┬──────────┘            └─────────────────────┘
           │
           ├──→ PostgreSQL (业务数据)
           ├──→ Redis (缓存 + 会话，可选)
           └──→ K8s API Server (创建/管理 CR)
                    ▲
                    │
┌───────────────────┴─────────────────────────────────────────────┐
│                    PAAP Operator                                 │
│                    (Go + controller-runtime + Kubebuilder)       │
│                    监听 CR → 管理 K8s 原生资源                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## 二、后端技术栈

### 2.1 PAAP Server（API 服务）

| 技术 | 版本 | 用途 |
|------|------|------|
| **Go** | 1.22+ | 主语言 |
| **Gin** | v1.9+ | HTTP 框架，REST API |
| **gorm** | v1.25+ | ORM，操作 PostgreSQL |
| **go-redis** | v9+ | Redis 客户端（可选） |
| **client-go** | 与 K8s 版本匹配 | K8s API 客户端，创建/管理 CR |
| **controller-runtime** | 与 Kubebuilder 匹配 | 共享 Operator 的 CRD 类型定义 |
| **JWT (golang-jwt)** | v5 | 用户认证 |
| **casbin** | v2 | RBAC 权限控制（平台级，非 K8s RBAC） |
| **zap** | v1.27+ | 结构化日志 |

**职责：**
- REST API（应用/环境/服务/组件的 CRUD）
- 用户认证（JWT）与权限（casbin）
- 模板管理（ServiceTemplate / EnvTemplate 的 CRUD，存储在 PostgreSQL）
- 模板渲染（Go text/template + Sprig，生成 K8s YAML）
- CR 管理（创建/更新/删除 Environment/ServiceInstance/Component CR）
- CR 状态同步（通过 K8s Informer 监听 CR status，同步到 PostgreSQL）
- WebSocket 推送（实时状态更新给前端）

### 2.2 PAAP Operator（控制平面）

| 技术 | 版本 | 用途 |
|------|------|------|
| **Go** | 1.22+ | 主语言 |
| **Kubebuilder** | v3.14+ | 脚手架，生成 CRD/Controller/Webhook 代码 |
| **controller-runtime** | v0.17+ | Controller 框架 |
| **client-go** | 与 K8s 版本匹配 | K8s API 操作 |

**职责：**
- 监听 4 个 CRD（Application, Environment, ServiceInstance, Component）
- 管理 K8s 原生资源（Namespace, SA, Role, RoleBinding, Deployment, Service, ConfigMap, NetworkPolicy, ResourceQuota）
- Finalizer 级联删除
- 状态汇报到 CR status
- K8s Event 记录

### 2.3 数据存储

| 存储 | 用途 | 必需 |
|------|------|------|
| **PostgreSQL 16** | 业务数据（应用、环境、用户、模板、审计日志） | 是 |
| **etcd** (K8s 内置) | CR 状态存储（由 K8s 管理） | 是（K8s 自带） |
| **Redis 7** | 会话缓存、CR status 缓存、模板渲染缓存 | 否，可选 |

**PostgreSQL 核心表：**

```
┌─────────────────────────────────────────────────────────┐
│  users              │ 用户表                             │
│  roles              │ 角色表                             │
│  user_roles         │ 用户角色关联                       │
│  applications       │ 应用表（标识、名称、描述）          │
│  environments       │ 环境表（关联应用、模板、CR 名）     │
│  service_templates  │ 服务模板表（YAML 定义存储）         │
│  env_templates      │ 环境模板表（引用服务模板列表）       │
│  service_instances  │ 服务实例表（关联环境、类型、参数）   │
│  components         │ 组件表（关联环境、镜像、副本）       │
│  audit_logs         │ 审计日志表                          │
└─────────────────────────────────────────────────────────┘
```

**为什么模板存在 PostgreSQL 而不是 CRD？**
- 模板是平台配置数据，由管理员通过 UI 维护
- Operator 不需要 watch 模板变化
- 模板渲染在 PAAP Server 完成，结果写入 CR
- 数据库支持版本管理、审计、回滚

**为什么需要 PostgreSQL 而不是只用 CRD？**
- CRD 状态由 Operator 管理，但业务数据（用户、权限、审计）需要关系型存储
- CR 查询能力有限（不支持复杂 JOIN、聚合）
- 前端分页、搜索、排序需要数据库支持
- 数据备份/恢复更简单

**Redis 是否必需？**
- **不必需**，但建议使用：
  - JWT Token 黑名单（登出/过期）
  - CR status 缓存（减少 K8s API 调用）
  - 模板渲染结果缓存（相同参数不重复渲染）
  - WebSocket 会话管理
- 如果不用 Redis，可以用内存缓存（go-cache）替代小规模场景

---

## 三、前端技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| **Vue 3** | 3.4+ | 框架 |
| **Vite** | 5.x | 构建工具 |
| **TypeScript** | 5.x | 类型安全 |
| **Carbon Design System** | @carbon/vue3 | UI 组件库 |
| **Pinia** | 2.x | 状态管理 |
| **Vue Router** | 4.x | 路由 |
| **Axios** | 1.x | HTTP 客户端 |
| **ECharts** | 5.x | 图表（监控仪表盘） |
| **xterm.js** | 5.x | 终端（日志查看） |

**职责：**
- 应用/环境/服务/组件的 CRUD 界面
- 环境画布（交互式拖拽）
- 部署/CI/监控服务的表单化配置
- 实时状态展示（WebSocket）
- 监控仪表盘（ECharts）

---

## 四、K8s 集群

### 4.1 开发环境

| 技术 | 用途 |
|------|------|
| **kind** | 本地 K8s 集群 |
| **Docker Desktop** | 容器运行时 |

### 4.2 生产环境

| 技术 | 用途 |
|------|------|
| **K8s** | 1.28+ |
| **Calico** | CNI（可选，用于 IP Pool） |
| **Nginx Ingress** | 入口流量管理 |
| **MetalLB** | 裸机 LoadBalancer（可选） |

### 4.3 Operator 部署

```
Namespace: paap-system
├── PAAP Server Deployment (1 replica)
│   └── paap-server:latest
├── PAAP Operator Deployment (1 replica)
│   └── paap-operator:latest
├── PostgreSQL Deployment (1 replica, 生产环境建议外部托管)
│   └── postgres:16-alpine
├── Redis Deployment (可选, 1 replica)
│   └── redis:7-alpine
├── Service: paap-server (ClusterIP)
├── Service: paap-postgres (ClusterIP)
├── Service: paap-redis (ClusterIP, 可选)
├── Secret: paap-db-credentials
├── Secret: paap-jwt-secret
├── ConfigMap: paap-config
└── CRD 定义 (4 个)
    ├── paap.io_applications.yaml
    ├── paap.io_environments.yaml
    ├── paap.io_serviceinstances.yaml
    └── paap.io_components.yaml
```

---

## 五、项目结构

```
paap/
├── cmd/
│   ├── server/                      # PAAP Server 入口
│   │   └── main.go
│   └── operator/                    # PAAP Operator 入口
│       └── main.go
│
├── api/                             # Kubebuilder 生成的 CRD 类型
│   └── v1/
│       ├── application_types.go
│       ├── environment_types.go
│       ├── serviceinstance_types.go
│       ├── component_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
│
├── internal/
│   ├── controller/                  # Operator Controller
│   │   ├── application_controller.go
│   │   ├── environment_controller.go
│   │   ├── serviceinstance_controller.go
│   │   ├── component_controller.go
│   │   └── suite_test.go
│   │
│   ├── server/                      # PAAP Server 业务逻辑
│   │   ├── handler/                 # Gin HTTP Handler
│   │   │   ├── app.go
│   │   │   ├── env.go
│   │   │   ├── service.go
│   │   │   ├── component.go
│   │   │   ├── template.go
│   │   │   └── user.go
│   │   ├── service/                 # 业务 Service 层
│   │   │   ├── app_service.go
│   │   │   ├── env_service.go
│   │   │   ├── template_renderer.go # 模板渲染引擎
│   │   │   └── cr_manager.go        # CR 创建/更新
│   │   ├── model/                   # GORM 数据模型
│   │   │   ├── application.go
│   │   │   ├── environment.go
│   │   │   ├── service_template.go
│   │   │   ├── env_template.go
│   │   │   ├── user.go
│   │   │   └── audit_log.go
│   │   ├── repository/              # 数据访问层
│   │   ├── middleware/              # Gin 中间件
│   │   │   ├── auth.go              # JWT 认证
│   │   │   ├── rbac.go              # casbin 权限
│   │   │   └── logger.go            # 请求日志
│   │   └── ws/                      # WebSocket
│   │       └── hub.go               # 状态推送
│   │
│   └── config/                      # 配置
│       └── config.go
│
├── config/                          # Kubebuilder 生成的 K8s 配置
│   ├── crd/
│   │   ├── bases/
│   │   └── kustomization.yaml
│   ├── rbac/
│   ├── manager/
│   ├── webhook/
│   └── samples/
│
├── deploy/
│   └── k8s/
│       ├── paap-server.yaml
│       ├── paap-operator.yaml
│       ├── paap-crd.yaml
│       ├── paap-postgres.yaml
│       └── paap-redis.yaml
│
├── frontend/                        # Vue 前端
│   ├── src/
│   │   ├── views/
│   │   ├── components/
│   │   ├── stores/                  # Pinia
│   │   ├── router/
│   │   ├── api/                     # Axios 封装
│   │   └── types/
│   ├── package.json
│   └── vite.config.ts
│
├── Makefile
├── Dockerfile
├── go.mod
└── go.sum
```

---

## 六、构建与运行

### 6.1 开发环境

```bash
# 1. 启动 K8s 集群
kind create cluster --name paap-dev

# 2. 安装 CRD
make install

# 3. 启动 Operator（本地运行）
make run-operator

# 4. 启动 Server（本地运行）
make run-server

# 5. 启动前端（本地运行）
cd frontend && npm run dev
```

### 6.2 构建镜像

```bash
# 构建 Server 镜像
docker build -t paap-server:latest -f Dockerfile.server .

# 构建 Operator 镜像
docker build -t paap-operator:latest -f Dockerfile.operator .

# 推送到镜像仓库
docker push paap-server:latest
docker push paap-operator:latest
```

### 6.3 部署到集群

```bash
# 1. 安装 CRD
kubectl apply -f deploy/k8s/paap-crd.yaml

# 2. 部署 PostgreSQL
kubectl apply -f deploy/k8s/paap-postgres.yaml

# 3. 部署 Redis（可选）
kubectl apply -f deploy/k8s/paap-redis.yaml

# 4. 部署 Operator
kubectl apply -f deploy/k8s/paap-operator.yaml

# 5. 部署 Server
kubectl apply -f deploy/k8s/paap-server.yaml
```

---

## 七、依赖版本锁定

```go
// go.mod 关键依赖
module github.com/paap/paap

go 1.22

require (
    // Web 框架
    github.com/gin-gonic/gin v1.9.1
    github.com/gin-contrib/cors v1.7.2

    // ORM
    gorm.io/gorm v1.25.10
    gorm.io/driver/postgres v1.5.9

    // Redis（可选）
    github.com/redis/go-redis/v9 v9.5.1

    // K8s
    k8s.io/api v0.29.3
    k8s.io/apimachinery v0.29.3
    k8s.io/client-go v0.29.3
    sigs.k8s.io/controller-runtime v0.17.3

    // 认证
    github.com/golang-jwt/jwt/v5 v5.2.1
    github.com/casbin/casbin/v2 v2.97.0

    // 模板
    github.com/Masterminds/sprig/v3 v3.2.3

    // 日志
    go.uber.org/zap v1.27.0

    // 配置
    github.com/spf13/viper v1.18.2
)
```

---

## 八、与 ArgoCD/Tekton/Prometheus 的集成方式

PAAP **不直接集成**这些工具的 API，而是通过 **K8s CRD + Operator** 管理：

```
PAAP Server
  ↓ 创建 ServiceInstance CR（type=deploy, parameters={version: v2.10}）
PAAP Operator
  ↓ 读取 manifestsRef ConfigMap
  ↓ 创建 ArgoCD Deployment/Service/ConfigMap/SA/Role/RoleBinding
ArgoCD 实例运行在环境 namespace 中
  ↓ 用户「配置部署」时
PAAP Server 创建 ArgoCD Application CR（argoproj.io/v1alpha1）
  ↓
ArgoCD Controller 消费 Application CR，部署业务组件
```

**PAAP 不调用 ArgoCD/Tekton/Prometheus 的 REST API**，全部通过 K8s 原生方式管理。
