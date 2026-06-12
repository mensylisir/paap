# Bitnami Redis 模板（零改动转换示例）

这是一个真实的第三方 Helm Chart 转换示例，展示如何在**不修改任何 Chart 代码**的情况下，将 Bitnami Redis 集成到 PAAP 平台。

## 核心理念

**不要修改 `chart/` 目录下的任何文件**，只通过配置覆盖和平台元数据来实现集成。

## 目录结构

```
bitnami-redis-template/
├── chart/                          # Bitnami 官方 Redis Chart（未修改）
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── dashboards/                     # Grafana 面板
│   └── redis-overview.json
├── preset-values.yaml              # 配置覆盖（禁用内置 RBAC）
└── platform-manifest.yaml          # 平台元数据
```

## 获取官方 Chart

```bash
# 1. 添加 Bitnami 仓库
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# 2. 下载 Redis Chart（不要修改任何文件）
helm pull bitnami/redis --version 18.19.0 --untar -d ./chart

# 3. 验证下载
ls -la chart/redis/
```

## 转换要点

### 1. 禁用内置 RBAC

Bitnami Redis Chart 提供了标准的 RBAC 开关，我们通过 `preset-values.yaml` 禁用它：

```yaml
rbac:
  create: false

serviceAccount:
  create: false
  name: ""  # 平台会注入
```

### 2. 权限声明

Redis 只需要在自己的 namespace 内运行，不声明跨 namespace 权限。

```yaml
permissions:
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods", "services", "configmaps", "secrets", "persistentvolumeclaims", "serviceaccounts"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
      - apiGroups: ["apps"]
        resources: ["statefulsets", "replicasets"]
        verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### 3. 参数映射

通过 `variable_mapping` 将平台生成的密码传递给 Redis：

```yaml
variable_mapping:
  - platform_var: "generated_password"
    helm_var: "auth.password"
```

## 安装效果

用户在环境中安装此模板后：

1. **自动创建 ServiceAccount**：平台在环境的主 namespace 创建 SA
2. **自动注入密码**：平台生成随机密码并注入到 Redis
3. **自动导入面板**：Grafana 自动导入 Redis 监控面板
4. **权限隔离**：Redis 只能访问自己所在的 namespace

## 升级 Chart

当 Bitnami 发布新版本时：

```bash
# 1. 下载新版本
helm pull bitnami/redis --version 18.20.0 --untar -d ./chart-new

# 2. 替换 chart/ 目录
rm -rf chart/redis
mv chart-new/redis chart/

# 3. 检查 preset-values.yaml 是否需要更新
diff chart-old/redis/values.yaml chart/redis/values.yaml

# 4. 重新打包
tar -czf bitnami-redis-template.tar.gz bitnami-redis-template/
```

**关键：** 因为没有修改 Chart 代码，升级只需要替换目录，风险极低。

## 本地测试

```bash
# 准备测试 values
cat > test-values.yaml <<EOF
global:
  namespace: test-ns
  serviceAccountName: test-sa

auth:
  password: "test-password-123"
EOF

# 渲染模板
helm template my-redis ./chart/redis \
  -f preset-values.yaml \
  -f test-values.yaml

# 检查输出：
# - 没有 ClusterRole/ClusterRoleBinding
# - 没有 ServiceAccount 创建
# - 所有资源都在 test-ns namespace
```

## 验证清单

- [x] **未修改** `chart/` 目录下的任何文件
- [x] `preset-values.yaml` 禁用了 `rbac.create`
- [x] `preset-values.yaml` 禁用了 `serviceAccount.create`
- [x] `platform-manifest.yaml` 只声明了 `toolNamespace` 权限
- [x] 使用 `variable_mapping` 传递密码
- [x] 本地测试通过，无集群级资源

## 与自定义 Chart 的对比

| 特性 | 自定义 Chart | 第三方 Chart（零改动） |
|------|-------------|---------------------|
| 开发成本 | 高（从零编写） | 低（只写配置文件） |
| 维护成本 | 高（需要持续维护） | 低（跟随官方升级） |
| 功能完整性 | 取决于开发能力 | 高（官方维护） |
| 社区支持 | 无 | 有（官方文档、Issue） |
| 升级风险 | 高（需要手动迁移） | 低（替换目录即可） |

## 参考文档

- [Bitnami Redis Chart](https://github.com/bitnami/charts/tree/main/bitnami/redis)
- [自定义模板开发指南](../../design/custom-template-guide.md)
- [零改动转换章节](../../design/custom-template-guide.md#10-零改动转换第三方-helm-chart)
