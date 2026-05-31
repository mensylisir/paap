# PAAP 部署架构规范

## 核心原则

**所有 PAAP 平台组件必须部署在同一个 namespace：`paap-system`**

这个设计原则确保：
- 简化网络访问和服务发现
- 统一权限管理和资源配额
- 降低运维复杂度
- 避免跨 namespace 的依赖问题

## Namespace 划分

### paap-system（平台 namespace）

所有平台基础设施组件部署在此：

| 组件 | 类型 | 说明 |
|-----|------|------|
| paap-server | Deployment + Service | API 服务器，提供 REST API 和 WebSocket |
| paap-operator | Deployment | CRD 控制器，管理 Application/Environment/ServiceInstance/Component |
| postgres | StatefulSet + Service | 元数据存储（用户、应用、环境、组件信息） |
| minio | Deployment + Service + PVC | S3 兼容对象存储，存储 Helm Chart 模板包 |
| Application CR | CustomResource | 应用定义（所有 Application 实例都在 paap-system） |

**服务访问地址：**
- PAAP Server: `paap-server.paap-system.svc.cluster.local:8080`
- PostgreSQL: `paap-postgres.paap-system.svc.cluster.local:5432`
- MinIO API: `minio.paap-system.svc.cluster.local:9000`
- MinIO Console: `minio.paap-system.svc.cluster.local:9001`

### paap-app-{app-id}（应用 namespace）

每个应用创建独立的 namespace，包含：

| 资源 | 说明 |
|-----|------|
| Environment CR | 环境定义（dev/test/prod） |
| ServiceInstance CR | 工具实例（Prometheus/Grafana/Redis 等） |
| Component CR | 业务组件（微服务） |
| ServiceAccount | 工具专用账号 |
| Role/RoleBinding | 工具权限 |
| Deployment/Service | 实际工作负载 |
| NetworkPolicy | 网络隔离策略 |
| ResourceQuota | 资源配额 |

## 部署流程

### 1. 创建基础 namespace

```bash
kubectl apply -f deploy/k8s/namespace.yaml
```

只创建 `paap-system`，不再创建 `paap-storage` 等其他 namespace。

### 2. 安装 CRD

```bash
kubectl apply -f config/crd/bases/
```

### 3. 部署基础设施（全部在 paap-system）

```bash
# PostgreSQL
kubectl apply -f deploy/k8s/postgres.yaml

# MinIO
kubectl apply -f deploy/k8s/minio.yaml

# 等待就绪
kubectl wait --for=condition=ready pod -l app=paap-postgres -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=minio -n paap-system --timeout=60s
```

### 4. 初始化模板

```bash
# 复制模板到 kind 节点
docker cp data/charts kind-control-plane:/tmp/paap-charts

# 运行初始化 Job
kubectl apply -f deploy/k8s/init-templates.yaml

# 等待完成
kubectl wait --for=condition=complete job/init-templates -n paap-system --timeout=300s
```

### 5. 部署 PAAP 组件

```bash
# Operator
kubectl apply -f deploy/k8s/paap-operator.yaml

# Server
kubectl apply -f deploy/k8s/paap-server.yaml

# 等待就绪
kubectl wait --for=condition=ready pod -l app=paap-operator -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=paap-server -n paap-system --timeout=60s
```

## 配置规范

### 环境变量

所有组件的环境变量中涉及服务地址的，必须使用 `paap-system` namespace：

```yaml
# ✅ 正确
- name: DATABASE_URL
  value: "postgres://user:pass@paap-postgres.paap-system.svc.cluster.local:5432/paap"

- name: S3_ENDPOINT
  value: "http://minio.paap-system.svc.cluster.local:9000"

# ❌ 错误
- name: S3_ENDPOINT
  value: "http://minio.paap-storage.svc.cluster.local:9000"
```

### 服务发现

在 paap-system 内部，可以使用短名称：

```yaml
# 在 paap-system 内部
DATABASE_URL: postgres://user:pass@paap-postgres:5432/paap
S3_ENDPOINT: http://minio:9000

# 跨 namespace 访问（从 paap-app-xxx 访问平台服务）
S3_ENDPOINT: http://minio.paap-system.svc.cluster.local:9000
```

## 迁移指南

如果已经部署了使用 `paap-storage` 的旧版本：

```bash
# 1. 删除旧的 paap-storage namespace
kubectl delete namespace paap-storage

# 2. 重新部署 MinIO 到 paap-system
kubectl apply -f deploy/k8s/minio.yaml

# 3. 重新初始化模板
kubectl delete job init-templates -n paap-system
kubectl apply -f deploy/k8s/init-templates.yaml

# 4. 重启 paap-server（刷新连接）
kubectl rollout restart deployment/paap-server -n paap-system
```

## 验证

部署完成后，验证所有组件都在 paap-system：

```bash
# 查看所有 Pod
kubectl get pods -n paap-system

# 应该看到：
# paap-postgres-xxx
# minio-xxx
# paap-operator-xxx
# paap-server-xxx

# 查看所有 Service
kubectl get svc -n paap-system

# 应该看到：
# paap-postgres
# minio
# minio-external (NodePort)
# paap-operator
# paap-server

# 不应该存在 paap-storage namespace
kubectl get namespace | grep paap
# 只应该看到 paap-system 和 paap-app-* 应用 namespace
```

## 故障排查

### MinIO 连接失败

检查服务地址是否正确：

```bash
# 在 paap-server Pod 中测试
kubectl exec -n paap-system deployment/paap-server -- \
  curl -I http://minio:9000/minio/health/live

# 应该返回 200 OK
```

### 模板上传失败

检查 init-templates Job 日志：

```bash
kubectl logs -n paap-system job/init-templates
```

确认 MinIO 服务地址使用 `minio.paap-system.svc.cluster.local`。

## 总结

- ✅ 所有平台组件在 `paap-system`
- ✅ 应用资源在 `paap-app-{id}`
- ❌ 不再使用 `paap-storage` 或其他平台 namespace
- ❌ 不要跨多个 namespace 分散平台组件
