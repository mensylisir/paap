# PAAP 项目问题记录

> 自动扫描时间: 2026-06-24 (更新: 2026-06-25)
> 范围: 全代码库 + 运行环境（cluster12 + cluster18 + kind）

---

## 一、构建与编译问题

### 1.1 前端 TypeScript 类型检查
- **状态**: ✅ 当前 clean（`vue-tsc -b` 无报错）
- **位置**: `frontend/`
- **建议**: 保持 CI 中运行 `vue-tsc -b` 作为门禁

### 1.2 Go vet
- **状态**: ✅ 当前 clean（`go vet ./...` 无输出）
- **位置**: `internal/`
- **建议**: 保持 `make lint`（`go vet`）在 CI 中运行

### 1.3 后端测试
- **状态**: ✅ 全部通过
- **位置**: `internal/controller`, `internal/handler`, `internal/helm`, `internal/k8s`, `internal/middleware`, `internal/model`, `internal/service`
- **建议**: 增加更多测试覆盖

---

## 二、安全相关

### 2.1 默认用户密码硬编码
- **位置**: `internal/handler/auth.go:158`
- **问题**: 普通用户 `user` 的密码 `user123` 硬编码在代码中。管理员密码虽然随机生成但迁移文件 `migration/20260624_001_update_platform_admin_password.sql` 将其改写为固定值 `Def@u1tpwd`
- **严重性**: 中
- **建议**:
  - 管理员首次登录强制改密
  - 从环境变量或 Secret 读取初始密码

### 2.2 JWT Secret 无校验
- **位置**: `internal/handler/auth.go:181-183`
- **问题**: JWT Secret 从环境变量读取但未校验强度，空值或弱密钥可能导致 token 伪造
- **严重性**: 高
- **建议**: 启动时校验 JWT Secret 长度 >= 32 字符

### 2.3 密码 Bcrypt 成本固定
- **位置**: `internal/handler/auth.go:29`
- **问题**: `bcrypt.DefaultCost` 可能过低（当前为 10）
- **严重性**: 低
- **建议**: 使用 `bcrypt.MinCost` 以上的值，推荐 12+

### 2.4 无 Rate Limiting / 暴力破解防护
- **问题**: 登录接口无频率限制，可能被暴力破解
- **严重性**: 高
- **建议**: 添加登录失败次数限制 + 验证码

---

## 三、部署与基础设施问题

### 3.1 Docker Hub 镜像拉取依赖
- **问题**: PAAP 部署依赖大量 Docker Hub 镜像（kpack, keycloak, postgres, minio, etc.），在中国大陆或离线环境拉取困难
- **严重性**: 高
- **建议**:
  - 所有镜像预先推送到私有 Harbor
  - 部署脚本增加镜像替换逻辑
  - containerd 配置 local mirror

### 3.2 kpack Kubernetes 版本兼容性
- **位置**: `deploy/k8s/kpack-v0.17.0.yaml`
- **问题**: kpack v0.17.0 要求 K8s >= 1.28，但 cluster12/cluster18 为 1.24.9；需设置 `KUBERNETES_MIN_VERSION=1.24.9` 绕过
- **严重性**: 中
- **建议**: 使用与集群版本匹配的 kpack 版本，或升级集群

### 3.7 kpack 定位评估：默认安装还是可选 CI 组件？

#### 背景
- 当前：`deploy/k8s/deploy.sh` 步骤 4 **强制安装** kpack v0.17.0，注释写 "source 组件默认走 Buildpacks/kpack，必须先有 CRD/controller/webhook"
- `internal/k8s/kpack.go`：629 行深度集成 — SA 创建、Docker registry 配置、CA 注入、Builder/ClusterStore/ClusterStack 管理
- 组件模型（`component.go`）已有 `SourceRepoURL` / `SourceMirrorRepoURL` / `SourceBranch` 字段
- 前端 deploy tab 支持 image delivery / source delivery 两种模式切换
- kpack 未注册为 ServiceCatalog 条目，是硬编码基础设施
- 依赖大量重型 Paketo Buildpacks 镜像（Java ~1.5GB, NodeJS ~500MB, Go ~400MB, Python ~500MB）

#### 评估

**kpack 不是 CI 组件。** 它与 Jenkins 有本质区别：

| 维度 | Jenkins (CI) | kpack |
|------|-------------|-------|
| 角色 | 流水线编排、多步骤任务 | 源码→容器镜像的编译引擎 |
| 用户交互 | 用户编写 pipeline、触发构建 | 无用户直接交互，PAAP 内部调用 |
| 资源模型 | 每个环境独立部署 | Cluster 级 CRD（ClusterStore, ClusterStack, ClusterLifecycle），所有环境共享 |
| 对 PAAP 的价值 | 可选集成 | source delivery 模式的**必要基础设施** |

**但也不应该保持"强制默认安装"。**

| 论证 | 说明 |
|------|------|
| ❌ 资源开销 | kpack + buildpack 镜像合计数 GB。只使用 image delivery 的用户完全不需要 |
| ❌ K8s 版本门槛 | v0.17.0 要求 ≥1.28（当前集群 1.24.9）。kpack 安装失败不应阻塞 PAAP 核心 |
| ❌ 部署耦合 | deploy.sh 顺序依赖：kpack 失败 → deploy 中断 → PostgreSQL/MinIO 都装不上 |
| ❌ 升级负担 | kpack 升级需要全量更新 deploy 脚本、重新导入镜像、重启 controller |
| ✅ Source Delivery 是核心功能 | 不是"可选加装"。组件模型/前端 UI 已经完整支持，不可让体验断链 |

#### 推荐方案：平台级基础设施（Platform Infrastructure）

kpack 应该是**平台基础设施**而非环境级服务，但不在 deploy.sh 中硬编码。

```
方案            install.sh 自动装     deploy.sh 硬编码     PAAP UI管理
─────────────────────────────────────────────────────────────────────
❌ 默认安装         ✔（强装）            ✔                    ✘
✅ 平台基础设施     ✘（按需）            ✘                    ✔（管理员一键装）
❌ CI 组件          ✘（用户选）          ✘                    ✔（作为 ServiceCatalog 条目）
```

**推荐路径（平台基础设施）：**

1. **deploy.sh 移除 kpack 步骤**，改为输出版本要求提示。kpack 安装移至独立的 `install-kpack.sh` 脚本
2. **后端增加 kpack 探测**：当用户创建 source delivery 组件时，检查 `kpack.io/v1alpha2` CRD 是否存在。不存在则返回明确引导错误："source delivery 需要安装 kpack，请联系平台管理员在管理页面安装"
3. **前端联动**：kpack 不可用时，component deploy tab 的 source delivery 模式灰显/显示提示，而非可选后报错
4. **平台管理界面**（对应路线图需求 7）增加"平台组件"tab，kpack 作为第一个平台级组件列在其中（安装/卸载/状态），调用 `install-kpack.sh` 逻辑
5. **版本兼容校验**：PAAP Server 启动时检测集群版本，推荐对应的 kpack 版本

#### 工作量预估
- deploy.sh 拆分 + install-kpack.sh：**2 小时**
- 后端 kpack 探测逻辑 + 引导错误：**半天**
- 前端 source delivery 灰显/提示：**半天**
- 平台管理界面集成 kpack：**1 天**（依赖于需求 7 的平台管理界面）

### 3.3 Calico + 新版内核 ipset 不兼容（cluster18）
- **位置**: `cluster18` 节点
- **问题**: Ubuntu 24.04 内核 6.14+/6.17+ 不支持 hash:ip,port rev 7，与 calico-node v3.23.2 内嵌的 ipset v7.11 不兼容，导致 Felix 就绪探针失败、DNS 不可用
- **严重性**: 严重（已通过 wrapper 修复）
- **修复方式**: 在 `/usr/local/bin/ipset-wrapper` 创建包装脚本，通过 hostPath 挂载替换容器内的 ipset 二进制
- **建议**: 升级 calico 到 v3.25+（或迁移到 Cilium）

### 3.4 PostgreSQL/MinIO 静态 PV 依赖
- **问题**: cluster18 需手动创建静态 PV（PostgreSQL 10Gi, MinIO 20Gi），无自动化存储配置
- **严重性**: 中
- **建议**: 使用 OpenEBS / Rook / Longhorn 等动态存储方案

### 3.5 kubespray 部署版本锁死
- **问题**: 环境使用 kubespray v2.20 部署 K8s v1.24.9，containerd 1.6.4，calico v3.23.2，均为较旧版本
- **严重性**: 中
- **建议**: 规划集群升级到 v1.28+

### 3.6 部署配置散落
- **问题**: 部署脚本（`deploy/k8s/deploy.sh`）和手动操作混合，部署流程不可完全重现
- **严重性**: 中
- **建议**: 将 calico fix、kpack env、image 替换等全部纳入自动化脚本

---

## 四、产品功能缺失（从 CLAUDE.md 路线图提取）

### 4.1 组件改名（需求 1）
- **状态**: ✅ 已完成
- **描述**: 画布显示名已支持双击/右键改名

### 4.2 Gitea/Registry 共享（需求 2）
- **状态**: ❌ 未实现
- **问题**: 工具/中间件是环境级的，无法跨环境共享 Gitea、Registry 等
- **建议**: 实现 `EnvironmentCapability` 模型，支持 `self / shared / external` 三种来源

### 4.3 分区分组（需求 3）
- **状态**: ❌ 未实现
- **问题**: 画布无法区分"平台公共"、"环境内"、"集群外部"节点
- **建议**: 画布节点带 `zone` 字段，三条泳道渲染

### 4.4 外部资源接入（需求 4）
- **状态**: ❌ 未实现
- **问题**: 只能通过 PAAP 安装服务，无法接入已有的外部资源（如已有的 PostgreSQL、GitLab 等）
- **建议**: 实现 External 来源 + 连接验证

### 4.5 中间件版本号（需求 5）
- **状态**: ✅ 已完成
- **描述**: ServiceTemplate 已支持多版本

### 4.6 中间件目录页（需求 6）
- **状态**: ✅ 已完成
- **描述**: 目录页路径 `/catalog` 已实现

### 4.7 平台管理员界面（需求 7）
- **状态**: ❌ 未实现
- **描述**: 无平台级管理页

### 4.8 Ingress/Gateway（需求 8）
- **状态**: ❌ 未实现
- **描述**: 组件外部访问未实现

### 4.9 ServiceIP（需求 9）
- **状态**: ✅ 已弃用（用 Service FQDN 替代）

### 4.10 KubeVirt 虚拟机（需求 10）
- **状态**: ❌ 未实现
- **描述**: 零基础

### 4.11 KEDA 水平扩展（需求 11）
- **状态**: ❌ 未实现
- **描述**: 组件副本数固定

### 4.12 双集群 / 跨集群网络（需求 12-13）
- **状态**: ❌ 未实现
- **描述**: 纯单集群，需架构级投入

### 4.13 三种角色（需求 14）
- **状态**: ❌ 未实现
- **描述**: 只有 user/admin 字符串枚举

### 4.14 CI/CD 管理（需求 15）
- **状态**: ❌ 未组织
- **描述**: 项目未推送到 Gitea，未在 Plane 拆任务

---

## 五、已知技术债务

### 5.1 Config Template 导入 UI
- **位置**: `frontend/src/`
- **问题**: 导入对话框样式重（灰色方块），未匹配白色 Carbon 风格；"适用组件"字段应为 select/combobox 而非逗号分隔文本
- **严重性**: 中

### 5.2 CDP 测试覆盖率不足
- **问题**: 产品抽屉未逐个进行 CDP 端到端测试。已知缺口：Registry/Harbor 演示、Jenkins 构建日志保真度、拓扑模式、PV 更新、失败状态 UX

### 5.3 无伪造数据（假数据/填充数据）
- **问题**: 所有工作区资源、指标、日志、备份、key、queue、topic、bucket、部署行必须可追溯到真实后端/API/集群源

### 5.4 关系检测不完善
- **问题**: 自动关系检测只能处理环境变量和服务引用，ConfigMap/Secret/文件配置未覆盖

### 5.5 Kubernetes 术语暴露
- **问题**: 画布/抽屉仍显示 namespace、service、pod、configmap、secret、pvc、helm 等标签

### 5.6 数据库备份不完整
- **问题**: 备份创建已实现，但恢复/下载/列表/失败状态 UX 未完成

---

## 六、运行时问题

### 6.1 cluster18 thanos-compactor CrashLoopBackOff
- **位置**: cluster12 的 `monitoring` 命名空间
- **问题**: `thanos-compactor-0` 处于 CrashLoopBackOff（7922 次重启）
- **严重性**: 低（不影响 PAAP 核心功能）

### 6.2 ArgoCD Chart K8s 版本要求不兼容
- **位置**: 两集群的 ServiceInstance `demo-argocd`
- **问题**: ArgoCD Helm chart 要求 `kubeVersion: >=1.25.0-0`，但两集群均为 K8s v1.24.9。Helm 拒绝渲染模板，ServiceInstance 处于 Error 阶段
- **严重性**: 中
- **影响**: 新建环境默认安装的 ArgoCD 永远无法成功
- **建议**:
  - 使用旧版本 ArgoCD chart（Chart.yaml 中 kubeVersion 约束更宽松）
  - 升级集群到 v1.25+
  - 在 operator 的 Helm client 层增加 `--kube-version` 覆盖参数，或修改 chart 的 kubeVersion 约束

### 6.3 cluster12 node-debugger ImagePullBackOff
- **位置**: `default` 命名空间
- **问题**: `node-debugger-node2` 处于 ImagePullBackOff（25 天）
- **严重性**: 低

### 6.3 中间件安装等待时间长
- **问题**: 通过 PAAP UI 安装 Gitea/ArgoCD 等服务时，Helm install 流程慢（5-15 分钟），WebSocket 状态推送可能超时
- **严重性**: 中
- **建议**: 优化 WebSocket 推送逻辑，增加安装进度条

### 6.4 cluster12 ServiceInstance 安装错误
- **位置**: cluster12, paap-app-cluster12-test
- **问题**: `demo-argocd` ServiceInstance 处于 Error 阶段，可能因镜像拉取失败或集群资源不足
- **严重性**: 中
- **建议**: 检查 operator 日志定位具体错误

---

## 七、测试与质量

### 7.1 后端测试覆盖率
- **问题**: go test 覆盖率低，关键路径（handler, service, k8s client）缺少单元测试
- **建议**: 增加 controller、handler、model 层的测试

### 7.2 前端测试
- **问题**: Vitest 测试存在但覆盖率不足，composable 和 workspace 组件缺少测试
- **建议**: 增加 composable 和 workspace 组件的单元测试

### 7.3 E2E 测试缺失
- **问题**: 无 Playwright/Cypress 端到端测试
- **建议**: 建立 E2E 测试套件覆盖核心用户流程

---

## 八、代码质量问题

### 8.1 超大函数
- **位置**: `internal/handler/environment.go`（8450 行）
- **问题**: 单文件过大（项目最大文件），部分函数过长。同样的 `internal/controller/serviceinstance_controller.go` 1758 行
- **建议**: 按领域拆分为多个 handler/controller 文件

### 8.2 重复代码
- **位置**: 多处
- **问题**: ServiceInstallation 的 Helm 安装/升级/卸载逻辑重复
- **建议**: 抽取公共 Helm 操作层

### 8.3 Error Handling 不一致
- **位置**: 多处
- **问题**: 部分函数返回 `(result, error)` 但调用方忽略错误；部分使用 panic 而非返回错误
- **建议**: 统一错误处理策略

### 8.4 硬编码配置
- **位置**: 多处
- **问题**: Registry endpoint、namespace 命名规则、服务端口等硬编码
- **建议**: 提取到配置结构体

---

## 九、改进建议优先级

### P0 - 立即修复
- 2.2 JWT Secret 校验
- 2.4 登录频率限制
- 3.1 Docker Hub 镜像镜像替换自动化

### P1 - 尽快修复
- 2.1 默认密码管理
- 3.3 Calico 升级或更换 CNI
- 3.6 部署流程自动化
- 6.4 ServiceInstance Error 排查

### P2 - 规划中
- 4.2-4.4 共享/外部资源主线
- 4.7 平台管理界面
- 4.13 三种角色

### P3 - 长期优化
- 4.10 KubeVirt
- 4.11 KEDA
- 4.12 多集群
- 8.1 handler 重构
- 7.3 E2E 测试

---

## 十、前端样式与视觉问题（2026-06-26 代码扫描）

> 以下数据基于 `grep`/`wc` 扫描 36 个 `.vue` 文件和 14 个 workspace 组件。

### 10.1 三套 CSS 变量体系并存（设计系统混乱）

每个 view 用不同的样式体系，无统一规范：

| 体系 | 文件举例 | 颜色来源 |
|------|---------|---------|
| `var(--cds-*)` Carbon 变量 | `EnvDetailView.vue`, `AppEnvironmentsView.vue` | `var(--cds-text-primary, #xxx)` |
| `var(--paap-*)` 自定义变量 | `AppListView.vue`, `AppMembersView.vue` | `var(--paap-text)` 无 Carbon 回退 |
| 纯硬编码 hex | `AppCIView.vue`, `AppDeployView.vue`, `CatalogView.vue` | `#687076`, `#11181c`, `#e6e8eb` |

- 36 个 view 都有大量硬编码 hex 颜色：EnvDetailView (254), ComponentDetailView (108), TemplatesView (129), CatalogView (44)
- 有些文件混合三种体系（如 EnvDetailView 用 `--cds-`/`--paap-`/hex 混用）
- 同样的颜色在不同文件用不同 hex（`#687076` vs `var(--paap-muted)` vs `var(--cds-text-secondary, #6f6f6f)`）
- **建议**: 统一使用 `var(--cds-*)` Carbon 变量，删除 `--paap-*` 自定义变量，彻底清除硬编码 hex

### 10.2 硬编码 font-size（每个文件重复定义）

Carbon 提供了完整字体 scale（`--cds-heading-*`/`--cds-body-*`/`--cds-label-*`），但代码全部硬编码：

| 文件 | 硬编码 font-size 数 |
|------|-------------------|
| `EnvDetailView.vue` | 172 |
| `ComponentDetailView.vue` | 39 |
| `TemplatesView.vue` | 33 |
| `AppDeployView.vue` | 25 |
| `AppMonitorView.vue` | 20 |
| `AppCIView.vue` | 19 |
| `CatalogView.vue` | 17 |
| `AppListView.vue` | 15 |

典型重复定义：
```
/* 三个 view，同一个概念，三种写法 */
AppCIView.vue:  .page-title { font-size: 22px; ... }
AppDeployView.vue: .page-title { font-size: 22px; ... }
AppMonitorView.vue: .page-title { font-size: 22px; ... }
```
**建议**: 全部替换为 `var(--cds-heading-03-font-size)` / `var(--cds-body-01-font-size)` / `var(--cds-label-01-font-size)`。

### 10.3 硬编码 border-radius

- 14 个 view 有 ≥4 个硬编码 `border-radius`（EnvDetailView 92 个）
- 无统一 radius token —— 有的用 `6px`，有的 `8px`，有的 `var(--paap-radius)`，有的 Carbon `0`
- **建议**: 使用 `var(--cds-border-radius)` 或定义项目级 radius token

### 10.4 重复的空状态/加载/错误样式

每个 view 各写一套，同质但不同色：
```
/* AppCIView */ .empty-card { background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px; padding: 64px 32px; text-align: center; }
/* AppDeployView */ .empty-card { background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px; padding: 64px 32px; text-align: center; }
/* AppMonitorView */ .empty-card { background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px; padding: 64px 32px; text-align: center; }
```
7 个 view 有各自的 `.loading-spinner` + `@keyframes spin`，代码完全相同。
**建议**: 抽取共享组件 `EmptyState`, `LoadingSpinner`, `ErrorBox`，消除重复。

### 10.5 Modal/Dialog 不统一

5 个 view 有 modal，modal 样式各自实现，CSS token 体系不一致：

| View | Token 体系 | 差异 |
|------|-----------|------|
| `AppListView.vue` | `var(--paap-panel)` / `var(--paap-border)` | 自定义变量 |
| `AppEnvironmentsView.vue` | `var(--cds-layer-01, var(--paap-panel))` | 混合 |
| `EnvDetailView.vue` | `var(--cds-layer-01, #fff)` | Carbon + raw fallback |
| `TemplatesView.vue` | raw hex | 纯硬编码 |

- `backdrop-filter: blur(10px)` 只在部分 modal 中（AppListView、EnvDetailView）
- Close button 大小/样式不一致（`32px width` vs 无固定尺寸）
- **建议**: 抽取 `BaseModal.vue` 共享组件

### 10.6 Kubernetes 术语暴露（见 5.5，补充详细数据）

- `EnvDetailView.vue`: **132 处** K8s 术语 — `namespace`、`secret`、`deployment`、`configmap`、`pvc`、`helm`
- `ComponentDetailView.vue`: 20 处
- `TemplatesView.vue`: 11 处
- 典型暴露：密码字段 label 包含 "Secret"，存储配置含 "PVC"，资源名显示完整 K8s resource name
- **建议**: 对用户只显示业务名称，K8s 术语放在 tooltip 或展开的高级区域

### 10.7 图标使用：内联 SVG 代替 Carbon Icons

- `EnvDetailView.vue`: 42 个内联 `<svg>` 元素
- `TemplatesView.vue`: 8 个内联 `<svg>`
- **零个** view 使用 `@carbon/icons-vue`
- package.json 包含 `@carbon/icons-vue` 但实际未使用
- **建议**: 全部替换为 `<svg>` 为 Carbon icon 组件，统一风格并缩小体积

### 10.8 动画/过渡时间不一致

| View | 过渡时间 | 手法 |
|------|---------|------|
| `EnvDetailView.vue` | `110ms` | Carbon 风格 |
| `AppEnvironmentsView.vue` | `0.15s` ease | 自定义 |
| `AppListView.vue` | `0.15s` | 自定义 |
| `AppOverviewView.vue` | `0.15s` | 自定义 |
| `CatalogView.vue` | `0.2s` ease | 自定义 |
| `AppDeployView.vue` | `0.2s` | 自定义 |

**建议**: 统一使用 Carbon 规范 `110ms` / `150ms` / `240ms`。

### 10.9 响应式设计不足

- 36 个 view 中仅 12 个有 `@media` 查询
- 每个 view 只有 1-2 个断点（最大 `@media (max-width: 768px)`）
- 画布/表格在小屏幕不可用
- **建议**: 增加断点覆盖（Carbon 参考：320 / 672 / 1056 / 1312 / 1584）

### 10.10 无障碍（A11Y）缺失

- `AppDeployView.vue`: 零个 `aria-*` 或 `role=` 属性
- `AppMonitorView.vue`: 零个
- 其他 view 仅部分有关键 aria label
- 无 keyboard navigation 支持（modal 不 trap focus）
- **建议**: 基础 a11y：aria-label、role、focus trap、keyboard event

### 10.11 box-shadow 不统一

| View | shadow 值 |
|------|-----------|
| `AppListView.vue` | `0 1px 3px rgba(0,0,0,0.04)` |
| `AppEnvironmentsView.vue` | `0 4px 12px rgba(0,0,0,0.04)` |
| `AppMonitorView.vue` | `0 2px 8px rgba(0,0,0,0.06)` |
| `CatalogView.vue` | `0 2px 6px rgba(0,0,0,0.1)` |
| `ComponentDetailView.vue` | `0 0 0 2px rgba(17,24,28,0.08)`（focus）|

- 无统一的 elevation token
- **建议**: 定义 `--paap-shadow-sm` / `--paap-shadow-md` / `--paap-shadow-lg`

### 10.12 Workspace 组件 CSS 体量差异大

| Workspace | CSS 行数 | 硬编码颜色 |
|-----------|---------|-----------|
| `ArgocdWorkspace.vue` | 490 | 15 |
| `GiteaWorkspace.vue` | 302 | 9 |
| `DatabaseWorkspace.vue` | 222 | 0 |
| `RedisWorkspace.vue` | 184 | 1 |
| `ToolWorkspaceFrame.vue` | 193 | 17 |
| `KafkaWorkspace.vue` | 38 | 0 |
| `MinioWorkspace.vue` | 22 | 0 |

工具抽屉之间视觉质量差异显著（Argocd 精细但 Kafka/MinIO 简陋）。
**建议**: 统一 workspace 基线样式，共享 `ToolWorkspaceFrame` 的 CSS token。
