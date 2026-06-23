# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What Is PAAP

PAAP (Platform-as-a-Application Platform) is a Railway-like self-service cloud-native application management platform. Users create applications, define environments, install tool/middleware services (Gitea, Harbor, Argo CD, Jenkins, PostgreSQL, Redis, etc.), and deploy business components — all through a canvas UI with drawers, not raw Kubernetes YAML.

## Development Commands

Requires Go 1.25+ via GVM (`source ~/.gvm/scripts/gvm && gvm use go1.25.7`). All `make` targets source GVM automatically.

```bash
# Backend
make run              # Run PAAP server (Go + Gin, default port 9090)
make run-operator     # Run PAAP operator (controller-runtime, connects to kind cluster)
make build            # Build server binary to bin/paap-server
make build-operator   # Build operator binary to bin/paap-operator
make all              # Build both

# Testing
make test             # Run all Go tests: go test ./...
make frontend-test    # Run frontend unit tests: cd frontend && npm run test
make frontend-smoke   # Headless browser smoke test (no Xorg needed)
make frontend-verify  # Full frontend check: unit tests + type check + build + smoke
make verify           # Backend tests + frontend-verify

# Code quality
make fmt              # go fmt ./...
make lint             # go vet ./...

# CRD management
make manifests        # Generate CRD YAML + deepcopy functions (controller-gen)
make install          # Apply CRDs to cluster
make uninstall        # Remove CRDs from cluster
make install-kpack    # Install kpack CRDs for source component Buildpacks builds

# Docker
make docker-build-server    # Build server image (includes frontend bundle + built-in templates)
make docker-build-operator  # Build operator image
make preload-kind-images    # Pre-load images into kind cluster

# Single Go test
source ~/.gvm/scripts/gvm && gvm use go1.25.7 && go test ./internal/handler/ -run TestFunctionName -v
```

## Architecture

Three-process system communicating through Kubernetes CRDs and a shared PostgreSQL/SQLite database:

```
Vue 3 Frontend  ──REST/WS──▶  PAAP Server (Gin)  ──CR CRUD──▶  PAAP Operator (controller-runtime)  ──▶  K8s
                                  │                                      │
                                  ├─ GORM models (PostgreSQL/SQLite)     ├─ Application/Environment/ServiceInstance/Component controllers
                                  ├─ Helm client (install/upgrade svc)   ├─ Creates namespaces, deployments, services, RBAC
                                  ├─ K8s client (proxy to tools)         └─ Reconciles CR state → K8s state
                                  └─ Direct tool APIs (Gitea, Harbor, ArgoCD, Jenkins, Prometheus, Loki, DBs, Redis, etc.)
```

### Server (`cmd/server/main.go` → `internal/`)

| Layer | Path | Role |
|-------|------|------|
| Entry | `cmd/server/main.go` | Init DB, seed defaults, sync cluster state, start Gin |
| Config | `config/config.go` | Env vars: `PORT` (default 9090), `DATABASE_URL` (default `paap.db`), `JWT_SECRET` |
| Router | `internal/handler/router.go` | All REST routes under `/api/v1/` + WebSocket at `/ws` + SPA static serving |
| Handlers | `internal/handler/` | HTTP handlers — one file per domain (application, environment, auth, template, sync, etc.) |
| Services | `internal/service/` | Business logic — cluster sync, component GitOps, DB admin, tool workspace actions, template rendering |
| Models | `internal/model/` | GORM models — application, environment, component, service_catalog, template, user, platform_manifest |
| K8s Client | `internal/k8s/` | Direct K8s API calls — CR manager, tool-specific clients (Gitea, Harbor, ArgoCD, Jenkins, Prometheus, Grafana, Loki, registry, database discovery, Redis discovery, runtime console/logs/metrics) |
| Helm Client | `internal/helm/` | Helm SDK wrapper for service install/upgrade/uninstall |
| Middleware | `internal/middleware/` | CORS, auth |
| Database | `internal/database/` | DB init/connection (SQLite dev, PostgreSQL prod) |

### Operator (`cmd/operator/main.go` → `internal/controller/`)

Standard Kubebuilder operator with four controllers:
- **ApplicationController** — creates/manages `paap-app-{id}` namespaces
- **EnvironmentController** — manages environment namespace, NetworkPolicy, ResourceQuota
- **ServiceInstanceController** — manages tool/middleware instances (SA, Role, Deployment via Helm)
- **ComponentController** — manages business component Deployment/Service

CRD types are defined in `api/v1/` (Application, Environment, ServiceInstance, Component).

### Frontend (`frontend/`)

Vue 3 + TypeScript + Carbon Design System + Pinia. Key structure:

- **Views** (`src/views/`): Top-level pages — `AppListView`, `EnvDetailView` (the main canvas), `ComponentDetailView`, `TemplatesView`, etc.
- **Composables** (`src/views/`): Co-located `.ts` files with business logic — `serviceWorkspace`, `componentWorkspace`, `envCapabilities`, `componentProfile`, `componentTopology`, `configTemplateRenderer`, etc. These contain the real domain logic; the `.vue` files are thin presentation.
- **Workspace Components** (`src/components/workspaces/`): Per-tool drawer content — `GiteaWorkspace`, `ArgocdWorkspace`, `DatabaseWorkspace`, `RedisWorkspace`, `MongoWorkspace`, `KafkaWorkspace`, `MinioWorkspace`, `RabbitWorkspace`, `MonitorWorkspace`, `LogWorkspace`, `PipelineWorkspace`, `RegistryWorkspace`. Each has a `ToolWorkspaceFrame` wrapper and `WorkspaceActionForm` for embedded actions.
- **API Client** (`src/api/client.ts`): Axios-based REST client
- **Store** (`src/stores/app.ts`): Pinia store
- **WebSocket** (`src/composables/useWebSocket.ts`): Real-time status updates
- **Tests**: Vitest — co-located `*.test.ts` files alongside source

### Built-in Templates (`data/charts/`)

Pre-packaged Helm chart tarballs for all supported services (Gitea, Harbor, ArgoCD, Jenkins, PostgreSQL, MySQL, Redis, MongoDB, Kafka, RabbitMQ, MinIO, Loki, Prometheus/Grafana, registry). These are embedded into the server image via `scripts/package-built-in-templates.sh`.

### Deployment (`deploy/k8s/`)

`deploy.sh` is the one-command deploy to a kind cluster. Individual manifests for server, operator, PostgreSQL, MinIO, namespace, and kpack.

## Key Patterns

- **Service install flow**: Server creates a draft ServiceInstance in DB → Operator reconciles it → Operator calls Helm install → Server polls for ready state → WebSocket pushes status to frontend.
- **Workspace actions**: Frontend workspace components call `POST /services/:id/workspace/actions` with a tool-specific action payload. Server dispatches to the appropriate tool client in `internal/k8s/` (e.g., `gitea.go`, `redis_admin.go`, `database_admin.go`).
- **Component delivery**: Two modes — image delivery (registry + image:tag) and source delivery (git repo + branch, built via kpack/Buildpacks). The Deploy tab form switches based on mode.
- **Canvas state**: The environment canvas layout (node positions, links) is persisted via `PUT /environments/:id/canvas-state`.
- **Config templates**: Component config templates use Go template syntax rendered by `internal/service/renderer.go`. Built-in templates are synced from `data/charts/` to MinIO + DB on startup.
- **Cluster sync**: On startup, `service.SyncClusterState` reconciles the DB with actual K8s cluster state to handle drift.
- **Tool proxy**: `ProxyServiceInstance` and `ProxyComponent` handlers forward HTTP/WebSocket requests directly to in-cluster tool pods.
- **External capability direction**: Tools and middleware should support externally-provided infrastructure, not only PAAP-managed installs. See "External Capability Design Direction" below.

## PAAP Agent Notes

### Unfinished Work And Known Gaps

Do not treat the long-running Railway-like drawer objective as complete until every item below has direct code, runtime, and CDP evidence.

1. Product-specific drawers still need a full audit for every concrete tool and middleware:
   Gitea, Registry, Harbor, Argo CD, Jenkins, Prometheus/Grafana, Loki, PostgreSQL, MySQL, MongoDB, Redis, RabbitMQ, Kafka, and MinIO.
   Existing drawers are partially product-specific, but not every product has been CDP-tested end to end.

2. MongoDB, Kafka, and MinIO now use embedded drawer action forms in source and the current `real-fullstack-prod` environment has running cards for all three.
   Recent CDP verification covered MongoDB insert/update/delete, Kafka topic/message create/read/delete, and MinIO bucket list/create/delete; continue deeper object-level and failure-state checks before treating this area as complete.

3. RabbitMQ embedded action forms are implemented in source and the current `real-fullstack-prod` environment has a running RabbitMQ card.
   Recent CDP verification covered queue, exchange, binding, publish, read, purge, and delete flows from the drawer; broader failure-state and edge-case checks remain open.

4. Database management is not fully proven.
   PostgreSQL drawer exposes database/table/row operations and backup creation, but table create/insert/update/delete and backup output need a fresh CDP run against a real database with visible before/after evidence.
   MySQL needs the same verification, including replication/Galera modes where applicable.

5. Database backup is only partially covered.
   Backup creation is implemented, but restore/download/list details and failure-state UX still need product-level decisions and CDP proof.

6. Persistent volume configuration needs full chart-by-chart proof.
   The UI shows PV size presets for many services, but each Helm values mapping and running-instance update must be verified against actual ServiceInstance specs, Helm output, PVCs, and chart behavior.
   Kubernetes PVC expansion limitations must be surfaced in user-facing language where a live resize cannot actually happen.

7. Topology modes need end-to-end verification.
   Redis standalone, replication, Sentinel, and cluster modes are represented in config.
   PostgreSQL/MySQL standalone, replica, dual-master/Galera/HA modes are represented in config.
   Each mode still needs a canvas deploy test proving the chosen values reach Helm and result in the expected pods/services/PVCs.

8. Runtime config updates for already-running services need more proof.
   Updating ServiceInstance values is implemented for running services, but every high-risk setting needs verification that the operator/Helm path reconciles the live release without stale UI state.

9. Per-card metrics need a Railway-like visual audit.
   CPU/memory charts exist in drawers, but every component/tool/middleware card must be checked for real data, empty states, time ranges, chart scaling, and no misleading placeholder values.

10. Per-card logs need a no-placeholder audit.
    Logs are available in drawers, but every component/tool/middleware card must be checked for real log lines and no "no such host" style failures.

11. Console needs broader verification.
    Attach/debug-container fallback was fixed and verified for selected component/service cases.
    It still needs CDP checks for all common tool and middleware pods, especially images without a shell and pods where ephemeral containers are restricted.

12. Config template coverage is incomplete.
    Built-in templates exist and the component drawer has a single template dropdown, but common framework templates still need broader coverage:
    nginx multi-backend routing, Spring Boot datasource/cache/mq profiles, Gin/Go config, Node/Vite frontend API config, and config-file based apps.

13. User-provided config template upload/edit/preview needs more UX proof.
    Template management exists, but the flow must clearly show raw template content, extracted fields, sensitive fields, generated files, and validation errors without requiring users to know Kubernetes object names.

14. Automatic relationship detection is incomplete.
    Env vars and selected service references can draw relationships, but configmaps/secrets/file-based configs need deeper parsing and safe heuristics so backend-to-db/cache/mq lines appear without manual wiring.

15. Kubernetes jargon is still visible in some places.
    Review all drawers and workspaces for labels such as namespace, service, pod, configmap, secret, pvc, helm, and replace or hide them unless the user explicitly opens an advanced/debug view.

16. Registry and image-source flow needs a final real demo pass.
    The component drawer separates environment registry host from image:tag, but the normal path still needs CDP proof:
    push image to registry, create component, push manifests to repo, Argo CD deploys, pod runs from the expected image.

17. The demo environment is now broad enough for drawer verification, but the full objective is still incomplete.
    Current verified environment has frontend, backend, PostgreSQL, Redis, Gitea, Argo CD, monitor, logs, registry, RabbitMQ, Kafka, MongoDB, MinIO, MySQL, Harbor, and Jenkins cards.
    Remaining gaps include registry/Harbor artifact demos, Jenkins build detail/log fidelity, per-card logs for every pod, topology modes, PV updates, and failure-state UX.

18. No fake or placeholder data is allowed.
    Every workspace resource, metric, log, backup, key, queue, topic, bucket, and deployment row must be traced to a real backend/API/cluster source.
    Add tests or remove UI blocks where data is synthetic.

19. CDP test coverage is still incomplete.
    Continue using the visible Chrome via CDP, not headless-only runs.
    Test every page/tab/drawer/action after each meaningful UI change.

20. Kind image loading remains required.
    The local kind cluster cannot reliably pull images.
    Always build/pull images locally and run `kind load docker-image --name rbac-governance-test ...` before applying manifests that reference new images.

21. Disk usage must be checked before and after image-heavy work.
    Current recent checks were safe, but frequent Docker builds can fill disk quickly.

22. Config template import UI still needs a focused redesign and implementation pass.
    The import dialog fields currently read as heavy gray boxes and do not match the white Carbon treatment.
    The "适用组件" field should be a select/combobox-style control instead of comma-separated text.
    Import must support both ordinary native config templates and advanced template + schema JSON uploads; the UI should make the difference explicit without forcing non-expert users into JSON-first authoring.

## External Capability Design Direction

Leadership wants tools and middleware to support externally provided infrastructure, not only PAAP-managed installs.
Design this as one unified "environment capability instance" model with two provisioning modes, instead of creating a parallel external-resource system.

- Capability instances should cover both tools and middleware:
  `git`, `registry`, `ci`, `cd`, `monitor`, `logging`, `database`, `cache`, `mq`, and `objectStorage`.
- Each capability instance should have a provider and provisioning mode:
  examples include `gitea`, `gitlab`, `harbor`, `registry`, `jenkins`, `argocd`, `prometheus`, `loki`, `postgresql`, `redis`, `rabbitmq`, `kafka`, and `minio`;
  `provisionMode` should be `managed` or `external`.
- Environment templates should declare required capabilities, not hard-code that PAAP must install every backing product.
  During environment creation, users should be able to choose:
  `platform install`, `external connection`, or `configure later`.
- Cards and drawers should show the source clearly:
  examples: `prod-gitea · platform managed`, `corp-gitlab · external`, `prod-postgresql · platform managed`, `corp-postgres · external`.
- Managed capabilities keep the current install/upgrade/uninstall flow:
  chart version, values, storage, resource sizing, runtime status, logs, metrics, and uninstall.
- External capabilities use a connection drawer:
  endpoint, credentials or Secret reference, project/namespace/database name, TLS settings, validation result, and usage output.
  External cards must support "disconnect" only; they must never delete the real external resource.
- Consumers should not care whether a capability is managed or external.
  Source delivery, image delivery, deployment, monitoring, logging, and app binding should consume standardized outputs such as:
  `git.cloneUrl`, `registry.pushEndpoint`, `registry.pullEndpoint`, `ci.webhookUrl`, `cd.applicationTarget`, `monitor.queryEndpoint`, and `logging.queryEndpoint`.
- External connections must have real validation, not just saved configuration:
  Git token can list repositories or create webhooks;
  registry auth can log in and, where allowed, push/pull;
  Argo CD token can list/create applications;
  Prometheus and Loki can query;
  PostgreSQL/Redis/RabbitMQ/Kafka/MinIO can connect and verify required permissions.
- Deletion semantics must be explicit:
  `managed` may uninstall releases and delete PAAP-owned resources after confirmation;
  `external` only removes PAAP's connection record and local credentials.
- Recommended implementation order:
  first external Git/Registry/Argo CD/Jenkins/Monitor/Logging;
  then PostgreSQL/Redis/RabbitMQ/Kafka/MinIO;
  then multi-instance selection within one environment.

## Last Known Runtime State

- Kind cluster: `kind-rbac-governance-test`
- Use kind node/container IP `172.18.0.2` for browser-accessible URLs; do not substitute `127.0.0.1`.
- Last deployed PAAP server image for this note: `paap-server:v0.1.423`
- Current page: `http://172.18.0.2:30091/apps/5/environments/5?tab=components`
- Demo app: `http://172.18.0.2:32360`
- Recent verified CDP flow: Redis key write/read/delete, MongoDB document insert/update/delete, Kafka topic/message create/read/delete, RabbitMQ queue/exchange/binding/message flows, MinIO bucket create/list/delete, PostgreSQL/MySQL table row operations and backup creation, Gitea repo/file view, Argo CD app/resource view, Jenkins job list/check, Prometheus/Grafana dashboard, and Loki log read.
- Current business components `frontend` and `backend` are image delivery components; neither has `source_repo_url`, `source_mirror_repo_url`, or `jenkins_job`, so this environment currently has no source delivery component.
- Recent feature: canvas card display name (rename). Double-click or right-click → 重命名 triggers inline editing on canvas cards. Display names are persisted in `environment_canvas_states.names` and are independent of real component/service names.
- Recent component drawer fix: the Deploy tab now uses a dynamic delivery form. Image delivery shows registry/image tag fields; source delivery shows source repository, branch, and build context fields with Buildpacks/kpack wording.
- Recent code fix: MongoDB update drawer field now says `设置字段 JSON`, matching the backend `$set` semantics and avoiding ambiguous `Update JSON` wording.
- Recent Jenkins fix: Jenkins API currently returns zero jobs in this environment; the PAAP drawer now shows a real empty `jenkins-jobs` catalog instead of fallback component jobs or `:pending` image artifacts.
- Recent Jenkins chart fix: Jenkins 2.414.3 now pins compatible Pipeline plugin dependencies (`pipeline-model-*`, `workflow-job`, `pipeline-stage-step`) and `git-server`, removing the previous failed plugin health state.

## Leadership Roadmap (2026-06-23 评估)

领导提出的 15 项需求。下面按"难度 + 工作量 + 现有基础"做了分级排序与初步方案。难度分四档:**S(small/天)**、**M(medium/周)**、**L(large/月)**、**XL(超大/季度+,架构级)**。

### 现状基线(决定各项起点)

- **单集群**:无 `Cluster` 模型,`internal/k8s/client.go` 直连一个集群,Environment 仅一个 `Namespace` 字段(`environment.go:20`)。
- **中间件目录已有**:`ServiceCatalog` 模型存在(`template.go:10`),但只在启动时 `SeedServiceCatalog()` 灌默认值(`template.go:311`),**无 admin 增删改界面**。
- **版本号有字段无界面**:`ServiceTemplate.ChartVersion` 存在(`service_catalog.go:28`),前端无版本选择器。
- **服务是环境级的**:`ServiceInstallation` 是 `EnvironmentID + ServiceType` 联合唯一(`service_catalog.go:111`),**没有跨环境共享/平台级单例**的概念。
- **外部资源采纳有雏形**:`ListAdoptableResources` + `adoptableEnvironmentNamespaces`(`environment.go:1662/1700`)已能列出环境内可纳管的资源,但**只扫环境自己 namespace**,纳管不了集群外/它 namespace 的。
- **权限极简**:`User.Role` 只有一个 `user/admin` 字符串(`user.go:17`),无角色管理界面。
- **无 KubeVirt / KEDA / 多集群网络**:全项目零结果。

### 分级排序与方案(由易到难)

#### 第一档 S — 快速可交付(天级,改动小)

**① 组件改名(需求1)** — 已完成
- 状态:**已实现**(双击/右键改名,画布显示名持久化到 `EnvironmentCanvasState.Names`)。
- 剩余:交互 bug 已在本次修复(单击不再延迟、双击不再触发两次)。
- 工作量:**0**,收尾。

**⑤ 中间件版本号(需求5)** — ✅ 已完成
- 后端 `ServiceTemplate` 加 `AppVersion` 字段，`Type` 改为普通 index（允许多版本）
- 新增 `extractChartYamlMeta()` 从 tarball `chart/Chart.yaml` 自动解析 `version` 和 `appVersion`
- 重写 `SeedServiceTemplates()` 遍历 `data/charts/*.tar.gz` 自动解析版本，取代硬编码
- `InstallServiceRequest` 加 `AppVersion`，安装时按 type + appVersion 查模板
- 前端 deploy tab 加版本下拉框（未部署时可选版本，已部署后只读显示）
- 画布卡片和 drawer 头部不再显示版本号
- 提交：`9946b64`（model+templates）、`b9be953`（API+前端）、`aff00ec`（image tag）
- 工作量：约 2 天

**⑥ 中间件列表展示(需求6)** — 数据已有,补聚合页
- 现状:`ServiceCatalog` + `ServiceTemplate` 已完整,`GET /service-catalog` 已返回。
- 缺口:无统一的"平台支持哪些中间件"浏览页。
- 方案:新增一个只读的中间件目录页(或放在平台管理界面内),按 `Category(tool/infra)` 分组,展示 类型/名称/可用版本/描述/图标。
- 工作量:**S(1 天)**。

#### 第二档 M — 中等(周级,需新增模型或界面,但不动核心架构)

**⑦ 平台管理员界面(需求7)** — 与 ⑥、⑭ 强相关
- 现状:无任何平台级管理页;`User.Role=admin` 存在但无管理入口。
- 方案:新增 `/platform` 路由 + `PlatformAdminView`,含几个 tab:
  - **中间件目录管理**(接需求 6):对 `ServiceCatalog`/`ServiceTemplate` 做 CRUD——平台管理员可新增中间件类型、上传 chart、维护版本列表。后端给 `ServiceCatalog` 加 `POST/PUT/DELETE` handler(当前只有 `ListServiceCatalog` 只读)。
  - **共享资源管理**(接需求 2、3):登记平台级共享实例(见下)。
  - **用户/角色管理**(接需求 14):列用户、改角色。
- 工作量:**M(1 周)**。新建一个 view + 若干 CRUD handler。

**⑭ 三种角色(需求14)** — 扩展 Role
- 现状:`User.Role` 单字符串 `user/admin`。
- 方案:
  - 角色定义:`platform_admin`(平台管理员:管中间件目录、共享资源、用户)、`app_admin`(应用管理员:管自己的应用/环境)、`user`(普通:只读/受限)。
  - 落地:后端 `middleware/auth.go` 加角色判断;路由分公开/应用/平台三层;前端按 `role` 显隐平台管理入口。
  - 不必引入 RBAC 框架,字符串枚举 + 中间件足够。
- 工作量:**M(1 周)**,与 ⑦ 合并做。

**⑧ Ingress/Gateway(需求8)** — 暴露面
- 现状:组件暴露方式未明确,`externalAccess` 已在画布上分组展示(`EnvDetailView.vue:188`)。
- 方案:
  - 选定一种暴露模型:Ingress(单集群够用)或 Gateway API(为多集群铺路,推荐)。
  - 给组件/环境加"暴露规则":域名、路径、TLS,生成 Ingress/Gateway HTTPRoute 资源。
  - 画布上已有 `externalAccess` 分组卡片,扩展为可配置入口。
- 工作量:**M(1–1.5 周)**。

**⑨ ServiceIP(需求9)** — 网络可见性
- 现状:无 Service IP 展示/分配。
- 方案:组件/中间件安装后,从 K8s 读回 `Service.spec.clusterIP` / `LoadBalancer IP`,在卡片和 drawer 展示;支持把固定 IP 写回 spec(`clusterIP` 预留场景)。本质是只读展示 + 少量编辑。
- 工作量:**M(0.5–1 周)**。

#### 第三档 L — 较大(月级,需要新模型 + 跨域打通,但仍在单集群内)

**②③④ Capability 来源模型:环境内 / 共享 / 外部 三选一(需求2+3+4 合并)** — 统一主线

> 这是 ②③④ 三条需求共同的统一模型,不分开做。核心:**共享是额外接入方式,不取代环境内安装;三种来源并存,应用管理员按需选**。default 应用/default 环境只是 `shared` 来源的实例仓库。

**三种 capability 来源(并列,用户每个能力单独选):**

| 来源 | 说明 | 实例归谁 | 删除语义 |
|------|------|----------|----------|
| `self` (环境内安装) | 现状,本环境装一份独占 | 本环境 ServiceInstallation | 卸载 release |
| `shared` (共享) ← 新增 | 引用 default 环境的工具/中间件 | default 环境 ServiceInstallation | 仅断开引用,不动实例 |
| `external` (外部接入) | 接集群外/已有资源,只存连接记录 | 无 PAAP 实例 | 仅删连接记录 |

覆盖的能力:`git`、`registry`、`ci`、`cd`、`monitor`、`logging`、`database`、`cache`、`mq`、`objectStorage`(与 "External Capability Design Direction" 章节一致)。

**default 共享环境:**
- 系统初始化时建一个固定的 `default` 应用 + `default` 环境,**只允许装工具/中间件,不允许装业务组件**(UI + 后端双重约束)。
- 平台管理员在 default 环境预装 gitea/registry/argocd 等,供其它环境 `shared` 引用。
- default 环境受保护:不可删除、业务组件不可选它做部署目标。

**模型改动(最小侵入):**
- 新增 `EnvironmentCapability` 模型:`EnvironmentID` + `Capability`(如 `git`/`registry`/`database`)+ `Source`(枚举 `self`/`shared`/`external`)+ `RefServiceID`(指向 self/shared 的 ServiceInstallation)+ `ExternalConfig`(external 来源的 endpoint/凭证/Secret)。一个环境对每个能力一条记录。
- `ServiceInstallation` 模型**不动**(仍是环境级),default 环境的共享实例就是 default 环境的 ServiceInstallation,被 `EnvironmentCapability.RefServiceID` 引用。
- 环境创建/模板:**模板声明需要哪些 capability**,创建环境时每个 capability 让用户四选一:`环境内安装` / `用平台共享` / `外部接入` / `稍后配置`。

**必须改的 3 处硬编码(否则跨来源寻址连不上):**
- `internal/service/registry_endpoint.go:16` `RuntimeRegistryHost`:现在 host 永远按"消费者 app-env"推导 toolNamespace(`{app}-{env}-{service}`)。需改为接收"**服务实际所在的 namespace/环境**",shared/external 时用 default 或外部地址,而非当前 app-env。
- `internal/handler/environment.go:7368` `toolHTTPBaseURL` 及同类 FQDN 拼接:host 的 namespace 要来自服务实例自己的 `inst.Namespace`(已基本正确),但要确保 shared 引用时解析到 default 环境的实例而非本环境。
- **绑定解析层**:组件消费 capability(`git.cloneUrl`、`registry.pushEndpoint`、`monitor.queryEndpoint` 等)时,按 `EnvironmentCapability.Source` 分流:`self`→本环境 ServiceInstallation;`shared`→default 环境 ServiceInstallation;`external`→用户填的 endpoint。**这是让三种来源能并存的核心**。

**网络放行(必须配套):**
- `EnvironmentController` 的 NetworkPolicy(`operator` 章节)默认可能 deny 跨 namespace。需放行"所有业务 namespace → default 工具 namespace"的 ingress,否则 shared 来源的中间件连不上。external 来源另需放行到集群外 endpoint 的 egress。

**画布分区(需求③,本主线的前端表现层):**
- 画布节点带 `zone` 字段(`environment`/`shared`/`external`),渲染时归入三条泳道:`本环境`、`平台公共`、`集群外部`。
- 代码已有 `laneLabels` 前后端分层(`componentTopology.ts`),扩展为可配置 zone 即可。
- shared 节点 = 引用 default 环境实例;external 节点 = 引用外部资源;不可拖到本环境区。

**外部接入(需求④,external 来源分支):**
- 现有 `ListAdoptableResources` 只扫环境自己 namespace(`environment.go:1700`)。扩展为可扫指定 namespace 列表/全集群(权限过滤),并新增"手动接入"表单(类型+endpoint+凭证,不依赖扫描)。
- external 卡片只支持"断开",永不删真实资源。与 "External Capability Design Direction" 章节完全对齐,落地那套设计。

- 工作量:**L(4–6 周)**,按来源分三步交付(见执行顺序)。02 是地基,03 是前端展现,04 是 external 分支。
- 与 CLAUDE.md 已有 "External Capability Design Direction" 章节的关系:**external 来源 = 落地那套设计;shared/self 来源 = 补齐另两种**。合并后需求 ②③④ 与该章节统一为同一套 Capability 模型,不重复设计。

**⑩ KubeVirt 起带数据库的虚拟机(需求10)** — 新资源类型
- 现状:零基础。
- 方案:
  - 把 VM 当作一种新的"服务类型"纳入 `ServiceCatalog`(如 `vm-database`),后端用 KubeVirt CRD(`VirtualMachine`)而非 Helm chart 部署。
  - 模型:复用 `ServiceInstallation`,但 `provisionMode` 区分;安装逻辑分支到 KubeVirt client。
  - 需要集群已装 KubeVirt operator。
- 工作量:**L(3–4 周)**。新 CRD 接入 + 生命周期 + 备份。

**⑪ KEDA 水平扩展(需求11)** — 新能力
- 现状:零基础,组件副本数是固定 `Replicas`(`component.go:25`)。
- 方案:
  - 组件配置加"弹性伸缩"段:最小/最大副本、触发器(CPU/Q/自定义)。
  - 后端生成 `ScaledObject`(KEDA CRD)而非固定副本数 Deployment。
  - 需要集群已装 KEDA。
- 工作量:**L(2–3 周)**。

#### 第四档 XL — 架构级(季度+,需要多集群地基)

**⑫ 双集群 ArgoCD 主从 + 跨集群网络(需求12)** — 架构级
- 现状:**无 Cluster 模型,纯单集群**,零多集群基础。
- 方案(分阶段):
  1. 引入 `Cluster` 模型(注册集群、kubeconfig、label),`Environment` 加 `ClusterID`。
  2. ArgoCD 主从:一个主 ArgoCD 管多集群(app-of-apps / destination cluster),或主从 ArgoCD 实例同步。
  3. 跨集群网络:Submariner(推荐,Pod/Service 跨集群直连)或 VXLAN overlay;headtail/WireGuard 做站点间隧道。环境模板里声明网络策略。
- 工作量:**XL(1–2 个月+)**。是整个 ⑫⑬ 的地基,必须最先动但最难。
- **建议**:这一项是需求 ⑬ 的前置,二者绑定。

**⑬ VXLAN 纳管虚拟机 / 跨集群(需求13)** — 依赖 ⑫
- 现状:零基础。
- 方案:在 ⑫ 的 Cluster 模型和网络层之上,纳管已有虚拟机(VXLAN 接入 + 资源注册),作为"集群外部资源"(接 ④)的一种特殊形态。
- 工作量:**XL**,排在 ⑫ 之后。

#### 收尾(非领导需求,但需排期)

**⑮ 放到 Plane 和 Gitea 上,李浩然做 CI/CD(需求15)**
- 这一项偏运维/协作交付,不是代码功能。
- 方案:把本项目仓库 + 文档(含本 CLAUDE.md 路线图)推到内部 Gitea(已配好双推)和 Plane;在 Plane 拆任务给李浩然,按上面的 S→M→L→XL 顺序排迭代。
- 工作量:**S(半天协调)**,但它是其余工作能推进的"项目管理基建"。

### 建议执行顺序(按依赖与性价比)

```
Week 0  : ⑮ 协调 + 把路线图落到 Plane/Gitea(项目管理地基)
Week 1-2: ⑤✅ → ⑥✅(目录页)→ ⑦⑭(平台管理+角色)   ← 快速见效,领导能立刻看到
Week 3-4: ②(平台共享 Gitea/Registry)→ ③(画布分区)  ← ②是③④的地基
Week 5-7: ④(外部资源接入)→ ⑧⑨(Ingress/ServiceIP)   ← ④落地 External Capability 设计
Week 8+  : ⑩⑪(KubeVirt/KEDA)并行                  ← 需集群装好对应 operator
季度级   : ⑫(多集群地基)→ ⑬(VM纳管)               ← 架构级,独立专项,最后做
```

**关键判断**:
- ⑤⑥⑦⑭⑨ 都是"数据已有/字段已有,补界面和 CRUD",性价比最高,先做。
- ②③④ 是一条主线(平台公共 ↔ 环境内 ↔ 集群外),②是地基,且与 CLAUDE.md 已有的 "External Capability Design Direction" 是同一件事,合并排期能省一半设计。
- ⑩⑪ 引入新 CRD,依赖集群 operator,技术风险中等。
- ⑫⑬ 是架构级,工作量最大,必须单独立项,但它解锁不了前面任何一项,**不应阻塞前面的快速见效项**——可以最后做或并行预研。

### 核对表(对应领导 15 条)

| # | 需求 | 难度 | 现有基础 | 依赖 |
|---|------|------|----------|------|
| 1 | 组件改名 | S | ✅已完成 | 无 |
| 2 | Gitea/Registry 共享 | L | 无(环境级) | ⑦ |
| 3 | 分区分组(平台/环境/外部) | L | 无 | ②④ |
| 4 | 接入集群外部资源 | L | 雏形(只扫环境内) | ② |
 | 5 | 中间件版本号 | S | ✅已完成 | 无 |
| 6 | 中间件列表 | S | ✅已完成 | 无 |
| 7 | 平台管理员界面 | M | 无 | ⑭ |
| 8 | Ingress/Gateway | M | externalAccess 展示 | 无 |
| 9 | ServiceIP | M | 无 | 无 |
| 10 | KubeVirt 带库虚拟机 | L | 无 | 集群装 KubeVirt |
| 11 | KEDA 水平扩展 | L | 无 | 集群装 KEDA |
| 12 | 双集群 ArgoCD+跨集群网络 | XL | 无(单集群) | — |
| 13 | VXLAN 纳管虚拟机 | XL | 无 | ⑫ |
| 14 | 三种角色 | M | Role 字段 | 无 |
| 15 | 放 Plane/Gitea 给李浩然 | S | 无 | 无 |
