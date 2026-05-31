# Custom Prometheus Template Example

这是一个完整的自定义 Prometheus 模板示例，展示了如何为 PAAP 平台打包一个 Helm Chart。

## 目录结构

```
custom-prometheus-template/
├── chart/                          # 标准的 Helm Chart
│   ├── Chart.yaml                  # Chart 元数据
│   ├── values.yaml                 # 默认配置值
│   └── templates/                  # K8s 资源模板
│       ├── _helpers.tpl            # 模板辅助函数
│       ├── configmap.yaml          # Prometheus 配置
│       ├── deployment.yaml         # Prometheus Deployment
│       ├── service.yaml            # Prometheus Service
│       └── pvc.yaml                # 持久化存储
├── dashboards/                     # Grafana 面板
│   └── prometheus-overview.json    # 概览面板
├── preset-values.yaml              # 预设值（禁用内置 RBAC）
└── platform-manifest.yaml          # 平台元数据声明
```

## 关键特性

### 1. 动态权限管理

通过 `platform-manifest.yaml` 声明 `scope: environment-wide`，平台会自动：
- 在环境的每个 namespace 创建 Role + RoleBinding
- 环境新增 namespace 时自动同步权限
- 卸载时自动清理所有权限

### 2. 平台变量注入

通过 `variable_mapping` 将平台上下文传递给 Helm：

```yaml
variable_mapping:
  - platform_var: "env_namespaces"
    helm_var: "prometheus.scrapeNamespaces"
```

Prometheus 配置会自动生成针对所有 namespace 的抓取任务。

### 3. 自动监控集成

通过 `observability.dashboard_path` 声明 Grafana 面板路径，平台会在安装时自动导入面板。

### 4. 禁用内置 RBAC

通过 `preset-values.yaml` 禁用 Chart 内置的 RBAC 创建：

```yaml
rbac:
  create: false
serviceAccount:
  create: false
```

平台会统一管理 ServiceAccount 和权限。

## 打包上传

```bash
# 1. 打包
tar -czf custom-prometheus.tar.gz custom-prometheus-template/

# 2. 上传到 PAAP 平台
# 通过 UI 或 API 上传 custom-prometheus.tar.gz
```

## 本地测试

```bash
# 准备测试 values
cat > test-values.yaml <<EOF
global:
  namespace: test-ns
  serviceAccountName: test-sa
  env: dev

prometheus:
  scrapeNamespaces: "ns1,ns2,ns3"
EOF

# 渲染模板
helm template my-prometheus ./chart \
  -f preset-values.yaml \
  -f test-values.yaml

# 检查输出的 YAML
```

## 验证清单

安装前请确认：

- [ ] `platform-manifest.yaml` 格式正确
- [ ] `permissions.scope` 设置为 `environment-wide`
- [ ] `permissions.rules` 只包含必需的权限
- [ ] `preset-values.yaml` 禁用了 `rbac.create` 和 `serviceAccount.create`
- [ ] Chart 中没有 ClusterRole、CRD 等集群级资源
- [ ] 所有资源都使用 `{{ .Values.global.namespace }}` 指定 namespace
- [ ] Deployment 使用 `{{ .Values.global.serviceAccountName }}` 指定 ServiceAccount
- [ ] Grafana 面板 JSON 格式正确

## 安装后效果

用户在环境中安装此模板后：

1. **自动创建权限**：平台在环境的所有 namespace 创建 Role + RoleBinding
2. **自动配置抓取**：Prometheus 自动抓取所有 namespace 的指标
3. **自动导入面板**：Grafana 自动导入 `prometheus-overview.json` 面板
4. **动态权限同步**：环境新增 namespace 时，权限自动同步

## 参考文档

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [服务模板规范](../../design/service-template-spec.md)
- [Helm Chart 开发指南](https://helm.sh/docs/chart_template_guide/)
