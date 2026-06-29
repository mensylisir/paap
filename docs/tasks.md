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
- [x] 检查 frontend/ 现有状态，补充缺失依赖
- [x] 安装 Carbon Vue、Pinia、Vue Router、Axios（当前依赖为 `@carbon/vue`、`pinia`、`vue-router`、`axios`）
- [x] 配置 vite.config.ts（proxy 到后端 9090 端口）
- [x] 创建基础布局（侧边栏 + 内容区）
- [x] 验证 `npm run dev` 启动成功
  - 2026-06-25 验证：`npm --prefix frontend run dev -- --host 127.0.0.1 --port 5174` 正常 ready

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
- [x] PostgreSQL 连接（平台数据库）
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
- [ ] 阶段六：产品化补齐（Task 7.1-7.18）

**核心架构已完成。** 产品化补齐阶段剩余 17 项未完成任务（Task 7.1-7.17），其中 Task 7.18（画布重命名）已完成。
CDP 验证已覆盖 11 个运行中服务的全部 CRUD 操作。

---

## 后续待补齐功能

> 说明：上面的阶段任务表示核心架构和基础流程已经跑通。下面记录的是继续产品化、生产化前仍未完整闭环的功能。

### Task 6.1: 认证、鉴权与应用成员权限
- [x] 前端登录页接入真实 `/api/v1/auth/login`，保存 token 和用户信息，并处理登录失败状态
- [x] API 路由增加统一认证中间件，除登录/注册/健康检查外默认要求登录
- [x] 将内存 token 替换为签名 JWT 或可持久化会话机制
- [x] 应用创建、列表、详情、更新、删除改为基于当前用户和 `AppMember` 判断权限
  - `ListApplications` 对普通用户按 `app_members.user_id` 过滤；`GetApplication` / `UpdateApplication` / `DeleteApplication` / 环境列表与创建均复用应用成员校验，平台管理员可跨应用查看
  - 测试覆盖：`TestGetApplicationRejectsNonMembers`、`TestUpdateApplicationRejectsNonMembers`、`TestDeleteApplicationRejectsNonMembers`、`TestListApplicationEnvironmentsRejectsNonMembers`、`TestCreateEnvironmentRejectsNonMembers`
- [x] 移除 `OwnerID=1`、`UserID=1` 等硬编码
  - `CreateApplication` 使用认证上下文中的用户作为 owner 并创建 `AppMember` 记录；非测试代码未再检出 `OwnerID: 1` / `UserID: 1` / `owner_id = 1` / `user_id = 1`
- [x] 补齐应用成员管理页面和 API，包括邀请、角色变更、移除成员
  - 后端新增 `GET/POST/PUT/DELETE /api/v1/applications/:id/members`，支持成员列表、邀请已有用户、角色变更和移除成员，并保留至少一个应用管理员
  - 前端应用概览页新增“应用成员”区域，可输入用户名邀请、下拉调整 `admin/member/viewer` 角色、移除非最后管理员成员
  - 测试覆盖：`go test ./internal/handler -run 'Test(ListApplicationMembers|InviteApplicationMember|UpdateApplicationMemberRole|RemoveApplicationMember)'`、`npm --prefix frontend run test -- src/api/client.test.ts -t 'application member management'`、`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t 'application member management'` 先红后绿
  - 部署验证：`paap-server:v0.1.495` 已加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；CDP 验证应用概览成员区可见，成员列表 API 返回 admin，并完成临时用户注册、邀请、角色更新、移除闭环

### Task 6.2: 环境模板管理与高级环境配置
- [x] 挂载环境模板创建、更新、删除 API 路由
  - 已挂载 `GET /api/v1/templates/:id`、`POST /api/v1/templates`、`PUT /api/v1/templates/:id`、`DELETE /api/v1/templates/:id`，复用已有环境模板 handler
  - 测试覆盖：`go test ./internal/handler -run TestEnvironmentTemplateCRUDRoutesAreMounted` 先红后绿
  - 部署验证：`paap-server:v0.1.496` 已加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；CDP 验证环境模板创建 201、详情 200、更新 200、列表可见更新值、删除 200、删除后详情 404
- [x] 在前端补齐环境模板管理 UI，而不只是读取模板列表
  - 模板管理页新增“环境模板”Tab，独立展示环境模板列表、服务/基础设施摘要和 CPU/内存/存储配额，不复用工具/中间件模板的 Helm 上传操作
  - 新增环境模板新建、编辑、删除弹窗；编辑时显式空数组可清空 `services` / `infra`，避免前端清空后后端仍保留旧列表
  - 测试覆盖：`npm --prefix frontend run test -- src/api/client.test.ts -t "environment template"`、`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "environment template"`、`go test ./internal/handler -run TestEnvironmentTemplateCRUDRoutesAreMounted` 先红后绿；完整 `npm --prefix frontend run test`、`npm --prefix frontend run build`、`make test` 通过
  - 部署验证：`paap-server:v0.1.497` 已构建、加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；实际 Deployment 镜像为 `paap-server:v0.1.497`，PAAP/kpack 相关 Pod Running，节点 Ready
  - CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/templates`；环境模板 Tab 和“新建环境模板”可见；通过 UI 完成临时环境模板新建、编辑（内存 `4GB` -> `6GB`，服务/基础设施清空）、删除闭环；带 token 查询 `/api/v1/templates` 返回 200、5 条基准模板、临时 `CDP环境模板-*` 残留 0
- [x] 创建环境时支持从模板写入 CPU、内存、存储配额到 `Environment.spec.resourceQuota`
  - `CreateEnvironment` 在非空模板创建路径读取 `EnvTemplate.resourceCpu/resourceMem/resourceDisk`，写入 Environment CR `spec.resourceQuota.cpu/memory/storage`
  - 测试覆盖：`go test ./internal/handler -run TestCreateEnvironmentAppliesTemplateResourceQuota` 先红后绿；受影响包 `go test ./internal/handler ./internal/k8s` 和完整 `make test` 通过
  - 部署验证：`paap-server:v0.1.498` 已构建、加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；实际 Deployment 镜像为 `paap-server:v0.1.498`，PAAP/kpack 相关 Pod Running，节点 Ready
  - CDP/API 验证：复用 Chrome tab 登录 token，在应用 1 下用模板 ID 4（轻量开发环境）创建临时环境 `quota-cdp-736204`，kubectl 读取 `paap-app-test/environment quota-cdp-736204` 得到 `{"cpu":"2核","memory":"4GB","storage":"20GB"}`；删除临时环境后 API 返回 404，CR 返回 NotFound
- [x] 创建环境时支持模板或表单配置附加 namespace，而不是固定只创建 `app` namespace
  - 创建环境 API 新增 `additionalNamespaces`，后端保留默认 `app/workload`，并对表单传入的 suffix/purpose 做规范化和去重后写入 `Environment.spec.additionalNamespaces`
  - 应用概览和环境列表两个“创建环境”弹窗新增“附加命名空间”输入，支持每行 `suffix:purpose`，例如 `database:database`、`cache:cache`
  - 测试覆盖：`go test ./internal/handler -run TestCreateEnvironmentAppliesAdditionalNamespaces` 先红后绿；`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "additional namespace"`、`go test ./internal/handler ./internal/k8s`、`npm --prefix frontend run test`、`npm --prefix frontend run build`、完整 `make test` 通过
  - 部署验证：`paap-server:v0.1.499` 已构建、加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；实际 Deployment 镜像为 `paap-server:v0.1.499`，PAAP/kpack 相关 Pod Running，节点 Ready
  - CDP/API/kubectl 验证：复用 Chrome tab，在应用 1 下创建临时环境 `ns-cdp-326016`，传入 `database/database` 与 `cache/cache`；kubectl 读取 CR 得到默认 `app/workload` 加两个自定义 namespace，operator 创建 `test-ns-cdp-326016-database`、`test-ns-cdp-326016-cache`；删除临时环境后 CR 和临时 namespace 均返回 NotFound
- [x] 评估并实现 `ipPool` 调和逻辑；若暂不支持，需要从 UI 和文档中明确标记为未启用
  - 当前决策：暂不启用自定义 `ipPool`，环境创建仍使用平台默认网络规划
  - UI 标记：应用概览和环境列表两个“创建环境”弹窗均展示只读“网络地址池 / 暂未启用”状态，不向创建环境 API 发送 `ipPool`
  - 前端验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t 'marks environment IP pool selection'` 先红后绿，`npm --prefix frontend run test`、`npm --prefix frontend run build` 通过
  - 部署验证：`paap-server:v0.1.494` 已加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；CDP 验证 `/apps/1/overview?createEnvironment=true` 和 `/apps/1/environments?create=true` 两个弹窗字段均为只读禁用状态
- [ ] 支持创建环境后的 namespace 增删，并触发工具 RBAC 与 Helm values 动态同步

### Task 6.3: 服务目录占位项落地
- [ ] 为 `kingbase` 补齐服务模板、安装参数、连接发现、工作台和测试
- [x] 为 `nacos` 补齐服务模板、安装参数、连接发现、工作台和测试
  - 2026-06-28 补齐：新增 `docs/examples/built-in-templates/nacos` lightweight standalone Helm chart、`platform-manifest.yaml`、`preset-values.yaml`，打包为 `data/charts/nacos.tar.gz`；`SeedServiceCatalog` 改为启用 Nacos，`builtInTemplateArchives` / `builtInServiceTemplateByType` 接入 `charts/nacos.tar.gz`。
  - 验证：`helm template test-nacos docs/examples/built-in-templates/nacos/chart --values docs/examples/built-in-templates/nacos/preset-values.yaml --set fullnameOverride=test-nacos --set serviceAccount.name=test-nacos` 通过；`go test -count=1 ./internal/model ./internal/handler -run 'Test(DocsExamplesPlatformManifestsUseExplicitNamespacePermissionTypes|PlatformManifest|ListServiceCatalogHidesUnsupportedPlaceholders|BuiltInServiceTemplatesExposeFeatureMatrix|NacosAndEurekaBuiltInChartArchivesParse|BuiltInServiceTemplates|ServiceCatalog)'` 通过。
- [x] 为 `eureka` 补齐服务模板、安装参数、连接发现、工作台和测试
  - 2026-06-28 补齐：新增 `docs/examples/built-in-templates/eureka` lightweight Helm chart、`platform-manifest.yaml`、`preset-values.yaml`，打包为 `data/charts/eureka.tar.gz`；`SeedServiceCatalog` 新增 Eureka，`builtInTemplateArchives` / `builtInServiceTemplateByType` 接入 `charts/eureka.tar.gz`。
  - 验证：`helm template test-eureka docs/examples/built-in-templates/eureka/chart --values docs/examples/built-in-templates/eureka/preset-values.yaml --set fullnameOverride=test-eureka --set serviceAccount.name=test-eureka` 通过；`go test -count=1 ./internal/handler -run 'Test(NacosAndEurekaBuiltInChartArchivesParse|ListServiceCatalogHidesUnsupportedPlaceholders|BuiltInServiceTemplatesExposeFeatureMatrix)'` 通过；`go test -count=1 ./internal/service ./internal/handler` 通过；`npm --prefix frontend run test -- src/utils/catalogGroups.test.ts --run` 通过。
- [x] 未落地前从可安装服务列表隐藏占位项，避免用户选择后安装失败

### Task 6.4: 模板体系收口
- [ ] 将内置模板完全统一到 `Helm Chart + platform-manifest.yaml + preset-values.yaml` 路径
- [ ] 废弃或移除旧的 `installer/rawYaml/chartRepo/chartName` 创建入口
- [ ] 将 `WorkloadRolePolicy`、`EnvironmentRolePolicy` 等旧权限字段收敛到 `platform-manifest.yaml`
- [ ] 补齐内置模板同步、上传到 MinIO、数据库种子数据的一致性校验
- [ ] 逐个验证内置模板安装、卸载、RBAC 隔离、工作台操作和运行态数据

### Task 6.5: CI/CD 端到端生产化
- [ ] 明确 Tekton 是否继续作为目标；若继续，需要实现 Tekton 模板和工作台；若不继续，需要清理旧设计文档
- [x] 为 source 组件链路补齐前置依赖检查：Gitea、Jenkins、kpack、registry/Harbor、ArgoCD
  - 2026-06-28 补齐：`DeployComponent` 在执行源码构建/GitOps 动作前从 service 层校验环境内 Gitea、Jenkins、registry/Harbor、ArgoCD 安装记录必须为 running 且有命名空间；源码待构建时额外只读校验 kpack CRD 与 `kpack-controller` ready；image 交付只要求 GitOps 依赖，不误要求 Jenkins/kpack/registry
  - 2026-06-28 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/service -run 'TestValidateComponent(SourceBuildPreflight|ImageDeploymentPreflight)|TestRunComponent(SourceBuildFlowRequiresGitAndCI|ImageDeliveryFlowRequiresGitAndCD|ImageDeliveryFlowPublishesGitOpsAndConfiguresArgoCD)|TestBuildComponentKpackSpec|TestPreferredSourceRegistryServiceType|TestPrepareComponentSourceBuildVersion'` 通过；`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/service` 通过
- [ ] 将 registry/Harbor 的 DNS、TLS、节点运行时信任配置做成可验证状态，而不是只展示说明
- [ ] 完整验证 source 模式：源码仓 → Gitea mirror → Jenkins/kpack → registry/Harbor → GitOps 清单 → ArgoCD → 集群
  - 2026-06-29 修复并验证：源码交付失败后重新点击“部署”会把组件置为 `planned` 并重新触发 Jenkins/kpack，不再被旧的 `BuildFailed` kpack Image 直接短路；UI/CDP 对 PiggyMetrics `gateway` 提交源码交付，`PUT /api/v1/components/32` 和 `POST /api/v1/components/32/deploy` 均返回 200，组件状态进入 `pipelineStatus=running`，kpack `Image gateway` 重新创建 build pod，Jenkinsfile 使用内部 registry `10.96.190.247:5000/piggymetrics-dev/gateway` 推送，组件声明镜像保持外部运行地址 `registry.piggymetrics-dev.paap.local/piggymetrics-dev/gateway:6bb2cf9`。
  - 当前剩余：kind 运行态仍在 Paketo/BellSoft JDK 下载阶段，最终能否完成取决于从 GitHub 拉取 `bellsoft-jdk21.0.11+11-linux-amd64.tar.gz`；该项尚未证明完整到 ArgoCD/集群。
- [x] 完整验证 image 模式：镜像输入 → GitOps 清单 → ArgoCD → 集群
  - 2026-06-29 修复并验证：镜像交付保存/部署时按所选 `registryTarget` 重新生成运行镜像地址，UI 和 Gitea 部署 YAML 不再泄漏环境内 ClusterIP；CDP 在 PiggyMetrics `auth-service` 页面选择本环境 `dev-registry` 后点击部署，`PUT /api/v1/components/28` 和 `POST /api/v1/components/28/deploy` 均返回 200，DB 中 `image/registryImage=registry.piggymetrics-dev.paap.local/piggymetrics-dev/auth-service:6bb2cf9`，Gitea `components/auth-service/deployment.yaml` 同步为该镜像，集群 Deployment 也回读到同一镜像。测试环境未配置该域名 DNS，Pod 最终 Ready 不能作为本地 kind 的成功判据。
- [ ] 将链路中的 warning/pending 状态映射到前端可操作的修复入口
- [x] 代码层交付流程已拆为动作函数、步骤函数和流程函数：source 构建只准备 Gitea mirror/Jenkinsfile/README/Jenkins/kpack，不写 GitOps 清单；image 交付和已构建 source 交付由 PAAP 写部署清单、配置 ArgoCD 并持久化组件 GitOps 元数据
  - 2026-06-28 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/service -run 'Test(CreateComponent|UpdateComponent|DeployComponent|RunComponent|BuildComponentJenkinsfile|ApplyComponentDeployVersion|PrepareComponentSourceBuildVersion|PreferredSourceRegistryServiceType)'` 通过
  - 2026-06-28 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/database ./internal/authz ./internal/middleware ./internal/service ./internal/handler` 通过
  - 2026-06-28 浏览器验证：`paap-server:v0.1.578-source-draft-entry` 部署到 `kind-rbac-manager-test` 后，使用浏览器登录 `http://172.20.0.2:30091`，在 PiggyMetrics 开发环境通过页面会话创建 source 草稿（`deliveryMode=source`、`version=""`、`image=""`、`status=draft`）和 image 草稿（`deliveryMode=image`、明确 tag），均返回 201；随后删除均返回 200，组件数恢复为 7。
  - 当前状态：image 交付链路已通过浏览器操作和 Gitea/集群回读验证；source 交付链路已通过浏览器操作验证到 Jenkins/kpack 重新构建中，完整成功仍等待外部依赖下载条件。

### Task 6.6: 平台配置与管理员功能
- [ ] 增加平台配置模块，并按角色控制普通用户是否可见
- [ ] 补齐用户管理、角色管理、模板管理权限、审计记录等管理员页面
- [ ] 管理全局配置项，例如 `PAAP_REGISTRY_HOST_TEMPLATE`、MinIO、默认镜像源、kpack 状态
- [ ] 提供集群级依赖健康检查页面，例如 CRD、Operator、kpack、存储、模板仓库

### Task 6.7: 文档与测试清理
- [ ] 清理或标记仍描述旧方案的文档，尤其是 Tekton、raw-yaml、生命周期钩子相关内容
- [ ] 将 `docs/DEPLOYMENT-STATUS.md` 中旧镜像、RBAC、模板上传问题重新验证并更新
- [ ] 增加端到端测试覆盖：登录鉴权、环境模板、服务安装、组件 image/source 两条部署链路
- [x] 增加模板包校验测试，确保所有 `data/charts/*.tar.gz` 包含 `chart/`、`platform-manifest.yaml`、`preset-values.yaml`
  - 新增 `TestBuiltInChartArchivesContainRequiredTemplateFiles`，逐个解压 `data/charts/*.tar.gz`，校验内置模板包必须包含 Helm `chart/`、`platform-manifest.yaml` 和 `preset-values.yaml`
  - 验证：`go test -count=1 ./internal/handler -run TestBuiltInChartArchivesContainRequiredTemplateFiles` 通过
- [ ] 增加权限隔离测试，验证工具权限不会外溢到其他环境或应用

### Task 6.8: 产品化抽屉与运行态证据审计
- [ ] 对 Gitea、Registry、Harbor、Argo CD、Jenkins、Prometheus/Grafana、Loki、PostgreSQL、MySQL、MongoDB、Redis、RabbitMQ、Kafka、MinIO 做逐项 CDP 抽屉审计
- [ ] 对每个组件、工具、中间件卡片验证真实指标、空状态、时间范围、图表比例和无误导占位值
- [ ] 对每个组件、工具、中间件卡片验证真实日志，消除 `no such host`、假数据和占位日志
- [ ] 对每个常见工具/中间件 Pod 验证控制台连接，包括无 shell 镜像和禁用 ephemeral container 的场景
- [ ] 对所有 workspace resource、metric、log、backup、key、queue、topic、bucket、deployment 行建立真实后端/API/集群来源证明
- [ ] 对每次重要 UI 改动使用可见 Chrome CDP 回归测试，不只依赖 headless smoke

### Task 6.9: 数据库、中间件与存储工作台补证
- [ ] PostgreSQL 工作台重新 CDP 验证 database/table/row create、insert、update、delete 和 backup 输出
- [ ] MySQL 工作台重新 CDP 验证 database/table/row create、insert、update、delete 和 backup 输出
- [ ] MySQL replica、dual-master、Galera 模式分别验证 Helm values、Pod、Service、PVC 与工作台行为
- [ ] MongoDB、Kafka、MinIO 补充对象级、失败状态和边界输入验证
- [ ] RabbitMQ 补充失败状态、边界输入和权限不足场景验证
- [ ] 数据库备份补齐 list、download、restore 的产品决策、实现和 CDP 证据

### Task 6.10: 服务配置更新与拓扑模式验证
- [ ] 对每个支持持久卷的服务逐 chart 验证 PV size values 映射、Helm 输出、PVC 创建和运行实例状态
- [ ] 对不支持在线扩容或受 Kubernetes PVC expansion 限制的场景展示清晰用户提示
- [ ] 验证 Redis standalone、replication、Sentinel、cluster 模式的部署结果和工作台状态
- [ ] 验证 PostgreSQL standalone、replica、HA 模式的部署结果和工作台状态
- [ ] 验证 MySQL standalone、replica、dual-master、Galera 模式的部署结果和工作台状态
- [ ] 对运行中 ServiceInstance 的高风险 values 更新执行 Helm/operator 调和验证，避免 stale UI 状态

### Task 6.11: 组件配置模板与关系自动识别
- [ ] 扩充内置组件配置模板：nginx 多后端路由、Spring Boot datasource/cache/mq profiles、Gin/Go 配置、Node/Vite API 配置、配置文件型应用
- [x] 重设计配置模板导入 UI，使用白色 Carbon 表单视觉，避免重灰输入块
  - 配置模板导入弹窗使用 `config-import-shell--carbon` 和 `config-import-mode-card`，表单控件沿用白色 Carbon 风格的 `rail-input`、`rail-select`、`rail-textarea`，不再使用重灰输入块
  - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "splits template management"` 通过；CDP 验证当前部署 `/templates` 的导入弹窗可见普通/高级模式卡片和白色表单控件
- [x] 将配置模板导入的“适用组件”改为 select/combobox 控件
  - 配置模板导入弹窗已使用 `config-template-component-type-select` 下拉控件，选项覆盖所有组件、前端、后端、前端 + 后端、Worker / 任务组件和自定义组件；导入时转换为 `componentTypes` 数组，影响组件配置 Tab 模板候选范围
  - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "splits template management"` 通过；CDP 验证当前部署 `/templates` 的“导入配置模板”弹窗存在该 select 和完整选项
- [x] 导入流程同时支持普通原生配置模板和高级 template/schema JSON，并清晰区分两种模式
  - 普通模式是默认路径，直接上传用户自己的 `.yml/.yaml/.json/.conf/.env` 等配置文件，文件里只把可变字段替换为 `__TEMPLATE__KEY__显示名__` / `DEFAULT` / `IF` / `FOR` 语法；即使普通文件名是 `template.json/schema.json`，或内容包含 `template/schema/fields` 字段，也必须按业务配置文件解析，不做自动高级模板猜测。
  - 高级模式仅供高级用户导入 PAAP `template.json/schema.json` 或包含它们的 `.tar.gz` 包；高级结构不作为普通用户上传配置文件的要求。
  - 2026-06-28 验证：`go test ./internal/service ./internal/handler -run 'ComponentConfigTemplate|ParseNative|ParseUploaded|ParsePackagedBuiltIn'`、`npm --prefix frontend run test -- src/views/componentConfigTemplateRuntime.test.ts src/views/viewMarkup.test.ts src/views/configTemplateSyntax.test.ts --run`、`npm --prefix frontend run build` 通过；浏览器访问 `/templates` 验证普通模式多文件上传提示和高级模板包提示。
  - 2026-06-28 浏览器验证：在 `/templates` 普通模式上传两个普通业务配置文件，文件名分别为 `schema.json`、`template.json`，页面成功新增模板；API 返回 `nativeConfigs=2`、`configKeys=schema.json/template.json`、字段 `DATABASE_HOST/DATABASE_PASSWORD/API_ENDPOINT`，并保留业务 JSON 内容，只把占位符转换为 `[[paap:...]]`。临时模板已通过页面会话删除，列表数量恢复。
  - 2026-06-28 运行态验证：构建并滚动 `paap-server:v0.1.579-template-port-flow` 到 kind；浏览器同源 API 保存 PiggyMetrics `backend-1` 为 `containerPort=80` 后触发平台镜像交付，Gitea contents API 返回 `components/backend-1/deployment.yaml` 为 `containerPort: 80`、`service.yaml` 为 `targetPort: 80`；只读 `kubectl get` 验证集群 Deployment/Service 均已同步到 80，Pod `1/1 Running`。
- [x] 模板预览展示原始内容、抽取字段、敏感字段、生成文件和校验错误，不要求用户理解 Kubernetes 对象名
- [ ] 增强 configmap、secret、file-based config 解析，让后端到数据库/缓存/消息队列关系能安全自动连线

### Task 6.12: 降低 Kubernetes 术语暴露
- [ ] 全量检查页面、抽屉和工作台中的 namespace、service、pod、configmap、secret、pvc、helm 等术语
- [ ] 默认视图用产品语言替换 Kubernetes 术语，仅在高级/调试视图保留底层字段
- [ ] 为必须展示的底层概念补充上下文，避免应用管理员理解成本过高

### Task 6.13: 外部能力与共享能力模型
- [x] 新增统一 `EnvironmentCapability` 模型，覆盖 `git`、`registry`、`ci`、`cd`、`monitor`、`logging`、`database`、`cache`、`mq`、`objectStorage`
- [x] 每个 capability 支持 `managed`、`shared`、`external` 来源，并记录 provider、连接配置、验证结果和标准输出
- [ ] 环境模板声明 required capabilities，而不是硬编码必须安装 PAAP 管理实例
- [x] 创建环境只选择环境形态：创建空环境 / 创建基础环境 / 从模板创建；能力来源不在创建环境弹窗选择
  - 2026-06-25 验证：复用 Chrome tab，`/apps/5/overview?createEnvironment=true` 与 `/apps/5/environments?create=true` 均使用同一弹窗，创建方式为“创建空环境 / 创建基础环境 / 从模板创建”，无“能力来源”字段。
- [x] 平台共享资源池通过 `/shared-resources` 进入同一套环境画布，目标为系统 `default/shared` 应用和环境；平台管理员在该画布创建、部署和维护共享工具/中间件
  - 2026-06-26 验证：`/shared-resources` 跳转到 `/apps/62/environments/64`，页面显示“共享资源池 / 共享环境”，无 `environment not found`；应用列表中的系统共享资源池卡片也先进入 `/shared-resources`，再解析固定 `shared` 环境进入画布。
- [x] 业务环境画布右键添加共享资源或外部资源，生成 capability 卡片；共享资源只读，外部资源在右侧栏编辑 endpoint/auth
- [x] 外部资源凭据写入当前环境 Kubernetes Secret，数据库只保存 `credentialSecretRef`；右侧栏用眼睛按钮显示/隐藏已保存用户名、密码或 token
- [x] default 共享环境只允许安装工具/中间件，不允许部署业务组件，且不可被普通用户删除
  - 2026-06-25 验证：共享环境画布右键菜单只显示“添加工具 / 添加中间件”，工具二级菜单含 ArgoCD、Registry、Gitea、Jenkins、Prometheus+Grafana、Loki、Harbor，中间件二级菜单含 PostgreSQL、MySQL、MongoDB、Redis、RabbitMQ、Kafka、MinIO；未出现创建组件、共享资源、外部资源、纳管入口。
- [x] 卡片和抽屉明确展示 `managed`、`shared`、`external` 来源和断开/卸载语义
  - 2026-06-26 验证：新增 focused markup 测试覆盖卡片/抽屉的来源与移除语义；环境 5 可见 Chrome 验证环境内服务显示“环境内资源 / 卸载服务”，共享资源引用显示“平台共享 / 断开引用”，临时 external custom 显示“外部资源 / 断开外部连接”且抽屉提示不会删除外部系统；临时 external custom 已删除，环境能力恢复为 shared cache/database。
- [ ] 对 external Git、Registry、Argo CD、Jenkins、Prometheus、Loki、PostgreSQL、Redis、RabbitMQ、Kafka、MinIO 做真实连接与权限验证
  - 2026-06-27 已落地通用验证底座：`POST /api/v1/environments/:id/capabilities/:capability/validate` 对 external capability 做 endpoint 解析、TCP 连通性检查、本环境凭据 Secret 可读性检查，并写回 `validationStatus/validationMessage`；右侧栏提供“验证连接”入口。协议级权限验证仍需按服务类型逐个补齐。
- [x] external 来源删除只移除 PAAP 连接记录和本地凭据，不能删除真实外部资源
  - 2026-06-25 验证：在环境 5 创建临时 `custom` 外部资源卡片，页面确认删除后 `/api/v1/environments/5/capabilities` 只剩原有 `mq` 外部资源；删除弹窗文案明确“只会移除当前环境中的共享或外部资源卡片”。
- [x] 画布分区方案：节点补 `zone`/group 元数据，按“本环境 / 平台公共 / 集群外部”渲染分区
  - 2026-06-25 验证：环境 5 画布显示分区图例“本环境 / 平台公共 / 集群外部”，背景分区显示“本环境 9 个资源”“集群外部 1 个资源”；临时 external custom 卡片创建后“集群外部”计数变 2，删除后回到 1。
- [x] RBAC/Casbin 评估与当前落地：已从旧 `user_roles` 切到 `roles` / `permissions` / `role_permissions` / `role_bindings`，支持内置角色、自定义角色、权限点树和路由中间件鉴权；Casbin domain 模型暂不引入运行时。
  - 当前结论：现阶段保留自研 Scope-Based RBAC，避免在权限规则仍围绕系统 / 应用 / 环境继承时引入 Casbin 复杂度；后续若出现 ABAC、跨租户 domain 策略或更复杂矩阵规则，再评估 Casbin。

### Task 6.14: 平台目录、版本选择与服务暴露
- [x] 安装/编辑中间件时提供版本下拉，数据来自同 `ServiceType` 下的 `ServiceTemplate.ChartVersion`
  - 服务部署抽屉的版本下拉改为按同 `serviceType` 的 `ServiceTemplate.chartVersion` 取值；安装请求新增 `chartVersion`，后端按 `chart_version` 精确选择模板，并保留旧 `appVersion` 兼容路径；用户界面只展示应用版本，不展示 Chart 版本
  - 测试覆盖：`go test -count=1 ./internal/handler -run TestInstallServiceSelectsTemplateByChartVersion` 先红后绿；`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "chart version"` 先红后绿；`go test -count=1 ./internal/handler ./internal/k8s`、`npm --prefix frontend run test`、`npm --prefix frontend run build`、完整 `make test` 通过
  - 部署验证：`paap-server:v0.1.500` 已构建、加载到 `kind-rbac-governance-test` 并完成 `paap-system/paap-server` 滚动更新；实际 Deployment 镜像为 `paap-server:v0.1.500`，PAAP/kpack 相关 Pod Running，节点 Ready
  - CDP/API/kubectl 验证：复用 Chrome tab，在应用 1 下创建临时环境 `chart-cdp-136303`，读取 Redis 模板 `chartVersion=18.6.0` 后用 `chartVersion` 安装 Redis；kubectl 读取 `serviceinstance chart-cdp-136303-redis` 得到 `.spec.helm.chartVersion=18.6.0`、`.spec.helm.s3Key=charts/redis.tar.gz`、`architecture=standalone`；删除临时环境后 Environment、ServiceInstance 和临时 namespace 均返回 NotFound
  - 2026-06-25 验证：复用 Chrome tab，PostgreSQL 抽屉版本区域显示“当前应用版本 / 应用 v15.4.0”，Prometheus+Grafana 显示“应用 v0.77.2”，Loki 显示“应用 v2.9.3”，抽屉文本中无 `Chart v` 或 `Chart 版本`。
- [x] 增加“平台支持的中间件/工具目录”只读浏览页，按工具/数据库/缓存/消息队列/对象存储分组
  - 已提供 `/catalog` 只读目录页，读取 `api.listServiceTemplates()`，按工具类、数据库、缓存、消息队列、对象存储分组展示服务类型、描述和版本标签，并支持名称、类型、分组、描述搜索
  - 验证：`npm --prefix frontend run test -- src/utils/catalogGroups.test.ts src/utils/catalogVersions.test.ts src/views/viewMarkup.test.ts` 通过；CDP 验证当前部署 `/catalog` 展示 14 张卡片，分组为工具类 7、数据库 3、缓存 1、消息队列 2、对象存储 1
- [ ] 平台管理员支持维护 `ServiceCatalog` 和 `ServiceTemplate`，包括新增类型、上传 chart、维护版本列表
- [ ] 组件和服务增加 Ingress/Gateway 暴露配置：域名、路径、TLS 和状态回读
- [ ] 对共享工具 namespace 和外部 endpoint 补齐 NetworkPolicy ingress/egress 放行策略

### Task 6.15: 弹性、多集群与虚拟化路线图
- [ ] 引入 KEDA 伸缩配置：最小/最大副本、触发器、生成 `ScaledObject` 和状态展示
- [ ] 评估并接入 KubeVirt，将 VM 数据库作为服务类型纳入生命周期和备份管理
- [ ] 引入 `Cluster` 模型，支持注册集群、存储 kubeconfig、按标签选择部署目标
- [ ] 为环境增加 `ClusterID`，让 Application/Environment/ServiceInstance/Component 能面向多集群调和
- [ ] 规划双集群 Argo CD 主从或主控多集群模式，并验证跨集群部署链路
- [ ] 评估 Submariner、VXLAN、WireGuard 等跨集群/虚拟机网络方案，形成可执行专项设计

### Task 6.16: 本地 kind 发布与运维约束
- [ ] 建立镜像 tag 自动递增流程，构建后同步更新部署清单并部署到 `kind-rbac-governance-test`
- [ ] 对所有新增镜像执行本地 pull/build 和 `kind load docker-image --name rbac-governance-test`
- [ ] 浏览器访问 kind 服务时固定使用 kind node/container IP，避免误用 `127.0.0.1`
- [ ] 镜像密集操作前后检查磁盘空间，避免 Docker/kind 占满磁盘
- [ ] 将当前路线图拆到 Plane/Gitea，用于后续任务分配和 CI/CD 协作

---

## 阶段七：领导需求与产品化补齐（2026-06-23 扫描）

### Task 7.1: 中间件版本号选择器 ✅ 已完成
- [x] 后端 `ServiceTemplate` 增加 `AppVersion` 字段，`Type` 改为普通 index（允许多版本）
- [x] 新增 `extractChartYamlMeta()` 从 tarball `chart/Chart.yaml` 自动解析 `version` 和 `appVersion`
- [x] 重写 `SeedServiceTemplates()` 遍历 `data/charts/*.tar.gz` 自动解析版本，取代硬编码
- [x] `InstallServiceRequest` 增加 `AppVersion`，安装时按 type + appVersion 查模板
- [x] 前端 deploy tab 增加版本下拉框（未部署时可选版本，已部署后只读显示当前版本）
- [x] 画布卡片和 drawer 头部不再显示版本号
- [x] 提交：`9946b64` / `b9be953` / `aff00ec`（3 个 commit）
- [x] 工作量：约 2 天（含反复修改 + 部署验证）
- [x] 对应文件：`internal/model/service_catalog.go`、`internal/handler/template.go`、`internal/handler/environment.go`、`frontend/src/views/EnvDetailView.vue`

### Task 7.2: 中间件目录浏览页 ✅
- [x] 新增只读中间件目录页，按 `Category`（tool/infra）分组
- [x] 展示：类型、名称、可用版本（版本标签）、描述
- [x] 独立页面 `/catalog`，添加路由 + 导航栏入口
- [x] CDP 端到端验证：14 卡片 2 分类（🔧 工具类 7 + 🗄️ 中间件/数据库 7），版本标签正确
- [x] Docker 镜像 `v0.1.425` 构建部署到 kind 集群
- [x] 工作量：1 天（代码完成 + 部署验证）
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/router/index.ts`、`frontend/src/layouts/MainLayout.vue`

### Task 7.3: 平台管理员界面
- [ ] 前端新增 `/platform` 路由 + `PlatformAdminView`
- [ ] Tab 页一：中间件目录管理（ServiceCatalog CRUD）
- [ ] Tab 页二：共享资源管理
- [ ] Tab 页三：用户/角色管理
- [ ] 后端给 `ServiceCatalog` 增加 `POST/PUT/DELETE` handler
- [ ] 当前状态：`ListServiceCatalog` 仅只读
- [ ] 工作量：1 周

### Task 7.4: 三种角色体系 ✅
- [x] 角色定义：`platform_admin` / `app_admin` / `user`，并升级为 `roles` / `permissions` / `role_permissions` / `role_bindings` 权限表模型
- [x] 旧 `user_roles` 角色绑定路径已移除；系统角色绑定统一写入 `role_bindings(scope_type=system, scope_id=0)`，应用角色绑定统一写入 `role_bindings(scope_type=app, scope_id=<app_id>)`
- [x] 后端 `internal/middleware/auth.go` 统一认证并把当前用户 ID、角色写入 Gin context
- [x] 应用列表/详情/环境/组件/服务等应用域接口通过 `authz.Can` / 资源域 helper 做权限点鉴权，平台管理员保留全局可见性
- [x] 创建应用要求 `app_admin`，`platform_admin` 单独不能创建业务应用；创建后自动给创建者写入 `AppMember.role=admin`
- [x] 前端按 `platform_admin` 显隐用户管理等平台入口
- [x] Keycloak 登录回调从 realm roles、client roles、groups 提取 `platform_admin` / `app_admin` / `user`，upsert PAAP 用户后同步到新 `role_bindings`
- [x] 权限列表接口 `GET /api/v1/auth/permissions` 返回当前用户 roles 和 permission codes；前端权限 store、路由守卫和按钮显隐使用同一份权限点
- [x] 权限读取增加 5 分钟进程内缓存，角色绑定和角色权限变更时主动失效，当前阶段不引入 Redis
- [x] 验证：`source "$HOME/.gvm/scripts/gvm" && go test ./internal/... -count=1` 通过；`/api/v1/auth/login`、`/api/v1/auth/me`、`/api/v1/auth/permissions` 在 `kind-rbac-manager-test` 上验证通过；运行库 `user_roles` 表不存在，新表计数为 `roles=6`、`permissions=22`、`role_permissions=56`、`role_bindings=5`
- [x] 当前边界：已完成三种平台主角色、Keycloak 对接、权限点树、自定义角色基础和路由中间件鉴权；Casbin domain 模型暂不引入运行时，后续出现 ABAC、跨租户 domain 策略或复杂矩阵规则时再评估

### Task 7.5: Capability 来源模型（环境内/共享/外部）
> 领导需求 2+3+4 的统一模型，也是 External Capability Design Direction 的落地

- [x] 新增 `EnvironmentCapability` GORM 模型（`EnvironmentID` + `Capability` + `Source` + `RefServiceID` + `ExternalConfig`）
- [x] `Source` 枚举：`managed` / `shared` / `external` / `deferred`
- [x] 部署清单内置 `default` 应用 + `shared` 系统环境（受保护、用于共享资源池）；读接口和列表接口不再按需创建系统资源
- [x] 平台管理员通过 `/shared-resources` 进入同一套环境画布创建和维护共享实例，供其它环境 `shared` 引用
- [ ] 重构 `registry_endpoint.go:16` `RuntimeRegistryHost` 的硬编码
- [ ] 重构 `environment.go` 中 `toolHTTPBaseURL` 等 FQDN 拼接
- [x] 组件消费 capability 时按 `Source` 分流（self→本环境，shared→default，external→用户 endpoint）
  - 2026-06-26 验证：组件连接目标支持 `capability:<id>`；`backend-1` 已切换到 shared PostgreSQL/Redis，`/api/status` 返回 PostgreSQL read/write OK、Redis AUTH/SET/GET OK。
- [x] 放行 NetworkPolicy：业务 namespace → default/shared 工具 namespace
  - 2026-06-26 验证：`paap-deny-cross-env` egress 增加 `paap.io/app=default, paap.io/env=shared`，业务 Pod 可访问 `default-shared-postgresql` 和 `default-shared-redis-master`。
- [ ] external 来源放行到集群外 endpoint 的 egress
- [x] 画布节点带 `zone` 字段（`environment` / `shared` / `external`）
- [x] 三条泳道渲染：本环境、平台公共、集群外部
- [x] `componentTopology.ts` 扩展为可配置 zone，并由 `EnvDetailView.vue` 渲染背景分区与图例
- [ ] 扩展 `ListAdoptableResources` 可扫指定 namespace / 全集群
- [x] 业务环境画布右键新增共享资源和外部资源入口；共享资源生成只读卡片，外部资源右侧栏编辑 endpoint、认证方式、用户名、密码/token、`credentialSecretRef`
  - 2026-06-26 验证：业务环境中的共享/外部资源卡片支持画布本地重命名，显示名写入当前环境 canvas-state `names`，不修改共享资源池本体服务名、`refServiceId`、外部 endpoint 或凭据。
  - [x] 2026-06-26 验证：业务环境引用共享资源后的右侧栏展示共享资源连接信息，包含地址、端口、用户名、密码和连接串；不把 namespace、StatefulSet 等 Kubernetes 元数据作为主信息展示；业务环境不出现保存、部署、卸载共享服务本体的入口，仅保留断开引用。
  - [x] 2026-06-26 验证：外部资源右键二级菜单不再展示示例域名/endpoint，避免用户误解为平台写死地址；shared `linked` 状态在卡片和抽屉状态点都使用绿色。
- [x] external 卡片只支持"断开"，不删真实资源
- [ ] 环境模板声明所需 capabilities
- [x] 创建环境阶段不再选择 capability 来源，只选择环境形态；从模板/基础环境安装本环境默认资源，后续在画布上引用共享或接入外部
- [x] 当前状态：`ServiceInstallation` 仍是环境级；共享通过系统共享环境中的 `ServiceInstallation` 引用，外部通过 `EnvironmentCapability` + Secret 引用
- [ ] 当前状态：`ListAdoptableResources` 只扫自己 namespace（`environment.go:1700`）
- [ ] 工作量：4-6 周，按来源分三步交付

### Task 7.6: Ingress/Gateway 暴露面配置
- [ ] 给组件/环境添加暴露规则表单（域名、路径、TLS）
- [ ] 后端生成 Ingress 或 Gateway HTTPRoute 资源
- [ ] 当前状态：`external_access.go` 可读取，`component_types.go` 有 `IngressSpec`，画布有分组卡片
- [ ] 工作量：1-1.5 周

### Task 7.7: Service FQDN 展示 ✅
- [x] 以 `<service>.<namespace>.svc.cluster.local` 替代 ClusterIP / LoadBalancer IP 作为默认运行地址
- [x] 组件和服务 drawer 展示 Service DNS 全名，避免用户依赖不稳定 ClusterIP
- [x] ServiceIP 展示需求已从路线图移除，不再作为独立任务

### Task 7.8: 认证鉴权体系升级
- [x] 内存 token 替换为签名 JWT（`auth.go`）
- [x] 增加集中式 auth 中间件（`internal/middleware/auth.go`），除登录/注册/健康检查外保护默认 API 路由
- [x] 应用操作基于 `AppMember` 判断权限
- [x] 移除 `OwnerID=1`、`UserID=1` 等硬编码
- [x] 补齐应用成员管理页面和 API
- [x] 前端登录页调用真实 `/api/v1/auth/login`，保存 `paap_token` / `paap_user`，失败时展示错误状态
- [x] 前端 API client 自动为已有 token 请求添加 `Authorization: Bearer <jwt>`
- [x] Docker 镜像 `v0.1.441` 构建并部署到 kind 集群
- [x] 平台 admin 登录密码通过 `migration/20260624_001_update_platform_admin_password.sql` 更新为 `Def@u1tpwd`，数据库保存 bcrypt 哈希
- [x] CDP 验证：错误密码停留 `/login` 且显示 `登录失败：invalid credentials`；正确 `admin/Def@u1tpwd` 登录后写入三段式 JWT 并进入应用主界面
- [x] 前端路由守卫：未登录访问业务页先进入 `/login`，登录后恢复主界面访问
- [x] Docker 镜像 `v0.1.442` 构建并部署到 kind 集群
- [x] CDP 验证：未登录访问 `/apps` 自动到 `/login`；登录后 `/api/v1/applications` 使用 Bearer token 返回 200
- [x] Docker 镜像 `v0.1.443` 构建并部署到 kind 集群，验证 PostgreSQL `schema_migrations` 已记录 admin 密码迁移
- [x] API/CDP 验证：`admin/admin123` 返回 401；`admin/Def@u1tpwd` 登录成功并可访问受保护 API
- [ ] 工作量：1 周

### Task 7.8a: 创建应用归属当前登录用户 ✅
> 用户通过受保护 API 创建应用时，应用 owner 和 owner 成员记录使用 JWT 解析出的当前用户 ID，不再固定写入 `1`。

- [x] `CreateApplication` 从 Gin auth context 读取 `authUserID`，缺失时返回 401
- [x] 新建应用 `OwnerID` 使用当前用户 ID
- [x] owner 成员 `AppMember.UserID` 使用当前用户 ID，并检查成员创建错误
- [x] 后端目标测试：`go test ./internal/handler -run TestCreateApplicationUsesAuthenticatedUserAsOwner -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.457` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.457`，Deployment `1/1 ready`，Pod `paap-server-5b547b65fb-ztck7` Running
- [x] API/数据库验证：临时普通用户 ID=3 创建应用后，API 返回 `ownerId=3`，数据库 `applications.owner_id/app_members.user_id=3,3`；临时应用和用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/application.go`、`internal/handler/application_test.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8b: 集群同步应用归属平台管理员 ✅
> 从 Kubernetes CR 同步回数据库的应用不再直接写死 owner/member 为 `1`，改为解析数据库中的平台管理员用户 ID。

- [x] `SyncClusterState` 启动时解析 cluster sync owner：优先 `username=admin`，其次 `platform_admin/admin` 角色用户
- [x] `syncApplications`、`ensureApplication`、`ensureOwnerMember` 显式传递解析出的 owner ID
- [x] 移除 `cluster_sync.go` 中直接写入 `OwnerID: 1` / `UserID: 1` 的路径
- [x] 后端目标测试：`go test ./internal/service -run TestSyncClusterStateRestoresDBFromExistingCRs -count=1` 先红后绿，覆盖 admin ID 非 1 场景
- [x] 后端 service 测试：`go test ./internal/service -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.458` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.458`，Deployment `1/1 ready`，Pod `paap-server-5c5c5d647f-qc9wh` Running
- [x] API 验证：`admin/Def@u1tpwd` 登录后调用 `GET /api/v1/applications` 返回 200，当前返回 3 个应用，cluster sync 路径正常
- [x] 对应文件：`internal/service/cluster_sync.go`、`internal/service/cluster_sync_test.go`
- [x] 工作量：S（半天）

### Task 7.8c: 应用列表按成员过滤 ✅
> 普通用户调用应用列表时只返回自己是成员的应用，平台管理员保留全量视图。

- [x] `ListApplications` 对普通用户通过 `app_members` 过滤应用列表
- [x] `admin` / `platform_admin` 角色保留全量应用列表
- [x] 缺少认证上下文时返回 401，避免未受保护路由误用
- [x] 后端目标测试：`go test ./internal/handler -run TestListApplicationsFiltersByAppMemberForRegularUsers -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.459` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.459`，Deployment `1/1 ready`，Pod `paap-server-86584986db-l4gg9` Running
- [x] API/数据库验证：临时普通用户 ID=4 创建应用后，`GET /api/v1/applications` 只返回 `list-auth-1782304105` 1 个应用；临时应用和用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/application.go`、`internal/handler/application_test.go`
- [x] 工作量：S（半天）

### Task 7.8d: 应用详情按成员鉴权 ✅
> 普通用户按 ID 查看应用详情时必须是该应用成员，平台管理员保留全量访问。

- [x] 新增 `requireApplicationAccess`，普通用户按 `app_members` 检查应用访问权限
- [x] `GetApplication` 查到应用后执行成员鉴权，非成员返回 403
- [x] 缺少认证上下文时返回 401，避免未受保护路由误用
- [x] 后端目标测试：`go test ./internal/handler -run TestGetApplicationRejectsNonMembers -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.460` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.460`，Deployment `1/1 ready`，Pod `paap-server-97cbb674b-jk4m2` Running
- [x] API/数据库验证：临时普通用户 ID=5 访问非成员应用 1 返回 403 和 `application access denied`；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/application.go`、`internal/handler/application_test.go`
- [x] 工作量：S（半天）

### Task 7.8e: 应用更新按成员鉴权 ✅
> 普通用户更新应用前必须具备该应用访问权限，避免非成员按 ID 修改应用信息。

- [x] `UpdateApplication` 先确认应用存在，再复用 `requireApplicationAccess`
- [x] 非成员更新返回 403，并且不执行名称/描述更新
- [x] 后端目标测试：`go test ./internal/handler -run TestUpdateApplicationRejectsNonMembers -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.461` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.461`，Deployment `1/1 ready`，Pod `paap-server-76dc675799-2t6x7` Running
- [x] API/数据库验证：临时普通用户 ID=6 更新非成员应用 1 返回 403 和 `application access denied`；应用名保持“测试应用”；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/application.go`、`internal/handler/application_test.go`
- [x] 工作量：S（半天）

### Task 7.8f: 应用删除按成员鉴权 ✅
> 普通用户删除应用前必须具备该应用访问权限，避免非成员按 ID 删除应用及其环境资源。

- [x] `DeleteApplication` 查到应用后立即复用 `requireApplicationAccess`
- [x] 非成员删除返回 403，并且不会执行环境/集群资源清理和数据库删除
- [x] 后端目标测试：`go test ./internal/handler -run TestDeleteApplicationRejectsNonMembers -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.462` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.462`，Deployment `1/1 ready`，Pod `paap-server-6d754b95dd-6zcr7` Running
- [x] API/数据库验证：临时普通用户 ID=7 删除非成员应用 1 返回 403 和 `application access denied`；应用仍存在；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/application.go`、`internal/handler/application_test.go`
- [x] 工作量：S（半天）

### Task 7.8g: 应用环境列表/创建按成员鉴权 ✅
> 普通用户列出或创建应用环境前必须具备该应用访问权限，避免非成员通过应用 ID 读取环境清单或写入新环境。

- [x] `ListApplicationEnvironments` 先确认应用存在，再复用 `requireApplicationAccess`
- [x] `CreateEnvironment` 在生成 identifier 和写库前复用 `requireApplicationAccess`
- [x] 非成员列表/创建均返回 403，并且创建请求不会写入环境记录
- [x] 后端目标测试：`go test ./internal/handler -run 'Test(CreateEnvironmentGeneratesIdentifierWhenMissing|ListApplicationEnvironmentsRejectsNonMembers|CreateEnvironmentRejectsNonMembers)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.463` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.463`，Deployment `1/1 ready`，Pod `paap-server-755bdc96bf-82zng` Running
- [x] API/数据库验证：临时普通用户 ID=8 列表和创建非成员应用 1 的环境均返回 403 和 `application access denied`；环境数量保持 2；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8h: 环境详情按成员鉴权 ✅
> 普通用户读取环境详情前必须具备所属应用访问权限，避免非成员通过环境 ID 读取组件、服务、infra 和外部访问入口。

- [x] `GetEnvironment` 查到环境后立即复用 `requireApplicationAccess(env.ApplicationID)`
- [x] 非成员详情请求返回 403，并且不会继续读取组件、服务、infra 或 K8s 外部访问
- [x] 后端目标测试：`go test ./internal/handler -run TestGetEnvironmentRejectsNonMembers -count=1` 先红后绿
- [x] 后端相关测试：`go test ./internal/handler -run 'Test(GetEnvironmentReturnsApplicationAndServiceExternalAccess|GetEnvironmentRejectsNonMembers)' -count=1` 通过
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.464` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.464`，Deployment `1/1 ready`，Pod `paap-server-6699984bb6-mhd9s` Running
- [x] API/数据库验证：临时普通用户 ID=9 读取非成员环境 1 返回 403 和 `application access denied`；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8i: 环境画布状态按成员鉴权 ✅
> 普通用户读取或保存环境画布状态前必须具备所属应用访问权限，避免非成员读取/篡改卡片位置、连线和显示名。

- [x] `GetEnvironmentCanvasState` 查到环境后立即复用 `requireApplicationAccess(env.ApplicationID)`
- [x] `SaveEnvironmentCanvasState` 在解析请求和写库前复用 `requireApplicationAccess(env.ApplicationID)`
- [x] 非成员读取/保存均返回 403，并且保存请求不会创建或修改画布状态
- [x] 后端目标测试：`go test ./internal/handler -run 'Test(GetEnvironmentCanvasStateRejectsNonMembers|SaveEnvironmentCanvasStateRejectsNonMembers)' -count=1` 先红后绿
- [x] 后端相关测试：`go test ./internal/handler -run 'Test(EnvironmentCanvasStatePersists(PositionsAndEdges|DisplayNames)|GetEnvironmentCanvasStateRejectsNonMembers|SaveEnvironmentCanvasStateRejectsNonMembers)' -count=1` 通过
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.465` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.465`，Deployment `1/1 ready`，Pod `paap-server-9f7cff44c-xv9rl` Running
- [x] API/数据库验证：临时普通用户 ID=10 读取/保存非成员环境 1 的画布状态均返回 403 和 `application access denied`；画布状态 hash 保持 `94cf0e248dbb3e3f4bd9e03f8540dfc2`；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8j: 环境删除按成员鉴权 ✅
> 普通用户删除环境前必须具备所属应用访问权限，避免非成员通过环境 ID 删除环境及其组件、服务、infra、画布和集群资源。

- [x] `DeleteEnvironment` 查到环境后立即复用 `requireApplicationAccess(env.ApplicationID)`
- [x] 非成员删除返回 403，并且不会执行集群清理或数据库级联删除
- [x] 后端目标测试：`go test ./internal/handler -run TestDeleteEnvironmentRejectsNonMembers -count=1` 先红后绿
- [x] 后端相关测试：`go test ./internal/handler -run 'Test(DeleteEnvironmentRejectsNonMembers|DeleteEnvironmentRemovesClusterCRsNamespacesAndDatabaseRows)' -count=1` 通过
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.466` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.466`，Deployment `1/1 ready`，Pod `paap-server-7f8cb8548d-nk95h` Running
- [x] API/数据库验证：临时普通用户 ID=11 删除非成员环境 1 返回 403 和 `application access denied`；环境计数保持 1，组件计数保持 7；临时用户已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8k: 环境组件/服务列表按成员鉴权 ✅
> 普通用户读取环境组件或服务列表前必须具备所属应用访问权限，避免非成员通过环境 ID 枚举运行组件和中间件。

- [x] `ListEnvironmentComponents` 先确认环境存在，再复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取组件列表
- [x] `ListServiceInstances` 先确认环境存在，再复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取服务列表和运行态信息
- [x] 非成员组件/服务列表请求均返回 403，并且不会继续读取子资源或运行态详情
- [x] 后端目标测试：`go test ./internal/handler -run 'Test(ListEnvironmentComponentsRejectsNonMembers|ListServiceInstancesRejectsNonMembers)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.467` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.467`，Deployment `1/1 ready`，Pod `paap-server-fcbbcb549-vwfph` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=12 读取 `/api/v1/environments/1/components` 和 `/api/v1/environments/1/services` 均返回 403 和 `application access denied`；临时加入应用 1 成员后两个接口均返回 200；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8l: 环境可接入资源按成员鉴权 ✅
> 普通用户查看或接入环境内可纳管资源前必须具备所属应用访问权限，避免非成员扫描环境命名空间或创建接入组件草稿。

- [x] `loadEnvironmentAndApp` 查到环境和应用后复用 `requireApplicationAccess(app.ID)`，保护 `ListAdoptableResources` 与 `AdoptResource`
- [x] 非成员可接入资源列表/接入请求均返回 403，并且不会继续进入 K8s 发现或创建组件草稿
- [x] 后端目标测试：`go test ./internal/handler -run 'Test(ListAdoptableResourcesRejectsNonMembers|AdoptResourceRejectsNonMembers)' -count=1` 先红后绿
- [x] 后端相关测试：`go test ./internal/handler -run 'Test(AdoptResourceDiscoversAndCreatesDraftFromRealWorkload|ListAdoptableResourcesRejectsNonMembers|AdoptResourceRejectsNonMembers)' -count=1` 通过
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.468` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.468`，Deployment `1/1 ready`，Pod `paap-server-d56d8646-vw7sv` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=13 读取/接入 `/api/v1/environments/1/adoptable-resources` 均返回 403 和 `application access denied`；临时加入应用 1 成员后列表返回 200 且 `data: []`；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8m: 服务实例详情按成员鉴权 ✅
> 普通用户读取单个服务实例详情前必须具备所属应用访问权限，避免非成员枚举服务实例、CR 状态和外部访问入口。

- [x] `GetServiceInstance` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查询服务实例、应用、CR 状态和外部访问入口
- [x] 非成员访问存在或不存在的服务实例均先返回 403，避免通过 404/200 判断服务实例是否存在
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetServiceInstanceRejectsNonMembers|TestGetServiceInstanceRejectsNonMembersBeforeServiceLookup' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.470` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.470`，Deployment `1/1 ready`，Pod `paap-server-67b8d9d95c-khgs6` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=14 读取 `/api/v1/environments/5/services/22` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，服务类型 `git`、命名空间 `real-fullstack-prod-gitea`；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8n: 服务实例凭据按成员鉴权 ✅
> 普通用户读取服务实例凭据前必须具备所属应用访问权限，避免非成员通过服务 ID 读取 Kubernetes Secret 派生的账号、密码、令牌等敏感值。

- [x] `GetServiceCredentials` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查询服务实例和 Kubernetes Secret
- [x] 非成员访问存在或不存在的服务实例凭据均先返回 403，避免通过 404/424 判断服务实例或 K8s 客户端状态
- [x] 正向凭据测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户读取 Secret
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetServiceCredentials(ReadsRealSecrets|RejectsNonMembers|RejectsNonMembersBeforeServiceLookup)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.471` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.471`，Deployment `1/1 ready`，Pod `paap-server-75d9b5d47b-44dwc` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=15 读取 `/api/v1/environments/5/services/22/credentials` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，`credentials: []`；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8o: 服务工作区按成员鉴权 ✅
> 普通用户读取服务实例工作区前必须具备所属应用访问权限，避免非成员枚举代码仓库、GitOps Application、工作区操作和代理入口。

- [x] `GetServiceWorkspace` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才加载服务实例、应用和组件工作区资源
- [x] 非成员访问存在或不存在的服务工作区均先返回 403，避免通过 404/200 判断服务实例是否存在
- [x] 正向工作区测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户读取 workspace
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetServiceWorkspace(ReturnsBackendWorkspace|RejectsNonMembers|RejectsNonMembersBeforeServiceLookup)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.472` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.472`，Deployment `1/1 ready`，Pod `paap-server-75fb5767b7-rc6cs` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=16 读取 `/api/v1/environments/5/services/22/workspace` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，工作区类型 `repository`、资源 1 个、操作 5 个；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8p: 服务运行指标按成员鉴权 ✅
> 普通用户读取服务实例运行指标前必须具备所属应用访问权限，避免非成员通过服务 ID 探测命名空间、Pod、容器和资源用量。

- [x] `GetServiceRuntimeMetrics` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才加载服务实例和查询 Kubernetes/Prometheus 指标
- [x] 非成员访问存在或不存在的服务运行指标均先返回 403，避免通过 404/424 判断服务实例或 K8s 指标状态
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetServiceRuntimeMetricsRejectsNonMembers|TestGetServiceRuntimeMetricsRejectsNonMembersBeforeServiceLookup' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.473` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.473`，Deployment `1/1 ready`，Pod `paap-server-5984f9f46b-gxb2s` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=17 读取 `/api/v1/environments/5/services/22/runtime-metrics` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，`available: true`，样本包含 `real-fullstack-prod-gitea` Pod；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8q: 服务运行日志按成员鉴权 ✅
> 普通用户读取服务实例运行日志前必须具备所属应用访问权限，避免非成员通过服务 ID 查看 Pod、容器和应用日志内容。

- [x] `GetServiceRuntimeLogs` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才加载服务实例和查询 Kubernetes 日志
- [x] 非成员访问存在或不存在的服务运行日志均先返回 403，避免通过 404/424 判断服务实例或 K8s 日志状态
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetServiceRuntimeLogsRejectsNonMembers|TestGetServiceRuntimeLogsRejectsNonMembersBeforeServiceLookup' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.474` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.474`，Deployment `1/1 ready`，Pod `paap-server-768597bb45-nnprh` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=18 读取 `/api/v1/environments/5/services/22/runtime-logs?tail=10` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，`available: true`，目标为 `real-fullstack-prod-gitea`，返回 10 行日志样本；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8r: 服务工作区动作按成员鉴权 ✅
> 普通用户执行服务实例工作区动作前必须具备所属应用访问权限，避免非成员触发刷新、仓库、GitOps 或监控类操作。

- [x] `RunServiceWorkspaceAction` 解析动作请求后先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才加载服务实例和执行 `refresh`/GitOps/Gitea/Grafana 动作
- [x] 非成员访问存在或不存在的服务工作区动作均先返回 403，避免通过 404/200 判断服务实例是否存在或执行只读 refresh
- [x] 既有 GitOps action 测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户执行 action
- [x] 后端目标测试：`go test ./internal/handler -run 'Test(RunServiceWorkspaceActionRejectsNonMembers|RunServiceWorkspaceActionRejectsNonMembersBeforeServiceLookup|ApplyArgoCDApplicationRejectsNamespaceOutsideEnvironment|ApplyArgoCDApplicationForcesEnvironmentProject|ApplyArgoCDApplicationSetForcesEnvironmentProject)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.475` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.475`，Deployment `1/1 ready`，Pod `paap-server-685bb65db8-9rmjc` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=19 POST `/api/v1/environments/5/services/22/workspace/actions` + `{"action":"refresh"}` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一 action 返回 200，工作区类型 `repository`、资源 1 个、动作 5 个；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8s: 服务 Registry CA 下载按成员鉴权 ✅
> 普通用户下载 registry/harbor 服务 CA 证书前必须具备所属应用访问权限，避免非成员探测服务实例和读取环境信任材料。

- [x] `DownloadRegistryCACertificate` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才加载服务实例和读取 Kubernetes Secret 中的 CA
- [x] 非成员访问存在或不存在的 registry CA 均先返回 403，避免通过 404/424 判断服务实例或 K8s Secret 状态
- [x] 正向 CA 下载测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户下载 CA，继续校验只返回 `ca.crt` 不泄露私钥
- [x] 后端目标测试：`go test ./internal/handler -run 'TestDownloadRegistryCACertificate(ReturnsPublicCA|RejectsNonMembers|RejectsNonMembersBeforeServiceLookup)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.476` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.476`，Deployment `1/1 ready`，Pod `paap-server-5746584459-7kj7b` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=20 读取 `/api/v1/environments/5/services/23/registry-ca.crt` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一接口返回 200，`Content-Type: application/x-pem-file`，来源 `real-fullstack-prod-registry-tls/ca.crt`，内容包含 `BEGIN CERTIFICATE` 且不包含 `PRIVATE KEY`；临时用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8t: 服务草稿创建按成员鉴权 ✅
> 普通用户创建环境服务草稿前必须具备所属应用访问权限，避免非成员探测服务模板或向他人环境写入服务草稿。

- [x] `CreateServiceDraft` 在读取环境后复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取应用、查找服务模板和创建/更新草稿
- [x] 非成员创建服务草稿返回 403，并且不会创建 `service_installations` 记录
- [x] 非成员请求不存在模板也先返回 403，避免通过 404 判断服务模板是否存在
- [x] 正向服务草稿测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户创建草稿，继续校验草稿不会创建 ServiceInstance CR
- [x] 后端目标测试：`go test ./internal/handler -run 'TestCreateServiceDraft(DoesNotCreateServiceInstanceCR|RejectsNonMembers|RejectsNonMembersBeforeTemplateLookup)|TestCreateComponent(RejectsImplicitLatestBeforeCreatingRecord|CreatesDraftWithoutDeploying|AllowsCanvasDraftWithoutImage)' -count=1` 通过
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.477` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.477`，Deployment `1/1 ready`，Pod `paap-server-79d4ddc54-zfl5z` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=21 POST `/api/v1/environments/5/services/drafts` + `{"serviceType":"not-a-template-1782318882303"}` 返回 403 和 `application access denied`；临时加入应用 5 成员后同一请求返回 404 和 `service template not found`，证明鉴权通过后才进入模板查找；临时用户和成员关系已清理，服务草稿残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8u: 组件草稿创建按成员鉴权 ✅
> 普通用户创建环境组件草稿前必须具备所属应用访问权限，避免非成员向他人环境写入组件草稿或通过 payload 校验结果探测环境状态。

- [x] `CreateComponent` 在读取环境后复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取应用、校验请求体和创建组件草稿
- [x] 非成员创建组件草稿返回 403，并且不会创建 `components` 记录
- [x] 非成员即使提交本应触发 `latest` 镜像校验的 payload，也先返回 403，避免泄露后续校验路径
- [x] 既有组件草稿正向测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户创建或校验组件草稿
- [x] 后端目标测试：`go test ./internal/handler -run 'TestCreateComponent(RejectsImplicitLatestBeforeCreatingRecord|RejectsNonMembers|RejectsNonMembersBeforePayloadValidation|CreatesDraftWithoutDeploying|AllowsCanvasDraftWithoutImage)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.478` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.478`，Deployment `1/1 ready`，Pod `paap-server-85bd544cc-hc7bv` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时普通用户 ID=22 POST `/api/v1/environments/5/components` 创建 `auth-draft-1782320211408` 返回 403 和 `application access denied`；临时加入应用 5 成员后，`latest` 镜像 payload 返回 400 和 `component image tag must be explicit; latest is not allowed`，正常 payload 返回 201、组件 ID=52、状态 `draft`；临时组件、用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8v: 组件更新按成员鉴权 ✅
> 普通用户更新组件运行配置或交付配置前必须具备组件所属应用访问权限，避免非成员修改组件草稿、运行参数或通过 payload 校验结果探测组件状态。

- [x] `UpdateComponent` 读取组件后先加载所属环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取应用、解析请求体和保存组件
- [x] 非成员更新组件返回 403，组件 `replicas/cpu` 等字段保持不变
- [x] 非成员即使提交本应触发 `latest` 镜像校验的 payload，也先返回 403，避免泄露后续校验路径
- [x] 既有组件更新正向测试补齐真实受保护路由上下文：创建 app/env/member 后以成员用户更新运行配置、镜像交付、源码交付和保留配置
- [x] 后端目标测试：`go test ./internal/handler -run 'TestUpdateComponent(PersistsRuntimeConfig|RejectsNonMembers|RejectsNonMembersBeforePayloadValidation|KeepsRegistryImageInSyncForImageDelivery|CanSwitchDraftToSourceDelivery|KeepsExistingConfigWhenConfigOmitted)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.479` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.479`，Deployment `1/1 ready`，Pod `paap-server-687945db96-l6m9v` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时组件 ID=53、临时普通用户 ID=23 PUT `/api/v1/components/53` 更新 `replicas/cpu` 返回 403 和 `application access denied`，组件仍为 `replicas=1,cpu=100m`；临时加入应用 5 成员后，`latest` 镜像 payload 返回 400 和 `component image tag must be explicit; latest is not allowed`，正常 payload 返回 200、`replicas=3,cpu=500m,memory=384Mi,version=v2,status=draft`；临时组件、用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8w: 组件部署按成员鉴权 ✅
> 普通用户触发组件部署前必须具备组件所属应用访问权限，避免非成员修改组件版本或触发 GitOps/K8s 部署流程。

- [x] `DeployComponent` 读取组件后先加载所属环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才读取应用、解析版本参数和执行 GitOps/K8s 更新
- [x] 非成员触发组件部署返回 403，组件 `version/status` 保持不变，不进入 GitOps/K8s 流程
- [x] 非成员即使提交空版本 payload，也先返回 403，避免通过 400 判断组件状态或校验路径
- [x] 成员提交空版本 payload 返回 400 和 `version is required`，证明鉴权通过后才进入版本校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestDeployComponent(RejectsNonMembers|RejectsNonMembersBeforeVersionValidation|ValidatesVersionAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.480` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.480`，Deployment `1/1 ready`，Pod `paap-server-6bd796888d-pcs5m` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时组件 ID=54、临时普通用户 ID=24 POST `/api/v1/components/54/deploy` + `{"version":"v2"}` 返回 403 和 `application access denied`，空版本 payload 同样返回 403，组件仍为 `version=v1,status=draft`；临时加入应用 5 成员后，空版本 payload 返回 400 和 `version is required`；临时组件、用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8x: 组件外部访问开关按成员鉴权 ✅
> 普通用户开启或关闭组件外部访问前必须具备组件所属应用访问权限，避免非成员触发 K8s Service 暴露变更或探测组件存在性。

- [x] `SetComponentExternalAccess` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才检查 namespace、查找组件和调用 K8s 外部访问开关
- [x] 非成员开关存在组件返回 403，不进入 K8s 操作
- [x] 非成员访问不存在组件也先返回 403，避免通过 404 探测组件
- [x] 成员访问 namespace 未就绪环境返回 409 和 `environment namespace is not ready`，证明鉴权通过后才进入业务校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestSetComponentExternalAccess(RejectsNonMembers|ChecksNamespaceAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.481` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.481`，Deployment `1/1 ready`，Pod `paap-server-7f97dcd6df-8z4gk` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 9/7/55、临时普通用户 ID=25 PUT `/api/v1/environments/7/components/55/external-access` 返回 403 和 `application access denied`，请求不存在组件 `/components/99999/external-access` 同样返回 403；临时加入应用 9 成员后，同一组件请求返回 409 和 `environment namespace is not ready`；临时 app/env/component、用户和成员关系已清理，残留计数 0
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8y: 组件 NodePort 访问开关按成员鉴权 ✅
> 普通用户开启或关闭组件 NodePort 访问前必须具备组件所属应用访问权限，避免非成员触发 K8s Service NodePort 暴露变更或探测组件存在性。

- [x] `SetComponentNodePortAccess` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才检查 namespace、查找组件和调用 K8s NodePort 开关
- [x] 非成员开关存在组件返回 403，不进入 K8s 操作
- [x] 非成员访问不存在组件也先返回 403，避免通过 404 探测组件
- [x] 成员访问 namespace 未就绪环境返回 409 和 `environment namespace is not ready`，证明鉴权通过后才进入业务校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestSetComponentNodePortAccess(RejectsNonMembers|ChecksNamespaceAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.482` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.482`，Deployment `1/1 ready`，Pod `paap-server-6f5cfd95d6-kgqqs` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 10/8/56、临时普通用户 ID=26 PUT `/api/v1/environments/8/components/56/nodeport-access` 返回 403 和 `application access denied`，请求不存在组件 `/components/999999/nodeport-access` 同样返回 403；临时加入应用 10 成员后，同一组件请求返回 409 和 `environment namespace is not ready`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8z: 服务外部访问开关按成员鉴权 ✅
> 普通用户开启或关闭服务外部访问前必须具备服务所属应用访问权限，避免非成员触发 K8s Service 暴露变更或探测服务存在性。

- [x] `SetServiceExternalAccess` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查找服务安装、检查服务 namespace 和调用 K8s 外部访问开关
- [x] 非成员开关存在服务返回 403，不进入 K8s 操作
- [x] 非成员访问不存在服务也先返回 403，避免通过 404 探测服务安装
- [x] 成员访问服务 namespace 未就绪返回 409 和 `service namespace is not ready`，证明鉴权通过后才进入业务校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestSetServiceExternalAccess(RejectsNonMembers|ChecksNamespaceAfterMemberAccess|PatchesLiveService)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] 后端全量测试：`make test` 通过
- [x] Docker 镜像 `v0.1.483` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.483`，Deployment `1/1 ready`，Pod `paap-server-57d58f7c79-8jhkp` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/service 为 11/9/64、临时普通用户 ID=27 PUT `/api/v1/environments/9/services/64/external-access` 返回 403 和 `application access denied`，请求不存在服务 `/services/999999/external-access` 同样返回 403；临时加入应用 11 成员后，同一服务请求返回 409 和 `service namespace is not ready`；临时 app/env/service、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8aa: 服务配置更新按成员鉴权 ✅
> 普通用户更新服务配置前必须具备服务所属应用访问权限，避免非成员修改服务 Helm values 或通过服务 ID 探测服务安装。

- [x] `UpdateService` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才触发集群同步、查找服务安装、查找服务模板和更新 DB/ServiceInstance CR
- [x] 非成员更新存在服务返回 403，不修改 `service_installations.values`
- [x] 非成员更新不存在服务也先返回 403，避免通过 404 探测服务安装
- [x] 成员访问缺失模板服务返回 404 和 `service template not found`，证明鉴权通过后才进入业务校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestUpdateService(DraftSavesValuesWithoutCreatingCR|RejectsNonMembers|RejectsNonMembersBeforeServiceLookup|ChecksTemplateAfterMemberAccess|RunningServiceReconcilesServiceInstanceCR)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] Docker 镜像 `v0.1.484` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.484`，Deployment `1/1 ready`，Pod `paap-server-6949b487b8-m6dhj` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/template/service 为 12/10/16/65、临时普通用户 ID=28 PUT `/api/v1/environments/10/services/65` 返回 403 和 `application access denied`，请求不存在服务 `/services/999999` 同样返回 403；临时加入应用 12 成员后，同一服务更新返回 200，values 保存 `mode=member-update` 与默认值合并；临时 template、service、app/env、用户和成员关系已清理，残留计数 `0|0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ab: 服务卸载按成员鉴权 ✅
> 普通用户卸载服务前必须具备服务所属应用访问权限，且 serviceId 必须属于当前环境，避免非成员或跨环境路径删除服务实例与命名空间。

- [x] `UninstallService` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才按 `id + environment_id` 查找服务安装并执行 CR/namespace/DB 删除
- [x] 非成员卸载存在服务返回 403，不删除 `service_installations`
- [x] 非成员卸载不存在服务也先返回 403，避免通过 404 探测服务安装
- [x] 成员用 A 环境路径卸载 B 环境 serviceId 返回 404，并保留 B 环境服务记录
- [x] 后端目标测试：`go test ./internal/handler -run 'TestUninstallService(RejectsNonMembers|RejectsNonMembersBeforeServiceLookup|ScopesServiceToEnvironmentAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] Docker 镜像 `v0.1.485` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.485`，Deployment `1/1 ready`，Pod `paap-server-6d59c99977-96548` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/service 为 13/11,12/66,67、临时普通用户 ID=29 DELETE `/api/v1/environments/11/services/66` 返回 403 和 `application access denied`，请求不存在服务 `/services/999999` 同样返回 403；临时加入应用 13 成员后，跨环境 `/environments/11/services/67` 返回 404，正确 `/environments/11/services/66` 返回 200；临时 app/env/service、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ac: 服务安装按成员鉴权 ✅
> 普通用户安装环境服务前必须具备服务所属应用访问权限，避免非成员触发服务安装、创建失败安装记录或通过模板查询结果探测环境内容。

- [x] `InstallService` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才触发集群同步、读取应用、查找服务模板和创建/更新 ServiceInstance
- [x] 非成员安装存在模板返回 403，不创建 `service_installations`
- [x] 非成员安装不存在模板也先返回 403，避免通过 404 探测服务模板
- [x] 成员安装不存在模板返回 404 和 `service template not found`，证明鉴权通过后才进入模板校验
- [x] 后端目标测试：`go test ./internal/handler -run 'TestInstallService(DeploysExistingDraft|RejectsNonMembers|RejectsNonMembersBeforeTemplateLookup|ChecksTemplateAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] Docker 镜像 `v0.1.486` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.486`，Deployment `1/1 ready`，Pod `paap-server-56dc9f8b46-2zzxf` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/template 为 14/13/17、临时普通用户 ID=30 POST `/api/v1/environments/13/services` + `{"serviceType":"ins8985"}` 返回 403 和 `application access denied`，请求不存在模板 `missing-ins-1782328985775668` 同样返回 403；临时加入应用 14 成员后，不存在模板请求返回 404 和 `service template not found`；临时 template、app/env、用户和成员关系已清理，残留计数 `0|0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ad: 组件运行指标按成员鉴权 ✅
> 普通用户读取组件运行指标前必须具备组件所属应用访问权限，避免非成员通过组件 ID 探测部署环境、运行实例和指标状态。

- [x] `GetComponentRuntimeMetrics` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才触发集群同步、查找组件和查询 Kubernetes/Prometheus 指标
- [x] 非成员读取存在组件指标返回 403，不进入 K8s 指标查询
- [x] 非成员读取不存在组件指标也先返回 403，避免通过 404 探测组件
- [x] 成员读取不存在组件指标返回 404 和 `component not found`，证明鉴权通过后才进入组件查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetComponentRuntimeMetrics(RejectsNonMembers|RejectsNonMembersBeforeComponentLookup|ChecksComponentAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端 handler 测试：`go test ./internal/handler -count=1` 通过
- [x] Docker 镜像 `v0.1.487` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.487`，Deployment `1/1 ready`，Pod `paap-server-7c8ccb4cc5-9r2ks` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 15/14/57、临时普通用户 ID=31 GET `/api/v1/environments/14/components/57/runtime-metrics` 返回 403 和 `application access denied`，请求不存在组件 `/components/999999/runtime-metrics` 同样返回 403；临时加入应用 15 成员后，不存在组件请求返回 404 和 `component not found`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ae: 组件运行日志按成员鉴权 ✅
> 普通用户读取组件运行日志前必须具备组件所属应用访问权限，避免非成员通过组件 ID 探测部署环境、运行实例和日志状态。

- [x] `GetComponentRuntimeLogs` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才触发集群同步、查找组件和查询 Kubernetes 日志
- [x] 非成员读取存在组件日志返回 403，不进入 K8s 日志查询
- [x] 非成员读取不存在组件日志也先返回 403，避免通过 404 探测组件
- [x] 成员读取不存在组件日志返回 404 和 `component not found`，证明鉴权通过后才进入组件查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestGetComponentRuntimeLogs(RejectsNonMembers|RejectsNonMembersBeforeComponentLookup|ChecksComponentAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.488` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.488`，Deployment `1/1 ready`，Pod `paap-server-d9dfd8596-b94kf` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 16/15/58、临时普通用户 ID=32 GET `/api/v1/environments/15/components/58/runtime-logs?tail=10` 返回 403 和 `application access denied`，请求不存在组件 `/components/999999/runtime-logs?tail=10` 同样返回 403；临时加入应用 16 成员后，不存在组件请求返回 404 和 `component not found`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8af: 组件代理入口按成员鉴权 ✅
> 普通用户访问组件浏览器代理前必须具备组件所属应用访问权限，避免非成员通过代理入口探测组件存在性或运行 Service 状态。

- [x] `ProxyComponent` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查找组件和解析 Kubernetes Service 代理目标
- [x] 非成员代理存在组件返回 403，不进入 K8s Service 查找
- [x] 非成员代理不存在组件也先返回 403，避免通过 404 探测组件
- [x] 成员代理不存在组件返回 404 和 `component not found`，证明鉴权通过后才进入组件查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestProxyComponent(RejectsNonMembers|RejectsNonMembersBeforeComponentLookup|ChecksComponentAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.489` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.489`，Deployment `1/1 ready`，Pod `paap-server-65d6fdb949-dw8nf` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 17/16/59、临时普通用户 ID=33 GET `/api/v1/environments/16/components/59/proxy/` 返回 403 和 `application access denied`，请求不存在组件 `/components/999999/proxy/` 同样返回 403；临时加入应用 17 成员后，不存在组件请求返回 404 和 `component not found`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ag: 组件控制台按成员鉴权 ✅
> 普通用户打开组件运行控制台前必须具备组件所属应用访问权限，避免非成员通过控制台入口探测组件存在性或运行 Pod 状态。

- [x] `HandleComponentConsole` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查找组件、解析运行目标并尝试 WebSocket 升级
- [x] 非成员打开存在组件控制台返回 403，不进入 K8s 运行目标解析
- [x] 非成员打开不存在组件控制台也先返回 403，避免通过 404 探测组件
- [x] 成员打开不存在组件控制台返回 404 和 `component not found`，证明鉴权通过后才进入组件查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestHandleComponentConsole(RejectsNonMembers|RejectsNonMembersBeforeComponentLookup|ChecksComponentAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.490` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.490`，Deployment `1/1 ready`，Pod `paap-server-7558b7786c-jwsks` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 18/17/60、临时普通用户 ID=34 GET `/api/v1/environments/17/components/60/console` 返回 403 和 `application access denied`，请求不存在组件 `/components/999999/console` 同样返回 403；临时加入应用 18 成员后，不存在组件请求返回 404 和 `component not found`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/runtime_console.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ah: 服务代理入口按成员鉴权 ✅
> 普通用户访问服务浏览器代理前必须具备服务所属应用访问权限，避免非成员通过代理入口探测服务安装、模板能力或代理地址状态。

- [x] `ProxyServiceInstance` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查找服务安装和解析代理目标
- [x] 非成员代理存在服务返回 403，不进入服务代理能力检查
- [x] 非成员代理不存在服务也先返回 403，避免通过 404 探测服务安装
- [x] 成员代理不存在服务返回 404 和 `service not found`，证明鉴权通过后才进入服务查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestProxyServiceInstance(RejectsNonMembers|RejectsNonMembersBeforeServiceLookup|ChecksServiceAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.491` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.491`，Deployment `1/1 ready`，Pod `paap-server-f578d98fd-hj6w6` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/service 为 19/18/68、临时普通用户 ID=35 GET `/api/v1/environments/18/services/68/proxy/` 返回 403 和 `application access denied`，请求不存在服务 `/services/999999/proxy/` 同样返回 403；临时加入应用 19 成员后，不存在服务请求返回 404 和 `service not found`；临时 app/env/service、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8ai: 服务控制台按成员鉴权 ✅
> 普通用户打开服务运行控制台前必须具备服务所属应用访问权限，避免非成员通过控制台入口探测服务安装或运行 Pod 状态。

- [x] `HandleServiceConsole` 先读取环境并复用 `requireApplicationAccess(env.ApplicationID)`，通过后才查找服务安装、解析运行目标并尝试 WebSocket 升级
- [x] 非成员打开存在服务控制台返回 403，不进入 K8s 运行目标解析
- [x] 非成员打开不存在服务控制台也先返回 403，避免通过 404 探测服务安装
- [x] 成员打开不存在服务控制台返回 404 和 `service not found`，证明鉴权通过后才进入服务查找
- [x] 后端目标测试：`go test ./internal/handler -run 'TestHandleServiceConsole(RejectsNonMembers|RejectsNonMembersBeforeServiceLookup|ChecksServiceAfterMemberAccess)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.492` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.492`，Deployment `1/1 ready`，Pod `paap-server-5b675648cc-stlkv` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/service 为 20/19/69、临时普通用户 ID=36 GET `/api/v1/environments/19/services/69/console` 返回 403 和 `application access denied`，请求不存在服务 `/services/999999/console` 同样返回 403；临时加入应用 20 成员后，不存在服务请求返回 404 和 `service not found`；临时 app/env/service、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/runtime_console.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.8aj: 组件删除按成员鉴权 ✅
> 普通用户删除组件前必须具备组件所属应用访问权限，避免非成员通过组件 ID 触发 ArgoCD、Component CR 和运行态资源清理。

- [x] `DeleteComponent` 读取组件所属环境后复用 `requireApplicationAccess(env.ApplicationID)`，通过后才执行 ArgoCD、Component CR、运行态资源和数据库删除
- [x] 非成员删除存在组件返回 403，不进入 K8s 删除路径
- [x] 非成员删除存在组件不会删除 `components` 数据库记录
- [x] 既有成功删除测试补齐真实受保护路由上下文：以平台管理员角色执行删除，继续校验 ArgoCD Application、Component CR、运行态资源和画布引用清理
- [x] 后端目标测试：`go test ./internal/handler -run 'TestDeleteComponent(RejectsNonMembers|RemovesArgoCDApplicationAndRuntimeResources|UsesArgoCDApplicationIdentifierForRuntimeCleanup)' -count=1` 先红后绿
- [x] 后端全量测试：`go test ./internal/handler -count=1`、`make test` 通过
- [x] Docker 镜像 `v0.1.493` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.493`，Deployment `1/1 ready`，Pod `paap-server-7b8576f496-j4h62` Running；`paap-system` 与 `kpack` Pod 均 Running，节点 Ready
- [x] CDP 验证：复用 Chrome tab `http://172.18.0.2:30091/catalog`，临时 app/env/component 为 21/20/61、临时普通用户 ID=37 DELETE `/api/v1/components/61` 返回 403 和 `application access denied`；删除后组件记录计数保持 `1`；临时 app/env/component、用户和成员关系已清理，残留计数 `0|0|0|0|0`
- [x] 对应文件：`internal/handler/environment.go`、`internal/handler/environment_test.go`
- [x] 工作量：S（半天）

### Task 7.9: KubeVirt 服务模板
- [x] 将 KubeVirt 作为平台基础设施能力，而不是面向用户的裸虚拟机创建入口
- [x] 服务产品支持 `provisionMode=kubevirt`，用户创建的是 PostgreSQL、Redis、MySQL 等服务实例
- [x] 平台管理员维护 KubeVirt 服务模板：基础镜像/DataVolume、CPU、内存、磁盘、cloud-init/启动脚本、服务端口、凭据生成、readiness、监控 agent/exporter、备份/快照策略
  - 2026-06-28 补齐：KubeVirt `runtimeSpec` 保存前校验 image/ports/readiness/monitoring；资源生成层支持 `readiness` 探针、`monitoring` 目标和 `backupPolicy` 元数据注解，管理员通过服务模板 API/模板包维护这些字段时可提前发现错误。
  - 2026-06-28 验证：`go test -count=1 ./internal/k8s ./internal/service -run 'Test(BuildKubeVirtService|CreateServiceTemplateValidatesKubeVirtRuntimeSpec|UpsertSeedServiceTemplateSeparatesProvisionModes)'` 通过。
- [x] 创建服务实例时由模板生成 KubeVirt `VirtualMachine`、Kubernetes `Service`、Secret、连接输出和监控目标的资源生成层
  - 2026-06-28 补齐：新增 `internal/k8s/kubevirt.go` 和 `internal/service/kubevirt.go`，从 `runtimeSpec` 生成 KubeVirt `VirtualMachine`、可选 CDI `DataVolume`、Kubernetes `Service`、凭据 `Secret`、连接输出和监控目标；生成层要求镜像和服务端口必填，缺失时直接拒绝。
  - 2026-06-28 验证：`go test -count=1 ./internal/k8s ./internal/service -run 'TestBuildKubeVirtService'` 通过。
- [x] `InstallService` API 接入 KubeVirt 分支，按 `provisionMode=kubevirt` 选择服务模板并 upsert KubeVirt 资源
  - 2026-06-28 补齐：`InstallServiceRequest` 增加 `provisionMode`；`CreateServiceDraft` / `InstallService` 按 provision mode 查询模板；KubeVirt 分支不再创建 Helm `ServiceInstance` CR，而是创建 Namespace、Secret、Service、DataVolume 和 VirtualMachine。
  - 2026-06-28 补齐：环境服务弹窗的 KubeVirt 模板交付入口从置灰改为可提交，调用 `api.installService(..., { provisionMode: 'kubevirt' })`，并按 provision mode 区分同一服务类型的 Helm/KubeVirt 已安装状态。
  - 2026-06-28 验证：`go test -count=1 ./internal/k8s ./internal/service ./internal/handler -run 'Test(Build|Upsert)KubeVirtService|TestInstallServiceDeploysKubeVirtResources|TestInstallServiceDeploysExistingDraft|TestCreateServiceDraftDoesNotCreateServiceInstanceCR'`、`npm --prefix frontend run test -- src/views/viewMarkup.test.ts --run`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过。
- [x] 卸载 KubeVirt 服务时清理运行态资源和安装记录
  - 2026-06-28 补齐：`UninstallService` 按 `provisionMode=kubevirt` 走 `DeleteKubeVirtServiceResources`，清理同 namespace 下带 `paap.io/provision-mode=kubevirt` 和 `paap.io/service-type=<type>` 标签的 `VirtualMachine`、`DataVolume`、`Service`、`Secret`，再删除服务 namespace；不再误删 Helm `ServiceInstance` CR。
  - 2026-06-28 验证：`go test -count=1 ./internal/k8s ./internal/service ./internal/handler -run 'Test(Delete|Upsert|Build)KubeVirtService|TestUninstallService(DeletesKubeVirtRuntimeResources|HardDeletesServiceInstallation|KeepsRuntimeInstallationWhenCRDeleteFails)'` 通过。
- [x] 服务列表/详情读取 KubeVirt VM 的基础状态、运行配置、Service 网络和凭据
  - 2026-06-28 补齐：`DiscoverKubeVirtServiceRuntimeConfig` 从 KubeVirt VM、DataVolume、Service、Secret 读取 `RuntimeConfig`；`DiscoverKubeVirtServiceStatus` 从 VM `status.printableStatus` / Ready condition 推导 running/installing/failed；服务读模型对 `provisionMode=kubevirt` 不再查询 Helm `ServiceInstance` CR。
  - 2026-06-28 验证：`go test -count=1 ./internal/k8s ./internal/service -run 'Test(Discover|Delete|Upsert|Build)KubeVirtService|TestListServiceInstancesEnrichesKubeVirtRuntimeState|TestDiscoverServiceCredentialsReadsKubeVirtCredentialSecret'` 通过。
- [ ] Operator/GitOps 接入 KubeVirt 分支，将上述资源纳入 controller 调谐和真实集群联调
- [ ] 需要集群已装 KubeVirt operator/CRD
- [ ] 当前状态：模型、统计、安装 API、服务模板校验、资源生成、卸载清理和基础运行态读取已具备，controller/GitOps 调谐和真实集群联调仍未完成
- [ ] 工作量：3-4 周

### Task 7.10: KEDA 水平扩展
- [ ] 组件配置加弹性伸缩段：最小/最大副本、触发器（CPU/Q/自定义）
- [ ] 后端生成 `ScaledObject`（KEDA CRD）而非固定副本数 Deployment
- [ ] 当前状态：`Component.Replicas` 固定值（`component.go:25`）
- [ ] 需要集群已装 KEDA
- [ ] 工作量：2-3 周

### Task 7.11: 双集群 ArgoCD + 跨集群网络（架构级）
- [ ] 引入 `Cluster` 模型（注册集群、kubeconfig、label）
- [ ] `Environment` 加 `ClusterID` 字段
- [ ] ArgoCD 主从：一个主 ArgoCD 管多集群
- [ ] 跨集群网络：Submariner（推荐）或 VXLAN overlay
- [ ] 当前状态：无 `Cluster` 模型，纯单集群
- [ ] 工作量：1-2 月+

### Task 7.12: VXLAN 纳管虚拟机（架构级，依赖 Task 7.11）
- [ ] 在 Cluster 模型和网络层之上纳管已有虚拟机
- [ ] VXLAN 接入 + 资源注册
- [ ] 当前状态：零基础
- [ ] 工作量：XL

### Task 7.13: 配置模板覆盖扩展
- [ ] nginx 多 backend 路由模板
- [ ] Spring Boot datasource/cache/mq 配置模板
- [ ] Gin/Go 应用配置模板
- [ ] Node/Vite 前端 API 配置模板
- [ ] 纯 config-file 型应用配置模板
- [ ] 工作量：2-3 周

### Task 7.14: 自动关系检测增强
- [ ] 深度解析 ConfigMap/Secret/文件挂载配置
- [ ] 后端-数据库/缓存/消息队列关系线自动出现
- [ ] 当前状态：仅 env vars 可连线
- [ ] 工作量：2-3 周

### Task 7.15: 配置模板导入 UI 重设计
- [x] 导入对话框改为 Carbon Design System 白色风格
- [x] "适用组件"字段改为 select/combobox 控件
- [x] 区分普通模板（表单）和高级模板（JSON schema 上传）两种导入模式
- [x] Docker 镜像 `v0.1.445` 构建并部署到 kind 集群
- [x] CDP 验证：配置模板导入弹窗为白色 Carbon shell；普通/高级模式卡片切换正常；适用组件 select 显示 6 个候选项并带帮助文案
- [x] 对应文件：`frontend/src/views/TemplatesView.vue`
- [x] 工作量：1-2 周

### Task 7.15a: 配置模板预览摘要 ✅
- [x] 配置模板预览弹窗顶部增加 4 项影响摘要：适用组件、可填写项、敏感配置、生成文件
- [x] 摘要区使用 Carbon white 风格：白色 layer、细边框、0 圆角、无阴影，和模板预览弹窗一致
- [x] Docker 镜像 `v0.1.446` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.446`，Deployment `1/1 ready`，Pod `paap-server-58f6f686db-kpgm4` Running
- [x] CDP 验证：复用现有 Chrome `/templates` 标签；配置模板 API 返回 200 和 7 个模板；Nginx 预览显示 `适用 frontend / 2 个可填写项 / 无敏感配置 / 1 个生成文件`；Spring Boot 预览显示 `适用 backend / 6 个可填写项 / 4 项敏感配置 / 1 个生成文件`；高级 JSON tab 显示 `schema.json` 和 `template.json`
- [x] 对应文件：`frontend/src/views/TemplatesView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（半天）

### Task 7.15b: 配置模板抽取字段预览 ✅
- [x] 配置模板预览弹窗增加“抽取字段”表，展示字段键、显示名、类型、默认值和来源
- [x] 列表字段展开为 `父字段.子字段`，便于查看 FOR/ITEM 模板变量，例如 `LOCATION_LIST.PATH`、`LOCATION_LIST.PROXY_PASS`
- [x] 表格沿用 Carbon white 风格：白色弹窗、细边框表格、0 圆角、无阴影，保持和摘要区/原生预览一致
- [x] Docker 镜像 `v0.1.447` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.447`，Deployment `1/1 ready`，Pod `paap-server-7f75df788c-sb6xs` Running
- [x] CDP 验证：复用现有 Chrome `/templates` 标签；Spring Boot 预览显示 6 个抽取字段并包含 `JDBC_URL`、`REDIS_HOST`；Nginx 预览显示 4 个抽取字段并展开 `LOCATION_LIST.PATH`、`LOCATION_LIST.PROXY_PASS`；高级 JSON tab 仍显示 `schema.json` 和 `template.json`
- [x] 对应文件：`frontend/src/views/TemplatesView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（半天）

### Task 7.15c: 配置模板生成文件与校验提示预览 ✅
- [x] 配置模板预览弹窗增加“生成文件明细”，展示文件名、来源、推荐挂载路径和访问方式
- [x] 配置模板预览弹窗增加“校验提示”，展示字段缺失、重复字段、缺少推荐挂载路径等非阻塞问题；无问题时明确显示“未发现预览层面的配置问题”
- [x] 文件明细优先使用模板 `files` 中的推荐挂载路径，避免和原生配置片段重复展示
- [x] Docker 镜像 `v0.1.448` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.448`，Deployment `1/1 ready`，Pod `paap-server-589b874cdd-f844r` Running
- [x] CDP 验证：复用现有 Chrome `/templates` 标签；Spring Boot 预览显示“生成文件明细”、`application-paap.yml`、推荐挂载路径 `/etc/paap/application-paap.yml` 和“校验提示/未发现预览层面的配置问题”
- [x] 对应文件：`frontend/src/views/TemplatesView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（半天）

### Task 7.16: 模板体系收口
- [ ] 废弃旧 `installer/rawYaml/chartRepo/chartName` 创建入口
- [ ] 将 `WorkloadRolePolicy` / `EnvironmentRolePolicy` 等旧权限字段收敛到 `platform-manifest.yaml`
- [ ] 具体文件：`internal/handler/template.go`、`internal/model/service_catalog.go`
- [ ] 工作量：1-2 周

### Task 7.17: K8s 术语隐藏 ✅
- [x] 审查所有 drawer 和 workspace 中的 namespace/service/pod/configmap/secret/helm 等术语
- [x] 替换或隐藏 K8s 概念，仅在 debug/高级模式下展示
- [x] 工作量：0.5-1 周
- [x] 改动文件：`EnvDetailView.vue`、`ComponentDetailView.vue`、`ArgocdWorkspace.vue`、`RegistryWorkspace.vue`、`LogWorkspace.vue`、`componentProfile.ts`
- [x] 改动内容：Secret→敏感配置，ConfigMap→普通配置/应用配置，Pod→运行实例，ReplicaSet→副本集，Namespace→部署环境，Kind→类型，Workload→工作负载，Service→服务名称，Image→镜像，Replicas→副本数，dropdown labels→敏感项/应用配置，placeholder→凭据名称/配置名称，kpack→构建服务
- [x] 附带清理：移除组件 drawer 无用的"设置" tab 和服务 drawer 无用的"运行态" tab，Command/Args 移至部署 tab 高级区域，删除 `componentDrawerDataRows` 死代码及 4 个未使用的 runtime summary helper 函数
- [x] CDP 验证：v0.1.437 部署后组件 drawer tabs 正确（部署/配置/指标/日志/控制台），Redis 服务 drawer tabs 正确（部署/数据/接入/指标/日志/控制台）

### Task 7.18: 产品化验证与审计队列
- [ ] 产品化 Drawer 审计：每个工具/中间件的 drawer CDP 端到端验证
- [ ] 无伪造/占位数据审计：每个 workspace 资源必须追溯到真实 backend
- [ ] Per-card 指标视觉审计：检查真实数据、空状态、时间范围
- [ ] Per-card 日志审计：检查真实日志行，无 "no such host" 式失败
- [ ] Console 审计：所有工具 pod 测试，特别是无 shell 镜像和 ephemeral container 受限的 pod
- [ ] 数据库备份 restore/download/list 完善
- [ ] PV 配置 chart-by-chart 验证
- [ ] 拓扑模式端到端验证（Redis/PostgreSQL/MySQL 各模式）
- [ ] Runtime 配置更新验证
- [ ] Registry 镜像源端到端验证
- [ ] Source 交付端到端验证
- [ ] CDP 测试覆盖：每次 UI 变更后用可见 Chrome 测试

### Task 7.19: Keycloak 部署 + 用户认证集成
- [x] 新增 Keycloak 到 `deploy/k8s/` 部署文件（`keycloak.yaml` + `deploy.sh` + `deploy-remote.sh` + 离线镜像清单）
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-keycloak`，Deployment `1/1 Available`，NodePort `30080`，健康口 `/health/ready` 返回 `UP`
- [x] 用户认证对接 Keycloak：OAuth2/OIDC login + callback 已接入，回调后签发 PAAP JWT 并回到前端登录页完成本地会话
- [x] 当前简单 JWT 与 Keycloak 并存：本地 `admin / Def@u1tpwd` 登录仍可用，Keycloak 登录成功后也进入同一套 PAAP JWT 和权限中间件
- [x] 用户管理同步：Keycloak userinfo upsert PAAP User，realm roles / client roles / groups 映射为 PAAP system role bindings
- [x] Keycloak 地址动态化：`KEYCLOAK_ISSUER_URL` / `KEYCLOAK_REDIRECT_URL` 支持 `{scheme}`、`{host}`、`{hostname}` 模板；`deploy/k8s/configure-auth-endpoints.sh` 支持显式 `PAAP_PUBLIC_URL` / `KEYCLOAK_PUBLIC_URL`，未配置域名时自动发现当前集群 NodeIP + NodePort 作为开发兜底
- [x] 部署修复：Keycloak 默认 realm 用户增加 `emailVerified`、`firstName`、`lastName`，避免 Keycloak 25 首次登录触发 `VERIFY_PROFILE`
- [x] 2026-06-27 验证：`kind-rbac-manager-test` 中 PAAP `http://172.20.0.2:30091`、Keycloak `http://172.20.0.2:30080`，回调地址为 `http://172.20.0.2:30091/api/v1/auth/keycloak/callback`；Keycloak realm 默认管理员已统一为 `admin/Def@u1tpwd`，`admin/admin` 返回 401，`admin/Def@u1tpwd` token 端点返回 200
- [x] 2026-06-28 验证：`deploy/k8s/configure-auth-endpoints.test.sh` 通过；`paap-runtime-endpoints` 当前值为 PAAP `http://172.20.0.2:30091`、Keycloak `http://172.20.0.2:30080`、issuer `http://172.20.0.2:30080/realms/paap`、redirect `http://172.20.0.2:30091/api/v1/auth/keycloak/callback`；浏览器点击“Keycloak 登录”进入 `172.20.0.2:30080`，使用 `admin/Def@u1tpwd` 登录后回到 PAAP，服务日志显示 `/api/v1/auth/keycloak/login` 和 `/api/v1/auth/keycloak/callback` 均 302 成功，回调后 `/api/v1/auth/me`、`/api/v1/auth/permissions` 返回 200
- [x] 对应文件：`deploy/k8s/`、`deploy/k8s/configure-auth-endpoints.sh`、`internal/handler/auth_keycloak.go`、`internal/authz/`、`internal/model/authz.go`
- [x] 工作量：1-2 周

### Task 7.20: 画布卡片分组分区
- [x] 画布上增加 zone 背景容器，每个 zone 聚合展示该组资源数量
- [ ] 目前每个服务/组件是独立卡片，打开后大卡片包含这些小卡片
- [x] 支持分组类型：本环境、平台公共、集群外部（对应 Task 7.5 的 zone 概念）
- [x] 组卡片可折叠/展开
- [x] 当前状态：画布已按来源显示背景分区和分区资源数，分区标题按钮可折叠/展开对应资源卡片；分区框可拖动，边框可调整大小，卡片可在所属分区内单独拖动且不能越界
- [x] 2026-06-26 验证：`paap-server:v0.1.521` 部署到 `kind-rbac-governance-test`；CDP 打开 `/apps/5/environments/5`，平台公共分区折叠后收成 180x44 标题条且共享卡片隐藏，展开后分区向右拖动 260px 无右侧阻挡，右边框拉宽 120px 后卡片不随 resize 移动，共享卡片单独拖动 72px 时分区不动，所有阶段卡片均保持在分区内；相关 API 均返回 200 且无控制台错误
- [x] 对应文件：`frontend/src/views/EnvDetailView.vue`、`frontend/src/views/componentTopology.ts`
- [ ] 工作量：1-2 周

### Task 7.22: 画布卡片端点地址展示
> 在 canvas 卡片副标题下方显示组件/服务的 externalUrl，方便快速识别访问地址

- [x] `ComponentTopologyComponent` 类型加 `externalUrl` 字段
- [x] 画布卡片在 subtitle 下方显示 externalUrl（有则显示，无则隐藏）
- [x] 超长 URL 自动截断（`shortenUrl` 函数，保留 host + 短 path）
- [x] URL 文字颜色使用 Carbon 品牌蓝 `var(--cds-interactive-01)`
- [x] CDP 验证通过：prod-gitea/argocd/loki/registry 等卡片均显示端点地址
- [x] 不会 externalUrl 的卡片（Redis、PostgreSQL、API 服务）不额外占位
- [x] 对应文件：`frontend/src/views/componentTopology.ts`、`frontend/src/views/EnvDetailView.vue`
- [x] 工作量：S（半天）

### Task 7.23: 中间件目录搜索/过滤
> 目录页面增加搜索输入框，按名称/类型/描述实时过滤中间件卡片

- [x] 添加搜索输入框（Carbon 风格，带搜索图标和清除按钮）
- [x] 输入时实时过滤卡片（按 name/type/description 模糊匹配）
- [x] 清除搜索恢复全部显示
- [x] CDP 验证通过：搜索 "postgres" 仅显示 PostgreSQL 卡片，清除后恢复 14 张
- [x] 对应文件：`frontend/src/views/CatalogView.vue`
- [x] 工作量：S（半天）

### Task 7.24: 目录页快捷键 "/" 聚焦搜索
> 按 "/" 键自动聚焦目录页搜索框，提升操作效率

- [x] 注册全局 keydown 监听 "/" 键
- [x] 仅在非输入框/文本域/选择框区域生效
- [x] CDP 验证：按 "/" 后搜索框获得焦点（`document.activeElement` 匹配）
- [x] 对应文件：`frontend/src/views/CatalogView.vue`
- [x] 工作量：S（半小时）

### Task 7.26: 目录页骨架屏加载状态
> 数据加载时显示 Carbon 风格骨架卡片（灰色块 + shimmer 动画），替代旧的"加载中..."文字

- [x] 骨架屏模板：6 个骨架卡片（图标行 + 描述行 + 标签行）
- [x] Carbon 风格 CSS：`background-size` shimmer 动画，`#f4f4f4` / `#e8e8e8` 灰底
- [x] `nextTick()` 确保骨架渲染在 API 调用前生效
- [x] CDP 验证：拦截 API 延迟 2.5s，800ms 时骨架可见、搜索栏隐藏，数据加载后 14 卡正常
- [x] 对应文件：`frontend/src/views/CatalogView.vue`
- [x] 工作量：S（半天）
- [x] 提交：待 Cycle 6 提交

### Task 7.27: 目录页卡片 hover 效果
> 中间件目录卡片添加 Carbon 风格的鼠标悬停效果（阴影上浮 + 微抬起）

- [x] 添加 `transition` 过度动画（box-shadow + transform，0.2s ease）
- [x] hover 时 `box-shadow: 0 2px 6px rgba(0,0,0,0.1)` + `translateY(-2px)`
- [x] 光标改为 `pointer` 提示可点击
- [x] CDP 验证：hover 后 shadow 从 `none` 变为 `rgba(0,0,0,0.1) 0px 2px 6px`，transform 为 `translateY(-2px)`
- [x] 对应文件：`frontend/src/views/CatalogView.vue`
- [x] 工作量：S（15 分钟）

### Task 7.25: 目录页总数展示
> 目录页副标题显示 "平台支持的中间件与工具一览（共 N 个）"

- [x] 添加 `totalItems` computed，随搜索过滤实时变化
- [x] CDP 验证：副标题显示 "（共 14 个）"
- [x] 对应文件：`frontend/src/views/CatalogView.vue`
- [x] 工作量：S（15 分钟）

### Task 7.28: 隐藏未落地服务目录占位项 ✅
> kingbase 尚未完成 chart、安装参数、连接发现、工作台和测试前，不对用户展示为可选能力；Nacos/Eureka 已在 Task 6.3 落地为可选中间件。

- [x] `ListServiceCatalog` 排除 `kingbase` 占位项，即使数据库里遗留为 enabled 也不会返回
- [x] `SeedServiceCatalog` 将 `kingbase` 默认写为 disabled，并显式修正旧数据；Nacos/Eureka 默认 enabled
- [x] 增加后端回归测试，覆盖 enabled 遗留占位项不会出现在 catalog 响应中，并覆盖 Nacos/Eureka 可见
- [x] Docker 镜像 `v0.1.449` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.449`，Deployment `1/1 ready`，Pod `paap-server-797495ddd9-bscfj` Running
- [x] CDP/API 验证：`/api/v1/service-templates` 返回 14 个服务模板，`kingbase` / `nacos` 均不存在
- [x] 数据库验证：`service_catalogs` 中 `kingbase|f`、`nacos|f`
- [x] 2026-06-28 更新验证：Nacos/Eureka service catalog 和内置 chart template 已启用；`./scripts/package-built-in-templates.sh` 生成 `data/charts/nacos.tar.gz`、`data/charts/eureka.tar.gz`；`go test -count=1 ./internal/service ./internal/handler` 通过
- [x] 对应文件：`internal/handler/template.go`、`internal/handler/template_test.go`
- [x] 工作量：S（半天）

### Task 7.29: 目录页无结果搜索空状态 ✅
> 搜索没有匹配项时保持搜索框可用，并提供清除入口，避免用户被困在空结果页。

- [x] 搜索栏展示条件从过滤后的 tab 数量改为原始目录数据存在，搜索无结果时搜索框不消失
- [x] 增加 Carbon white 风格空状态，展示当前搜索词和“清除搜索”按钮
- [x] 清除搜索后恢复全部目录卡片，并将焦点留在搜索框，便于继续输入
- [x] 前端测试：`npm run test -- src/views/viewMarkup.test.ts`，75 passed
- [x] 前端全量测试：`npm run test`，24 files / 208 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.450` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.450`，Deployment `1/1 ready`，Pod `paap-server-679f4647bc-k42lf` Running
- [x] CDP 验证：使用正确 kind 地址 `http://172.18.0.2:30091/catalog`；搜索 `zzzz-no-result` 后卡片数为 0、搜索框仍可见、显示“没有匹配的中间件或工具”和清除按钮；点击清除后恢复 14 张卡片且搜索框保持焦点
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（半天）

### Task 7.30: 目录页版本号按语义版本倒序 ✅
> 同一目录项存在多个版本时，按语义版本 newest-first 展示，避免 `v1.2.10` 排在 `v1.2.2` 后面。

- [x] 新增 `semanticVersionParts` / `compareCatalogVersions`，先去掉 `v` 前缀，再按数字段比较
- [x] 版本列表从默认字符串 `.sort()` 改为 `.sort(compareCatalogVersions)`
- [x] 前端目标测试：`npm run test -- src/utils/catalogVersions.test.ts src/views/viewMarkup.test.ts`，2 files / 78 tests passed
- [x] 前端全量测试：`npm run test`，25 files / 211 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.452` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.452`，Deployment `1/1 ready`，Pod `paap-server-9f8b8cdb9-c4lsq` Running
- [x] CDP/API 验证：`http://172.18.0.2:30091/catalog` 页面加载 14 个目录项和版本标签；`/api/v1/service-templates` 返回 14 条，当前内置数据没有同类型多版本，排序行为由回归测试覆盖
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`、`frontend/src/utils/catalogVersions.ts`、`frontend/src/utils/catalogVersions.test.ts`
- [x] 工作量：S（15 分钟）

### Task 7.31: 目录页按产品类型细分分组 ✅
> 将原来的工具 / 中间件两组细化为工具、数据库、缓存、消息队列、对象存储，贴合 Task 6.14 的目录浏览要求。

- [x] 新增 `catalogGroupForTemplate` / `compareCatalogGroupMeta`，按服务类型映射目录产品分组
- [x] 数据库：PostgreSQL / MySQL / MongoDB；缓存：Redis；消息队列：RabbitMQ / Kafka；对象存储：MinIO；工具类保持原分组
- [x] 目录 tab 按固定产品顺序展示，并保留每组数量
- [x] 前端目标测试：`npm run test -- src/utils/catalogGroups.test.ts src/views/viewMarkup.test.ts`，2 files / 79 tests passed
- [x] 前端全量测试：`npm run test`，26 files / 214 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.453` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.453`，Deployment `1/1 ready`，Pod `paap-server-6bfff5b996-8lqsp` Running
- [x] CDP 验证：`http://172.18.0.2:30091/catalog` 逐 tab 切换通过，工具类 7、数据库 3、缓存 1、消息队列 2、对象存储 1，卡片类型分别匹配预期
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`、`frontend/src/utils/catalogGroups.ts`、`frontend/src/utils/catalogGroups.test.ts`
- [x] 工作量：S（半天）

### Task 7.32: 目录页支持按产品分组名搜索 ✅
> 搜索框支持输入“数据库”“缓存”“消息队列”“对象存储”等分组名，直接定位对应能力。

- [x] 新增 `catalogTemplateMatchesQuery`，搜索匹配范围覆盖名称、类型、描述、原始分类和产品分组名
- [x] 目录页过滤逻辑复用统一 helper，保留原有名称/类型/描述搜索能力
- [x] 前端目标测试：`npm run test -- src/utils/catalogGroups.test.ts src/views/viewMarkup.test.ts`，2 files / 81 tests passed
- [x] 前端全量测试：`npm run test`，26 files / 216 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.454` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.454`，Deployment `1/1 ready`，Pod `paap-server-5db8fbd86c-fzxwl` Running
- [x] CDP 验证：`http://172.18.0.2:30091/catalog` 搜索“缓存”显示 Redis；“消息队列”显示 RabbitMQ/Kafka；“对象存储”显示 MinIO；“数据库”显示 PostgreSQL/MySQL/MongoDB；清空后 5 个分组恢复
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`、`frontend/src/utils/catalogGroups.ts`、`frontend/src/utils/catalogGroups.test.ts`
- [x] 工作量：S（15 分钟）

### Task 7.33: 目录页搜索文案同步分组搜索能力 ✅
> 搜索框和空结果说明同步提示“名称、类型、分组或描述”，避免用户不知道可以按产品分组名搜索。

- [x] 搜索框 placeholder 从“搜索中间件或工具名称...”改为“搜索名称、类型、分组或描述...”
- [x] 无结果说明同步包含“名称、类型、分组或描述”
- [x] 前端目标测试：`npm run test -- src/views/viewMarkup.test.ts`，1 file / 79 tests passed
- [x] 前端全量测试：`npm run test`，26 files / 217 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.455` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.455`，Deployment `1/1 ready`，Pod `paap-server-75dd899886-b5w2h` Running
- [x] CDP 验证：`http://172.18.0.2:30091/catalog` 搜索框 placeholder 显示“搜索名称、类型、分组或描述...”；搜索 `zzzz-no-result` 后空结果说明显示“名称、类型、分组或描述”，卡片数为 0
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（15 分钟）

### Task 7.34: 目录页 Escape 清空搜索 ✅
> 搜索框获得焦点时按 Escape 直接清空当前查询，并保持焦点，匹配 "/" 快捷聚焦和清除按钮的操作习惯。

- [x] 搜索输入框绑定 `@keydown.esc="clearCatalogSearch"`，复用现有清除逻辑
- [x] 清空后通过 `nextTick` 保持搜索框焦点，便于继续输入
- [x] 前端目标测试：`npm run test -- src/views/viewMarkup.test.ts`，1 file / 80 tests passed
- [x] 前端全量测试：`npm run test`，26 files / 218 tests passed
- [x] 前端构建：`npm run build` 通过
- [x] Docker 镜像 `v0.1.456` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.456`，Deployment `1/1 ready`，Pod `paap-server-784f69bbf7-d958t` Running
- [x] CDP 验证：`http://172.18.0.2:30091/catalog` 输入 `zzzz-no-result` 后显示空状态；按 Escape 后输入框清空且保持焦点，恢复 14 张目录卡片和 5 个分组
- [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/viewMarkup.test.ts`
- [x] 工作量：S（15 分钟）

### Task 7.21: `docs/配置示例.md` → 内置配置模板 ✅
> 将配置示例转为 PAAP 内置配置模板（Go template），供组件配置 Tab 使用。内置模板源文件放在 `docs/examples/buildin-config-in-templates/`，通过打包脚本生成 `data/config-templates/*.tar.gz` 后由内置同步流程上传到 MinIO；产品代码只保留模板解析、同步和应用引擎。

- [x] 梳理模板目录结构：内置模板源文件在 `docs/examples/buildin-config-in-templates/`，发布包在 `data/config-templates/`
- [x] **Spring Boot 系列**: 基础 / +PG Hikari / +PG Druid / +PG 集群 Druid / +PG+Redis 单实例 / +PG+Redis 哨兵 / +PG+Redis 集群 / +PG+RabbitMQ / +PG+Nacos
- [x] **Nginx 系列**: 基础 nginx.conf / default.conf / +Upstream 负载均衡 / +SSL HTTPS / +静态资源分离缓存
- [x] **Go/Gin 系列**: YAML / TOML / INI 格式，并保留 MySQL+Redis JSON 常用模板
- [x] **Python 系列**: FastAPI + PG+Redis / Django + PG+Redis
- [x] **Node/TS 系列**: NestJS + PG+Redis (.env) / Vue/React Vite (.env.production)，并保留 n8n + PostgreSQL 环境变量模板
- [x] 每个模板只提取部署时常改的地址、端口、库名、账号、密码、域名、证书路径和运行模式等关键字段为模板变量
- [x] 前端组件配置 Tab 中的"配置模板"下拉菜单选择后填充配置编辑区，并记录 `configTemplateId/key/name`
- [x] 配置模板解析引擎下沉到 service 层：`handler` 只负责 HTTP 入参、上传文件和 S3 同步调度；模板包解析、原生 `__TEMPLATE__` 占位符解析、字段推断、文件挂载建议和内置包发现由 `internal/service/component_config_template_parser.go` 负责，产品代码不再写死具体用户应用模板
- [x] 当前状态：`docs/配置示例.md` 的 21 个纯文本示例已被内置模板覆盖；总计 26 个内置配置模板包
- [x] 对应文件：`docs/examples/buildin-config-in-templates/`、`data/config-templates/`、`internal/service/component_config_template_parser.go`、`internal/handler/component_config_template.go`、`frontend/src/views/EnvDetailView.vue`
- [x] 验证：`find docs/examples/buildin-config-in-templates -name 'template.json' -o -name 'schema.json' | sort | xargs jq empty` 通过；`./scripts/package-built-in-config-templates.sh` 生成 26 个包；`go test -count=1 ./internal/service ./internal/handler -run 'Test(ParsePackagedBuiltInComponentConfigTemplates|ParseComponentConfigTemplatePackageFile|ParseNativeComponentConfigTemplate|BuiltInComponentConfigTemplateArchivePathsIncludesNginxDefault|ComponentConfigTemplates)'` 通过；`go test -count=1 ./internal/service ./internal/handler` 通过
- [x] 2026-06-28 运行态验证：构建 `paap-server:v0.1.566-config-parser-service` 成功并验证镜像内 `/paap-server` 可执行；加载到 `kind-rbac-manager-test` 后滚动 `paap-server` 成功，Pod `1/1 Running`；镜像内存在 `/config-templates/nginx-default-conf.tar.gz`、`/charts/nacos.tar.gz`、`/charts/eureka.tar.gz`；浏览器访问 `/templates` 显示 33 个配置模板，`Nginx default.conf` 显示 `12 个可填写项`
- [x] 工作量：1 周

---

## 阶段八：领导新需求（2026-06-26 提出）

> 来源：领导对现有 UI 和产品方向的新意见，共 15 项需求。以下按优先级和依赖组织。
>
> 总体判断：这批需求不是简单改几个菜单名，而是要求把 PAAP 从“中间件安装器”提升成“平台服务目录 + 服务实例运营”的产品形态。后续页面和接口应围绕三层对象设计：
> - **服务产品**：平台支持什么服务，例如工具、数据库、中间件、环境服务、网络服务、存储服务、DNS、Ingress。服务目录展示的是服务产品，不再叫中间件目录。
> - **服务实例**：某个环境、共享资源池、外部接入或 KubeVirt 服务模板中真实存在的一份服务，例如 `shared-redis`、某个业务环境内 PostgreSQL、外部 S3、KubeVirt 模板交付的 Redis。
> - **服务使用关系**：哪些应用/环境/组件在使用这个实例，使用来源是环境内、平台公共、外部连接或 KubeVirt 模板交付。统计实例数、使用次数、监控入口、连接信息都从这层聚合。
>
> 推荐信息架构：
> - 左侧菜单保留全局入口：应用、共享资源池、服务目录、配置模板、用户管理。共享资源池从应用列表中弱化，作为平台级资源池直接进入 `default/shared` 画布。
> - 服务目录是平台服务总入口，面向“这个服务是什么、怎么部署、有哪些参数、有哪些实例、被谁使用、有没有监控”，不是 Helm chart 或模板文件列表，也不是直接修改实例配置的入口。
> - 配置模板只承载应用配置文件模板和组件配置模板；Helm chart / 服务包模板迁移到 Git 后作为服务产品的交付包，不在主导航继续叫“模板”。
> - 业务用户在实例详情里默认看到连接地址、端口、用户名、密码、连接串、使用示例、监控入口；namespace、StatefulSet、CRD 等 Kubernetes 元数据只放“高级/运维信息”，不作为主信息。

### Task 8.1: 左侧菜单栏增加共享资源池入口（S）
> 当前：`/shared-resources` 是独立路由但只做了一个 redirect 页，侧边栏无入口。

- [x] MainLayout.vue sidebar nav 增加"共享资源池"菜单项（平台管理员可见）
- [x] `/shared-resources` 路由保留，PlatformSharedResourcesView 先展示资源池概览和服务列表，用户点击后再进入 `default/shared` 环境画布
- [x] 共享环境当前应用名显示"共享资源池"而非普通应用名
- [x] 非平台管理员不显示此菜单项（同"用户"菜单逻辑）
- [x] 应用列表不再展示系统共享资源池卡片，主入口以左侧菜单为准
- [x] 对应文件：`frontend/src/layouts/MainLayout.vue`、`frontend/src/router/index.ts`、`frontend/src/views/PlatformSharedResourcesView.vue`
- [x] 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "shared resource"`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过
- [ ] 工作量：S（半天）

### Task 8.2: "模板"更名为"配置模板"（S）
> 领导要求改名，明确聚焦应用配置模板而非 Helm chart 模板。

- [x] MainLayout.vue nav label `模板` → `配置模板`
- [x] 路由 `/templates` 不变，视图名不依赖硬编码字符串
- [x] 页面标题 `TemplatesView.vue` 中 "模板" → "配置模板"
- [x] 页面说明明确：配置模板用于组件/应用配置生成，不承载服务 Helm chart 管理
- [ ] 关联：Task 7.21（配置模板覆盖扩展）和 7.15（配置模板导入 UI）是此改名的实质内容
- [x] 工作量：S（30 分钟）
- [x] 2026-06-28 验证：浏览器 CDP 登录后访问 `/templates`，页面显示“配置模板 / 导入配置模板 / 服务与环境模板统一在服务目录查看”，未出现“工具模板 / 中间件模板 / 新建环境模板 / 上传模板”

### Task 8.3: 服务目录扩展（含环境服务）（M）
> 当前目录只显示 ServiceCatalog 中的工具/中间件。领导要求目录也是"服务目录"，增加环境级别的服务实例（environment capability）。

- [x] 左侧菜单和页面标题统一为"服务目录"，不再出现"中间件目录"
- [x] 目录 tab 按当前产品口径调整为：CI 服务、CD 服务、监控服务、日志服务、数据库服务、中间件服务、环境服务、虚拟机服务、其他服务
- [x] CatalogView 增加"环境服务"分组，读取环境模板并作为服务目录项展示
- [ ] 每个服务产品卡片显示：服务类型、版本、功能标签、可用来源（环境内 / 平台公共 / 外部连接 / KubeVirt 模板交付）、说明入口、实例/监控入口
- [ ] 服务产品详情不是“在线配置实例”，而是类似 Helm 包 README 的说明界面：服务介绍、适用场景、部署手册、参数说明、默认 values、示例 values、连接方式说明、常见问题
- [ ] 服务产品详情中的“参数说明”只解释字段含义和示例，不直接保存到现有实例；真正配置发生在创建实例、引用共享资源、接入外部资源或实例工作区中
- [ ] 每个服务实例卡片显示：实例名、状态、所属应用/环境、来源、连接入口、监控入口；默认不展示 namespace、StatefulSet 等 K8s 细节
- [x] 后端：GET `/api/v1/catalog/services` 返回服务产品读模型和环境能力统计数据（每个类型被多少环境使用）
  - 2026-06-28 补齐：目录页不再由前端拼 `ServiceTemplate`、环境模板和共享资源列表；后端统一返回服务产品、版本、feature 矩阵、环境内实例、KubeVirt 实例、共享资源池预装实例、公共引用、外部连接、运行实例、使用应用数和使用环境数。
- [x] 服务卡片增加"使用统计"：被 X 个环境安装，Y 个环境引用
  - 2026-06-28 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/handler -run 'Test(BuildPlatformService|ListCatalogService|CatalogServices|PlatformService)'`、`npm --prefix frontend run test -- src/api/client.test.ts src/views/viewMarkup.test.ts src/utils/catalogGroups.test.ts --run` 通过。
- [x] 服务产品详情页增加"怎么用"区：环境内创建、引用共享资源、接入外部资源、使用 KubeVirt 服务模板四种路径，按服务支持能力显示
  - 2026-06-28 补齐：服务目录卡片可点击选中，同页右侧服务产品详情显示“怎么用”、支持能力、实例与使用统计和版本；四种使用路径按 `managed/shared/external/kubevirt` feature 矩阵显示“可用/暂不可用”，PostgreSQL 等数据库显示 KubeVirt 模板交付可用。
  - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts src/utils/catalogGroups.test.ts --run`、`cd frontend && npm exec vue-tsc -- -b --noEmit`、`npm --prefix frontend run build` 通过；浏览器打开 `http://localhost:5173/catalog`，登录 `admin/Def@u1tpwd`，确认服务目录右侧详情栏存在，点击 PostgreSQL 后 `KubeVirt 模板交付` 为“可用”。
- [ ] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/utils/catalogGroups.ts`、`internal/handler/`
- [ ] 工作量：M（3-4 天）
- [x] 2026-06-28 验证：浏览器 CDP 登录后访问 `/catalog`，页面显示“服务目录”以及 CI/CD/监控/日志/数据库/中间件服务分类；`npm --prefix frontend run test -- src/utils/catalogGroups.test.ts src/views/viewMarkup.test.ts --run` 通过

### Task 8.4: Redis 工作区数据精简（S）
> 领导觉得 Redis 界面信息过载。

- [ ] 审计 RedisWorkspace 当前展示内容，识别低价值数据（过多 key 列表、内部统计指标）
- [ ] 精简为：连接状态、key 搜索/CRUD、基本信息（内存使用、命中率）
- [ ] 隐藏/折叠细节数据：cluster nodes 列表、慢查询日志、config 参数
- [ ] 与 Mongo/DB workspace 保持一致的简化风格
- [ ] 对应文件：`frontend/src/components/workspaces/RedisWorkspace.vue`
- [ ] 工作量：S（半天）

### Task 8.5: 服务使用统计（M）
> 领导想要看到每个服务被安装了多少次、被多少个环境使用。

- [ ] 后端：`ServiceUsageStats` API — `GET /api/v1/catalog/stats`
  - 按 service type 统计：`total_installations`、`active_installations`、`unique_environments`、`unique_applications`
  - 按应用/服务维度统计：应用中有多少组件实例、服务产品有多少服务实例、每个共享/外部实例被引用多少次
  - 按来源统计：环境内实例数、平台公共引用数、外部连接数、KubeVirt 模板交付实例数
  - 可选：按时间维度（近 7 天、近 30 天）
- [ ] 后端增加服务使用关系表或视图，统一聚合 `service_installations`、`environment_capabilities`、组件依赖配置、外部资源引用
- [ ] 后端：服务实例增加 `last_used_at` 字段（每次 workspace 访问、凭据读取、组件引用时更新）
- [ ] 目录页服务卡片显示使用统计（小角标或 tooltip）
- [ ] 应用概览/组件页显示当前应用使用了多少服务实例，以及每个服务实例被哪些组件使用
- [ ] 平台管理页面增加"服务使用概览" tab（与 Task 8.12 协同）
- [ ] 对应文件：`internal/model/service_catalog.go`、`internal/handler/catalog.go`、`frontend/src/views/CatalogView.vue`
- [ ] 工作量：M（3 天）

### Task 8.6: 服务分类重构（M）
> 当前分类从旧的 tool / database / cache / mq / objectStorage / middleware 收敛到服务目录产品分类：CI 服务、CD 服务、监控服务、日志服务、数据库服务、中间件服务、环境服务、虚拟机服务。

- [ ] 重新设计分类体系：
  ```
  CI 服务: Jenkins, Tekton
  CD 服务: ArgoCD
  监控服务: Prometheus, Grafana, kube-prometheus
  日志服务: Loki
  数据库服务: PostgreSQL, MySQL, MongoDB
  中间件服务: Redis, RabbitMQ, Kafka, MinIO, Nacos, Eureka
  环境服务: 环境本身、环境能力、基础服务套餐（Task 8.3）
  虚拟机服务: KubeVirt 模板交付的 PostgreSQL/Redis 等服务
  网络服务: Firewall, Network Exposure, WAF, MetalLB（新增 Task 8.7）
  存储服务: Block, Object(MinIO), File（新增 Task 8.8）
  ```
- [x] 更新 `frontend/src/utils/catalogGroups.ts` 分类映射表
- [x] 更新 `seedServiceCatalog()` 中的 category 标记
- [x] 数据库中的 `service_catalog.category` 同步新分类
  - 2026-06-28 补齐：后端新增 `ProductServiceCategory`，seed catalog 和内置 service template 自动把旧 `tool/infra` 收敛为 `ci/cd/monitor/log/database/middleware/environment/virtualMachine/other` 产品分类；旧数据在启动 seed 时通过 `Assign(entry).FirstOrCreate` 同步更新。
  - 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/handler ./internal/service ./internal/model -run 'Test(BuiltInServiceTemplatesExposeFeatureMatrix|ServiceCatalogSeedNormalizesProductCategories|ListServiceCatalogHidesUnsupportedPlaceholders|BuildPlatformService|ListCatalogService|CatalogServices|PlatformService)'`、`npm --prefix frontend run test -- src/utils/catalogGroups.test.ts src/views/viewMarkup.test.ts --run` 通过。
- [ ] 分类支持嵌套（子分类让领导理解，UI 上一级平铺即可）
- [ ] 对应文件：`frontend/src/utils/catalogGroups.ts`、`internal/model/template.go`、`internal/database/seed.go`
- [ ] 工作量：M（2 天）
- [x] 2026-06-28 验证：`frontend/src/utils/catalogGroups.test.ts` 覆盖 Jenkins/ArgoCD/Monitor/Loki/PostgreSQL/MySQL/MongoDB/Redis/RabbitMQ/环境服务/KubeVirt 分类映射

### Task 8.7: 网络服务（L）
> 新增"网络服务"能力，包含：防火墙规则、网络暴露（Ingress/Gateway）、WAF、MetalLB（LoadBalancer IP）。

- [ ] ServiceCatalog 增加网络服务条目
- [ ] **MetalLB 集成**：
  - [ ] 检测集群是否安装 MetalLB
  - [ ] 创建/管理 IPAddressPool 和 L2Advertisement
  - [ ] 用户可申请 LoadBalancer IP（指定池/自动分配）
- [ ] **防火墙规则**：
  - [ ] UI 管理 NetworkPolicy 规则（允许/拒绝、来源/目的、端口）
  - [ ] 预置规则模板（允许 ingress from monitor、允许 egress to internet）
  - [ ] 展示规则影响范围，避免用户必须理解原始 NetworkPolicy YAML
- [ ] **WAF**：
  - [ ] 集成可选 WAF（如 ModSecurity/CoreWAF 作为 DaemonSet）
  - [ ] 管理规则集和策略
- [ ] **网络暴露（复用 Task 7.6 Ingress/Gateway）**：
  - [ ] 将 Ingress/Gateway 配置纳入"网络服务"范畴
  - [ ] 组件暴露操作统一归到网络服务配置
- [ ] 网络服务实例工作区展示：域名/IP、暴露端口、TLS、WAF 策略、健康状态、访问日志入口
- [ ] 每个服务需要独立的配置 workspace/drawer
- [ ] 对应文件：`internal/k8s/`（新建 metallb.go、networkpolicy.go、waf.go）、`frontend/src/components/workspaces/`（新建 NetworkWorkspace.vue）、`internal/model/service_catalog.go`
- [ ] 工作量：L（2-3 周），MetalLB 先做（1 周）

### Task 8.8: 存储服务（L）
> 新增"存储服务"能力，包含：块存储（PVC）、对象存储（MinIO）、文件存储（NFS/SMB）。用户可"申请"（request）使用。

- [ ] ServiceCatalog 增加存储服务条目
- [ ] **块存储**：
  - [ ] UI 创建/管理 PVC，选择 StorageClass（高速/普通 == Task 8.11）
  - [ ] 支持扩容（如果 StorageClass allowVolumeExpansion）
  - [ ] 展示 PV/PVC 状态、容量、访问模式
- [ ] **对象存储**（复用 MinIO，增加外部 S3 兼容存储支持）：
  - [ ] MinIO 作为 managed 对象存储
  - [ ] 外部 S3（MinIO/Ceph/AWS S3）作为 external 对象存储
  - [ ] Bucket 管理 UI（当前 MinIO workspace 已有，需统一）
- [ ] **文件存储**：
  - [ ] NFS 服务提供（如 nfs-server-provisioner）
  - [ ] 或对接已有 NAS 作为 external 文件存储
  - [ ] PVC 挂载模式 `ReadWriteMany`
- [ ] "申请"流程：用户选择类型 → 填写规格 → 创建 → 状态跟踪 → 连接信息展示
- [ ] 存储服务实例工作区展示：容量、存储类型（高速/普通）、访问模式、挂载方式、凭据/Endpoint、使用组件
- [ ] 对应文件：`internal/k8s/`（新建 storage.go）、`frontend/src/components/workspaces/`（新建 StorageWorkspace.vue）、`internal/model/`
- [ ] 工作量：L（2-3 周），块存储先做（1 周）

### Task 8.9: 服务产品说明与实例工作区统一（M）
> 领导要求每个服务都需要配置界面，但目录里的界面不是直接改配置，而是服务产品说明、部署手册和参数文档；真实配置仍在实例创建或实例工作区中完成。

- [ ] 服务目录产品详情统一包含：服务介绍、架构/适用场景、部署手册、参数说明、默认 values、示例 values、版本说明、使用限制、常见问题
- [ ] 参数说明支持从服务包元数据生成，优先读取 `platform-manifest.yaml`、`values.schema.json`、`README.md` 或 Git 模板仓库中的文档
- [ ] 产品详情页提供“创建实例 / 使用共享 / 接入外部 / 查看实例”动作，但不直接修改已有实例配置
- [ ] 审计所有 14 个实例 workspace 的运行管理完整度：
  - [ ] ArgocdWorkspace（490 行 CSS）— 达标
  - [ ] GiteaWorkspace（302 行）— 达标
  - [ ] DatabaseWorkspace（222 行）— 达标
  - [ ] RedisWorkspace（184 行）— 需精简(Task 8.4)
  - [ ] LogWorkspace（125 行）— 检查
  - [ ] MonitorWorkspace（85 行）— 检查
  - [ ] PipelineWorkspace（67 行）— 检查
  - [ ] KafkaWorkspace（38 行）— ⚠️ 简陋，补充
  - [ ] MinioWorkspace（22 行）— ⚠️ 简陋，补充
  - [ ] MongoWorkspace（25 行）— ⚠️ 简陋，补充
  - [ ] RabbitWorkspace（21 行）— ⚠️ 简陋，补充
- [ ] 统一产品详情页和实例 workspace 的样式基线（复用 ToolWorkspaceFrame 的 CSS token 和 Carbon white 主题）
- [ ] 每个实例 workspace 至少包含：状态展示 + 连接信息 + 配置查看/编辑 + 核心操作 + 监控入口 + 使用关系
- [ ] 实例 workspace 主视图面向业务用户，优先展示地址、端口、用户名、密码/Token、连接串、使用示例；K8s 原始对象信息放"高级/运维信息"折叠区
- [ ] 对应文件：所有 `frontend/src/components/workspaces/*.vue`
- [ ] 工作量：M（1 周）

### Task 8.10: DNS/Ingress 作为服务（M）
> 领导要求 DNS 和 Ingress 作为独立服务条目。

- [ ] **DNS 服务**：
  - [ ] DNS 记录管理（A/AAAA/CNAME/TXT 记录）
  - [ ] 集成 external-dns 或 CoreDNS 管理
  - [ ] 自动为组件分配子域名
- [ ] **Ingress 服务**（复用 Task 7.6）：
  - [ ] 作为独立服务类型展示
  - [ ] 配置：域名、路径、TLS 证书
  - [ ] 展示当前 Ingress 列表和状态
- [ ] 对应文件：`frontend/src/components/workspaces/DNSWorkspace.vue`（新建）、`frontend/src/components/workspaces/IngressWorkspace.vue`（新建）、`internal/k8s/`
- [ ] 工作量：M（1 周）

### Task 8.11: 存储分层（S）
> 高速存储（SSD）和普通存储（HDD）区分。与 Task 8.8 存储服务协同。

- [ ] 后端定义 StorageClass 标签：`高速` / `普通`
- [ ] 块存储申请时让用户选择存储类型（下拉选择 StorageClass）
- [ ] 目录页存储卡片展示支持的分层
- [ ] 对应文件：结合 Task 8.8 实现
- [ ] 工作量：S（半天，与 Task 8.8 合并）

### Task 8.12: 平台服务概览页面（M）
> 平台级页面，展示每个服务类型：被实例化次数、活跃实例列表、监控入口。

- [x] 新建 `PlatformServicesView.vue`（左侧“平台服务”入口）
- [x] 表格展示：服务类型、支持 feature、环境内实例数、KubeVirt 模板交付实例数、公共引用、外部连接、使用应用数、使用环境数、运行实例数
- [x] 点击某行展开该类型所有实例列表（所属环境、状态、使用数、监控目标）
- [x] 服务详情右侧栏展示：创建方式、连接方式、支持 feature（外部连接、KubeVirt 模板交付、公共服务）、最近使用、告警/指标
  - 2026-06-28 补齐：平台服务页选中服务后以右侧栏展示创建方式、连接方式、支持能力、最近使用、告警/指标摘要，并继续展示实例和使用方明细；主表服务列 sticky，避免横向滚动后丢失行身份。
  - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts --run`、`cd frontend && npm exec vue-tsc -- -b --noEmit`、`npm --prefix frontend run build` 通过；浏览器打开 `http://localhost:5173/platform/services`，登录态 `admin`，点击 Jenkins 后右侧栏显示创建方式、连接方式、支持能力、最近使用、告警/指标、实例和使用方，截图 `/tmp/paap-browser-checks/platform-services-detail-sticky.png`。
- [x] 监控入口链接到 Prometheus/Grafana（如果已安装）
  - 2026-06-28 补齐：平台服务实例/使用关系读模型新增 `monitoringUrl`；同环境存在 running `monitor`/`prometheus-grafana` 安装时，返回同源代理路径 `/api/v1/environments/{envID}/services/{monitorID}/proxy/...`，数据/中间件实例默认指向 `paap-middleware-workload`，工具类实例默认指向 `paap-tool-workload`；无监控服务时保留 `monitoringTarget` 并前端正常降级。
  - 2026-06-28 验证：`go test -count=1 ./internal/service ./internal/handler`、`npm --prefix frontend run test -- src/views/viewMarkup.test.ts --run`、`cd frontend && npm exec vue-tsc -- -b --noEmit`、`npm --prefix frontend run build` 通过；浏览器打开 `/platform/services`，登录 `admin/Def@u1tpwd`，点击 Jenkins 详情，右侧栏实例/指标区域正常渲染；当前 kind 数据无 running monitor 实例，因此页面按预期不展示 Grafana 链接。
- [x] 后端：`GET /api/v1/platform/services/stats` — 聚合 `ServiceInstallation` 与 `EnvironmentCapability`，返回 managed/shared/external/deferred 统计和 feature 矩阵
- [x] 后端：`GET /api/v1/platform/services/:type/instances` — 某类型所有实例详情
- [x] 后端：`GET /api/v1/platform/services/:type/usage` — 某类型被哪些应用/环境使用；组件级使用关系等待结构化关系表落地
- [x] 路由：`/platform/services`
- [x] 对应文件：`frontend/src/views/PlatformServicesView.vue`、`internal/handler/platform_service.go`、`frontend/src/router/index.ts`
- [x] 工作量：M（3 天）

### Task 8.13: 平台服务用户与Feature支持（L）
> 三项核心 Feature：外部连接、KubeVirt 模板交付、公共服务。

- [x] 服务目录显示 feature 矩阵，数据来自 `ServiceCatalog.Features` / `ServiceTemplate.SupportedFeatures`，不再由前端硬编码：
  - 外部连接：使用已有外部系统，只保存 endpoint 和凭据引用
  - KubeVirt 模板交付：使用平台维护的 KubeVirt 服务模板创建数据库、缓存等服务实例
  - 公共服务：使用共享资源池中的平台公共服务
- [x] 平台服务概览接入同一套 feature 矩阵（与 Task 8.12 的平台服务统计页面一起实现）
  - 2026-06-27 补齐：平台服务页点击服务类型后读取实例和使用方列表，feature 矩阵仍来自 `ServiceCatalog.Features` / `ServiceTemplate.SupportedFeatures`。
- [x] 用户选择服务时先选使用方式，再进入对应创建/接入流程，避免把三类能力混在一个表单里
  - 2026-06-28 补齐：环境服务弹窗新增“环境内创建 / 使用平台公共服务 / 接入外部连接 / KubeVirt 模板交付”使用方式选择；环境内创建继续走服务模板安装，公共服务进入共享资源引用，外部连接创建 external capability，KubeVirt 模板交付调用 `api.installService(..., { provisionMode: 'kubevirt' })`。
  - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "service usage mode"`、`npm --prefix frontend run test -- src/views/EnvDetailView.test.ts src/views/viewMarkup.test.ts -t "service|feature|install|capability"`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过。
- [x] 服务目录从“模板管理”中拆出服务类目，按 `CI服务 / CD服务 / 监控服务 / 日志服务 / 数据库服务 / 中间件服务 / 环境服务 / 虚拟机服务` 展示；配置模板页只保留组件运行配置模板。
  - 2026-06-28 验证：`npm --prefix frontend run test -- src/utils/catalogGroups.test.ts src/views/viewMarkup.test.ts src/views/componentConfigTemplateRuntime.test.ts src/views/configTemplateSyntax.test.ts --run`、`npm --prefix frontend run build` 通过。

- **Feature 1: 外部连接**（External Capability Source，复用 Task 7.5）
  - [x] 当前已实现 external source（环境画布右键添加外部资源）
  - [x] `EnvironmentCapability` 支持实例级 `capabilityKey`，同一环境可保存多个同类外部连接且凭据 Secret 不互相覆盖
  - [x] 补充：外部连接验证（endpoint 可达性测试、credential 验证）
    - 2026-06-28 补齐：`ValidateEnvironmentCapability` 在 service 层按类型做只读校验；PostgreSQL/MySQL/MongoDB/Redis/Kafka/MinIO 复用现有客户端，RabbitMQ/Git/Registry/Prometheus/Loki/Jenkins/ArgoCD 走 HTTP 健康或版本接口，未知类型回退 TCP 探测。
  - [x] 外部资源断开后清理 Canvas 状态和服务引用
    - 2026-06-28 补齐：删除 shared/external capability 时同步清理 `EnvironmentCanvasState` 的 positions/edges/names 以及组件配置 `bindings` 中指向该 capability 的引用；读取画布状态时也会清理 orphan names。
  - [x] 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/service ./internal/handler -run 'Test(DeleteEnvironmentCapabilityRemovesCapabilityCard|ValidateExternalHTTPCapabilityChecksCredentials|ValidateExternalEnvironmentCapabilityStoresValidStatus|EnvironmentCanvasStatePersists)'`、`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/database ./internal/authz ./internal/middleware ./internal/service ./internal/handler` 通过。
  - [x] 对应文件：`internal/service/environment_capability.go`、`internal/service/environment_state.go`、`frontend/src/views/EnvDetailView.vue`

- **Feature 2: KubeVirt 服务模板**（复用 Task 7.9）
  - [x] 当前 Task 7.9 从“创建裸 VM”调整为“通过 KubeVirt 模板交付服务实例”
  - [x] KubeVirt 是平台基础设施，用户创建的是 PostgreSQL、Redis、MySQL 等服务，不是单纯虚拟机
    - 2026-06-28 第一片补齐：`ServiceTemplate` / `ServiceInstallation` 增加 `provisionMode`、`runtimeSpec`，服务类型仍保持 PostgreSQL/Redis/MySQL 等真实产品类型；平台服务统计、实例列表和使用关系可以区分 `managed` 与 `kubevirt`。
    - 2026-06-28 验证：`/home/mensyli1/.gvm/gos/go1.25.7/bin/go test -count=1 ./internal/database ./internal/authz ./internal/middleware ./internal/service ./internal/handler`、`npm --prefix frontend run test -- src/views/EnvDetailView.test.ts src/views/viewMarkup.test.ts src/views/createEnvironmentSharedServices.test.ts`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过。
  - [x] 平台管理员维护服务模板：镜像/DataVolume、规格、启动脚本、端口、凭据、readiness、监控和备份策略
    - 2026-06-28 补齐：KubeVirt `runtimeSpec` 可表达并校验 image/DataVolume、规格、cloud-init、服务端口、凭据、readiness、monitoring 和 backupPolicy；生成的 VM/Service/Secret 带对应探针、连接输出、监控目标和策略注解。
    - 验证：`go test -count=1 ./internal/k8s ./internal/service -run 'Test(BuildKubeVirtService|CreateServiceTemplateValidatesKubeVirtRuntimeSpec|UpsertSeedServiceTemplateSeparatesProvisionModes)'` 通过。
  - [x] 创建服务实例时生成 `VirtualMachine`、Kubernetes `Service`、Secret、连接输出和监控目标的后端资源生成层
    - 2026-06-28 补齐：`internal/k8s/kubevirt.go` 负责无 KubeVirt Go 依赖的 unstructured VM/DataVolume 生成，`internal/service/kubevirt.go` 负责从应用、环境、安装记录和模板桥接 `runtimeSpec`；`InstallService` API 已接入并可 upsert 资源，controller/GitOps 调谐和真实集群联调仍待完成。
    - 验证：`go test -count=1 ./internal/k8s ./internal/service -run 'TestBuildKubeVirtService'` 通过。
  - [x] 对应文件：`internal/k8s/kubevirt.go`、`internal/service/kubevirt.go`，后续可扩展 `ServiceInstanceController`

- **Feature 3: 公共服务**（Shared Capability Source）
  - [x] 当前已实现 shared source（default/shared 环境）
  - [x] shared 引用支持实例级 `capabilityKey`，同一环境可引用多个同类共享服务
  - [x] 补充：环境创建时的"使用平台公共服务"一键配置引导
    - 2026-06-28 补齐：创建环境弹窗读取共享资源池实例，用户可在创建阶段勾选平台公共服务；提交时转换为 `capabilities` 的 shared 引用，后端创建环境事务内生成 `EnvironmentCapability`。
    - 验证：`npm --prefix frontend run test -- src/views/EnvDetailView.test.ts src/views/viewMarkup.test.ts src/views/createEnvironmentSharedServices.test.ts`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过。
  - [x] 提升 shared 来源的可见性：目录页标记"平台已预装"
    - 2026-06-28 补齐：服务目录并行读取共享资源池实例，按 `serviceType/provider` 匹配服务产品，匹配到共享实例时在目录卡片 feature 区显示“平台已预装”；共享资源接口失败时不影响目录主数据加载。
    - 验证：`npm --prefix frontend run test -- src/views/viewMarkup.test.ts -t "service feature matrix|service usage mode"`、`cd frontend && npm exec vue-tsc -- -b --noEmit` 通过。
  - [x] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/views/createEnvironmentSharedServices.ts`、`frontend/src/components/CreateEnvironmentModal.vue`

- 工作量：L（2-3 周，三个 Feature 可并行）

### Task 8.14: 模板存储迁移至Git（XL）
> 领导要求模板（配置模板 + Helm charts）从 MinIO/DB 迁移到 Git 仓库管理。

- **当前现状**：
  - Helm charts 存储在 `data/charts/*.tar.gz`，启动时 seed 到 MinIO
  - 配置模板存储在 `data/config-templates/`（待实现，Task 7.21）
  - 用户在 UI 上传的模板存数据库 `config_templates` 表
- **目标方案**：
  - Git 仓库作为模板的唯一来源（source of truth）
  - PAAP Server 增加"从 Git 同步模板"功能（cron 或 webhook 触发）
  - 内置模板 → Git 仓库 `templates/charts/` 和 `templates/config/`
  - 用户自定义模板 → Git 仓库用户分支或 fork
  - 保留 MinIO 作为运行时缓存（加速读取，Git 变更时自动更新缓存）
- **关键设计**：
  - 模板 Git 仓库结构规范
  - 同步策略（定时同步 / webhook 触发 / GitOps Reconciliation）
  - 版本管理（Git tag/semver 映射到 ServiceTemplate.AppVersion）
  - 回滚（Git revert 触发模板回滚）
  - 权限（谁可以 push 模板到 Git）
- **迁移路径**：
  1. 创建模板 Git 仓库 + 目录结构 + CI 验证
  2. 后端 Git 客户端（clone/pull/read 文件）
  3. 同步引擎（定期同步 + webhook 模式）
  4. 将现有 MinIO 数据迁移到 Git
  5. 用户上传 → Git push（而非写数据库）
- **对应文件**：全栈涉及
  - `internal/git/`（新建—Git 客户端和同步引擎）
  - `internal/service/template_sync.go`（新建—模板同步服务）
  - `internal/database/seed.go`（修改—改为从 Git 读取而非硬编码路径）
  - `data/charts/`（移入独立 Git 仓库）
- **工作量**：XL（1 个月+），需独立专项

---

## 执行顺序

### 原路线图（阶段七及以前）
```
Week 0  : ~~Task 7.1(版本号)~~ ✅ → Task 7.2(目录页)
Week 1-2: Task 7.3+7.4(平台管理+角色) → Task 7.8(认证鉴权)
Week 3-4: Task 7.5a~7.5c(Capability 模型地基)
Week 5-7: Task 7.5d~7.5g(画布分区+外部接入) → Task 7.6(Ingress)
Week 8+  : Task 7.13~7.15(配置模板) 并行 Task 7.9+7.10(KubeVirt+KEDA)
季度级   : Task 7.11(多集群) → Task 7.12(VM纳管)
穿插     : Task 7.17~7.18(验证与审计)
```

### 增量计划（阶段八—领导新需求）
```
Week 1  : 8.1(共享菜单) + 8.2(改名) + 8.4(Redis精简) + 8.6(分类重构)   ← 快速见效
Week 2-3: 8.3(目录扩展) + 8.5(服务统计) + 8.9(Workspace统一)           ← 前后端配合
Week 3-4: 8.7(网络服务-MetalLB先) + 8.8(存储服务-块存储先)             ← 新服务类型
Week 4-5: 8.10(DNS/Ingress) + 8.11(存储分层) + 8.12(平台服务概览)      ← 平台页面
Week 6-8: 8.13(外部连接/KubeVirt模板/公共服务)     并行 8.14(模板Git迁移)评估设计
季度级   : 8.14(模板Git迁移) → 需求稳定后启动专项
```

---

## 阶段九：当前收敛范围与四分支执行方案（2026-06-26）

> 领导最新口径：先做平台服务和平台服务用户相关能力。其它服务目录大改、模板 Git 化、网络服务、存储服务、DNS/Ingress 等先记录但不进入这四个 PR。
>
> 本阶段需求主要是平台管理员视角，但必须保留应用成员的局部视角：平台管理员看全局服务、实例、使用方和监控；应用成员只看自己有权限的应用/环境内使用了哪些服务、如何连接、是否可用。

### 9.1 架构兼容性判断

- [ ] 当前 PAAP 三段式架构（Vue 前端 → PAAP Server/GORM/PostgreSQL → CRD/Operator/K8s）可以承接这批需求，不需要推翻重写。
- [x] 这批需求不是单纯 UI 改造，需要新增“平台服务域模型/读模型”，否则统计逻辑会分散到各页面。
  - 2026-06-27 预重构：先给 `ServiceCatalog` / `ServiceTemplate` 增加 feature 矩阵字段，服务目录已消费；后端补齐平台服务统计、实例列表和使用方读模型。
- [x] 当前 `ServiceInstallation` 是环境级安装记录，且 `environment_id + service_type` 唯一；后续若一个环境允许多个 Redis/PostgreSQL，需要调整唯一约束或引入更通用的服务实例模型。
  - 2026-06-28 已调整为 `environment_id + service_type + provision_mode`，先支持同一环境内 Helm 托管实例与 KubeVirt 模板交付实例并存；多个同模式同类型实例仍需后续服务实例模型。
- [x] 当前 `EnvironmentCapability` 是环境级能力引用，且 `environment_id + capability` 唯一；后续若一个环境允许多个 database/cache 外部连接，也需要扩展为实例级能力。
  - 2026-06-27 已改为 `environment_id + capability_key` 唯一，`capability` 保留为能力类别；同一环境可同时存在多个 database/cache 外部连接或共享引用。
- [x] 组件和服务使用关系不能只依赖画布连线，必须有结构化关系表或稳定读模型。
  - 2026-06-29 补齐：平台服务使用读模型从 `Component.Config.Bindings` 解析 `service:<id>` / `capability:<id>`，`GET /api/v1/platform/services/:type/usage` 返回组件级 `componentId/componentName/componentType`，实例 `usageCount` 也计入组件绑定引用；不再只依赖画布连线判断组件和服务关系。
- [ ] 监控、日志、凭据发现当前大量依赖 Kubernetes namespace；外部服务和 KubeVirt 模板交付服务不能假设和 Helm 服务拥有同样的 namespace 结构，必须抽象为 source-aware monitoring target / connection output。
  - 当前状态：平台服务统计/实例/使用 API 已返回 source-aware `monitoringTarget` / `monitoringUrl`，KubeVirt 生成层已有连接输出契约；组件运行日志、运行控制台和部分工具工作区仍需继续拆出通用 source-aware runtime target。
- [x] 新增平台服务 API 时预留 `clusterId` / `clusterName` 字段，避免后续多集群返工。
  - 2026-06-28 平台服务统计、实例、使用关系响应已预留 `clusterId` / `clusterName`，当前单集群为空。

推荐抽象：

- **服务产品**：平台支持什么服务，例如 PostgreSQL、Redis、Gitea、Harbor、Jenkins、Prometheus、Loki。
- **服务实例**：某个环境、共享资源池、外部系统或 KubeVirt 服务模板中真实存在的一份服务。
- **服务使用关系**：哪个应用/环境/组件使用了哪个服务实例，来源是环境内、平台公共、外部连接或 KubeVirt 模板交付。
- **监控目标**：服务实例的监控入口，不能假设一定来自当前集群 namespace。

### 9.2 分支与 PR 拆分

> 每个功能一个 feature 分支，走 PR 合并。分支名必须符合 `feature/<kebab-case-name>`。

| 顺序 | 分支 | 目标 |
|---|---|---|
| 1 | `feature/platform-service-usage` | 平台服务概览：每个服务怎么用、实例化次数、活跃实例、使用方、监控入口、服务使用读模型。 |
| 2 | `feature/shared-service-consumption` | 公共/共享服务：业务环境引用共享资源池服务，只读连接信息、使用关系统计、断开引用语义。 |
| 3 | `feature/external-service-connections` | 外部连接：外部数据库、缓存、消息、对象存储、代码仓、镜像仓库等接入、凭据引用、真实校验。 |
| 4 | `feature/kubevirt-service-templates` | KubeVirt 服务模板：通过平台维护的 KubeVirt 模板交付数据库、缓存等服务实例，并纳入使用关系和监控统计。 |

推荐顺序：

1. 先做 `feature/platform-service-usage`，建立统计和读模型底座。
2. 再做 `feature/shared-service-consumption`，因为共享资源池和 `EnvironmentCapability.Source=shared` 已有基础。
3. 再做 `feature/external-service-connections`，重点补真实连接验证和凭据语义。
4. 最后做 `feature/kubevirt-service-templates`，因为它引入 KubeVirt 模板、`VirtualMachine` 生命周期和服务连接输出，技术风险最高。

开分支前必须先处理当前工作区状态：

- [ ] 运行 `git status --short --branch`。
- [ ] 明确哪些是上一轮需要提交的改动，哪些是用户或运行时产生的改动。
- [ ] 不要把 `.omo`、`.playwright-mcp`、`runtime/`、临时 issue 文件混进新功能 PR。
- [ ] 不要在脏 `main` 上直接切四个功能分支并带入无关改动。

### 9.3 PR 1：平台服务使用统计

分支：`feature/platform-service-usage`

目标：平台管理员能看到每个服务怎么用、实例化多少次、活跃多少、被哪些应用/环境/组件使用，以及监控入口。

后端任务：

- [x] 新增平台管理员 API：`GET /api/v1/platform/services/stats`。
- [x] 新增平台管理员 API：`GET /api/v1/platform/services/:type/instances`。
- [x] 新增平台管理员 API：`GET /api/v1/platform/services/:type/usage`。
- [x] 聚合 `ServiceInstallation` 为 managed 服务实例。
- [x] 聚合 `EnvironmentCapability` 为 shared/external/deferred 能力引用。
- [x] 返回字段包含 service type、service name、provider、source、status、application、environment、component、usage count、monitoring target；component 字段来自组件运行配置 bindings 的稳定读模型。
- [x] API 响应预留 `clusterId` 或 `clusterName`，当前单集群可为空或默认值。
- [x] 全局统计 API 不返回密码/token 等敏感值。
- [x] 平台 API 必须要求平台管理员权限。

前端任务：

- [x] 新增平台服务页面或平台管理 tab。
- [x] 表格展示：服务类型、环境内实例数、活跃实例数、使用应用数、使用环境数、支持 feature。
- [x] 点击服务类型后展示实例列表和使用方列表。
- [x] 空状态、失败状态来自真实 API，不写假数据；权限不足由路由守卫和后端中间件返回。

验收：

- [x] 平台管理员能看到跨应用/环境的服务统计。
- [x] 非平台管理员访问全局平台服务 API 返回 403。
- [x] 统计不依赖画布连线。
- [x] 后端测试覆盖 managed installation、capability reference 和组件 binding 关系聚合。
- [x] 前端测试覆盖统计表、展开实例入口和“应用 / 环境 / 组件”使用方展示；无权限状态由后端路由权限测试覆盖。
- [x] 2026-06-27 验证：`/api/v1/platform/services/stats` 返回平台服务统计；浏览器验证左侧 `平台服务` 页面加载统计表，`目录` 页面展示 14 个服务和 feature chips

不纳入：

- [ ] 不完整实现外部连接校验。
- [ ] 不实现 KubeVirt 生命周期。
- [ ] 不做服务目录全量重构。

### 9.4 PR 2：公共/共享服务消费

当前开发：本地 `main` 分支。

目标：业务环境可以使用共享资源池中的平台公共服务，引用后能查看只读连接信息，并进入平台服务使用统计。

后端任务：

- [ ] 共享资源池继续使用系统应用/系统环境中的真实 `ServiceInstallation`。
- [x] 业务环境通过 `EnvironmentCapability.Source=shared` + `RefServiceID` 引用共享服务。
- [ ] PR 1 的统计接口把 shared 引用单独统计，不与 managed installation 混淆。
- [ ] 共享服务在业务环境中的凭据/连接信息从被引用的服务实例解析，但业务环境 API 不允许修改共享服务本体。
- [x] 断开共享能力时只删除当前环境引用、相关 canvas 状态和组件绑定引用，不删除共享服务。
  - 2026-06-28 补齐：删除 capability 的 service 层事务会清理 `EnvironmentCanvasState` 和组件配置 `bindings`。

前端任务：

- [ ] 共享资源池是平台管理员入口。
- [ ] 共享环境只允许添加工具/中间件，不允许创建业务组件、不允许再添加共享/外部资源。
- [ ] 业务环境可以从共享资源列表中添加共享资源。
- [ ] 业务环境中的共享资源卡片允许本地重命名和断开引用。
- [ ] 业务环境中的共享资源右侧栏只读展示地址、端口、用户名、密码/token、连接串、状态和监控入口。

验收：

- [ ] 业务环境引用共享 Redis/PostgreSQL 后，组件配置解析到真实共享服务 endpoint，不出现 `service:<id>` 这类占位地址。
- [ ] 共享资源本体名称和业务环境本地显示名可以不同。
- [x] 全局平台服务统计能看到共享实例被哪些应用/环境/组件引用。
  - 2026-06-29 补齐：共享 capability 引用会映射到被引用的 managed service instance，组件配置中的 `capability:<id>` binding 会在平台服务使用方列表中显示组件名称，并计入实例 usageCount。
- [x] 删除引用不会删除共享服务实例。

不纳入：

- [ ] 不做跨集群共享可达性。
- [ ] 不做外部连接校验。

### 9.5 PR 3：外部服务连接

当前开发：本地 `main` 分支。

目标：外部服务成为平台服务的一种消费方式。PAAP 只保存连接记录和凭据引用，不拥有真实外部资源。

后端任务：

- [x] 外部资源类型覆盖：数据库、缓存、消息中间件、对象存储、代码仓、镜像仓库、CI/CD、日志、监控、自定义。
- [x] `EnvironmentCapability` 保存 endpoint、provider、serviceType、validationStatus、validationMessage、credentialSecretRef。
- [x] 凭据写入 Kubernetes Secret 或后续凭据后端，数据库只保存引用。
- [x] 增加真实连接校验第一版：
  - [x] PostgreSQL/MySQL/MongoDB：连接和凭据探测。
  - [x] Redis：ping/auth 探测。
  - [x] RabbitMQ/Kafka：RabbitMQ HTTP management / Kafka metadata 只读探测；RabbitMQ 非 HTTP endpoint 回退 TCP。
  - [x] MinIO/S3：bucket/list 权限探测。
  - [x] Git：Gitea/GitLab 版本或首页 HTTP credential 探测；repo/webhook 深权限后续单独补。
  - [x] Registry/Harbor：`/v2/` 或首页 HTTP credential 探测；login/pull 深权限后续单独补。
  - [x] Prometheus/Loki：ready/status HTTP 探测；PromQL/LogQL query 深权限后续单独补。
- [x] 删除 external capability 只删除 PAAP 记录、PAAP 生成的本地 Secret、canvas 状态和组件绑定引用，不删除真实外部系统。

前端任务：

- [ ] 右键添加外部资源的二级菜单只显示类型，不展示示例域名，避免用户误解为写死地址。
- [ ] 外部资源右侧栏保存后继续显示 endpoint、用户名、密码/token，密码用眼睛按钮显示/隐藏。
- [ ] 展示验证状态和重新验证入口。
- [ ] 外部资源卡片允许重命名和删除连接。
- [ ] 外部连接进入组件绑定和平台服务统计。

验收：

- [x] 外部资源删除不会触发真实外部系统删除。
- [ ] 保存后刷新页面，endpoint 和凭据显示逻辑正常。
- [x] 校验失败展示真实后端探测错误，不写占位文案。
- [x] 平台服务统计把 external connection 单独计数。

不纳入：

- [ ] 不做全命名空间自动扫描。
- [ ] 不做跨集群网络。

### 9.6 PR 4：KubeVirt 服务模板

工作线：本地 `main`，不再新建 feature 分支

目标：把 KubeVirt 作为平台基础设施，通过服务模板交付 PostgreSQL、Redis、MySQL 等服务实例；用户不创建裸虚拟机，平台统计的仍然是具体服务产品和服务实例。

推荐首版：

- [x] 服务产品支持 `provisionMode=kubevirt`，同一服务产品可区分 Helm 托管、共享引用、外部连接、KubeVirt 模板交付。
  - 2026-06-28 第一片补齐：模型、迁移、模板查询、安装记录、平台服务统计/实例/使用关系读模型已支持 `provisionMode=kubevirt`；尚未实现 KubeVirt VM 生命周期。
- [ ] 平台管理员维护 KubeVirt 服务模板，例如 `postgresql-vm-template`、`redis-vm-template`、`mysql-vm-template`。
- [ ] 模板记录基础镜像/DataVolume、CPU、内存、磁盘、cloud-init/启动脚本、服务端口、凭据生成方式、readiness、监控 agent/exporter 和备份/快照策略。
- [x] 创建服务实例时生成 KubeVirt `VirtualMachine`、Kubernetes `Service`、Secret、标准连接输出和监控目标的后端资源生成层。
  - 2026-06-28 补齐：资源生成层已落地并有单测；安装 API 已接入 `provisionMode=kubevirt` 并通过 fake client 验证资源创建，controller/GitOps 调谐仍是下一步。
- [x] 服务实例仍按真实服务类型统计，例如 `serviceType=redis`、`provisionMode=kubevirt`，不要把“虚拟机”作为业务服务类型。
- [x] 进入平台服务统计和使用关系统计。

建议：

- [ ] 第一版不提供“创建空白 VM”入口，避免变成 IaaS 虚拟机管理平台。
- [ ] 优先做 PostgreSQL/Redis 这类模板化服务，跑通连接输出、使用关系和监控。
- [ ] KubeVirt 逻辑第一版可以扩展现有 `ServiceInstanceController`，因为用户创建的仍然是服务实例；后续复杂度上来再拆独立 controller。

验收：

- [x] KubeVirt 模板交付的 Redis/PostgreSQL 出现在对应服务产品的实例列表中。
- [ ] 应用/环境/组件可以通过同一套服务使用关系引用 KubeVirt 模板交付的服务。
- [x] 连接输出与 Helm 托管、共享、外部连接使用同一契约：地址、端口、用户名、密码/Token、连接串、状态、监控入口的生成层。
  - 2026-06-28 补齐：KubeVirt 资源生成层返回 host、port、secret key、URI 和 `namespace:<ns>` 监控目标；运行态状态和 UI 引用仍待安装分支接入后联调。
- [ ] 服务统计主视角显示 PostgreSQL/Redis 等服务实例数量；虚拟机数量只作为高级/运维信息。

不纳入首版：

- [ ] 不做创建空白虚拟机。
- [ ] 不做跨集群 KubeVirt 网络。
- [ ] 不做完整备份/快照自动化，除非需求明确。

### 9.7 多集群预留方案

> 领导后续可能增加“管理其他集群 / 部署应用到其他集群”。这需要架构扩展，但不阻塞当前四个 PR。

当前单集群假设：

- [ ] 没有 `Cluster` 模型。
- [ ] `Environment` 只有 `Namespace`，没有 `ClusterID`。
- [ ] `ServiceInstallation` 只有 namespace，没有 cluster。
- [ ] 多数运行态、凭据、日志、监控逻辑使用全局 `k8s.GetClient()`。
- [ ] Operator 只调谐当前集群。
- [ ] 共享资源池 `default/shared` 目前是单集群语义。

现在就要避免的新坑：

- [x] 新增平台服务统计 API 响应预留 `clusterId` / `clusterName`。
- [x] 新增服务使用关系时不要写死全局单集群。
  - 平台服务统计、实例和使用关系响应已预留 `clusterId` / `clusterName`，当前单集群为空。
- [ ] 共享资源池语义按集群隔离：未来应是每个集群一个 shared 环境，跨集群只能通过 external 或显式跨集群网络。
- [ ] 新增 Kubernetes 访问逻辑尽量收敛到可替换 client/provider，避免继续扩散全局 client。

未来独立专项：

- [ ] 新增 `Cluster` 模型和平台管理员集群注册页面。
- [ ] `Environment` 增加 `ClusterID`。
- [ ] `ServiceInstallation` 增加 `ClusterID`。
- [ ] `ServiceUsageRelation` 增加 `ClusterID`。
- [ ] 引入 cluster-aware Kubernetes client factory。
- [ ] 每个目标集群部署 PAAP Operator/Agent，由中心控制面下发期望状态，目标集群回传状态、指标、日志和资源清单。

建议未来分支：

```text
feature/multi-cluster-control-plane
```

### 9.8 从 CLAUDE.md 迁出的有价值待办

> 以下不是当前四个 PR 的全部范围，但仍是后续产品化必须关注的任务。后续不要继续把这些只放在 `CLAUDE.md`。

- [ ] 所有具体工具/中间件抽屉继续做产品级审计：Gitea、Registry、Harbor、Argo CD、Jenkins、Prometheus/Grafana、Loki、PostgreSQL、MySQL、MongoDB、Redis、RabbitMQ、Kafka、MinIO。
- [ ] 不允许假数据或占位数据。workspace 资源、指标、日志、备份、key、queue、topic、bucket、部署行必须来自真实后端/API/集群。
- [ ] 每张卡片的 metrics/logs 需要验证真实数据、空状态、时间范围、图表缩放和错误信息。
- [ ] Console 需要继续用可见 Chrome/CDP 覆盖常见工具和中间件 Pod。
- [ ] 数据库备份的 restore/download/list 需要产品决策和验证。
- [ ] 拓扑模式和持久卷配置需要逐 chart 验证，不能只看 UI 配置项。
- [ ] Kubernetes 术语默认隐藏到高级/运维信息，不应出现在普通用户主视图。
- [ ] 配置模板导入/预览仍是重要待办，但不属于当前四个领导收敛 PR，除非重新排优先级。

### 9.9 验证规则

每个 PR 至少完成：

- [ ] 后端涉及的 handler/service/model 单测。
- [ ] 前端涉及页面/组件的 Vitest。
- [ ] 用户界面变更运行 `npm --prefix frontend run build`。
- [ ] 需要部署验证时构建新镜像、kind load、更新 tag，并等待 rollout。
- [ ] 重要 UI 路径用可见 Chrome/CDP 验证，不只跑 headless。
- [ ] PR 说明中明确哪些路径已验证，哪些是后续风险。
