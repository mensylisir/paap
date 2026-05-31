# 内置模板标准化 - 实施指南

## 📋 当前状态总结

### ✅ 已完成
1. **文档完善**
   - ✅ 创建了完整的模板开发指南
   - ✅ 创建了零改动转换第三方 Chart 的指南
   - ✅ 创建了 2 个标准格式示例（自定义 Prometheus + Bitnami Redis）
   - ✅ 文档诚实描述了当前状态（用户上传模板已实现，内置模板迁移中）

2. **模板文件**
   - ✅ `data/charts/` 下有 12 个打包好的模板（已包含 platform-manifest.yaml）

3. **基础设施脚本**
   - ✅ 创建了 MinIO 部署配置 (`deploy/k8s/minio.yaml`)
   - ✅ 创建了模板初始化 Job (`deploy/k8s/init-templates.yaml`)
   - ✅ 更新了部署脚本 (`deploy/k8s/deploy.sh`)
   - ✅ 创建了模板解压脚本 (`scripts/extract-built-in-templates.sh`)

### ⚠️ 待完成
1. **解压模板到 docs/examples**
2. **更新 Server 代码**（从 MinIO 加载模板）
3. **测试完整部署流程**

---

## 🚀 立即执行步骤

### 步骤 1：解压内置模板到 docs/examples

```bash
cd /home/mensyli1/Documents/workspace/paap
chmod +x scripts/extract-built-in-templates.sh
./scripts/extract-built-in-templates.sh
```

**预期结果：**
```
docs/examples/built-in-templates/
├── argocd/
│   ├── chart/
│   ├── platform-manifest.yaml
│   ├── preset-values.yaml
│   └── README.md
├── redis/
├── postgresql/
├── mysql/
├── mongodb/
├── rabbitmq/
├── kafka/
├── minio/
├── harbor/
├── jenkins/
├── loki/
└── monitor/  (应该重命名为 prometheus)
```

### 步骤 2：更新 Server 代码

需要修改 `internal/service/seed_templates.go`，从 MinIO 加载模板而不是硬编码。

**关键修改：**

```go
// 删除旧的硬编码模板定义
// 改为从 MinIO 加载

func SeedServiceTemplates() {
    var count int64
    database.DB.Model(&model.ServiceTemplate{}).Count(&count)
    if count > 0 {
        log.Println("Templates already seeded, skipping")
        return
    }

    log.Println("Seeding service templates from MinIO...")

    // 内置模板列表
    builtInTemplates := []string{
        "argocd", "redis", "postgresql", "mysql",
        "mongodb", "rabbitmq", "kafka", "minio",
        "harbor", "jenkins", "loki", "monitor",
    }

    for _, templateType := range builtInTemplates {
        if err := seedTemplateFromMinIO(templateType); err != nil {
            log.Printf("Failed to seed template %s: %v", templateType, err)
        } else {
            log.Printf("✓ Seeded template: %s", templateType)
        }
    }
}

func seedTemplateFromMinIO(templateType string) error {
    // 1. 从 MinIO 下载模板
    // 2. 解析 platform-manifest.yaml
    // 3. 创建数据库记录
    // 详见 docs/BUILT-IN-TEMPLATES-SETUP.md
}
```

**完整代码参考：** `docs/BUILT-IN-TEMPLATES-SETUP.md` 中的"步骤 5"

### 步骤 3：测试部署

```bash
cd deploy/k8s

# 设置 kind 集群名称
export KIND_CLUSTER=paap-dev

# 执行部署
./deploy.sh
```

**部署流程：**
1. 创建 namespace
2. 安装 CRD
3. 部署 PostgreSQL
4. 部署 MinIO
5. 上传模板到 MinIO（通过 init-templates Job）
6. 构建并加载镜像
7. 部署 Operator
8. 部署 Server（启动时从 MinIO 加载模板到数据库）

**验证：**
```bash
# 1. 检查 MinIO 中的模板
kubectl port-forward -n paap-storage svc/minio 9000:9000 &
# 访问 http://localhost:9000
# 登录：minioadmin / minioadmin123
# 检查 paap-charts/templates/ 下是否有所有模板

# 2. 检查数据库中的模板
kubectl exec -it -n paap-system deployment/paap-server -- \
  psql -h postgres -U paap -d paap -c \
  "SELECT type, name, is_custom, s3_key FROM service_templates;"

# 3. 检查 Server 日志
kubectl logs -n paap-system deployment/paap-server --tail=50
```

---

## 📝 详细文档索引

### 核心文档
1. **[BUILT-IN-TEMPLATES-SETUP.md](BUILT-IN-TEMPLATES-SETUP.md)**
   - 完整的实施方案
   - 包含所有脚本和配置
   - Server 代码修改示例

2. **[custom-template-guide.md](design/custom-template-guide.md)**
   - 标准模板格式规范
   - 零改动转换第三方 Chart

3. **[template-system-overview.md](design/template-system-overview.md)**
   - 模板系统架构
   - 当前实现状态

4. **[migration-roadmap.md](design/migration-roadmap.md)**
   - 内置模板迁移计划
   - 从旧格式迁移到标准格式

### 示例模板
1. **[custom-prometheus-template](examples/custom-prometheus-template/)**
   - 从零编写的完整示例
   - 展示 environment-wide 权限

2. **[bitnami-redis-template](examples/bitnami-redis-template/)**
   - 零改动转换第三方 Chart
   - 展示 tool-only 权限

3. **[built-in-templates](examples/built-in-templates/)** (待创建)
   - 所有内置模板的源码
   - 标准格式

---

## 🎯 最终效果

### 部署后的架构

```
┌─────────────────────────────────────────┐
│  MinIO (paap-storage namespace)         │
│  ├── paap-charts/templates/             │
│  │   ├── argocd.tar.gz                  │
│  │   ├── redis.tar.gz                   │
│  │   └── ...                             │
└─────────────────────────────────────────┘
                 ↓ (Server 启动时加载)
┌─────────────────────────────────────────┐
│  PostgreSQL (paap-system namespace)     │
│  ├── service_templates 表                │
│  │   ├── type: argocd                   │
│  │   │   s3_key: templates/argocd.tar.gz│
│  │   ├── type: redis                    │
│  │   │   s3_key: templates/redis.tar.gz │
│  │   └── ...                             │
└─────────────────────────────────────────┘
                 ↓ (用户安装时)
┌─────────────────────────────────────────┐
│  PAAP Server                             │
│  1. 从数据库读取模板信息                 │
│  2. 从 MinIO 下载 tar.gz                │
│  3. 解析 platform-manifest.yaml         │
│  4. 创建 ServiceInstance CR              │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  PAAP Operator                           │
│  1. 监听 ServiceInstance CR              │
│  2. 创建 ServiceAccount                  │
│  3. 创建 Role + RoleBinding              │
│  4. 安装 Helm Chart                      │
└─────────────────────────────────────────┘
```

### 用户体验

1. **平台管理员**
   - 部署时自动上传所有内置模板到 MinIO
   - 无需手动配置

2. **普通用户**
   - 在 UI 中看到所有可用模板
   - 点击安装，自动从 MinIO 拉取
   - 权限自动配置，动态同步

3. **开发者**
   - 可以上传自定义模板
   - 使用相同的标准格式
   - 与内置模板无差异

---

## ⚠️ 注意事项

### 1. 模板命名
- `data/charts/monitor.tar.gz` 应该重命名为 `prometheus.tar.gz`
- 或者在代码中映射：`monitor` → `prometheus`

### 2. MinIO 访问
- Server 需要能访问 `minio.paap-storage.svc.cluster.local:9000`
- 确保网络策略允许跨 namespace 访问

### 3. 凭证管理
- 当前使用硬编码凭证（minioadmin/minioadmin123）
- 生产环境应该使用 Secret 管理

### 4. 模板验证
- 上传前应该验证所有模板的格式正确性
- 确保每个模板都有 `platform-manifest.yaml`

### 5. 存储空间
- MinIO PVC 默认 10Gi
- 根据模板数量调整大小

---

## 🔄 后续优化

### 短期（1-2 周）
1. ✅ 完成基础设施部署（MinIO + 初始化）
2. ✅ 更新 Server 代码从 MinIO 加载模板
3. ✅ 测试完整部署流程

### 中期（1-2 个月）
4. ⚠️ 将 `raw-yaml` 格式的模板转换为标准格式
   - ArgoCD、Tekton、Prometheus、Loki
5. ⚠️ 验证所有模板的功能正确性

### 长期（3-6 个月）
6. ⚠️ 移除旧的 `SeedServiceTemplates()` 硬编码逻辑
7. ⚠️ 统一所有模板使用标准格式
8. ⚠️ 更新文档，移除"迁移中"标记

---

## 📞 问题排查

### 问题 1：MinIO 无法启动
```bash
# 检查 PVC
kubectl get pvc -n paap-storage

# 检查 Pod 日志
kubectl logs -n paap-storage deployment/minio
```

### 问题 2：模板初始化失败
```bash
# 检查 Job 状态
kubectl get job -n paap-system

# 查看 Job 日志
kubectl logs -n paap-system job/init-templates

# 检查模板文件是否复制到 kind 节点
docker exec paap-dev-control-plane ls -la /tmp/paap-charts
```

### 问题 3：Server 无法加载模板
```bash
# 检查 Server 日志
kubectl logs -n paap-system deployment/paap-server

# 检查 MinIO 连接
kubectl exec -n paap-system deployment/paap-server -- \
  curl -v http://minio.paap-storage.svc.cluster.local:9000/minio/health/live
```

### 问题 4：模板安装失败
```bash
# 检查 ServiceInstance CR
kubectl get serviceinstance -n paap-app-1

# 检查 Operator 日志
kubectl logs -n paap-system deployment/paap-operator

# 检查 MinIO 中的模板
kubectl exec -n paap-storage deployment/minio -- \
  mc ls local/paap-charts/templates/
```

---

## ✅ 完成标准

当满足以下条件时，内置模板标准化视为完成：

1. ✅ 所有内置模板解压到 `docs/examples/built-in-templates/`
2. ✅ MinIO 成功部署到 `paap-storage` namespace
3. ✅ 模板初始化 Job 成功上传所有模板到 MinIO
4. ✅ Server 启动时成功从 MinIO 加载模板到数据库
5. ✅ 用户可以在 UI 中看到并安装所有模板
6. ✅ 安装的模板功能正常，权限正确

---

## 🎉 总结

**已准备好的内容：**
- ✅ 完整的文档体系
- ✅ MinIO 部署配置
- ✅ 模板初始化脚本
- ✅ 更新的部署脚本
- ✅ 模板解压脚本

**需要执行的操作：**
1. 运行 `./scripts/extract-built-in-templates.sh`
2. 更新 `internal/service/seed_templates.go`
3. 运行 `./deploy/k8s/deploy.sh` 测试

**预计工作量：** 2-4 小时

**参考文档：** `docs/BUILT-IN-TEMPLATES-SETUP.md`
