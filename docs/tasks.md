# PAAP 实现任务清单

## 阶段一：项目脚手架（Day 1-2）

> 现有项目已有：Go 1.25 + Gin + GORM + PostgreSQL，cmd/server/main.go，internal/handler/model/database/k8s。
> 需要集成 Operator（CRD + Controller + cmd/operator）。

### Task 1.1: 集成 Operator 脚手架到现有项目 ✅
- [x] 添加 controller-runtime + k8s.io 依赖到 go.mod
- [x] 创建 `api/v1/` 目录，编写 4 个 CRD types 文件
- [x] 创建 `api/v1/groupversion_info.go`
- [x] 创建 `api/v1/zz_generated.deepcopy.go`（controller-gen 生成）
- [x] 验证 `go build ./api/...` 通过

### Task 1.2: 定义 4 个 CRD types ✅
- [x] 编写 `api/v1/application_types.go`（Spec + Status + kubebuilder marker）
- [x] 编写 `api/v1/environment_types.go`（Spec + Status）
- [x] 编写 `api/v1/serviceinstance_types.go`（Spec + Status）
- [x] 编写 `api/v1/component_types.go`（Spec + Status）
- [x] 用 controller-gen 生成 CRD YAML
- [x] 用 controller-gen 生成 deepcopy
- [x] 验证 `kubectl apply -f config/crd/bases/` 安装到 kind 集群
- [x] 验证创建 Application CR 成功

### Task 1.3: 创建 Operator 入口 ✅
- [x] 创建 `cmd/operator/main.go`（controller-runtime manager 启动）
- [x] 注册 4 个 CRD scheme
- [x] 创建 `internal/controller/` 目录（4 个 controller stub）
- [x] 更新 Makefile：增加 `manifests`, `generate`, `install`, `run-operator` targets
- [x] 验证 `go build ./cmd/operator` 通过
- [x] 验证 `go run ./cmd/operator` 能启动并连接 kind 集群 ✅ 4 个 Controller 全部运行

### Task 1.4: 初始化 Vue 前端
- [ ] 检查 frontend/ 现有状态，补充缺失依赖
- [ ] 安装 `@carbon/vue3 pinia vue-router axios`
- [ ] 配置 vite.config.ts（proxy 到后端 9090 端口）
- [ ] 创建基础布局（侧边栏 + 内容区）
- [ ] 验证 `npm run dev` 启动成功

---

## 阶段二：Operator Controller（Day 3-6）✅

### Task 2.1: Application Controller ✅
- [x] 实现 Reconciler：创建 `paap-app-{identifier}` namespace
- [x] 添加 Finalizer
- [x] 实现删除逻辑：级联删除 namespace 下所有 Environment CR
- [x] 汇总 Environment 状态到 Application status
- [x] 部署到 kind 集群验证

### Task 2.2: Environment Controller ✅
- [x] 实现 Reconciler：创建业务 namespace（primary + additional）
- [x] 为 namespace 打 label（`paap.io/app`, `paap.io/env`）
- [x] 创建 NetworkPolicy（同环境允许 + 外部放行 + 跨环境拒绝）
- [x] 添加 Finalizer，实现删除逻辑
- [x] 部署到 kind 集群验证

### Task 2.3: ServiceInstance Controller ✅
- [x] 实现 Reconciler：检查 Environment 就绪（Requeue 机制）
- [x] 创建 ServiceAccount（在 primaryNamespace）
- [x] 遍历环境所有 namespace 创建 Role + RoleBinding
- [x] 添加 Finalizer，实现删除逻辑
- [x] 部署到 kind 集群验证

### Task 2.4: Component Controller ✅
- [x] 实现 Reconciler：创建 Deployment + Service
- [x] 支持 `managedBy: operator` 和 `managedBy: argocd` 两种模式
- [x] 添加 Finalizer，实现删除逻辑
- [x] 部署到 kind 集群验证

### Task 2.5: Operator 集成测试 ✅
- [x] E2E 测试：创建 Application → Environment → ServiceInstance → Component
- [x] 验证级联删除：删除 Application → 所有子资源清理 ✅
- [x] 部署到 kind 集群验证

---

## 阶段三：PAAP Server 核心（Day 7-12）✅

### Task 3.1: 数据库模型 + 迁移 ✅
- [x] GORM 模型：User, Application, Environment, ServiceTemplate, EnvTemplate, ServiceInstallation, Component 等
- [x] SQLite 连接（本地测试）
- [x] AutoMigrate 自动建表
- [x] 种子数据：demo 用户 + ServiceCatalog

### Task 3.2: 用户认证 ✅
- [x] `POST /api/v1/auth/login`（简单 token 登录）
- [x] `POST /api/v1/auth/register`
- [x] `GET /api/v1/auth/me`

### Task 3.3: 模板管理 API ✅
- [x] `GET /api/v1/templates`（ServiceCatalog 列表）
- [x] 预置种子数据：deploy, ci, monitor, postgresql, redis

### Task 3.4: 模板渲染引擎 ✅
- [x] 实现 `TemplateRenderer`：Go text/template + Sprig
- [x] 注入运行时变量（.appIdentifier, .envIdentifier, .primaryNamespace 等）
- [x] 支持 RenderString / RenderEnvNsAdded / RenderEnvNsRemoved

### Task 3.5: CR 管理器 ✅
- [x] 实现 `k8s.CreateApplicationCR` / `DeleteApplicationCR`
- [x] 实现 `k8s.CreateEnvironmentCR` / `DeleteEnvironmentCR`
- [x] 实现 `k8s.CreateServiceInstanceCR` / `DeleteServiceInstanceCR`
- [x] 实现 `k8s.CreateComponentCR` / `DeleteComponentCR`
- [x] 集成到 Server handler

### Task 3.6: 应用管理 API ✅
- [x] `GET /api/v1/applications`（列表）
- [x] `POST /api/v1/applications`（创建 + Application CR）
- [x] `GET /api/v1/applications/:id`（详情 + 环境列表）
- [x] `PUT /api/v1/applications/:id`（更新）
- [x] `DELETE /api/v1/applications/:id`（删除 + CR 级联删除）

### Task 3.7: 环境管理 API ✅
- [x] `GET /api/v1/applications/:id/environments`（列表）
- [x] `POST /api/v1/applications/:id/environments`（创建 + Environment CR）
- [x] `GET /api/v1/environments/:id`（详情 + 组件/服务/基础设施）
- [x] `DELETE /api/v1/environments/:id`（删除 + CR 级联删除）

### Task 3.8: 服务实例 API ✅
- [x] `GET /api/v1/applications/:id/services`（列表）
- [x] `POST /api/v1/applications/:id/services`（安装 + ServiceInstance CR）

### Task 3.9: 组件 API ✅
- [x] `GET /api/v1/environments/:id/components`（列表）
- [x] `POST /api/v1/environments/:id/components`（创建 + Component CR）
- [x] `DELETE /api/v1/components/:id`（删除 + CR 级联删除）

### Task 3.10: WebSocket 状态推送 ✅
- [x] 实现 WebSocket Hub（连接管理、消息广播）
- [x] `BroadcastStatusChange` 函数
- [x] 前端 WebSocket composable（`useWebSocket`）

---

## 阶段四：前端页面（Day 13-18）✅

### Task 4.1: 基础框架 ✅
- [x] 路由配置（Vue Router）— 完整路由表
- [x] 全局状态（Pinia stores）
- [x] API 封装（fetch client）
- [x] 布局组件（MainLayout + AppLayout）

### Task 4.2: 登录页 ✅
- [x] 登录表单
- [x] Token 存储
- [x] 路由守卫

### Task 4.3: 我的应用 ✅
- [x] 应用列表（卡片布局）
- [x] 创建应用表单

### Task 4.4: 环境管理 ✅
- [x] 环境列表
- [x] 创建环境对话框
- [x] 删除环境

### Task 4.5: 环境画布 ✅
- [x] 工具区（已安装工具 + 添加工具按钮）
- [x] 组件区（组件卡片 + 创建组件按钮）
- [x] 基础设施区
- [x] 安装工具对话框（deploy/ci/monitor/log）
- [x] 创建组件对话框

### Task 4.6: 部署服务页 ✅
- [x] 环境切换下拉
- [x] 部署组件列表

### Task 4.7: CI 服务页 ✅
- [x] 流水线列表
- [x] 创建流水线表单

### Task 4.8: 监控服务页 ✅
- [x] 环境切换 + 时间范围选择
- [x] 应用健康卡片
- [x] 组件资源表格

### Task 4.9: WebSocket 实时状态 ✅
- [x] WebSocket composable（`useWebSocket`）
- [x] 自动重连机制

---

## 阶段五：集成测试 + 部署（Day 19-21）

### Task 5.1: E2E 测试 ✅
- [x] 完整流程：创建应用 → 创建环境 → 安装工具 → 创建组件
- [x] 验证：删除应用时所有资源级联清理 ✅
- [x] 验证：Namespace 自动创建 + NetworkPolicy ✅
- [x] 验证：SA + Role + RoleBinding 自动创建 ✅

### Task 5.2: 部署配置 ✅
- [x] `deploy/k8s/paap-server.yaml`（含 RBAC）
- [x] `deploy/k8s/paap-operator.yaml`（含 ClusterRole）
- [x] `deploy/k8s/postgres.yaml`
- [x] `deploy/k8s/deploy.sh`（一键部署脚本）
- [x] `config/crd/bases/`（4 个 CRD YAML）
- [x] Makefile targets（build, run, manifests, install, run-operator）

### Task 5.3: 文档完善 ✅
- [x] README.md（项目介绍、架构图、快速开始、API 列表、项目结构）

---

## 实现顺序依赖图

```
Task 1.1 (Kubebuilder init)
  ↓
Task 1.2 (CRD 定义) ──────────────────────┐
  ↓                                        │
Task 1.3 (Server 模块) ──┐                 │
  ↓                       │                 │
Task 2.1 (App Controller) │                 │
  ↓                       │                 │
Task 2.2 (Env Controller) │                 │
  ↓                       │                 │
Task 2.3 (SvcInst Ctrl)   │                 │
  ↓                       │                 │
Task 2.4 (Component Ctrl) │                 │
  ↓                       │                 │
Task 2.5 (Operator E2E)   │                 │
  ↓                       ↓                 │
Task 3.1 (DB Models) ──→ Task 3.4 (Renderer)
  ↓                       ↓
Task 3.2 (Auth) ────→ Task 3.5 (CR Manager)
  ↓                       ↓
Task 3.3 (Templates) ─→ Task 3.6-3.10 (APIs)
  ↓                       ↓
Task 1.4 (Vue Init) ──→ Task 4.x (Frontend)
                          ↓
                     Task 5.x (E2E + Deploy)
```

---

## 当前状态

- [x] 设计文档完成
- [x] 阶段一：项目脚手架 ✅
- [x] 阶段二：Operator Controller ✅
- [x] 阶段三：PAAP Server 核心 ✅
- [x] 阶段四：前端页面 ✅
- [x] 阶段五：集成测试 + 部署 ✅

**全部完成。** 核心架构已跑通：PAAP Server 管"想要什么"（CR），Operator 管"怎么实现"（K8s 资源）。
