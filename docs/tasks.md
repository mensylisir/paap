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
- [ ] 应用创建、列表、详情、更新、删除改为基于当前用户和 `AppMember` 判断权限
- [ ] 移除 `OwnerID=1`、`UserID=1` 等硬编码
- [ ] 补齐应用成员管理页面和 API，包括邀请、角色变更、移除成员

### Task 6.2: 环境模板管理与高级环境配置
- [ ] 挂载环境模板创建、更新、删除 API 路由
- [ ] 在前端补齐环境模板管理 UI，而不只是读取模板列表
- [ ] 创建环境时支持从模板写入 CPU、内存、存储配额到 `Environment.spec.resourceQuota`
- [ ] 创建环境时支持模板或表单配置附加 namespace，而不是固定只创建 `app` namespace
- [ ] 评估并实现 `ipPool` 调和逻辑；若暂不支持，需要从 UI 和文档中明确标记为未启用
- [ ] 支持创建环境后的 namespace 增删，并触发工具 RBAC 与 Helm values 动态同步

### Task 6.3: 服务目录占位项落地
- [ ] 为 `kingbase` 补齐服务模板、安装参数、连接发现、工作台和测试
- [ ] 为 `nacos` 补齐服务模板、安装参数、连接发现、工作台和测试
- [x] 未落地前从可安装服务列表隐藏占位项，避免用户选择后安装失败

### Task 6.4: 模板体系收口
- [ ] 将内置模板完全统一到 `Helm Chart + platform-manifest.yaml + preset-values.yaml` 路径
- [ ] 废弃或移除旧的 `installer/rawYaml/chartRepo/chartName` 创建入口
- [ ] 将 `WorkloadRolePolicy`、`EnvironmentRolePolicy` 等旧权限字段收敛到 `platform-manifest.yaml`
- [ ] 补齐内置模板同步、上传到 MinIO、数据库种子数据的一致性校验
- [ ] 逐个验证内置模板安装、卸载、RBAC 隔离、工作台操作和运行态数据

### Task 6.5: CI/CD 端到端生产化
- [ ] 明确 Tekton 是否继续作为目标；若继续，需要实现 Tekton 模板和工作台；若不继续，需要清理旧设计文档
- [ ] 为 source 组件链路补齐前置依赖检查：Gitea、Jenkins、kpack、registry/Harbor、ArgoCD
- [ ] 将 registry/Harbor 的 DNS、TLS、节点运行时信任配置做成可验证状态，而不是只展示说明
- [ ] 完整验证 source 模式：源码仓 → Gitea mirror → Jenkins/kpack → registry/Harbor → GitOps 清单 → ArgoCD → 集群
- [ ] 完整验证 image 模式：镜像输入 → GitOps 清单 → ArgoCD → 集群
- [ ] 将链路中的 warning/pending 状态映射到前端可操作的修复入口

### Task 6.6: 平台配置与管理员功能
- [ ] 增加平台配置模块，并按角色控制普通用户是否可见
- [ ] 补齐用户管理、角色管理、模板管理权限、审计记录等管理员页面
- [ ] 管理全局配置项，例如 `PAAP_REGISTRY_HOST_TEMPLATE`、MinIO、默认镜像源、kpack 状态
- [ ] 提供集群级依赖健康检查页面，例如 CRD、Operator、kpack、存储、模板仓库

### Task 6.7: 文档与测试清理
- [ ] 清理或标记仍描述旧方案的文档，尤其是 Tekton、raw-yaml、生命周期钩子相关内容
- [ ] 将 `docs/DEPLOYMENT-STATUS.md` 中旧镜像、RBAC、模板上传问题重新验证并更新
- [ ] 增加端到端测试覆盖：登录鉴权、环境模板、服务安装、组件 image/source 两条部署链路
- [ ] 增加模板包校验测试，确保所有 `data/charts/*.tar.gz` 包含 `chart/`、`platform-manifest.yaml`、`preset-values.yaml`
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
- [ ] 重设计配置模板导入 UI，使用白色 Carbon 表单视觉，避免重灰输入块
- [ ] 将配置模板导入的“适用组件”改为 select/combobox 控件
- [ ] 导入流程同时支持普通原生配置模板和高级 template/schema JSON，并清晰区分两种模式
- [x] 模板预览展示原始内容、抽取字段、敏感字段、生成文件和校验错误，不要求用户理解 Kubernetes 对象名
- [ ] 增强 configmap、secret、file-based config 解析，让后端到数据库/缓存/消息队列关系能安全自动连线

### Task 6.12: 降低 Kubernetes 术语暴露
- [ ] 全量检查页面、抽屉和工作台中的 namespace、service、pod、configmap、secret、pvc、helm 等术语
- [ ] 默认视图用产品语言替换 Kubernetes 术语，仅在高级/调试视图保留底层字段
- [ ] 为必须展示的底层概念补充上下文，避免应用管理员理解成本过高

### Task 6.13: 外部能力与共享能力模型
- [ ] 新增统一 `EnvironmentCapability` 模型，覆盖 `git`、`registry`、`ci`、`cd`、`monitor`、`logging`、`database`、`cache`、`mq`、`objectStorage`
- [ ] 每个 capability 支持 `managed`、`shared`、`external` 来源，并记录 provider、连接配置、验证结果和标准输出
- [ ] 环境模板声明 required capabilities，而不是硬编码必须安装 PAAP 管理实例
- [ ] 创建环境时支持 `platform install`、`use shared`、`external connection`、`configure later`
- [ ] default 共享环境只允许安装工具/中间件，不允许部署业务组件，且不可被普通用户删除
- [ ] 卡片和抽屉明确展示 `platform managed`、`shared`、`external` 来源和断开/卸载语义
- [ ] 对 external Git、Registry、Argo CD、Jenkins、Prometheus、Loki、PostgreSQL、Redis、RabbitMQ、Kafka、MinIO 做真实连接与权限验证
- [ ] external 来源删除只移除 PAAP 连接记录和本地凭据，不能删除真实外部资源

### Task 6.14: 平台目录、版本选择与服务暴露
- [ ] 安装/编辑中间件时提供版本下拉，数据来自同 `ServiceType` 下的 `ServiceTemplate.ChartVersion`
- [ ] 增加“平台支持的中间件/工具目录”只读浏览页，按工具/数据库/缓存/消息队列/对象存储分组
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
- [ ] 工作量：1 天（代码完成 + 部署验证）
- [ ] 对应文件：`frontend/src/views/CatalogView.vue`、`frontend/src/router/index.ts`、`frontend/src/layouts/MainLayout.vue`

### Task 7.3: 平台管理员界面
- [ ] 前端新增 `/platform` 路由 + `PlatformAdminView`
- [ ] Tab 页一：中间件目录管理（ServiceCatalog CRUD）
- [ ] Tab 页二：共享资源管理
- [ ] Tab 页三：用户/角色管理
- [ ] 后端给 `ServiceCatalog` 增加 `POST/PUT/DELETE` handler
- [ ] 当前状态：`ListServiceCatalog` 仅只读
- [ ] 工作量：1 周

### Task 7.4: 三种角色体系
- [ ] 角色定义：`platform_admin` / `app_admin` / `user`
- [ ] 后端 `internal/middleware/` 增加 auth + role 检查中间件（当前仅 `cors.go`）
- [ ] 路由分公开/应用/平台三层
- [ ] 前端按 `role` 显隐平台管理入口
- [ ] 当前状态：`User.Role` 只有 `user`/`admin`（`user.go:17`）
- [ ] 工作量：1 周（与 Task 7.3 合并做）

### Task 7.5: Capability 来源模型（环境内/共享/外部）
> 领导需求 2+3+4 的统一模型，也是 External Capability Design Direction 的落地

- [ ] 新增 `EnvironmentCapability` GORM 模型（`EnvironmentID` + `Capability` + `Source` + `RefServiceID` + `ExternalConfig`）
- [ ] `Source` 枚举：`self` / `shared` / `external`
- [ ] 系统初始化时创建 `default` 应用 + `default` 环境（受保护、只装工具/中间件）
- [ ] 平台管理员在 default 环境预装共享实例，供其它环境 `shared` 引用
- [ ] 重构 `registry_endpoint.go:16` `RuntimeRegistryHost` 的硬编码
- [ ] 重构 `environment.go` 中 `toolHTTPBaseURL` 等 FQDN 拼接
- [ ] 组件消费 capability 时按 `Source` 分流（self→本环境，shared→default，external→用户 endpoint）
- [ ] 放行 NetworkPolicy：业务 namespace → default 工具 namespace ingress
- [ ] external 来源放行到集群外 endpoint 的 egress
- [ ] 画布节点带 `zone` 字段（`environment` / `shared` / `external`）
- [ ] 三条泳道渲染：本环境、平台公共、集群外部
- [ ] `componentTopology.ts` 已有 `laneLabels`，扩展为可配置 zone
- [ ] 扩展 `ListAdoptableResources` 可扫指定 namespace / 全集群
- [ ] 新增"手动接入"外部资源表单（类型 + endpoint + 凭证）
- [ ] external 卡片只支持"断开"，不删真实资源
- [ ] 环境模板声明所需 capabilities
- [ ] 创建环境时每个 capability 让用户四选一
- [ ] 当前状态：`ServiceInstallation` 是环境级（`service_catalog.go:111`），无共享/外部概念
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
- [ ] 应用操作基于 `AppMember` 判断权限
- [ ] 移除 `OwnerID=1`、`UserID=1` 等硬编码
- [ ] 补齐应用成员管理页面和 API
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

### Task 7.9: KubeVirt 虚拟机
- [ ] 将 VM 作为新服务类型纳入 `ServiceCatalog`
- [ ] 用 KubeVirt CRD（`VirtualMachine`）而非 Helm chart 部署
- [ ] 需要集群已装 KubeVirt operator
- [ ] 当前状态：全项目零基础
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
- [ ] 用户认证对接 Keycloak：登录/注册/OAuth2/OIDC 流程
- [ ] 替换或并存当前简单 JWT 认证
- [ ] 用户管理（同步/创建/角色映射 Keycloak ←→ PAAP User）
- [ ] 当前状态：`internal/handler/auth.go` 自产 JWT 无外部 IdP
- [ ] 对应文件：`deploy/k8s/`、`internal/handler/auth.go`、`internal/model/user.go`
- [ ] 工作量：1-2 周

### Task 7.20: 画布卡片分组分区
- [ ] 画布上大卡片容器：每个大卡片 = 一个组/区（zone）
- [ ] 目前每个服务/组件是独立卡片，打开后大卡片包含这些小卡片
- [ ] 支持分组类型：本环境、平台公共、集群外部（对应 Task 7.5 的 zone 概念）
- [ ] 组卡片可折叠/展开
- [ ] 当前状态：画布上所有卡片平铺无分组
- [ ] 对应文件：`frontend/src/views/EnvDetailView.vue`、`frontend/src/composables/componentTopology.ts`
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
> kingbase 和 nacos 尚未完成 chart、安装参数、连接发现、工作台和测试前，不对用户展示为可选能力。

- [x] `ListServiceCatalog` 排除 `kingbase` / `nacos` 占位项，即使数据库里遗留为 enabled 也不会返回
- [x] `SeedServiceCatalog` 将 `kingbase` / `nacos` 默认写为 disabled，并显式修正旧数据
- [x] 增加后端回归测试，覆盖 enabled 遗留占位项不会出现在 catalog 响应中
- [x] Docker 镜像 `v0.1.449` 构建并部署到 kind 集群
- [x] kind 验证：显式使用 `--context kind-rbac-governance-test` 检查 `paap-server:v0.1.449`，Deployment `1/1 ready`，Pod `paap-server-797495ddd9-bscfj` Running
- [x] CDP/API 验证：`/api/v1/service-templates` 返回 14 个服务模板，`kingbase` / `nacos` 均不存在
- [x] 数据库验证：`service_catalogs` 中 `kingbase|f`、`nacos|f`
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

### Task 7.21: `docs/配置示例.md` → 内置配置模板
> 将 20 个配置示例转为 PAAP 内置配置模板（Go template），供组件配置 Tab 使用

- [ ] 梳理模板目录结构：`data/config-templates/` 按框架分组
- [ ] **Spring Boot 系列 (9 个)**: 基础 / +PG Hikari / +PG Druid / +PG 集群 Druid / +PG+Redis 单实例 / +PG+Redis 哨兵 / +PG+Redis 集群 / +PG+RabbitMQ / +PG+Nacos
- [ ] **Nginx 系列 (4 个)**: 基础静态 / +Upstream 负载均衡 / +SSL HTTPS / +静态资源分离缓存
- [ ] **Go/Gin 系列 (3 个)**: YAML / TOML / INI 格式
- [ ] **Python 系列 (2 个)**: FastAPI + PG+Redis / Django + PG+Redis
- [ ] **Node/TS 系列 (2 个)**: NestJS + PG+Redis (.env) / Vue/React Vite (.env.production)
- [ ] 每个模板只提取关键字段为模板变量
- [ ] 前端组件配置 Tab 中的"模板"下拉菜单选择后填充配置编辑区
- [ ] 当前状态：`docs/配置示例.md` 含 20 个纯文本示例，未被模板系统收录
- [ ] 对应文件：`data/config-templates/`（新建）、`internal/service/renderer.go`、`frontend/src/views/ComponentDetailView.vue`（配置 Tab）
- [ ] 工作量：1 周

---

## 执行顺序

```
Week 0  : ~~Task 7.1(版本号)~~ ✅ → Task 7.2(目录页)
Week 1-2: Task 7.3+7.4(平台管理+角色) → Task 7.8(认证鉴权)
Week 3-4: Task 7.5a~7.5c(Capability 模型地基)
Week 5-7: Task 7.5d~7.5g(画布分区+外部接入) → Task 7.6(Ingress)
Week 8+  : Task 7.13~7.15(配置模板) 并行 Task 7.9+7.10(KubeVirt+KEDA)
季度级   : Task 7.11(多集群) → Task 7.12(VM纳管)
穿插     : Task 7.17~7.18(验证与审计)
```
