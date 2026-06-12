# PAAP 自定义模板快速参考

## 📦 模板包结构

```
my-template/
├── chart/                      # 必需：Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── platform-manifest.yaml      # 必需：平台元数据
├── preset-values.yaml          # 推荐：禁用内置 RBAC
└── dashboards/                 # 可选：Grafana 面板
```

## 🔑 platform-manifest.yaml 模板

```yaml
name: "MyTool"
version: "v1.0.0"
description: "工具描述"

permissions:
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "configmaps", "secrets"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # 需要控制业务负载 namespace 时声明。
  workloadNamespaces:
    rules:
      - apiGroups: ["*"]
        resources: ["*"]
        verbs: ["*"]
  # 需要观察/操作同环境其它工具、中间件、数据库 namespace 时声明。
  environmentNamespaces:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services"]
        verbs: ["get", "list", "watch"]

observability:  # 可选
  dashboard_path: "./dashboards/main.json"
  metrics:
    port: 9090
    path: "/metrics"

variable_mapping:  # 可选
  - platform_var: "env_namespaces"
    helm_var: "config.namespaces"
```

## 🚫 禁用内置 RBAC (preset-values.yaml)

```yaml
rbac:
  create: false

serviceAccount:
  create: false
  name: ""

networkPolicy:
  enabled: false
```

## 🎯 Chart 必须使用的平台变量

```yaml
# values.yaml
global:
  namespace: ""              # 平台注入
  serviceAccountName: ""     # 平台注入
  env: ""                    # 平台注入

# templates/deployment.yaml
metadata:
  namespace: {{ .Values.global.namespace }}
spec:
  template:
    spec:
      serviceAccountName: {{ .Values.global.serviceAccountName }}
```

## 🔒 权限类型选择

| 权限块 | 说明 | 示例 |
|-------|------|------|
| `toolNamespace` | 工具自身 namespace 内权限 | PostgreSQL, Redis, Grafana 自身资源 |
| `workloadNamespaces` | 业务负载 namespace 权限，可声明 `*/*/*` | ArgoCD, Jenkins |
| `environmentNamespaces` | 同环境其它 namespace 权限，包含工具/中间件/数据库/业务 namespace | Prometheus, Loki |

## 🌐 可用的平台变量

| 平台变量 | 说明 | 示例值 |
|---------|------|--------|
| `current_env_name` | 环境名称 | `开发环境` |
| `current_env_identifier` | 环境标识 | `dev` |
| `primary_namespace` | 主 namespace | `order-service-dev` |
| `env_namespaces` | 所有 namespace | `ns1,ns2,ns3` |
| `app_name` | 应用名称 | `订单服务` |
| `app_identifier` | 应用标识 | `order-service` |

## ⚠️ 禁止的资源

❌ 不能在 Chart 中创建：
- `ClusterRole` / `ClusterRoleBinding`
- `CustomResourceDefinition` (CRD)
- `MutatingWebhookConfiguration` / `ValidatingWebhookConfiguration`
- `PodSecurityPolicy`
- 任何集群级别的资源

## 🧪 本地测试

```bash
# 1. 准备测试 values
cat > test-values.yaml <<EOF
global:
  namespace: test-ns
  serviceAccountName: test-sa
  env: dev
EOF

# 2. 渲染模板
helm template my-release ./chart \
  -f preset-values.yaml \
  -f test-values.yaml

# 3. 检查输出
```

## 📤 打包上传

```bash
# 打包
tar -czf my-template.tar.gz my-template/

# 上传到 PAAP 平台（通过 UI 或 API）
```

## ✅ 上传前检查清单

- [ ] `platform-manifest.yaml` 格式正确
- [ ] `permissions.toolNamespace` / `workloadNamespaces` / `environmentNamespaces` 按实际需求声明
- [ ] `preset-values.yaml` 禁用了内置 RBAC
- [ ] Chart 中没有集群级资源
- [ ] 所有资源使用 `global.namespace`
- [ ] Deployment 使用 `global.serviceAccountName`
- [ ] 权限遵循最小化原则

## 🔗 完整文档

- [自定义模板开发指南](../design/custom-template-guide.md)
- [零改动转换第三方 Chart](../design/custom-template-guide.md#10-零改动转换第三方-helm-chart)
- [完整示例：自定义 Prometheus](../examples/custom-prometheus-template/)
- [完整示例：Bitnami Redis（零改动）](../examples/bitnami-redis-template/)
- [服务模板规范](../design/service-template-spec.md)

## 🎯 零改动转换第三方 Chart

### 核心原则

**不要修改第三方 Chart 的代码**，只通过配置覆盖和平台元数据来实现集成。

### 三个核心技巧

#### 1. 利用 values.yaml 禁用内置 RBAC

```yaml
# preset-values.yaml
rbac:
  create: false

serviceAccount:
  create: false
  name: ""

installCRDs: false
```

#### 2. 参数映射

```yaml
# platform-manifest.yaml
variable_mapping:
  - platform_var: "generated_password"
    helm_var: "auth.password"
  
  - platform_var: "service_account_name"
    helm_var: "master.serviceAccount.name"
```

#### 3. 平台层资源清洗（可选）

```
helm template → 扫描 YAML → 清洗违规资源 → kubectl apply
```

### 转换步骤

```bash
# 1. 下载官方 Chart（不要修改）
helm pull bitnami/redis --version 18.19.0 --untar -d ./chart

# 2. 编写 platform-manifest.yaml
# 3. 编写 preset-values.yaml
# 4. 打包上传
tar -czf redis-template.tar.gz redis-template/
```

### 升级 Chart

```bash
# 下载新版本
helm pull bitnami/redis --version 18.20.0 --untar -d ./chart-new

# 替换目录
rm -rf chart/redis && mv chart-new/redis chart/

# 重新打包
tar -czf redis-template.tar.gz redis-template/
```

**关键：** 因为没有修改 Chart 代码，升级只需要替换目录，风险极低。

---

## 💡 最佳实践

1. **权限最小化**：只申请必需的权限，避免使用 `*`
2. **语义化版本**：使用 `v1.2.3` 格式
3. **清晰描述**：提供详细的工具描述
4. **测试多 namespace**：本地测试时模拟多个 namespace
5. **文档完善**：在 Chart.yaml 中添加 keywords 和 maintainers

## 🆘 常见问题

**Q: 为什么要禁用 Chart 内置的 RBAC？**
A: 平台需要统一管理权限，确保权限不溢出、自动同步、自动清理。

**Q: 如果我的 Chart 必须创建 CRD 怎么办？**
A: 联系平台管理员预先安装 CRD，或将 CRD 加入白名单。

**Q: 三类权限应该怎么选？**
A: 工具自身安装运行写 `toolNamespace`；需要控制业务应用写 `workloadNamespaces`；需要覆盖同环境工具/中间件/数据库 namespace 写 `environmentNamespaces`。

**Q: 如何调试我的模板？**
A: 使用 `helm template` 本地渲染，检查生成的 YAML 是否符合预期。
