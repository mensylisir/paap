# 内置模板迁移路线图

## 🎯 目标

将所有内置模板迁移到标准格式：`Helm Chart + platform-manifest.yaml + preset-values.yaml`

---

## 📊 当前状态

### 模板清单

| 模板 | 类型 | 当前方式 | 迁移状态 | 优先级 | 预计工时 |
|------|------|---------|---------|--------|---------|
| ArgoCD | tool | raw-yaml | 待迁移 | P0 | 4h |
| Tekton | tool | raw-yaml | 待迁移 | P1 | 4h |
| Prometheus | tool | raw-yaml | 待迁移 | P0 | 4h |
| Loki | tool | raw-yaml | 待迁移 | P1 | 3h |
| Docker Registry | tool | raw-yaml | 待迁移 | P2 | 2h |
| Harbor | tool | helm (直接引用) | 待迁移 | P1 | 2h |
| Jenkins | tool | helm (直接引用) | 待迁移 | P2 | 2h |
| PostgreSQL | infra | helm (直接引用) | 待迁移 | P1 | 2h |
| MySQL | infra | helm (直接引用) | 待迁移 | P1 | 2h |
| MongoDB | infra | helm (直接引用) | 待迁移 | P2 | 2h |
| Redis | infra | helm (直接引用) | 待迁移 | P1 | 2h |
| RabbitMQ | infra | helm (直接引用) | 待迁移 | P2 | 2h |
| Kafka | infra | helm (直接引用) | 待迁移 | P2 | 3h |
| MinIO | infra | helm (直接引用) | 待迁移 | P2 | 2h |

**总计：** 14 个模板，约 38 小时工作量

### 优先级说明

- **P0（高）**：核心工具，使用频率高（ArgoCD、Prometheus）
- **P1（中）**：常用工具和基础设施（Tekton、PostgreSQL、Redis）
- **P2（低）**：可选工具（Jenkins、MongoDB、Kafka）

---

## 🔄 迁移步骤

### 方式 1：转换 raw-yaml 模板

**适用于：** ArgoCD、Tekton、Prometheus、Loki、Docker Registry

**步骤：**

1. **创建 Helm Chart 结构**
   ```bash
   mkdir -p templates/argocd/chart/templates
   cd templates/argocd
   ```

2. **拆分 YAML 为 Helm 模板**
   ```
   当前：一个大的 RawYamlTemplate 字符串
   目标：
   ├── chart/
   │   ├── Chart.yaml
   │   ├── values.yaml
   │   └── templates/
   │       ├── deployment.yaml
   │       ├── service.yaml
   │       ├── configmap.yaml
   │       └── rbac.yaml (移除，由平台管理)
   ```

3. **创建 platform-manifest.yaml**
   ```yaml
   name: "ArgoCD"
   version: "v2.10.0"
   description: "GitOps 持续部署工具"
   
   permissions:
     scope: "environment-wide"
     rules:
       - apiGroups: ["", "apps"]
         resources: ["deployments", "services"]
         verbs: ["*"]
   ```

4. **创建 preset-values.yaml**
   ```yaml
   # 禁用内置 RBAC（由平台管理）
   rbac:
     create: false
   
   serviceAccount:
     create: false
     name: ""
   ```

5. **打包并上传到 S3**
   ```bash
   tar -czf argocd-template.tar.gz argocd/
   # 上传到 S3: paap-charts/templates/argocd.tar.gz
   ```

6. **更新数据库记录**
   ```go
   // 从
   {
       Type: "deploy",
       Installer: "raw-yaml",
       RawYamlTemplate: "...",
       WorkloadRolePolicy: "[...]",
   }
   
   // 到
   {
       Type: "deploy",
       PlatformManifestJSON: "{...}",
       S3Bucket: "paap-charts",
       S3Key: "templates/argocd.tar.gz",
       PresetValues: "...",
   }
   ```

### 方式 2：转换 helm 直接引用

**适用于：** PostgreSQL、MySQL、Redis、RabbitMQ 等

**步骤：**

1. **下载官方 Chart**
   ```bash
   helm repo add bitnami https://charts.bitnami.com/bitnami
   helm pull bitnami/redis --version 18.19.0 --untar -d ./chart
   ```

2. **创建 platform-manifest.yaml**
   ```yaml
   name: "Redis"
   version: "7.2.4"
   description: "Bitnami Redis 缓存服务"
   
   permissions:
     scope: "tool-only"  # Redis 不需要跨 namespace 权限
   
   observability:
     metrics:
       port: 9121
       path: "/metrics"
   
   variable_mapping:
     - platform_var: "generated_password"
       helm_var: "auth.password"
   ```

3. **创建 preset-values.yaml**
   ```yaml
   rbac:
     create: false
   
   serviceAccount:
     create: false
     name: ""
   
   auth:
     enabled: true
     password: ""  # 平台注入
   
   metrics:
     enabled: true
   ```

4. **打包并上传**
   ```bash
   tar -czf redis-template.tar.gz redis-template/
   # 上传到 S3
   ```

5. **更新数据库记录**
   ```go
   // 从
   {
       Type: "redis",
       Installer: "helm",
       ChartRepo: "https://charts.bitnami.com/bitnami",
       ChartName: "bitnami/redis",
       ChartVersion: "18.19.0",
   }
   
   // 到
   {
       Type: "redis",
       PlatformManifestJSON: "{...}",
       S3Bucket: "paap-charts",
       S3Key: "templates/redis.tar.gz",
       PresetValues: "...",
   }
   ```

---

## 📅 时间表

### Q2 2026（4-6月）

**目标：** 迁移 P0 模板

- [ ] Week 1-2: ArgoCD（4h）
- [ ] Week 3-4: Prometheus（4h）
- [ ] Week 5: 测试和验证

**交付物：**
- 2 个标准格式的模板包
- 迁移脚本和文档
- 测试报告

### Q3 2026（7-9月）

**目标：** 迁移 P1 模板

- [ ] Week 1: Tekton（4h）
- [ ] Week 2: Loki（3h）
- [ ] Week 3: Harbor（2h）
- [ ] Week 4-5: PostgreSQL、MySQL、Redis（6h）
- [ ] Week 6: 测试和验证

**交付物：**
- 6 个标准格式的模板包
- 更新的迁移脚本

### Q4 2026（10-12月）

**目标：** 迁移 P2 模板 + 清理旧代码

- [ ] Week 1-4: 迁移剩余 6 个模板（14h）
- [ ] Week 5-6: 移除旧代码
  - 删除 `Installer`、`RawYamlTemplate`、`ChartRepo` 等字段
  - 简化安装逻辑
  - 更新文档
- [ ] Week 7-8: 全面测试

**交付物：**
- 所有模板迁移完成
- 代码库简化
- 文档更新

---

## 🔧 技术实现

### 迁移脚本

```go
// cmd/migrate-templates/main.go
package main

import (
    "fmt"
    "log"
    
    "paap/internal/database"
    "paap/internal/model"
    "paap/internal/service"
)

func main() {
    database.Connect()
    
    // 迁移 ArgoCD
    if err := migrateArgoCD(); err != nil {
        log.Fatal(err)
    }
    
    // 迁移 Prometheus
    if err := migratePrometheus(); err != nil {
        log.Fatal(err)
    }
    
    // ... 其他模板
    
    fmt.Println("Migration completed!")
}

func migrateArgoCD() error {
    var template model.ServiceTemplate
    if err := database.DB.Where("type = ?", "deploy").First(&template).Error; err != nil {
        return err
    }
    
    // 检查是否已迁移
    if template.PlatformManifestJSON != "" {
        fmt.Println("ArgoCD already migrated")
        return nil
    }
    
    // 1. 创建 Helm Chart（从 RawYamlTemplate 拆分）
    chart := service.ConvertRawYamlToHelmChart(template.RawYamlTemplate)
    
    // 2. 创建 platform-manifest.yaml
    manifest := model.PlatformManifest{
        Name:    "ArgoCD",
        Version: "v2.10.0",
        Permissions: model.PermissionsSpec{
            Scope: model.PermissionScopeEnvironmentWide,
            Rules: convertWorkloadRolePolicy(template.WorkloadRolePolicy),
        },
    }
    
    // 3. 创建 preset-values.yaml
    presetValues := `
rbac:
  create: false
serviceAccount:
  create: false
  name: ""
`
    
    // 4. 打包并上传到 S3
    s3Key, err := service.PackageAndUploadTemplate("argocd", chart, manifest, presetValues)
    if err != nil {
        return err
    }
    
    // 5. 更新数据库
    manifestJSON, _ := json.Marshal(manifest)
    template.PlatformManifestJSON = string(manifestJSON)
    template.S3Bucket = "paap-charts"
    template.S3Key = s3Key
    template.PresetValues = presetValues
    
    return database.DB.Save(&template).Error
}
```

### 向后兼容逻辑

```go
// internal/service/installer.go
func InstallService(template *model.ServiceTemplate, env *model.Environment) error {
    // 优先使用标准格式
    if template.PlatformManifestJSON != "" {
        return installFromPlatformManifest(template, env)
    }
    
    // 向后兼容：旧格式
    log.Warn("Using legacy template format for %s, please migrate", template.Type)
    
    switch template.Installer {
    case "raw-yaml":
        return installFromRawYaml(template, env)
    case "helm":
        return installFromHelmRepo(template, env)
    default:
        return fmt.Errorf("unknown installer: %s", template.Installer)
    }
}
```

---

## ✅ 验证清单

每个模板迁移后需要验证：

- [ ] 模板包结构正确
  - [ ] 包含 `chart/` 目录
  - [ ] 包含 `platform-manifest.yaml`
  - [ ] 包含 `preset-values.yaml`（如果需要）

- [ ] platform-manifest.yaml 格式正确
  - [ ] `name` 和 `version` 字段存在
  - [ ] `permissions.scope` 正确设置
  - [ ] `permissions.rules` 遵循最小权限原则

- [ ] Chart 不包含违规资源
  - [ ] 没有 ClusterRole/ClusterRoleBinding
  - [ ] 没有 CRD
  - [ ] 所有资源使用 `{{ .Values.global.namespace }}`

- [ ] 功能测试
  - [ ] 安装成功
  - [ ] 权限正确（能访问应该访问的资源，不能访问不应该访问的资源）
  - [ ] 环境新增 namespace 时权限自动同步
  - [ ] 卸载干净（无残留资源）

- [ ] 数据库记录正确
  - [ ] `PlatformManifestJSON` 字段已填充
  - [ ] `S3Bucket` 和 `S3Key` 正确
  - [ ] `PresetValues` 已填充（如果有）

---

## 📝 文档更新

迁移完成后需要更新的文档：

1. **template-system-overview.md**
   - 移除"迁移中"标记
   - 更新为"所有模板统一使用标准格式"

2. **custom-template-guide.md**
   - 移除"内置模板正在迁移"的说明
   - 更新为"所有模板使用相同格式"

3. **README.md**
   - 更新文档链接
   - 移除迁移相关说明

4. **本文档（migration-roadmap.md）**
   - 标记为"已完成"
   - 归档到 `docs/archive/`

---

## 🎉 迁移完成标准

当满足以下条件时，迁移视为完成：

1. ✅ 所有 14 个内置模板已迁移到标准格式
2. ✅ 所有模板通过验证清单
3. ✅ 旧代码已移除（`Installer`、`RawYamlTemplate`、`ChartRepo` 等字段）
4. ✅ 安装逻辑已简化（只保留 `installFromPlatformManifest`）
5. ✅ 文档已更新
6. ✅ 全面测试通过

---

## 📞 联系方式

如有问题或需要帮助，请联系：

- 技术负责人：[姓名]
- 邮箱：[email]
- Slack 频道：#paap-template-migration
