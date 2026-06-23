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
- Last deployed PAAP server image for this note: `paap-server:v0.1.401`
- Current page: `http://172.18.0.2:30091/apps/5/environments/5?tab=components`
- Demo app: `http://172.18.0.2:32360`
- Recent verified CDP flow: Redis key write/read/delete, MongoDB document insert/update/delete, Kafka topic/message create/read/delete, RabbitMQ queue/exchange/binding/message flows, MinIO bucket create/list/delete, PostgreSQL/MySQL table row operations and backup creation, Gitea repo/file view, Argo CD app/resource view, Jenkins job list/check, Prometheus/Grafana dashboard, and Loki log read.
- Current business components `frontend` and `backend` are image delivery components; neither has `source_repo_url`, `source_mirror_repo_url`, or `jenkins_job`, so this environment currently has no source delivery component.
- Recent component drawer fix: the Deploy tab now uses a dynamic delivery form. Image delivery shows registry/image tag fields; source delivery shows source repository, branch, and build context fields with Buildpacks/kpack wording.
- Recent code fix: MongoDB update drawer field now says `设置字段 JSON`, matching the backend `$set` semantics and avoiding ambiguous `Update JSON` wording.
- Recent Jenkins fix: Jenkins API currently returns zero jobs in this environment; the PAAP drawer now shows a real empty `jenkins-jobs` catalog instead of fallback component jobs or `:pending` image artifacts.
- Recent Jenkins chart fix: Jenkins 2.414.3 now pins compatible Pipeline plugin dependencies (`pipeline-model-*`, `workflow-job`, `pipeline-stage-step`) and `git-server`, removing the previous failed plugin health state.
