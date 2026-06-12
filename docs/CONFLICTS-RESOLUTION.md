# 文档冲突修复方案

## 结论

经过代码审查，确认：

### ✅ **实际实现：BYO 模板系统（platform-manifest.yaml）**

**证据：**
1. `internal/model/platform_manifest.go` - 完整实现了 `PlatformManifest` 结构
2. `internal/handler/template.go` - 所有内置模板都使用 `PlatformManifestJSON` 字段
3. `api/v1/serviceinstance_types.go` - ServiceInstance CRD 使用 `WorkloadRole`（对应 platform-manifest 的 `permissions.rules`）
4. 代码中**没有** `lifecycle.install` / `onEnvNsAdded` 的实现

### ❌ **未实现：lifecycle 钩子系统**

`service-template-spec.md` 中描述的 `lifecycle.install` / `onEnvNsAdded` 等钩子**并未在代码中实现**。

---

## 修复方案

### 方案：将 service-template-spec.md 标记为"设计草案"，统一到 BYO 系统

**理由：**
- 代码实现与 `custom-template-guide.md` 一致
- 所有内置模板都使用 `PlatformManifestJSON`
- 没有 lifecycle 钩子的实现

**需要修改的文档：**
1. `service-template-spec.md` - 添加"设计草案"标记，说明实际实现使用 platform-manifest.yaml
2. `custom-template-guide.md` - 添加说明，明确这是平台的标准模板格式
3. 创建 `template-system-overview.md` - 统一说明模板系统架构

---

## 实际的模板系统架构

```
PAAP 模板系统
├── 内置模板（平台预装）
│   ├── ArgoCD (PlatformManifestJSON)
│   ├── Jenkins (PlatformManifestJSON)
│   ├── Prometheus (PlatformManifestJSON)
│   └── ...
│
└── 用户上传模板（BYO）
    ├── 上传 tar.gz 包
    ├── 包含 platform-manifest.yaml
    ├── 包含 chart/ 目录（Helm Chart）
    └── 可选 preset-values.yaml、dashboards/

两者使用相同的格式：platform-manifest.yaml
```

---

## 字段映射关系

| service-template-spec.md（未实现） | 实际实现（platform-manifest.yaml） | ServiceInstance CRD |
|-----------------------------------|----------------------------------|---------------------|
| `rbac.deploymentRole` | `permissions.toolNamespace.rules` | `ToolNamespaceRole` |
| `rbac.envRole` | `permissions.workloadNamespaces.rules` / `permissions.environmentNamespaces.rules` | `WorkloadRole` / `EnvironmentRole` |
| `lifecycle.install` | 由 Helm Chart 管理 | - |
| `lifecycle.onEnvNsAdded` | 由 Operator 自动创建 RoleBinding | - |

---

## 修复清单

- [ ] 更新 `service-template-spec.md` 开头，标记为"设计草案"
- [ ] 在 `service-template-spec.md` 中添加指向 `custom-template-guide.md` 的链接
- [ ] 更新 `custom-template-guide.md` 开头，说明这是平台的标准模板格式
- [ ] 创建 `template-system-overview.md` 统一说明
- [ ] 更新 `README.md` 中的文档链接顺序
