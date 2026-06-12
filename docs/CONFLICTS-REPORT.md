# 文档冲突和不一致性报告

## 发现的问题

### 1. ⚠️ **严重冲突：两种不同的模板系统**

#### 问题描述
`service-template-spec.md` 和 `custom-template-guide.md` 描述了两种完全不同的模板系统：

**service-template-spec.md（旧系统？）：**
- 使用 `kind: ServiceTemplate` 格式
- 使用 `rbac.deploymentRole` 和 `rbac.envRole` 字段
- 使用 `lifecycle.install` / `lifecycle.onEnvNsAdded` 生命周期钩子
- 使用 Go 模板引擎渲染
- 存储在数据库中

**custom-template-guide.md（新系统 - BYO）：**
- 使用 `platform-manifest.yaml` 格式
- 使用 `permissions.toolNamespace`、`permissions.workloadNamespaces`、`permissions.environmentNamespaces` 字段
- 基于 Helm Chart
- 上传 tar.gz 压缩包

**Go 代码实现（platform_manifest.go）：**
- 支持三类 namespace 权限：`toolNamespace`、`workloadNamespaces`、`environmentNamespaces`
- 支持 `clusterResources` 集群级只读权限
- 支持 `observability`
- 支持 `variable_mapping`

#### 影响
用户会困惑：到底应该使用哪种格式？这是两个不同的系统吗？

---

### 2. ⚠️ **中等冲突：术语不一致**

#### 问题 2.1：权限类型命名
- **Go 代码**: `toolNamespace`、`workloadNamespaces`、`environmentNamespaces`
- **custom-template-guide.md**: `toolNamespace`、`workloadNamespaces`、`environmentNamespaces` ✅ 一致
- **service-template-spec.md**: 没有使用三类权限概念，仍是旧设计草案

#### 问题 2.2：字段命名
- **custom-template-guide.md**: `variable_mapping`
- **Go 代码**: `VariableMapping` ✅ 一致

---

### 3. ⚠️ **轻微冲突：文档引用不完整**

#### 问题 3.1：service-template-spec.md 没有提到 BYO 模板
`service-template-spec.md` 描述的是平台内置模板系统，但没有明确说明与 BYO（用户上传）模板的关系。

#### 问题 3.2：custom-template-guide.md 没有说明与内置模板的关系
`custom-template-guide.md` 专注于 BYO 模板，但没有说明平台是否也支持内置模板。

---

## 推荐的解决方案

### 方案 A：两种模板系统并存（推荐）

**假设：** 平台支持两种模板系统：
1. **内置模板**（service-template-spec.md）：平台管理员预定义的模板，使用生命周期钩子
2. **BYO 模板**（custom-template-guide.md）：用户上传的 Helm Chart + platform-manifest.yaml

**需要做的修改：**
1. 在 `service-template-spec.md` 开头添加说明，区分内置模板和 BYO 模板
2. 在 `custom-template-guide.md` 开头添加说明，明确这是 BYO 模板
3. 创建一个总览文档，说明两种模板系统的区别和使用场景

### 方案 B：统一为 BYO 模板系统

**假设：** 平台只支持 BYO 模板，`service-template-spec.md` 是旧设计。

**需要做的修改：**
1. 将 `service-template-spec.md` 标记为"已废弃"或"设计草案"
2. 更新所有文档引用，指向 `custom-template-guide.md`
3. 确认 Go 代码中没有实现 `lifecycle.install` 等旧系统的逻辑

---

## 详细冲突对比表

| 特性 | service-template-spec.md | custom-template-guide.md | Go 代码 (platform_manifest.go) |
|------|-------------------------|-------------------------|-------------------------------|
| 文件格式 | `kind: ServiceTemplate` | `platform-manifest.yaml` | `PlatformManifest` struct |
| 权限声明 | `rbac.deploymentRole` + `rbac.envRole` | `permissions.toolNamespace/workloadNamespaces/environmentNamespaces` | `Permissions.ToolNamespace/WorkloadNamespaces/EnvironmentNamespaces` |
| 生命周期 | `lifecycle.install` / `onEnvNsAdded` | 无（由 Helm 管理） | 无 |
| 模板引擎 | Go `text/template` | Helm | - |
| 存储方式 | 数据库 | 文件系统/S3 | - |
| 上传方式 | 平台管理员创建 | 用户上传 tar.gz | - |
| 可观测性 | 无 | `observability.dashboard_path` | `Observability` struct |
| 变量映射 | 无 | `variable_mapping` | `VariableMapping` array |

---

## 需要确认的问题

请回答以下问题，以便我进行正确的修复：

1. **平台是否支持两种模板系统？**
   - [ ] 是，同时支持内置模板和 BYO 模板
   - [ ] 否，只支持 BYO 模板（service-template-spec.md 是旧设计）

2. **如果支持两种系统，它们的关系是什么？**
   - [ ] 内置模板用于平台预装服务（如 ArgoCD、Prometheus）
   - [ ] BYO 模板用于用户自定义服务
   - [ ] 两者可以互相转换

3. **Go 代码中是否实现了 lifecycle 钩子？**
   - [ ] 是，已实现
   - [ ] 否，未实现（只实现了 platform-manifest.yaml）

4. **ServiceTemplate 数据库模型中的字段对应哪个系统？**
   - 查看 `internal/model/service_catalog.go` 中的 `ServiceTemplate` struct
   - 它有 `PlatformManifestJSON` 字段，说明支持 BYO 模板
   - 它也有 `WorkloadRolePolicy` 字段，可能对应内置模板

---

## 临时建议

在确认上述问题之前，我建议：

1. **在 service-template-spec.md 开头添加警告**：
   ```markdown
   > **注意：** 本文档描述的是平台内置模板系统（用于平台预装服务）。
   > 如果你想上传自己的 Helm Chart，请参考 [自定义模板开发指南](custom-template-guide.md)。
   ```

2. **在 custom-template-guide.md 开头添加说明**：
   ```markdown
   > **适用范围：** 本文档适用于用户上传自定义 Helm Chart（BYO - Bring Your Own）。
   > 平台内置模板的开发请参考 [服务模板规范](service-template-spec.md)。
   ```

3. **创建一个总览文档** `docs/design/template-systems-overview.md`，说明两种系统的区别。
