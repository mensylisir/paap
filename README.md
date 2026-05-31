# PAAP - Platform-as-a-Application Platform

以应用为中心的自助式云原生应用管理平台。

## 架构

```
┌─────────────────────────────────────────────────┐
│                   PAAP Frontend                  │
│               (Vue 3 + Carbon Design)            │
└──────────────────────┬──────────────────────────┘
                       │ REST API + WebSocket
                       ▼
┌─────────────────────────────────────────────────┐
│                   PAAP Server                    │
│              (Go + Gin + GORM + SQLite)          │
└──────────────────────┬──────────────────────────┘
                       │ 创建/管理 CR
                       ▼
┌─────────────────────────────────────────────────┐
│                  PAAP Operator                   │
│         (Go + controller-runtime + Kubebuilder)  │
│  ┌──────────────┐ ┌──────────────┐              │
│  │ Application  │ │ Environment  │              │
│  │ Controller   │ │ Controller   │              │
│  └──────────────┘ └──────────────┘              │
│  ┌──────────────┐ ┌──────────────┐              │
│  │ ServiceInst  │ │ Component    │              │
│  │ Controller   │ │ Controller   │              │
│  └──────────────┘ └──────────────┘              │
└──────────────────────┬──────────────────────────┘
                       │ 管理 K8s 原生资源
                       ▼
┌─────────────────────────────────────────────────┐
│                 K8s 集群                         │
│  Namespace × N │ SA/Role/RoleBinding             │
│  Deployment    │ Service/ConfigMap               │
│  NetworkPolicy │ ResourceQuota                   │
└─────────────────────────────────────────────────┘
```

## 部署架构

**重要规范：所有 PAAP 平台组件统一部署在 `paap-system` namespace**

| 组件 | Namespace | 说明 |
|-----|-----------|------|
| PAAP Server | `paap-system` | API 服务器 |
| PAAP Operator | `paap-system` | CRD 控制器 |
| PostgreSQL | `paap-system` | 元数据存储 |
| MinIO | `paap-system` | Chart 模板存储 |
| CRD 实例 | `paap-system` / `paap-app-{id}` | Application 在 paap-system，其他在应用 namespace |

## CRD 定义

| CRD | 存放位置 | 职责 |
|-----|---------|------|
| Application | `paap-system` | 应用入口，管理 `paap-app-{id}` namespace |
| Environment | `paap-app-{app}` | 管理业务 namespace、NetworkPolicy、Quota |
| ServiceInstance | `paap-app-{app}` | 管理工具实例（SA, Role, 工具 Deployment） |
| Component | `paap-app-{app}` | 管理业务组件（Deployment, Service） |

## 快速开始

### 前置条件

- Go 1.25+
- Docker
- kind 集群
- kubectl

### 部署到 kind 集群

```bash
# 1. 克隆项目
git clone <repo-url>
cd paap

# 2. 一键部署
KIND_CLUSTER=<your-cluster> bash deploy/k8s/deploy.sh

# 3. 访问
# PAAP Server: http://<node-ip>:30091
# 前端开发: http://localhost:5173 (npm run dev)
```

### 本地开发

```bash
# 安装依赖
make deps

# 生成 CRD
make manifests

# 安装 CRD 到集群
make install

# 运行 Operator
make run-operator

# 运行 Server
make run

# 运行前端
cd frontend && npm run dev
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/register` | 注册 |
| GET | `/api/v1/applications` | 应用列表 |
| POST | `/api/v1/applications` | 创建应用 |
| GET | `/api/v1/applications/:id` | 应用详情 |
| DELETE | `/api/v1/applications/:id` | 删除应用 |
| GET | `/api/v1/applications/:id/environments` | 环境列表 |
| POST | `/api/v1/applications/:id/environments` | 创建环境 |
| GET | `/api/v1/environments/:id` | 环境详情 |
| DELETE | `/api/v1/environments/:id` | 删除环境 |
| POST | `/api/v1/applications/:id/services` | 安装服务 |
| GET | `/api/v1/environments/:id/components` | 组件列表 |
| POST | `/api/v1/environments/:id/components` | 创建组件 |
| DELETE | `/api/v1/components/:id` | 删除组件 |
| GET | `/api/v1/templates` | 模板列表 |
| GET | `/ws` | WebSocket 状态推送 |

## 项目结构

```
paap/
├── api/v1/                    # CRD 类型定义
│   ├── application_types.go
│   ├── environment_types.go
│   ├── serviceinstance_types.go
│   └── component_types.go
├── cmd/
│   ├── server/main.go         # PAAP Server 入口
│   └── operator/main.go       # PAAP Operator 入口
├── internal/
│   ├── controller/            # Operator Controller
│   ├── handler/               # Server HTTP Handler
│   ├── k8s/                   # K8s CR 管理器
│   ├── model/                 # GORM 数据模型
│   ├── service/               # 业务逻辑（模板渲染）
│   └── database/              # 数据库连接
├── config/crd/bases/          # CRD YAML
├── deploy/k8s/                # 部署配置
├── frontend/                  # Vue 前端
│   ├── src/views/             # 页面组件
│   ├── src/api/               # API 客户端
│   ├── src/composables/       # 组合式函数
│   └── src/router/            # 路由配置
├── Makefile
├── Dockerfile.server
└── Dockerfile.operator
```

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | Vue 3 + TypeScript + Carbon Design System + Pinia |
| API Server | Go + Gin + GORM + client-go |
| Operator | Go + Kubebuilder + controller-runtime |
| 数据库 | SQLite（开发）/ PostgreSQL（生产） |
| 集群 | K8s 1.28+ (kind 开发) |

## 设计文档

### 核心文档（推荐阅读）

- [产品设计](docs/design/product-design.md)
- [技术选型](docs/design/tech-stack.md)
- [Operator 设计](docs/design/operator-design.md)
- [模板系统总览](docs/design/template-system-overview.md) - **了解平台模板架构和当前状态**
- [自定义模板开发指南](docs/design/custom-template-guide.md) - **BYO (Bring Your Own) 模板开发完整指南**
- [自定义 vs 第三方 Chart](docs/design/custom-vs-third-party.md) - **如何选择自定义 Chart 还是转换第三方 Chart**
- [快速参考卡片](docs/QUICK-REFERENCE.md) - **一页纸快速参考**

### 其他设计文档

- [权限隔离设计](docs/design/service-isolation-design.md)
- [环境交互设计](docs/design/environment-interaction-design.md)
- [服务模板规范](docs/design/service-template-spec.md) - ⚠️ 早期设计方案（未实现）
- [内置模板迁移路线图](docs/design/migration-roadmap.md) - ⚠️ 内置模板迁移计划
- [任务清单](docs/tasks.md)

## 示例模板

- [自定义 Prometheus 模板](docs/examples/custom-prometheus-template/) - 从零编写的完整示例
- [Bitnami Redis 模板](docs/examples/bitnami-redis-template/) - 零改动转换第三方 Chart 示例
