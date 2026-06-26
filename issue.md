# 前端 UI 视觉问题清单

> 通过 Playwright 自动化深度检查生成，2026-06-25（含 Railway 设计系统对比）

## 概览

| 页面 | 问题数 | 严重程度 |
|------|--------|----------|
| 应用列表 `/apps` | 6 | 中 |
| 用户管理 `/users` | 5 | 中 |
| 服务目录 `/catalog` | 3 | 高 |
| 应用成员 `/apps/:id/members` | 5 | 中 |
| 应用概览 `/apps/:id` | 5 | 中 |
| 环境详情 `/apps/:id/environments/:id` | 6 | 中 |
| 创建应用对话框 | 5 | 中 |
| 服务抽屉 (Redis) | 8 | 中 |
| 模板页面 `/templates` | 4 | 中 |
| 全局问题 | 10 | 高 |
| 画布节点专项问题 | 8 | 高 |
| 元素级对比（Railway 设计系统） | 15 | 高 |
| **总计** | **80** | - |

---

## 📋 全项目问题总索引

> 以下为 **17 个扫描章节** 的完整汇总，按层分类，便于导航执行。
>
> **去重说明：** 本次清洗合并了跨章节重复内容。每个唯一问题保留在**最适合的章节**，其他位置替换为「见 §X.X」交叉引用。主要合并项：Docker/测试覆盖 → §17，@carbon 图标 → §15，EnvDetailView 拆分 → §12 与 §15 互引，K8s 安全 → §16 与 §17 互引。

### 目录（按章节）

| # | 章节 | 领域 | 关键 P1-P2 | 行号 |
|---|------|------|-----------|------|
| 一 ~ 三 | Playwright 视觉检查 | 前端 UI | — | L1-L1070 |
| 附录：方法 | 检查方法 | 工具说明 | — | L1071 |
| [四](#四css-design-token-审计-2026-06-26) | CSS Design Token 审计 | 前端设计系统 | 色值/圆角/字体/阴影 | L1093 |
| [五](#五深层样式与视觉一致性扫描-2026-06-26) | 深层样式一 (Canvas/Workspace/Modal/Icon/Loading) | 前端样式 | Workspace 圆角/Modal header | L1372 |
| [六](#六深层样式扫描第二期-2026-06-26) | 深层样式二 (ComponentDetail/DarkMode/Form/Z-index) | 前端样式 | Carbon 组件样式/断点逻辑 | L1609 |
| [七](#七深层样式扫描第三期-2026-06-26) | 深层样式三 (AppRegistry/SVG/动画/DeadCSS) | 前端样式 | SVG 非 Carbon/双轨动画 | L1795 |
| [八](#八深层样式扫描第四期-2026-06-26) | 深层样式四 (测试/对比度/空/加载/错误状态) | 前端质量 | 测试覆盖/空状态 | L1943 |
| [九](#九深层样式扫描第五期-2026-06-26) | 深层样式五 (字体/选择器/动态样式) | 前端样式 | 字体声明/动态样式注入 | L2066 |
| [十](#十基础设施与性能扫描-2026-06-26) | 基建与性能 | 前端基建 | WebSocket 未接/ESLint | L2239 |
| [十一](#十一全项目可维护性审计-2026-06-26) | 全项目可维护性 | 跨层 | i18n/A11y/错误消息 | L2357 |
| [十二](#十二后端安全与架构审计-2026-06-26) | 后端安全与架构 | 后端 | JWT default/CSRF/DB索引 | L2492 |
| [十三](#十三后端质量与基础设施审计-2026-06-26) | 后端质量与基建 | 后端 | Docker image 体积/GORM 版本 | L2602 |
| [十四](#十四后端一致性与工程化审计-2026-06-26) | 后端一致性 | 后端 | environment.go 拆分/视图测试 | L2725 |
| [十五](#十五打包与依赖审计-2026-06-26) | 打包与依赖 | 前端打包 | Bundle 48%/Carbon 包未用 | L2856 |
| [十六](#十六crd-设计与-operator-审计-2026-06-26) | CRD 设计与 Operator | 基础设施 | k8s client.exec/硬编码密码/CRD webhook | L2983 |
| [十七](#十七最终扫描测试覆盖--错误类型--docker--k8s-清单--依赖-2026-06-26) | 最终扫描 | 全栈 | 测试覆盖/K8s 安全/Docker | L3184 |

### 优先级汇总

| 优先级 | 问题数 | 关键项 |
|--------|--------|--------|
| 🔴 **P1 生产安全** | ~4 | k8s/client.go 硬编码密码，JWT 默认密钥，CSRF 缺失，Operator wildcard RBAC 无限制 |
| 🟡 **P2 重要** | ~35 | EnvDetailView 拆分，Carbon 包未用，i18n 缺失，Handler 测试覆盖，CRD webhook，CSS 双轨，Docker 构建优化 |
| 🟢 **P3 建议** | ~15 | 废弃 CSS 清除，对比度改进，minio-go 版本，framer-motion 冗余 |

### 按模块分组

<details>
<summary><b>🎨 前端视觉层（四 ~ 九）— 约 100+ 项</b></summary>

- Design Token 不一致（色值/圆角/字体/阴影/滚动条）
- Canvas/Workspace/Modal/Drawer 样式脱离 Carbon 规范
- ComponentDetailView 未碳化（P1）
- SVG 使用内联而非 @carbon/icons-vue
- CSS 过渡与 framer-motion 双轨（冗余）
- 空状态/加载态/错误态覆盖不足
- 字体声明未统一
- 颜色对比度不足
- 响应式断点不完善
</details>

<details>
<summary><b>⚡ 前端基建与工程化（十 ~ 十一、十五）— 约 20 项</b></summary>

- WebSocket 状态推送未接入
- ESLint 配置缺失/规则松散
- 画布渲染无虚拟化（大量节点性能差）
- 缩放/平移 UX 不完整
- i18n 完全缺失
- A11y 基础缺失（aria-label/role/keyboard nav）
- Error toast/banner 不统一
- 打包体积：EnvDetailView 占 48%（P2）
- @carbon/vue + @carbon/styles + @carbon/icons-vue 装而不用（P2）
- 无 Vue 组件测试框架（@vue/test-utils 缺失）
</details>

<details>
<summary><b>🔧 后端安全与架构（十二、十四、十六）— 约 25 项</b></summary>

- JWT_SECRET 有 dev 默认值（P1）
- CSRF 保护缺失（P1）
- GORM 索引缺失（全表扫描风险）
- environment.go 8615 行（需拆分）
- 无 sentinel errors，%w wrapping 仅 5%
- CRD 无 validating/mutating webhook
- Controller 无子资源 Watch（依赖轮询）
- RequeueAfter 9 种间隔不一致
- k8s/client.go 使用 exec.Command 而非 client-go
- context.Background() 广泛使用（替代传播 context）
</details>

<details>
<summary><b>🏗️ 基础设施与部署（十三、十六、十七）— 约 15 项</b></summary>

- Docker：无 .dockerignore、CGO 可禁、GOCACHE 可用 BuildKit
- K8s：paap-server.yaml 缺 securityContext/resources
- K8s：postgres/minio.yaml 缺探针
- Operator：RBAC wildcard 需文档注释
- Go 测试覆盖 16-35%（handler/database 最低）
- minio-go AGPL-3.0 License 需法务确认
- GORM 版本落后（v1.31.1）
</details>

---

## 原扫描内容（以下保留各章节细节）

---

## Railway 设计系统 vs PAAP 元素级对比

> 基于 Railway DESIGN.md 和行业最佳实践，忽略颜色方案，专注设计原则

### 设计系统核心参数对比

| 参数 | Railway | PAAP 当前 | 行业最佳实践 | PAAP 应改为 |
|------|---------|-----------|-------------|------------|
| **圆角基础** | 4px | 0px | 4-6px | **6px** |
| **按钮圆角** | 6px | 0px | 6px | **6px** |
| **卡片圆角** | 8px | 0px | 8px | **8px** |
| **输入框圆角** | 6px | 0px | 6px | **6px** |
| **间距系统** | 4/8/12/16/24/32px | 不统一 | 4/8/12/16/24/32px | **统一** |
| **阴影层级** | 3级（sm/md/lg） | 无 | 3级 | **3级** |
| **边框风格** | 半透明 `rgba(0,0,0,0.08)` | `solid rgb(198,198,198)` | 半透明 | **半透明** |

---

### 按钮元素对比

| 属性 | Railway 按钮 | PAAP 按钮 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **圆角** | 6px | 0px | ❌ 严重 | 改为 6px |
| **高度** | 32px（标准） | 28px/36px 混用 | ❌ 不统一 | 统一为 32px |
| **内边距** | `6px 12px` | 不统一 | ❌ | 统一为 `8px 16px` |
| **字体大小** | 14px | 12px/13px/14px 混用 | ❌ | 统一为 14px |
| **字重** | 500 | 400/500 混用 | ⚠️ | 统一为 500 |
| **阴影** | 无（用边框） | 无 | ✅ | 保持 |
| **悬停** | 背景色变化 | 无 | ❌ | 添加悬停效果 |
| **过渡** | `150ms ease` | 无 | ❌ | 添加 `transition: all 150ms ease` |

**Railway 按钮示例：**
```css
.btn-primary {
  background-color: #553f83;
  color: #ffffff;
  font-size: 14px;
  font-weight: 500;
  padding: 8px 16px;
  border-radius: 6px;
  border: 1px solid #553f83;
  transition: background-color 150ms ease;
}
```

---

### 卡片元素对比

| 属性 | Railway 卡片 | PAAP 卡片 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **圆角** | 8px | 0px | ❌ 严重 | 改为 8px |
| **边框** | `1px solid rgba(0,0,0,0.08)` | `1px solid rgb(198,198,198)` | ❌ 颜色过深 | 改为半透明 |
| **阴影** | 层叠阴影 | 无 | ❌ 严重 | 添加阴影 |
| **内边距** | 16px | 20px 24px | ⚠️ 不统一 | 统一为 16px |
| **悬停** | 阴影增强 | 无 | ❌ | 添加悬停效果 |
| **背景** | `#ffffff` | `#ffffff` | ✅ | 保持 |

**Railway 卡片示例：**
```css
.card {
  background-color: #ffffff;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
```

---

### 输入框元素对比

| 属性 | Railway 输入框 | PAAP 输入框 | 差距 | 修复建议 |
|------|---------------|-------------|------|---------|
| **圆角** | 6px | 0px | ❌ 严重 | 改为 6px |
| **边框** | `1px solid rgba(0,0,0,0.15)` | `1px solid rgb(198,198,198)` | ❌ 颜色过深 | 改为半透明 |
| **高度** | 40px | 38px/40px 混用 | ⚠️ | 统一为 40px |
| **内边距** | `10px 12px` | `9px 12px` | ⚠️ | 统一为 `10px 12px` |
| **字体大小** | 14px | 14px | ✅ | 保持 |
| **聚焦** | 边框变色 + 阴影 | 无 | ❌ | 添加聚焦效果 |
| **过渡** | `150ms ease` | 无 | ❌ | 添加过渡 |

**Railway 输入框示例：**
```css
.input {
  background-color: #ffffff;
  border: 1px solid rgba(0, 0, 0, 0.15);
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 14px;
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.input:focus {
  border-color: #553f83;
  box-shadow: 0 0 0 3px rgba(85, 63, 131, 0.1);
  outline: none;
}
```

---

### 表格元素对比

| 属性 | Railway 表格 | PAAP 表格 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **圆角** | 8px（容器） | 0px | ❌ 严重 | 容器改为 8px |
| **边框** | 半透明 | 无 | ❌ | 添加边框 |
| **表头背景** | 浅色 | `rgb(255,255,255)` | ⚠️ | 改为 `rgb(244,244,244)` |
| **行高** | 48px | 47px | ✅ | 接近，可保持 |
| **悬停行** | 背景色变化 | 无 | ❌ | 添加悬停效果 |
| **字体大小** | 14px | 12px/14px | ⚠️ | 统一为 14px |

---

### 导航项对比

| 属性 | Railway 导航 | PAAP 导航 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **圆角** | 6px | 0px | ❌ 严重 | 改为 6px |
| **高度** | 32px | 40px | ⚠️ 偏大 | 改为 36px |
| **激活指示** | 背景色变化 | 仅文字变色 | ⚠️ | 添加背景色或左边框 |
| **悬停** | 背景色变化 | 无 | ❌ | 添加悬停效果 |
| **字体大小** | 16px | 14px | ⚠️ | 保持 14px（适合侧边栏） |

---

### 对话框/抽屉对比

| 属性 | Railway 弹窗 | PAAP 对话框 | 差距 | 修复建议 |
|------|-------------|-------------|------|---------|
| **圆角** | 12px | 0px | ❌ 严重 | 改为 12px |
| **阴影** | `0 4px 12px rgba(0,0,0,0.1)` | 无 | ❌ 严重 | 添加阴影 |
| **遮罩** | 半透明黑色 | `rgba(17,19,24,0.46)` | ✅ | 保持 |
| **关闭按钮** | 24px 圆形 | 28px 方形 | ❌ | 改为圆形 |
| **内边距** | 24px | 24px | ✅ | 保持 |

---

### 标签页对比

| 属性 | Railway 标签 | PAAP 标签 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **激活指示** | 底部边框 + 字重变化 | 仅底部边框 | ⚠️ | 添加字重变化 |
| **圆角** | 6px | 0px | ❌ | 改为 6px |
| **悬停** | 背景色变化 | 无 | ❌ | 添加悬停效果 |
| **过渡** | `150ms ease` | 无 | ❌ | 添加过渡 |

---

### 状态标签对比

| 属性 | Railway 标签 | PAAP 标签 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **圆角** | 4px | 3px | ⚠️ | 统一为 4px |
| **内边距** | `2px 8px` | `2px 8px` | ✅ | 保持 |
| **字体大小** | 12px | 12px | ✅ | 保持 |
| **背景** | 浅色半透明 | 纯色 | ⚠️ | 改为半透明 |

---

### 间距系统对比

| 位置 | Railway 间距 | PAAP 间距 | 差距 | 修复建议 |
|------|-------------|-----------|------|---------|
| **卡片内边距** | 16px | 16px/20px/24px | ❌ 不统一 | 统一为 16px |
| **卡片间距** | 16px | 不统一 | ❌ | 统一为 16px |
| **按钮内边距** | 8px 16px | 不统一 | ❌ | 统一为 8px 16px |
| **表单间距** | 12px | 不统一 | ❌ | 统一为 12px |
| **页面内边距** | 24px | 0px/24px | ❌ | 统一为 24px |

---

### 字体系统对比

| 属性 | Railway | PAAP 当前 | 差距 | 修复建议 |
|------|---------|-----------|------|---------|
| **主字体** | Inter | 系统字体 | ⚠️ | 考虑引入 Inter |
| **标题字体** | IBM Plex Serif | 系统字体 | ⚠️ | 保持系统字体 |
| **代码字体** | JetBrains Mono | 无 | ❌ | 添加等宽字体 |
| **H1 大小** | 54px | 28px | ⚠️ | 保持 28px（适合后台） |
| **H2 大小** | 40px | 24px | ⚠️ | 保持 24px |
| **正文大小** | 14px | 14px | ✅ | 保持 |
| **辅助文字** | 12px | 12px | ✅ | 保持 |

---

### 阴影系统对比

| 层级 | Railway | PAAP | 行业标准 | 修复建议 |
|------|---------|------|---------|---------|
| **sm（按钮/下拉）** | `0 0 1px rgba(0,0,0,0.03), 0 1px 2px rgba(0,0,0,0.06)` | 无 | `0 1px 2px rgba(0,0,0,0.05)` | 添加 |
| **md（卡片）** | `0 1px 3px rgba(0,0,0,0.05)` | 无 | `0 1px 3px rgba(0,0,0,0.05)` | 添加 |
| **lg（弹窗）** | `0 4px 12px rgba(0,0,0,0.1)` | 无 | `0 4px 12px rgba(0,0,0,0.1)` | 添加 |

---

### 边框系统对比

| 用途 | Railway | PAAP 当前 | 行业标准 | 修复建议 |
|------|---------|-----------|---------|---------|
| **卡片边框** | `1px solid rgba(0,0,0,0.08)` | `1px solid rgb(198,198,198)` | 半透明 | 改为半透明 |
| **输入框边框** | `1px solid rgba(0,0,0,0.15)` | `1px solid rgb(198,198,198)` | 半透明 | 改为半透明 |
| **分隔线** | `1px solid rgba(0,0,0,0.08)` | `1px solid rgb(198,198,198)` | 半透明 | 改为半透明 |
| **聚焦环** | `0 0 0 3px rgba(85,63,131,0.1)` | 无 | 需要 | 添加 |

---

### 过渡动画对比

| 状态 | Railway | PAAP | 行业标准 | 修复建议 |
|------|---------|------|---------|---------|
| **按钮悬停** | `150ms ease` | 无 | `150ms ease` | 添加 |
| **输入框聚焦** | `150ms ease` | 无 | `150ms ease` | 添加 |
| **卡片悬停** | `200ms ease` | 无 | `200ms ease` | 添加 |
| **标签切换** | `150ms ease` | 无 | `150ms ease` | 添加 |

---

### 响应式断点对比

| 断点 | Railway | PAAP | 行业标准 |
|------|---------|------|---------|
| **移动端** | < 768px | 未测试 | < 768px |
| **平板端** | 768px - 1024px | 未测试 | 768px - 1024px |
| **桌面端** | > 1024px | > 1024px | > 1024px |

---

### 无障碍对比

| 属性 | Railway | PAAP | 行业标准 | 修复建议 |
|------|---------|------|---------|---------|
| **焦点环** | 有 | 无 | WCAG AA | 添加 |
| **对比度** | 4.5:1+ | 未测试 | 4.5:1+ | 测试并修复 |
| **ARIA 标签** | 完整 | 不完整 | 完整 | 补充 |
| **键盘导航** | 支持 | 部分支持 | 完整支持 | 完善 |

---

## Railway 设计系统关键原则（PAAP 应采用）

### 1. 圆角系统
```css
:root {
  --radius-sm: 4px;   /* 标签、小元素 */
  --radius-md: 6px;   /* 按钮、输入框 */
  --radius-lg: 8px;   /* 卡片、弹窗 */
  --radius-full: 9999px; /* 头像、状态点 */
}
```

### 2. 间距系统（4px 基数）
```css
:root {
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-6: 24px;
  --space-8: 32px;
  --space-12: 48px;
  --space-16: 64px;
}
```

### 3. 阴影系统
```css
:root {
  --shadow-sm: 0 0 1px rgba(0,0,0,0.03), 0 1px 2px rgba(0,0,0,0.06);
  --shadow-md: 0 1px 3px rgba(0,0,0,0.05);
  --shadow-lg: 0 4px 12px rgba(0,0,0,0.1);
}
```

### 4. 边框系统
```css
:root {
  --border-subtle: 1px solid rgba(0,0,0,0.08);
  --border-default: 1px solid rgba(0,0,0,0.15);
  --border-strong: 1px solid rgba(0,0,0,0.25);
}
```

### 5. 过渡系统
```css
:root {
  --transition-fast: 150ms ease;
  --transition-normal: 200ms ease;
  --transition-slow: 300ms ease;
}
```

---

## 画布节点竞品分析

### PAAP 现状 vs 行业标准

| 属性 | PAAP 当前值 | Railway | Vercel | React Flow 默认 | 行业建议 |
|------|------------|---------|--------|----------------|---------|
| `borderRadius` | `0px` | `4px` | `8px` | `8px (rounded-md)` | **8px** |
| `boxShadow` | `none` | 层叠阴影 | 层叠阴影 | `0 1px 4px 1px rgba(0,0,0,0.08)` | **层叠阴影** |
| `border` | `1px solid rgb(198,198,198)` | `1px solid #33323e` | `1px solid rgba(255,255,255,0.08)` | `1px solid #1a192b` | **半透明边框** |
| 悬停效果 | 无 | 有 | 有 | `hover:ring-1` | **需要** |
| 选中状态 | 仅蓝色边框 | 有 | 有 | `shadow-lg` + `border-muted-foreground` | **需要** |
| 背景色 | `rgb(255,255,255)` | `#1E1C25` (深色) | `#0a0a0a` (深色) | `bg-card` | **可选深色** |

### 竞品画布设计亮点

#### 1. Railway
- **深色画布**: `#13111c` (靛蓝午夜色)，减少视觉疲劳
- **4px 基础圆角**: 全局统一，使用 439 次
- **发丝边框**: `#33323e`，极细但清晰
- **卡片阴影**: `shadow-[inset_0_0_0_1px_rgba(0,0,0,0.08)]`
- **字体**: Inter (无衬线) + IBM Plex Serif (标题)

#### 2. Vercel
- **层叠阴影**: 模拟环境光 + 直射光，至少两层
- **嵌套圆角**: 子元素圆角 ≤ 父元素圆角
- **半透明边框**: `rgba(255,255,255,0.08)` 提升边缘清晰度
- **色调一致性**: 边框/阴影/文字倾向相同色调
- **交互增强对比**: `:hover`、`:active`、`:focus` 状态

#### 3. React Flow (官方推荐)
- **BaseNode 组件**: `rounded-md border bg-card`
- **悬停状态**: `hover:ring-1`
- **选中状态**: `[.react-flow__node.selected_&]:border-muted-foreground [.react-flow__node.selected_&]:shadow-lg`
- **默认阴影**: `0 1px 4px 1px rgba(0,0,0,0.08)`
- **默认边框**: `1px solid #1a192b`

#### 4. kScratch / InfraCanvas (K8s 可视化)
- **健康状态颜色**: 绿色(运行)、琥珀色(降级)、红色(异常)
- **圆角节点**: 统一 8px 圆角
- **可点击节点**: 点击展开详情面板
- **实时更新**: 无需刷新即可反映状态变化

### PAAP 画布节点具体问题

#### 58. 画布节点无圆角（严重）
- **位置**: `.component-canvas-node`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`（与 React Flow 默认一致）
- **影响**: 节点边缘生硬，不符合现代 UI 标准
- **竞品对比**: Railway 4px, Vercel 8px, React Flow 8px

#### 59. 画布节点无阴影（严重）
- **位置**: `.component-canvas-node`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 1px 4px 1px rgba(0,0,0,0.08)`（React Flow 默认）
- **影响**: 节点与背景融合，缺乏浮起感
- **竞品对比**: 所有竞品都有阴影

#### 60. 画布节点无悬停效果
- **位置**: `.component-canvas-node`
- **现状**: 悬停无视觉变化
- **期望**: `hover:ring-1` 或 `hover:box-shadow` 增强
- **影响**: 用户无法感知可交互性
- **竞品对比**: Railway/Vercel/React Flow 都有悬停反馈

#### 61. 画布节点选中状态不明显
- **位置**: `.component-canvas-node.active`
- **现状**: 仅蓝色边框 `border: 1px solid rgb(15, 98, 254)`
- **期望**: 蓝色边框 + `box-shadow: 0 0 0 2px rgba(15,98,254,0.2)` + 轻微放大
- **影响**: 选中状态不够突出
- **竞品对比**: React Flow 用 `shadow-lg` + `border-muted-foreground`

#### 62. 画布节点边框颜色过时
- **位置**: `.component-canvas-node`（非活跃状态）
- **现状**: `border: 1px solid rgb(198, 198, 198)`
- **期望**: `border: 1px solid rgba(0,0,0,0.08)` 或 `border: 1px solid #e2e2e2`
- **影响**: 边框颜色不现代
- **竞品对比**: Vercel 用半透明边框，React Flow 用深色边框

#### 63. 画布节点无状态指示器
- **位置**: 节点状态显示
- **现状**: 仅文字状态（如"运行中"）
- **期望**: 彩色状态指示点（绿色=运行，琥珀色=警告，红色=异常）
- **影响**: 状态不够直观
- **竞品对比**: kScratch/InfraCanvas 用彩色圆点

#### 64. 画布连线无动画
- **位置**: 节点之间的连线
- **现状**: 静态线条
- **期望**: 悬停时显示数据流动画或脉冲效果
- **影响**: 连线缺乏活力
- **竞品对比**: 部分竞品有流动动画

#### 65. 画布背景过于简单
- **位置**: 画布背景
- **现状**: 纯白色或浅灰色
- **期望**: 点阵背景或网格背景（React Flow 默认有 `background-pattern-dots`）
- **影响**: 画布缺乏深度感
- **竞品对比**: React Flow 默认有背景图案

---

## 应用列表 `/apps`

### 1. 应用卡片无圆角
- **位置**: `.app-card`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 视觉效果生硬，不专业

### 2. 应用卡片无阴影
- **位置**: `.app-card`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 卡片与背景融合，缺乏层次

### 3. 删除按钮尺寸过小
- **位置**: `.app-delete-btn`
- **现状**: 高度 28px，宽度 70px
- **期望**: 高度 36px（与其他主按钮一致）
- **影响**: 操作区域视觉不一致

### 4. 删除按钮无圆角
- **位置**: `.app-delete-btn`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 按钮边缘生硬

### 5. 页面头部无底部分隔线
- **位置**: 页面头部区域
- **现状**: 无 `borderBottom`
- **期望**: 添加 `border-bottom: 1px solid rgb(198, 198, 198)`
- **影响**: 头部与内容区域缺乏视觉分隔

### 6. 主内容区域无内边距
- **位置**: 主内容容器
- **现状**: `padding: 0px`
- **期望**: `padding: 24px`
- **影响**: 内容紧贴边缘

---

## 用户管理 `/users`

### 7. 用户表格无圆角
- **位置**: `.users-table`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 表格边缘生硬

### 8. 用户表格无边框
- **位置**: `.users-table`
- **现状**: `border: 0px none`
- **期望**: 添加外边框 `border: 1px solid rgb(198, 198, 198)`
- **影响**: 表格与背景融合

### 9. "普通用户"角色标签背景不明显
- **位置**: `.role-tag.role--user`
- **现状**: 白色背景 `rgb(255, 255, 255)`，与页面背景相同
- **期望**: 浅灰色背景 `rgb(244, 244, 244)` 以区分其他状态
- **影响**: 普通用户标签几乎不可见

### 10. 表格行无左侧边框
- **位置**: `tbody tr`
- **现状**: 无左侧边框
- **期望**: 添加左侧边框或交替行背景色
- **影响**: 行与行之间缺乏视觉分隔

### 11. 表头字体过小
- **位置**: `th`
- **现状**: `fontSize: 12px`
- **期望**: `fontSize: 13px` 或 `14px`
- **影响**: 表头与数据行字体大小差异过大

---

## 服务目录 `/catalog`

### 12. 半数目录卡片尺寸为零（严重）
- **位置**: `.catalog-card`（第 7-13 项）
- **现状**: `height: 0px, width: 0px`（不可见）
- **期望**: 所有卡片应正常显示
- **影响**: 7 个服务不可见，功能缺失

### 13. 目录卡片无阴影
- **位置**: `.catalog-card`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 卡片与背景融合

### 14. 目录网格无间距
- **位置**: 目录网格容器
- **现状**: `gap: normal`（无明确间距）
- **期望**: `gap: 16px`
- **影响**: 卡片之间过于紧密

---

## 应用成员 `/apps/:id/members`

### 15. 成员邀请输入框过宽
- **位置**: `.rail-input`（用户名输入框）
- **现状**: `width: 1966px`（几乎占满整个页面）
- **期望**: `width: 300-400px`（合理表单宽度）
- **影响**: 表单布局失衡，输入框过长

### 16. 输入框无圆角
- **位置**: `.rail-input`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 输入框边缘生硬

### 17. 下拉框无圆角
- **位置**: `.rail-select`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 下拉框边缘生硬

### 18. 移除按钮尺寸过小
- **位置**: 移除按钮
- **现状**: 宽度 46px，高度 28px
- **期望**: 宽度 60px，高度 32px
- **影响**: 操作区域视觉不一致

### 19. 移除按钮无圆角
- **位置**: 移除按钮
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 按钮边缘生硬

---

## 应用概览 `/apps/:id`

### 20. 环境卡片无圆角
- **位置**: `.env-card`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 卡片边缘生硬

### 21. 环境卡片无阴影
- **位置**: `.env-card`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 卡片与背景融合

### 22. 区域卡片无圆角
- **位置**: `.section-card`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 卡片边缘生硬

### 23. 区域卡片无阴影
- **位置**: `.section-card`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 卡片与背景融合

### 24. 事件列表无分隔线
- **位置**: 事件列表项
- **现状**: 无 `borderBottom`
- **期望**: 添加 `border-bottom: 1px solid rgb(244, 244, 244)`
- **影响**: 事件之间缺乏视觉分隔

---

## 环境详情 `/apps/:id/environments/:id`

### 25. 画布节点无圆角
- **位置**: `.component-canvas-node`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 节点边缘生硬

### 26. 画布节点无阴影
- **位置**: `.component-canvas-node`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 节点与背景融合

### 27. 非活跃节点边框颜色过浅
- **位置**: `.component-topology-node`（非活跃状态）
- **现状**: `border: 1px solid rgb(198, 198, 198)`
- **期望**: `border: 1px solid rgb(224, 224, 224)`
- **影响**: 节点边界不清晰

### 28. 缩放工具栏无边框
- **位置**: 缩放控制区域
- **现状**: 无边框
- **期望**: 添加边框或背景色
- **影响**: 工具栏与画布融合

### 29. 标签页激活指示器不明显
- **位置**: `.ws-tab.active`
- **现状**: 仅底部 2px 蓝色边框
- **期望**: 增加背景色变化或更明显的指示器
- **影响**: 当前标签页不够突出

### 30. 标签页容器无底部边框
- **位置**: `.ws-tabs`
- **现状**: `borderBottom: 1px solid rgb(198, 198, 198)`
- **期望**: 保持（已正确）
- **备注**: 此项为正面示例

---

## 创建应用对话框

### 31. 对话框无圆角
- **位置**: `[role="dialog"]`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 8px`
- **影响**: 对话框边缘生硬

### 32. 对话框无阴影
- **位置**: `[role="dialog"]`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: 0 4px 16px rgba(0,0,0,0.12)`
- **影响**: 对话框与背景融合

### 33. 对话框输入框无圆角
- **位置**: 对话框内的 input/textarea
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 输入框边缘生硬

### 34. 对话框按钮无圆角
- **位置**: 对话框内的 button
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 按钮边缘生硬

### 35. 关闭按钮尺寸过小
- **位置**: 对话框关闭按钮
- **现状**: 宽度 28px，高度 28px
- **期望**: 宽度 32px，高度 32px
- **影响**: 点击区域过小

---

## 服务抽屉 (Redis)

### 36. 抽屉无圆角
- **位置**: `.config-drawer`
- **现状**: `borderRadius: 0px`
- **期望**: 左侧 `borderRadius: 8px 0 0 8px`
- **影响**: 抽屉边缘生硬

### 37. 抽屉无阴影
- **位置**: `.config-drawer`
- **现状**: `boxShadow: none`
- **期望**: `box-shadow: -4px 0 16px rgba(0,0,0,0.08)`
- **影响**: 抽屉与主内容融合

### 38. 抽屉头部无底部分隔线
- **位置**: `.config-drawer-header`
- **现状**: `borderBottom: 0px none`
- **期望**: `border-bottom: 1px solid rgb(198, 198, 198)`
- **影响**: 头部与内容区域缺乏视觉分隔

### 39. 抽屉导航项无激活指示
- **位置**: 抽屉顶部导航（部署、数据、接入等）
- **现状**: 激活项无底部边框或其他视觉指示
- **期望**: 激活项应有明显视觉区分（如底部边框、背景色变化）
- **影响**: 用户难以区分当前所在标签页

### 40. 抽屉文章卡片无圆角
- **位置**: 抽屉内的 article 元素（存储卷配置）
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 卡片边缘生硬

### 41. 抽屉输入框边框颜色不一致
- **位置**: 抽屉内的 select/input
- **现状**: `border: 1px solid rgb(141, 141, 141)`
- **期望**: `border: 1px solid rgb(198, 198, 198)`（与其他页面一致）
- **影响**: 边框颜色不统一

### 42. 抽屉按钮尺寸不一致
- **位置**: 抽屉内的按钮
- **现状**: 多种尺寸（28x28, 60x48, 70x28, 104x36）
- **期望**: 统一按钮尺寸规范
- **影响**: 操作区域视觉不协调

### 43. 抽屉按钮无圆角
- **位置**: 抽屉内的所有按钮
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 按钮边缘生硬

---

## 模板页面 `/templates`

### 44. 模板行无圆角
- **位置**: `.template-row`
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 行边缘生硬

### 45. 模板行无边框
- **位置**: `.template-row`
- **现状**: 无边框
- **期望**: 添加底部分隔线 `border-bottom: 1px solid rgb(244, 244, 244)`
- **影响**: 模板之间缺乏视觉分隔

### 46. 模板行无阴影
- **位置**: `.template-row`
- **现状**: `boxShadow: none`
- **期望**: 添加悬停阴影效果
- **影响**: 交互反馈不明显

### 47. 标题层级不清晰
- **位置**: H1 和 H2
- **现状**: H1=24px, H2=24px（相同大小）
- **期望**: H1=28px, H2=20px（层级区分）
- **影响**: 标题层级不清晰

---

## 全局问题

### 48. 圆角风格不统一
- **问题**: 仅目录卡片有圆角（8px），其他所有卡片、对话框、抽屉、按钮、输入框均为 0px
- **建议**: 全局统一 `borderRadius: 8px`（卡片）、`4px`（按钮/输入框）
- **影响**: 全局视觉不一致

### 49. 阴影风格不统一
- **问题**: 几乎所有卡片、对话框、抽屉均无阴影
- **建议**: 统一添加轻微阴影 `box-shadow: 0 2px 8px rgba(0,0,0,0.08)`
- **影响**: 全局缺乏层次感

### 50. 按钮样式不统一
- **问题**: 不同页面按钮圆角、大小、颜色不一致
  - 主按钮：36px 高，无圆角
  - 次按钮：28px 高，无圆角
  - 图标按钮：28x28 或 32x32，无圆角
- **建议**: 建立统一的按钮设计规范
- **影响**: 操作区域视觉不协调

### 51. 表单元素样式不统一
- **问题**: 输入框、下拉框圆角、边框风格不一致
  - 有些输入框边框 `rgb(198, 198, 198)`，有些 `rgb(141, 141, 141)`
  - 所有表单元素无圆角
- **建议**: 统一表单元素样式
- **影响**: 表单视觉不一致

### 52. 侧边栏导航项无圆角
- **位置**: 侧边栏导航项
- **现状**: `borderRadius: 0px`
- **期望**: `borderRadius: 4px`
- **影响**: 导航项边缘生硬

### 53. 侧边栏激活项背景不明显
- **位置**: `.nav-item.active`
- **现状**: 白色背景 `rgb(255, 255, 255)`
- **期望**: 浅蓝色背景或左侧边框指示器
- **影响**: 当前页面导航不够突出

### 54. 页面头部无统一样式
- **问题**: 不同页面头部样式不一致
  - 有些有底部分隔线，有些没有
  - 有些有内边距，有些没有
- **建议**: 统一页面头部样式
- **影响**: 页面切换时视觉不连贯

### 55. 代码元素无样式
- **位置**: 代码元素（如地址、URL）
- **现状**: 无背景色、无边框、无圆角
- **期望**: 添加浅灰色背景、边框、小圆角
- **影响**: 代码元素与普通文本无区分

### 56. 间距不统一
- **问题**: 不同页面、不同组件之间的间距不一致
  - 有些使用 `16px`，有些使用 `24px`，有些使用 `32px`
- **建议**: 建立统一的间距规范（8px 倍数）
- **影响**: 布局不整齐

### 57. 字体大小不统一
- **问题**: 不同位置字体大小不一致
  - 表头 `12px`，数据行 `14px`，标题 `24px` 或 `28px`
- **建议**: 建立统一的字体大小规范
- **影响**: 文字层次不清晰

---

## 修复优先级

| 优先级 | 问题 | 影响范围 |
|--------|------|----------|
| P0 | #12 目录卡片尺寸为零 | 功能缺失 |
| P0 | #39 抽屉导航无激活指示 | 可用性 |
| P0 | #58 画布节点无圆角 | 画布视觉 |
| P0 | #59 画布节点无阴影 | 画布视觉 |
| P1 | #15 成员输入框过宽 | 布局失衡 |
| P1 | #48 圆角风格不统一 | 全局视觉 |
| P1 | #49 阴影风格不统一 | 全局视觉 |
| P1 | #50 按钮尺寸不一致 | 视觉一致性 |
| P1 | #60 画布节点无悬停效果 | 交互反馈 |
| P1 | #61 画布节点选中状态不明显 | 交互反馈 |
| P2 | #1, #7, #20, #22, #25, #31, #36, #44 卡片/对话框/抽屉圆角 | 视觉一致性 |
| P2 | #2, #13, #21, #23, #26, #32, #37, #46 卡片/对话框/抽屉阴影 | 视觉层次 |
| P2 | #16, #17, #33, #34, #40, #43 表单元素圆角 | 视觉一致性 |
| P2 | #52 侧边栏导航项圆角 | 视觉一致性 |
| P2 | #55 代码元素样式 | 可读性 |
| P2 | #56 间距统一 | 布局整齐度 |
| P2 | #62 画布节点边框颜色过时 | 画布视觉 |
| P3 | #9 普通用户标签背景 | 可读性 |
| P3 | #18, #19 移除按钮尺寸 | 视觉一致性 |
| P3 | #24 事件列表分隔线 | 视觉层次 |
| P3 | #47 标题层级 | 文字层次 |
| P3 | #63 画布节点无状态指示器 | 状态显示 |
| P3 | #64 画布连线无动画 | 交互反馈 |
| P3 | #65 画布背景过于简单 | 视觉深度 |

---

## 建议修复方案

### 1. 创建全局样式系统
在 `src/styles/` 中定义 CSS 变量：
```css
:root {
  --border-radius-sm: 4px;
  --border-radius-md: 8px;
  --border-radius-lg: 12px;
  --box-shadow-sm: 0 1px 3px rgba(0,0,0,0.06);
  --box-shadow-md: 0 2px 8px rgba(0,0,0,0.08);
  --box-shadow-lg: 0 4px 16px rgba(0,0,0,0.12);
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --spacing-lg: 24px;
  --spacing-xl: 32px;
  --color-border: rgb(198, 198, 198);
  --color-border-light: rgb(224, 224, 224);
  --color-bg-hover: rgb(244, 244, 244);
}
```

### 2. 统一组件样式
- **卡片组件**: 统一 `borderRadius: 8px`, `boxShadow: var(--box-shadow-md)`
- **按钮组件**: 统一 `borderRadius: 4px`, 尺寸规范（sm: 28px, md: 36px, lg: 44px）
- **输入框组件**: 统一 `borderRadius: 4px`, `border: 1px solid var(--color-border)`
- **对话框组件**: 统一 `borderRadius: 8px`, `boxShadow: var(--box-shadow-lg)`
- **抽屉组件**: 统一左侧圆角 `borderRadius: 8px 0 0 8px`, `boxShadow: var(--box-shadow-lg)`

### 3. 检查 Catalog 渲染逻辑
排查为什么 7 个卡片尺寸为零（可能是 v-if/v-show 条件问题）

### 4. 修复成员输入框宽度
限制输入框最大宽度为 `400px`

### 5. 统一间距规范
使用 8px 倍数系统：4px, 8px, 12px, 16px, 24px, 32px, 48px

### 6. 统一字体大小规范
- 标题: H1=28px, H2=24px, H3=20px, H4=16px
- 正文: 14px
- 辅助文字: 12px
- 表头: 13px

---

## 画布节点样式修复方案（基于竞品分析）

### 推荐的画布节点 CSS

```css
/* 画布节点基础样式 */
.component-canvas-node {
  border-radius: 8px;                    /* 与 React Flow 默认一致 */
  border: 1px solid rgba(0, 0, 0, 0.08); /* 半透明边框，更现代 */
  background-color: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);  /* 默认阴影 */
  transition: all 0.15s ease;             /* 平滑过渡 */
}

/* 悬停状态 */
.component-canvas-node:hover {
  box-shadow: 0 1px 4px 1px rgba(0, 0, 0, 0.08);  /* React Flow 默认 */
  border-color: rgba(0, 0, 0, 0.12);
}

/* 选中状态 */
.component-canvas-node.active {
  border-color: rgb(15, 98, 244);
  box-shadow:
    0 0 0 2px rgba(15, 98, 244, 0.15),  /* 外发光 */
    0 1px 4px 1px rgba(0, 0, 0, 0.08);   /* 阴影 */
}

/* 状态指示器 */
.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  position: absolute;
  top: 8px;
  right: 8px;
}

.status-indicator.running { background-color: #22c55e; }  /* 绿色 */
.status-indicator.pending { background-color: #f59e0b; }  /* 琥珀色 */
.status-indicator.error { background-color: #ef4444; }    /* 红色 */
.status-indicator.unknown { background-color: #9ca3af; }  /* 灰色 */
```

### 推荐的画布背景

```css
/* 点阵背景（React Flow 默认风格） */
.canvas-container {
  background-color: #fafafa;
  background-image: radial-gradient(circle, #e5e7eb 1px, transparent 1px);
  background-size: 20px 20px;
}
```

### 推荐的连线样式

```css
/* 连线基础样式 */
.react-flow__edge-path {
  stroke: #b1b1b7;
  stroke-width: 1.5;
}

/* 悬停时显示数据流动画 */
.react-flow__edge:hover .react-flow__edge-path {
  stroke: #1a192b;
  stroke-dasharray: 5 5;
  animation: flowAnimation 1s linear infinite;
}

@keyframes flowAnimation {
  to { stroke-dashoffset: -10; }
}
```

### 实施优先级

| 优先级 | 改动 | 预期效果 |
|--------|------|----------|
| P0 | 添加 `border-radius: 8px` | 立即提升现代感 |
| P0 | 添加 `box-shadow` | 增加层次感 |
| P1 | 添加悬停效果 | 提升交互反馈 |
| P1 | 优化选中状态 | 更明显的选中指示 |
| P2 | 添加状态指示器 | 更直观的状态显示 |
| P2 | 添加点阵背景 | 增加画布深度感 |
| P3 | 连线动画 | 增加活力 |

---

## 附录：检查方法

使用 Playwright MCP 工具进行自动化检查：
1. 导航到每个页面
2. 使用 `browser_evaluate` 执行 JavaScript 获取元素样式
3. 分析 `getComputedStyle()` 返回的 CSS 属性
4. 记录所有不符合设计规范的问题

检查的 CSS 属性包括：
- `borderRadius` - 圆角
- `boxShadow` - 阴影
- `border` - 边框
- `padding` - 内边距
- `margin` - 外边距
- `fontSize` - 字体大小
- `fontWeight` - 字体粗细
- `color` - 颜色
- `backgroundColor` - 背景色
- `width` / `height` - 尺寸

---

## 四、CSS Design Token 审计 (2026-06-26)

> 对比 `carbon-theme.css`（Carbon Design System token）与各 `.vue` 文件的 CSS 使用情况。
> 只做记录，未改码。

### 4.1 样式架构现状

| 层级 | 文件 | 角色 |
|------|------|------|
| Carbon 基础 Token | `frontend/src/styles/carbon-theme.css` (898行) | 定义所有 `--cds-*` 色阶、间距、排版、White + g90 主题 |
| PAAP 语义层 | `frontend/src/style.scss` (461行) | 定义 `--paap-*` 变量，fallback 到 `--cds-*` |
| 组件自定 | 各 `.vue` `<style scoped>` | 组件内直接写 CSS |

**导入链正确**：`main.ts` 先 `import './styles/carbon-theme.css'` 再 `import './style.scss'`。

### 4.2 三种 CSS 模式并存（不一致）

| 模式 | 写法 | 使用文件 |
|------|------|---------|
| **A. 直接 Carbon token** | `var(--cds-layer-01)` | `TemplatesView.vue`, `PlatformUsersView.vue`, `LoginView.vue` ✅ |
| **B. PAAP token → Carbon fallback** | `var(--paap-panel)` = `var(--cds-layer-01, #fff)` | `EnvDetailView.vue`, Workspace 系列组件 |
| **C. 全硬编码** | `#11181c`, `#687076` | `ComponentDetailView.vue`, `AppDeployView.vue`, `AppRegistryView.vue` ❌ |

### 4.3 色值污染——混用非 Carbon 色值

> **严重问题**：代码中混入了 Tailwind CSS 和 Google Material Design 的色值，与 IBM Carbon Design System 不兼容。

#### 非 Carbon 蓝色（Google Blue / Tailwind Blue）

| 文件中出现的位置 | 当前值 | 来源 | 应替换为 |
|-----------------|--------|------|---------|
| `CatalogView.vue:542` | `#1967d2` | Google Blue 600 | `--cds-blue-60` `#0f62fe` |
| `CatalogView.vue:541` | `#e8f0fe` | Google Blue soft | `--cds-blue-20` `#d0e2ff` |
| `PlatformUsersView.vue:244-245` | `#e8f0fe` / `#1967d2` | Google Blue | `--cds-blue-20` / `--cds-blue-60` |
| `AppCIView.vue:154, 193` | `#3b82f6` / `#1d4ed8` / `#eff6ff` | Tailwind blue-500/700/50 | `--cds-link-primary` / `--cds-blue-70` / `--cds-blue-10` |
| `AppDeployView.vue:181` | `#3b82f6` | Tailwind blue-500 | `--cds-link-primary` `#0f62fe` |
| `AppMonitorView.vue:208, 240` | `#3b82f6` / `#1d4ed8` | Tailwind blue-500/700 | `--cds-link-primary` / `--cds-blue-70` |
| `AppRegistryView.vue:178, 186` | `#1d4ed8` / `#eff6ff` | Tailwind blue-700/50 | `--cds-blue-70` / `--cds-blue-10` |
| `AppEnvironmentsView.vue:367` | `#2563eb` / `#eff6ff` | Tailwind blue-600/50 | `--cds-blue-60` / `--cds-blue-10` |
| `ComponentDetailView.vue:552, 634, 679, 730, 745, 797` | `#2563eb` / `#1d4ed8` / `#eff6ff` / `#93c5fd` / `#bfdbfe` | Tailwind blues | `--cds-blue-*` 对应色阶 |
| `ArgocdWorkspace.vue:514, 605` | `#dbeafe` / `#1d4ed8` | Tailwind blue-100/700 | `--cds-blue-20` / `--cds-blue-70` |

#### 非 Carbon 灰色（Tailwind Gray）

| 文件中出现的位置 | 当前值 | 应替换为 |
|-----------------|--------|---------|
| `ComponentDetailView.vue`, `AppDeployView.vue`, `AppCIView.vue`, `AppMonitorView.vue`, `AppRegistryView.vue` 大量使用 | `#11181c` (tailwind gray-900) | `--cds-text-primary` `#161616` |
| 同上组文件 | `#687076` (tailwind gray-500) | `--cds-text-secondary` `#525252` 或 `--cds-gray-60` `#6f6f6f` |
| 同上组文件 | `#e6e8eb` (tailwind gray-200) | `--cds-border-subtle-01` `#e0e0e0` |
| 同上组文件 | `#f1f3f5` (tailwind gray-50) | `--cds-gray-10` `#f4f4f4` |
| 同上组文件 | `#f9fafb` (tailwind gray-50) | `--cds-gray-10` `#f4f4f4` |
| 同上组文件 | `#9ba1a6` | `--cds-gray-50` `#8d8d8d` |
| 同上组文件 | `#d1d5db` (tailwind gray-300) | `--cds-gray-30` `#c6c6c6` |
| 同上组文件 | `#9ca3af` (tailwind gray-400) | `--cds-gray-50` `#8d8d8d` |

#### 硬编码颜色统计

| 项 | 数量 |
|:---|---:|
| 硬编码 `color:` 值（所有 `.vue`） | ~275 处 |
| 硬编码 `background:` 值（所有 `.vue`） | ~167 处 |
| 含硬编码值的 `.vue` 文件 | 15+ |

#### 重灾区（按硬编码数量排序）

| 文件 | 硬编码数 | 使用色值体系 |
|------|---------|-------------|
| `frontend/src/views/ComponentDetailView.vue` | ~87 | Tailwind 色值（`#11181c`, `#687076`, `#e6e8eb`） |
| `frontend/src/views/AppDeployView.vue` | ~45 | Tailwind 色值，完全不用 CSS 变量 |
| `frontend/src/views/AppMonitorView.vue` | ~35 | Tailwind 色值 |
| `frontend/src/views/EnvDetailView.vue` | ~34 | 混合 `--paap-*` + 硬编码 |
| `frontend/src/views/AppCIView.vue` | ~31 | Tailwind 色值 |
| `frontend/src/views/CatalogView.vue` | ~26 | 混合 Carbon + 硬编码 |
| `frontend/src/views/AppEnvironmentsView.vue` | ~17 | 混合 `--paap-*` + Carbon fallback + 硬编码 |
| `frontend/src/views/AppRegistryView.vue` | ~16 | Tailwind 色值 |
| `frontend/src/components/workspaces/LogWorkspace.vue` | ~9 | Log level 颜色（`#7f1d1d`, `#991b1b` 等） |
| `frontend/src/components/workspaces/ArgocdWorkspace.vue` | ~9 | 混合硬编码 |

#### LogWorkspace 硬编码问题（典型）

```css
.log-level.fatal { background: #7f1d1d; color: #fecaca; }
.log-level.error { background: #991b1b; color: #fecaca; }
.log-level.warn  { background: #854d0e; color: #fef08a; }
.log-level.info  { background: #14532d; color: #bbf7d0; }
.log-level.debug { background: #374151; color: #d1d5db; }
.log-level.log   { background: #1f2937; color: #9ca3af; }
```
全部是 Tailwind UI 色值，应使用 `--cds-support-error`、`--cds-support-warning`、`--cds-support-success`。

#### ToolWorkspaceFrame 深色编辑器背景硬编码

```css
background: #0f1117;   /* Gitea 编辑器（line 167） */
background: #111827;   /* Terminal（line 177） */
```

### 4.4 `--paap-*` 变量与 Carbon Token 映射错位

**间距值偏移**（PAAP 比 Carbon 少 1 个索引）：

| PAAP | 值 | 应映射到的 Carbon Token | 实际 Carbon 值 |
|------|----|------------------------|---------------|
| `--paap-space-1` | 4px | `--cds-spacing-02` (4px) | `0.25rem` |
| `--paap-space-2` | 8px | `--cds-spacing-03` (8px) | `0.5rem` |
| `--paap-space-3` | 12px | `--cds-spacing-04` (12px) | `0.75rem` |
| `--paap-space-4` | 16px | `--cds-spacing-05` (16px) | `1rem` |
| `--paap-space-5` | 20px | → Carbon 无 20px 间距（跳过） | — |
| `--paap-space-6` | 24px | `--cds-spacing-06` (24px) | `1.5rem` |
| `--paap-space-8` | 32px | `--cds-spacing-07` (32px) | `2rem` |
| `--paap-space-10` | 40px | `--cds-spacing-08` (40px) | `2.5rem` |

**圆角**：`--paap-radius: 0` / `--paap-radius-sm: 0` — Carbon 的 `--cds-border-radius-sm` 也是 0（Carbon 产品 UI 圆角默认为 0）。

**间距 5（20px）** 在 Carbon spacing 系统里没有直接对应值（1rem → 2rem → 4rem，中间无 20px）。

### 4.5 Border-radius 不一致

全局存在 7 种不同的 `border-radius` 值，没有统一使用 token：

| 值 | 出现文件 | 用途 |
|----|---------|------|
| `0px` | `LoginView.vue`, `style.scss` (via `--paap-radius-sm`) | 按钮、输入框 |
| `3px` | `CatalogView.vue`, `PlatformUsersView.vue`, `ArgocdWorkspace.vue` | tags, badges |
| `4px` | `PlatformUsersView.vue`, `AppDeployView.vue`, `AppMonitorView.vue` | badges |
| `6px` | `AppDeployView.vue`, `AppCIView.vue`, `AppMonitorView.vue` | 按钮（`rail-btn`） |
| `8px` | `CatalogView.vue`, `AppDeployView.vue`, `AppMonitorView.vue`, `AppRegistryView.vue` | 卡片、section |
| `10px` | `AppDeployView.vue`, `AppMonitorView.vue` | tab-badge |
| `9999px` (pill) | `style.scss` (via `--paap-radius-full`) | tags |

**关键冲突**：
- `style.scss` 的 `.rail-btn` 使用 `--paap-radius-sm` (= `0px`)
- `AppDeployView.vue:255` / `AppCIView.vue:204` / `AppMonitorView.vue:265` 各自重新定义 `.rail-btn` 并设 `border-radius: 6px`
- `CatalogView.vue:465` 卡片 `border-radius: 8px`，而 DESIGN.md 说 cards 0px

### 4.6 按钮定义重复（3 个文件各自重写 `.rail-btn`）

`style.scss` 已正确定义 `.rail-btn`，但 3 个视图文件各自重新定义了自己的 `rail-btn` 类：

```css
/* style.scss:189 — 官方定义 */
.rail-btn { font-size: 13px; font-weight: 500; border-radius: var(--paap-radius-sm); }

/* AppDeployView.vue:255 — 重复定义，覆盖官方 */
.rail-btn { font-size: 13px; font-weight: 600; border-radius: 6px; color: #11181c; border: 1px solid #e6e8eb; }

/* AppCIView.vue:204 — 同上 */
.rail-btn { font-size: 13px; font-weight: 600; border-radius: 6px; ... }

/* AppMonitorView.vue:265 — 同上 */
.rail-btn { font-size: 13px; font-weight: 600; border-radius: 6px; ... }
```

**差异**：这些重定义使用 `font-weight: 600`（官方为 500）、`border-radius: 6px`（官方为 0px）、硬编码色值而非 token。

### 4.7 排版（Typography）未使用 Carbon Token

所有 `.vue` 文件的 `font-size` 均为硬编码 px 值，未引用 `--cds-*-font-size`、`--cds-type-scale-*` 或 `--cds-heading-*-font-size` token。

**典型问题**：

| 文件 | 当前写法 | 应使用 |
|------|---------|-------|
| `LoginView.vue:153` | `font-size: 28px` | `--cds-heading-05-font-size` (32px) 或自定义 |
| `LoginView.vue:175` | `font-size: 24px` | `--cds-heading-03-font-size` (20px) 或 `--cds-heading-04-font-size` (28px) |
| `MainLayout.vue:204` | `font-size: 18px` | `--cds-body-02-font-size` (16px) 或 `--cds-heading-03-font-size` (20px) |
| `AppLayout.vue:376` | `font-size: 16px` | `--cds-body-02-font-size` (16px) ✅ 但应使用 token 而非硬编码 |
| `AppLayout.vue:504` | `font-size: 18px` | `--cds-productive-heading-03-font-size` (20px) |
| `ComponentDetailView.vue` 多处 | 各种 `font-size` 混合 | 统一改用 `--cds-*-font-size` |
| `AppDeployView.vue:184` | `font-size: 22px` | `--cds-productive-heading-04-font-size` (28px) |
| `EnvDetailView.vue` | `font-size: 12px, 13px, 14px, 16px, 18px, 20px, 24px` (硬编码) | 统一改用 `--cds-*-font-size` |

**全项目 font-size 趋势**：全部硬编码 px，没有一个引用 `--cds-` 字号 token。

### 4.8 Tab 系统不一致（3+ 种实现）

| Tab 实现 | 位置 | 选中态指示器 | 活跃 tab color |
|----------|------|-------------|----------------|
| `.catalog-tabs` | `CatalogView.vue` | `border-bottom: 2px solid #0f62fe` | `#161616` 硬编码 |
| `.ws-tabs` / `.ws-tab` | `style.scss` | `border-bottom: 2px solid var(--paap-accent)` | `var(--paap-text)` |
| `.config-drawer-tabs` | `EnvDetailView.vue` | `::after` 伪元素 bottom bar | `var(--cds-text-primary)` |
| `.env-tab` | `AppDeployView.vue`, `AppMonitorView.vue` | `background: #11181c` 全填色切换 | `#fff` / `#11181c` 硬编码 |

**问题**：
- 选中态指示器的颜色、厚度、动画均不一致
- `AppDeployView`/`AppMonitorView` 的 `.env-tab` 使用 Tailwind 色值（`#11181c` 填色切换），与 Carbon tab 设计（底部分隔线指示器）完全不同

### 4.9 Transition 过渡不一致

全项目存在 8+ 种不同的 `transition` 写法：

| 写法 | 使用文件数 | 是否使用 Carbon motion token |
|------|-----------|---------------------------|
| `all 0.15s` | 9 处 | ❌ |
| `border-color 110ms, box-shadow 110ms` | 7 处 | ❌（值正确但硬编码） |
| `background 0.1s` | 6 处 | ❌ |
| `border-color 0.15s, box-shadow 0.15s` | 4 处 | ❌ |
| `all 0.15s ease` | 3 处 | ❌ |
| `border-color 110ms, color 110ms, box-shadow 110ms` | 3 处 | ❌ |
| `border-color 110ms, background 110ms, color 110ms` | 2 处 | ❌ |
| `background 110ms, color 110ms, border-color 110ms` | 2 处 | ❌ |
| 其他单次使用 | 多 | ❌ |

**应使用** `--cds-motion-duration-fast: 110ms`、`--cds-motion-duration-moderate-01: 160ms`、`--cds-motion-easing-standard-productive` 等 Carbon motion token。

### 4.10 Focus Ring 不一致

| 位置 | 当前 focus ring | 应使用 |
|------|----------------|-------|
| `style.scss:394` (`rail-input:focus`) | `0 0 0 3px rgba(37,99,235,0.1)` | `--cds-focus` + `--cds-border-interactive` |
| `ComponentDetailView.vue:599` | `0 0 0 2px rgba(17,24,28,0.08)` | `--cds-focus` `#0f62fe` |
| `ArgocdWorkspace.vue:590` | `0 0 0 3px rgba(15,98,254,0.12)` | `--cds-focus` |
| `EnvDetailView.vue:8946-8947` | `outline: 2px solid var(--cds-focus, ...)` | ✅ 正确使用 Carbon token |
| `LoginView.vue:212` | `box-shadow: inset 0 0 0 2px var(--cds-border-interactive)` | ✅ 正确 |

**问题**：EnvDetailView 部分区域正确使用了 `--cds-focus`，但 style.scss 和 ComponentDetailView 使用硬编码的 focus ring。

### 4.11 Shadow 使用与 DESIGN.md 冲突

DESIGN.md 明确说 "Carbon resists drop shadows — depth is carried by surface change and 1px hairlines"，但发现：

| 位置 | 使用 | 冲突 |
|------|------|------|
| `CatalogView.vue:475` (card hover) | `box-shadow: 0 2px 6px rgba(0,0,0,0.1)` | ❌ 卡片不应有阴影 |
| `CatalogView.vue:470` | `transition: box-shadow 0.2s ease, transform 0.2s ease` | ❌ 悬停上浮 |
| `AppMonitorView.vue:245` (link-card hover) | `box-shadow: 0 2px 8px rgba(0,0,0,0.06)` | ❌ |
| `ArgocdWorkspace.vue:580` | `box-shadow: 0 1px 2px rgba(15,23,42,0.08)` | ❌ |
| `AppLayout.vue:436` | `box-shadow: 0 0 0 3px rgba(37,99,235,0.1)` | focus ring 误用 box-shadow |
| `LoginView.vue:209` | `transition: box-shadow 0.15s` | 正确（focus transition） |

### 4.12 非标准 Font-weight

Carbon Design System 定义的字重只有 `300`、`400`、`600`。但代码中出现：

| 位置 | 值 | 问题 |
|------|----|------|
| `EnvDetailView.vue:9449` | `font-weight: 750` | ❌ 非标准 |
| `EnvDetailView.vue:9459` | `font-weight: 720` | ❌ 非标准 |
| `EnvDetailView.vue:9922` | `font-weight: 550` | ❌ 非标准 |

应使用 `600` (semibold) 或 `700` (bold) 替代这些中间值。

### 4.13 carbon-theme.css 不完整项

| 缺失项 | 影响 |
|--------|------|
| `--cds-cool-gray-*` 色阶未定义 | `--cds-tag-background-cool-gray` 等 Tag token 引用缺失 |
| `--cds-warm-gray-*` 色阶未定义 | `--cds-tag-background-warm-gray` 等 Tag token 引用缺失 |
| g90 主题中 `--cds-notification-action-hover: ;` 空值 | 引用时会回退为无效值 |

### 4.14 汇总与建议迁移路径

**核心问题总结**：

| # | 问题 | 严重程度 | 影响文件数 |
|---|------|---------|-----------|
| 1 | 色值污染（Tailwind + Google Blue + 硬编码） | 🔴 高 | 10+ |
| 2 | Button 定义重复覆盖 | 🟡 中 | 3 |
| 3 | Tab 系统 3+ 种实现 | 🟡 中 | 4 |
| 4 | 排版全部硬编码 | 🟡 中 | 15+ |
| 5 | border-radius 7 种值混用 | 🟡 中 | 8+ |
| 6 | transition 8+ 种写法 | 🟢 低 | 10+ |
| 7 | focus ring 不一致 | 🟢 低 | 5+ |
| 8 | 非 Carbon 阴影使用 | 🟡 中 | 4 |
| 9 | 非标准 font-weight | 🟢 低 | 1 |

**建议迁移顺序**：

1. **（P0）色值标准化**：清理 Tailwind/Google 色值，全部使用 `--cds-*` token，从 AppDeployView、AppCIView、AppMonitorView、AppRegistryView、ComponentDetailView 开始
2. **（P1）Button 去重**：删除 3 个文件中重复定义的 `.rail-btn`，统一使用 `style.scss` 的定义
3. **（P1）Tab 统一**：抽象一个共享 tab 组件或至少统一选中态指示器样式（底部分隔线 2px blue）
4. **（P2）排版 token 化**：逐步将 `font-size` 硬编码改为 `--cds-*-font-size`
5. **（P2）Transition 统一**：使用 `--cds-motion-duration-fast: 110ms` 和 `--cds-motion-easing-standard-productive`
6. **（P3）Focus ring 统一**：全部使用 `--cds-focus` + `--cds-border-interactive`
7. **（P3）Shadow 清理**：移除卡片 hover 阴影，改用表面切换
8. **viewMarkup.test.ts** 已有 1700+ 行 CSS token 验证断言，每次修改后补充断言防止回归

---

## 五、深层样式与视觉一致性扫描 (2026-06-26)

> 覆盖画布节点、Workspace 组件、Loading/Modal/Empty 状态、Icon/Accessibility 等。
> 只做记录，未改码。

### 5.1 画布节点与拓扑（Canvas / Topology）

画布节点（`EnvDetailView.vue`）整体质量较好，使用 Carbon token + `--paap-*` fallback：

| 属性 | 当前实现 | 评价 |
|------|---------|------|
| 节点背景 | `var(--cds-layer-01, #ffffff)` | ✅ |
| 节点边框 | `var(--cds-border-subtle-01, #e0e0e0)` | ✅ |
| 选中态 | `box-shadow: inset 0 0 0 1px var(--cds-border-interactive)` | ✅ |
| 节点 hover | `var(--cds-border-interactive)` | ✅ |
| 泳道(zone)背景 | `rgba(var(--cds-white-rgb), 0.72)` | ✅ |
| 泳道 shared | `var(--cds-blue-20, #d0e2ff)` | ✅ |
| rename input radius | `border-radius: 3px` | ❌ 非 Carbon 值 |
| rename input border | `var(--cds-interactive-01, #0f62fe)` | ❌ `--cds-interactive-01` 不存在（应为 `--cds-border-interactive`） |
| 节点拖拽 cursor | `cursor: grab` / `grabbing` | ✅ |

**结论**：画布是前端中样式最好的部分之一。

### 5.2 Workspace 组件污染严重

#### ToolWorkspaceFrame.vue（最严重）

```css
/* 表格行 */
border-bottom: 1px solid #f3f4f6;         /* ❌ Tailwind gray-50 */

/* Badge */
background: #f3f4f6;                       /* ❌ Tailwind gray-50 */

/* 黄色状态标签 */
background: #fefce8; color: #a16207;       /* ❌ Tailwind yellow */

/* 编辑器（深色） */
background: #0f1117;                       /* ❌ 硬编码 */
border: 1px solid #1f2937;                 /* ❌ */

/* 终端 */
background: #111827;                       /* ❌ 硬编码 */
color: #e5e7eb;                            /* ❌ */

/* 状态指示器 */
.status-pill.yellow { background: #fefce8; color: #a16207; }  /* ❌ */

/* Shadow */
box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04); /* ❌ 非 Carbon */
```

#### GiteaWorkspace.vue

```css
/* 文件编辑器 */
background: #0f1117;  color: #e5e7eb;        /* ❌ 硬编码深色 */

/* 表格头部 */
background: #f3f4f6;                         /* ❌ Tailwind */

/* 行分隔 */
border-bottom: 1px solid #f3f4f6;            /* ❌ */

/* 字号全硬编码 */
font-size: 16px, 13px, 12px, 11px, 10px;    /* ❌ */
```

#### RedisWorkspace.vue

```css
background: #f3f4f6;   /* ❌ Tailwind gray-50 */
font-size: 全部硬编码   /* ❌ */
```

其他不影响：其余属实用 `--paap-*` 变量。

#### MonitorWorkspace.vue

```css
background: #fff;                              /* ❌ 硬编码 */
border-bottom: 1px solid #f3f4f6;             /* ❌ */
font-size: 全部硬编码                          /* ❌ */
```

#### LogWorkspace.vue（已在 4.3 记录 log level 颜色硬编码）

额外的：
```css
border-bottom: 1px solid #f3f4f6;             /* ❌ */
background: #fff;                              /* ❌ */
```

#### WorkspaceActionForm.vue

```css
/* 错误背景 */
background: #fff7f7;                           /* ❌ 应使用 --cds-red-10 */

/* 非标准字重 */
font-weight: 650;                              /* ❌ Carbon 无 650 */
font-weight: 700;                              /* ❌ Carbon 无 700（可用 600） */

/* 字号全硬编码 */
font-size: 11px, 12px, 14px;                  /* ❌ */
```

### 5.3 Loading Spinner 6 次重复定义

无共享组件，6 个文件各自定义 `.loading-spinner`：

| 文件 | width/height | border | border-top-color | border-radius |
|------|-------------|--------|-----------------|---------------|
| `AppListView.vue:285` | 24px | `var(--paap-border)` | `var(--paap-text)` | `50%` |
| `AppDeployView.vue:188` | 28px | `#e6e8eb` | `#11181c` | `50%` |
| `AppCIView.vue:161` | 28px | `#e6e8eb` | `#11181c` | `50%` |
| `AppMonitorView.vue:215` | 28px | `#e6e8eb` | `#11181c` | `50%` |
| `AppRegistryView.vue:163` | 24px | `#e6e8eb` | `#11181c` | `50%` |
| `PlatformSharedResourcesView.vue` | (使用 class) | — | — | — |

**问题**：
- 5 个文件用 Tailwind 色值，1 个用 CSS 变量
- 尺寸不一致（24px vs 28px）
- 无统一的 `<rail-loading>` 或 `<rail-spinner>` 组件
- `@keyframes spin` 动画也各写各的

### 5.4 Modal 6 个不同实现

6 个文件各自定义 `.modal-overlay` / `.modal-container` / `.modal-header` / `.modal-footer`：

| 文件 | 使用 Token | overlay 颜色 | backdrop-filter | close btn |
|------|-----------|-------------|----------------|-----------|
| `AppEnvironmentsView.vue` | ✅ `--cds-*` | `rgba(17,19,24,0.46)` | `blur(10px)` | 32px |
| `AppOverviewView.vue` | ⚠️ `--paap-*` | `rgba(17,19,24,0.46)` | `blur(10px)` | 28px |
| `AppListView.vue` | ❌ 需检查 | — | — | — |
| `EnvDetailView.vue` | ⚠️ 混合 | — | — | — |
| `TemplatesView.vue` | ⚠️ 混合 | — | — | — |
| `CreateEnvironmentModal.vue` | ✅ `--cds-*` | (无 overlay) | — | — |

**差异**（AppOverviewView vs AppEnvironmentsView）：

| 属性 | AppOverviewView | AppEnvironmentsView |
|------|----------------|---------------------|
| label font-size | `11px` 硬编码 | `var(--cds-label-01-font-size, 12px)` |
| heading font-size | `18px` 硬编码 | `var(--cds-heading-03-font-size, 20px)` |
| heading font-weight | `600` 硬编码 | `var(--cds-heading-03-font-weight, 400)` |
| close btn | 28px | 32px |
| container bg | `var(--paap-panel)` | `var(--cds-layer-01, ...)` |
| overlay bg | `rgba(17,19,24,0.46)` | `rgba(17,19,24,0.46)`（相同） |

**问题**：
- Overlay 使用 `rgba(17,19,24,0.46)` 硬编码 + `backdrop-filter: blur(10px)`，Carbon 的 overlay token 是 `--cds-overlay: rgba(0,0,0,0.6)`
- `backdrop-filter: blur(10px)` — DESIGN.md 明确说 "No atmospheric depth"
- 6 次重复代码，无统一 Modal 组件

### 5.5 无统一 Notification / Toast 系统

**搜索 `notification|toast|alert|snackbar` 在整个 `.vue` 文件中均无匹配**，只有 `MonitorWorkspace.vue` 的 "Alert" 指 K8s Alert 资源，非 UI 通知。

**影响**：
- 错误反馈仅通过内联文字（如 `modalError` 字符串）展示
- 无 Toast 组件用于成功/失败操作的瞬时反馈
- 无 Snackbar 或 Notification banner 组件

### 5.6 无 Carbon 图标系统

| 项 | 值 |
|---|-----|
| 使用 Carbon icon 包 | ❌ 零引用 |
| 在线 `<svg>` 总数 | 106 个全部内联 inline |
| 不含 emoji/文字图标 | 无法统计 |

每个 workspace 组件的 icon 是手动内联 SVG path，未使用 `@carbon/icons-vue` 或 Carbon icon 组件。

### 5.7 无共享 Modal 和 Loading 组件

代码中缺少共享可复用组件：

```
缺少: src/components/rail/rail-modal.vue
缺少: src/components/rail/rail-loading.vue
缺少: src/components/rail/rail-notification.vue
缺少: src/components/rail/rail-toast.vue
```

每个 view 自实现自己的 Modal/Loading，导致 6+ 份重复 CSS 和细微行为差异。

### 5.8 可访问性（Accessibility）

| 项 | 统计 |
|---|------|
| `aria-*` 属性总数 | 126 处 |
| `role=` 属性 | 有限（主要在 modal 上） |
| `tabindex` | 4 处 |
| 键盘导航测试 | 未覆盖 |

**具体问题**：
- 画布节点 `role="button"` 缺失——节点可点击但无语义
- `EnvDetailView.vue` 的 component modal 有 `role="dialog"` + `aria-modal="true"` ✅
- 但 workspace 抽屉无 `aria-label` 或 `role="dialog"`
- Sorting / filtering 控件缺少 `aria-sort` / `aria-pressed`
- 缺少 `:focus-visible` 统一处理（部分组件有，部分没有）

### 5.9 Layout font-weight 非标准

| 文件 | 值 | 应使用 |
|------|----|-------|
| `MainLayout.vue:205` | `font-weight: 700` | `--cds-font-weight-semibold` (600) |
| `AppLayout.vue:377` | `font-weight: 700` | `--cds-font-weight-semibold` (600) |
| `AppLayout.vue:450` | `font-weight: 700` | `--cds-font-weight-semibold` (600) |
| `AppLayout.vue:469` | `font-weight: 700` | `--cds-font-weight-semibold` (600) |
| `AppLayout.vue:480` | `font-weight: 650` | ❌ 不存在，应为 600 |

Carbon 字重系统仅 `300`(light)、`400`(regular)、`600`(semibold)，无 `650` 或 `700`。

### 5.10 汇总

| # | 问题 | 严重程度 | 影响文件数 |
|---|------|---------|-----------|
| 1 | Workspace 组件 Tailwind 色值污染 | 🔴 高 | 6 |
| 2 | Loading spinner 6x 重复定义 | 🟡 中 | 6 |
| 3 | Modal 6 种不同实现 | 🟡 中 | 6 |
| 4 | 无 Notification/Toast 系统 | 🟡 中 | 全项目 |
| 5 | 无 Carbon 图标系统 — 106 个内联 SVG | 🟡 中 | 10+ |
| 6 | 无共享 Modal/Loading 组件 | 🟡 中 | 全项目 |
| 7 | Layout 非标准 font-weight (650/700) | 🟢 低 | 2 |
| 8 | 可访问性待提升（焦点/语义/键盘导航） | 🟢 低 | 全项目 |

**建议**：
1. **（P1）抽象共享组件**：统一 Loading、Modal、Notification 等可复用组件到 `src/components/rail/` 目录
2. **（P1）Workspace 组件色值标准化**：ToolWorkspaceFrame、GiteaWorkspace、MonitorWorkspace、LogWorkspace 中的硬编码 Tailwind 色值替换为 `--cds-*`
3. **（P2）引入 Carbon icon 包**：`@carbon/icons-vue` 替换内联 SVG
4. **（P2）统一 Modal**：以 AppEnvironmentsView 为模板，清理其余 5 个 modal 实现
5. **（P3）Accessibility 审计**：补充 `aria-label`、`role`、键盘事件处理

---

## 六、深层样式扫描第二期 (2026-06-26)

### 6.1 ComponentDetailView — 完全碳化缺失（P1）

| 指标 | 值 |
|------|-----|
| 行数 | 959 |
| `--cds-*` 引用 | **0** |
| `--paap-*` 引用 | **0** |
| 硬编码 `color:` | 65 处 |
| 硬编码 `background:` | 44 处 |
| 硬编码 `font-size:` | 39 处 |
| 硬编码 `border-radius:` | 21 处 |

**硬编码 Tailwind/Google 色值使用频率 TOP5:**
| 色值 | 原始出处 | 出现次数 | 应替换为 |
|------|---------|---------|---------|
| `#687076` | Tailwind gray-500 | 24 | `--cds-text-secondary` |
| `#11181c` | Tailwind gray-900 | 16 | `--cds-text-primary` |
| `#fef2f2` | Tailwind red-50 | 6 | `--cds-red-10` |
| `#f0fdf4` | Tailwind green-50 | 6 | `--cds-green-10` |
| `#eff6ff` | Tailwind blue-50 | 6 | `--cds-blue-10` |

**关键问题：**
- 整个文件没有使用任何 CSS 变量，100% 硬编码
- `.status-badge` 系列（running/stopped/pending/error）使用了 Tailwind 绿/红/蓝色值，没有使用 `--cds-text-success/--cds-text-error/--cds-support-*`
- `.delivery-mode-tag.source` 使用了 `#1d4ed8`（Google Blue 800），非 Carbon 体系色值
- `.config-message` 使用了 Tailwind green/red 色值，应使用 `--cds-notification-*`
- `.env-row.head` 硬编码 `#687076`，应使用 `--cds-text-secondary`
- `.modal-overlay` 虽然与其它 view 一致（rgba 17,19,24,0.46 + backdrop-filter blur），但依然使用硬编码色值而非 `--cds-overlay` 或 `--cds-layer-01` 衍生

---

### 6.2 Dark Mode (g90) 就绪度（P2）

**现状：** `carbon-theme.css` 第 583 行开始定义了完整的 g90（深色背景 `#262626`）主题 CSS 变量，但 **没有代码激活它**。

| 检查项 | 结果 |
|--------|------|
| carbon-theme.css g90 主题 | ✅ 完整定义（583-898行） |
| 代码中切换 `data-carbon-theme="g90"` | ❌ 不存在 |
| 代码中引用 `document.documentElement.setAttribute` | ❌ 不存在 |
| 组件层响应 dark theme | ❌ 未测试 |

**Carbon 标准的 dark mode 激活方式：**
```html
<!-- 在 document 节点设置属性 -->
<html data-carbon-theme="g90">
```
或通过 JS：
```js
document.documentElement.setAttribute('data-carbon-theme', 'g90');
```

**风险：** 即使 g90 主题卡片被激活，许多没有使用 `--cds-*` 变量的组件（如 ComponentDetailView）也不会自动切换——它们是纯硬编码亮色模式。

**受影响的 view（ZERO `--cds-*`，dark mode 不回应）：**
- `AppListView.vue`
- `AppRegistryView.vue`
- `ComponentDetailView.vue`
- `PlatformSharedResourcesView.vue`

**部分回应（部分 `--cds-*`）：**
- `TemplatesView.vue` — 部分使用 `--cds-border-strong-01` 等，但 CSS 选择器未嵌套在 `[data-carbon-theme]` 下

---

### 6.3 响应式断点不一致（P2）

全局 **没有** 定义统一的 breakpoint 变量或 `$breakpoints` map。项目中使用了 **7 种不同的断点值**：

| 断点值 | 出现次数 | 使用位置 |
|--------|---------|---------|
| `768px` | 5 | MainLayout, AppLayout, RedisWs, DatabaseWs, style.scss |
| `640px` | 3 | LoginView, CreateAppView, AppListView |
| `672px` | 3 | CreateEnvironmentModal, AppEnvironmentsView, AppRegistryView |
| `900px` | 2 | ComponentDetailView, MonitorWorkspace |
| `960px` | 1 | AppMembersView |
| `760px` | 1 | ComponentConfigTemplateFields |
| `640px/672px/768px` | 混合 | ArgocdWorkspace（640px + 960px） |

**问题：** 没有遵循 Carbon Design 的断点体系（Carbon 标准断点：`320/672/1056/1312/1584px`）。团队按照"目测"随意取值，在 iPad 竖屏（768px）等常用尺寸上行为不一致。

**建议：** 引入 Carbon 断点 token 或定义统一 `$paap-breakpoints` map。

---

### 6.4 `<input>` / `<select>` 样式一致性（P1）

**结论：.rail-input 和 .rail-select 有 3 套并存定义，互不兼容。**

| 来源 | border-radius | border | 色值体系 | font-size |
|------|--------------|--------|---------|-----------|
| `style.scss`（全局） | `var(--paap-radius-sm)` | `var(--paap-border)` | `--paap-*` | 14px |
| `TemplatesView.vue` | **0**（Carbon 规范） | `var(--cds-border-strong-01)` | `--cds-*` | 14px |
| `ComponentDetailView.vue` | **6px** | `#d7dbdf` | 全硬编码 | 13px |

**关键差异：**
- **border-radius**：style.scss = `--paap-radius-sm`（4px), TemplatesView = `0`（Carbon 规范 0px), ComponentDetailView = `6px`（Railway 风格）
- **:focus 样式**：style.scss 使用 `rgba(37,99,235,0.1)` Google Blue shadow；TemplatesView 使用 `--cds-border-interactive`；ComponentDetailView 使用 `#11181c` black 和 `rgba(17,24,28,0.08)` shadow
- **placeholder**：style.scss 使用 `--paap-muted-2`；TemplatesView 使用 `--cds-text-placeholder`；ComponentDetailView 无 placeholder 样式
- **transition**：style.scss 使用 `0.15s`；TemplatesView 使用 Carbon `110ms`

**组件数量：**
- `.rail-input` 至少 6 个文件使用
- `.rail-select` 至少 5 个文件使用
- 每个文件各自定义，没有共享 mixin 或 `@apply`

---

### 6.5 Z-index 堆叠管理混乱（P2）

**全局没有定义 Z-index 层级体系。** 项目中使用了 10+ 个不同值，按 1 递增随意分布：

| z-index | 使用位置 |
|---------|---------|
| 1 | style.scss（默认）、EnvDetailView |
| 2 | ArgocdWorkspace、EnvDetailView |
| 3 | EnvDetailView |
| 4 | EnvDetailView |
| 10 | EnvDetailView |
| 100 | MainLayout、AppLayout、EnvDetailView |
| 500 | AppLayout（sidebar) |
| 9000 | **所有 modal-overlay**（6 个文件） |
| 9400 | EnvDetailView（drawer) |
| 9500 | EnvDetailView |

**问题：**
- 没有命名层级系统（如 `$z-nav: 100; $z-modal: 900; $z-drawer: 800; $z-toast: 1000`）
- 全部使用 magic number。如果未来需要插入新层（如 toast 在 modal 之上），不知道应取何值
- `9000` 在 6 个 modal 文件中相同但未定义为共享变量
- `9500` 和 `9400` 在 EnvDetailView 中含义不明（可能是 drawer 和 drawer-backdrop，但没有注释）

---

### 6.6 6 个重复的 modal-overlay 实现（P1）

**每个 view 都重复写了几乎完全相同的 modal-overlay/container/header/label/heading 样式**：

```css
/* AppEnvironmentsView 版本 — 已使用 --cds-* */
.modal-overlay { z-index: 9000; background: rgba(17,19,24,0.46); backdrop-filter: blur(10px); }
.modal-container { border-radius: 0; background: var(--cds-layer-01); border: 1px solid var(--cds-border-subtle-01); }

/* AppListView 版本 — --paap-* 老体系 */
.modal-container { border-radius: var(--paap-radius); background: var(--paap-panel); border: 1px solid var(--paap-border); }

/* EnvDetailView 版本 — 混合但 border-bottom 仍用 --paap-border */
.modal-header { border-bottom: 1px solid var(--paap-border); }

/* TemplatesView 版本 — 使用 #161616 rgba 色值 */
.modal-overlay { background: rgba(22, 22, 22, 0.48); }  /* 与其它 view 的 rgba(17,19,24,0.46) 不同 */

/* AppOverviewView 版本 — 缺失 backdrop-filter */
.modal-overlay { /* 没有 backdrop-filter: blur(10px) */ }
```

**微观差异：**
- 4 个 view 使用 `rgba(17,19,24,0.46)`，TemplatesView 使用 `rgba(22,22,22,0.48)`
- AppEnvironmentsView 已升级到 Carbon 规范（`border-radius: 0`），但 AppListView 还维持老版 `var(--paap-radius)`
- `.modal-label` 有的用 `--cds-label-01-*`（AppEnvironmentsView），有的用硬编码 `font-size: 11px`
- `.modal-heading` 有的用 `--cds-heading-03-*`，有的用硬编码 `font-size: 18px`

---

### 6.7 汇总

| # | 问题 | 严重程度 | 影响范围 |
|---|------|---------|---------|
| 1 | ComponentDetailView 0% Carbon token、纯硬编码 | 🔴 P1 | 1 file（959行） |
| 2 | Dark mode g90 从未激活 | 🟡 P2 | 全项目 |
| 3 | 响应式断点 7 种不同值 | 🟡 P2 | 全项目 |
| 4 | .rail-input 3 套定义不兼容 | 🔴 P1 | 6+ files |
| 5 | z-index 无层级管理体系 | 🟡 P2 | 全项目 |
| 6 | 6 个 view 重复 modal-overlay | 🔴 P1 | 6 files |

**建议：**
1. **（P1）ComponentDetailView 碳化迁移**：全文件替换硬编码色值为 `--cds-*`，是最紧急的单文件任务
2. **（P1）抽象 `.rail-input`/`.rail-select`**：在 `style.scss` 中统一定义一次，移除各 view 内的重复定义，使用 Carbon 规范（`border-radius: 0`、`--cds-border-strong-01`、`--cds-field-01`）
3. **（P1）抽象 Modal**：将 modal-overlay/container/header/body/footer 抽为 `<RailModal>` 共享组件，现有 6 个 view 引用
4. **（P2）引入 z-index 命名层**：在 `style.scss` 中定义 `--z-nav: 100; --z-sidebar: 500; --z-drawer: 800; --z-modal: 9000; --z-toast: 9500; --z-tooltip: 10000;`
5. **（P2）激活 dark mode**：在 `main.ts` 或 `App.vue` 中加入 `data-carbon-theme` 切换能力；修复不会响应 `--cds-*` 的 4 个 view
6. **（P2）统一断点体系**：定义 `$breakpoints` 或 `--paap-bp-*` 变量，参照 Carbon 标准（672/1056/1312）

---

## 七、深层样式扫描第三期 (2026-06-26)

### 7.1 AppRegistryView — 第二个零 Carbon 视图（P2）

| 指标 | 值 |
|------|-----|
| 行数 | 196 |
| `--cds-*` 引用 | **0** |
| 硬编码 `color:` | 14 处 |
| 硬编码 `background:` | 9 处 |
| 硬编码 `font-size:` | 11 处 |
| 硬编码 `border-radius:` | 6 处 |

**关键问题：**
- 使用自定义 `.section-title` 样式（`font-size: 22px; font-weight: 600; color: #11181c`），应使用 Carbon `--cds-heading-03-*` tokens
- `.registry-row` 使用 `border-bottom: 1px solid #f1f3f5`（Tailwind gray-100），应使用 `--cds-border-subtle-01`
- `.registry-row:hover` 使用硬编码色值，无 Carbon 主题色对应
- 无 `--cds-*` 的 `<style scoped>` 完全独立，全局 theme 变化对此文件无影响

---

### 7.2 SVG 图标系统审计（P2）

**106 个内联 SVG 分布在 10+ 个文件中。** 大部分已遵循「fill=currentColor 原则」，但缺乏统一管理。

| 指标 | 数据 |
|------|------|
| 内联 SVG 总数 | 106 |
| 带 `viewBox` | 99（93%） |
| 使用 `fill="currentColor"` | 59（56%） |
| 硬编码 `fill="#..."` | 9（8%） |
| `fill="none"` | 39（37%） |
| 带 `focusable="false"` | 部分（EnvDetailView 少数） |

**尺寸分布（同一 view 内不一致）：**

| 宽度 | 出现次数 | 位置 |
|------|---------|------|
| 16px | 6+ | EnvDetailView, AppLayout, MainLayout |
| 20px | 6+ | EnvDetailView |
| 12px | 7+ | EnvDetailView |
| 14px | 3+ | GiteaWorkspace |
| 24px | 2+ | EnvDetailView |
| 18px | 4+ | EnvDetailView |

**问题：**
- **EnvDetailView.vue 尤为严重**：40 个 SVG，使用 6 种不同尺寸（12/14/16/18/20/24px），无一致图标尺寸体系
- 🡒 **替代方案见 §15.2**：`@carbon/icons-vue` 已安装但未使用，迁移后可直接用 `<CarbonIcon>` 替换全部内联 SVG
- 尺寸没有使用 Carbon 图标大小规范（Carbon 标准：16/20/24/32px）
- 9 处硬编码 `fill="#..."`，不响应主题切换
- 没有引入 `@carbon/icons-vue` 包，全部手写 SVG path
- 缺少 `aria-label`、`role="img"` 等可访问性属性

---

### 7.3 CSS 过渡与动画不一致（P1）

**存在两种完全不同的过渡速度体系，混用在同一项目中：**

**体系 A — Carbon 规范（110ms）：**
```css
/* ComponentConfigTemplateFields、TemplatesView、AppEnvironmentsView 部分使用 */
transition: border-color 110ms, box-shadow 110ms;
```

**体系 B — 自定义 0.15s：**
```css
/* AppLayout、MainLayout、AppOverviewView、CreateAppView 等 */
transition: all 0.15s;
```

| 时长 | 出现次数 | 来源 |
|------|---------|------|
| `110ms` | 47 | Carbon 标准 |
| `0.15s` | 33 | 自定义（接近但不同） |
| `0.1s` | 9 | Workspace 组件 |
| `0.12s` | 3 | ArgocdWorkspace |
| `0.2s` | 4 | 零星使用 |
| `0.8s` | 8 | @keyframes spin 动画时长 |

**Carbon 标准 motion 规范：**
- `--cds-motion-duration-fast`: 110ms（hover/focus 反馈）
- `--cds-motion-duration-moderate`: 240ms（展开/折叠）
- `--cds-motion-duration-slow`: 400ms（出现/消失）

**关键问题：**
- `transition: all 0.15s` 在 7 个文件中出现，既不是 Carbon 110ms 也不是自定义 0.1s，是随意取值
- Workspace 组件大多使用 `0.1s`，比 Carbon fast 还快，hover 反馈可能感觉不到
- 9 个文件使用硬编码字符串而不是 `var(--cds-motion-duration-fast)`

**@keyframes spin 8 次重复定义：**

| # | 文件 | 代码 |
|---|------|------|
| 1 | AppListView.vue | `@keyframes spin { to { transform: rotate(360deg); } }` |
| 2 | AppMonitorView.vue | 同上 |
| 3 | AppRegistryView.vue | 同上 |
| 4 | AppCIView.vue | 同上 |
| 5 | AppEnvironmentsView.vue | 同上 |
| 6 | TemplatesView.vue | 同上 |
| 7 | AppDeployView.vue | 同上 |
| 8 | ComponentDetailView.vue | 同上 |

**问题：** 完全相同的 `@keyframes spin` 在 8 个文件中重复定义。应在 `style.scss` 中定义一次全局复用。

---

### 7.4 style.scss 废弃 / 未使用 CSS（P3）

**style.scss 中定义的全局类，但没有被任何 `.vue` 文件引用：**

| 选择器 | 定义行 | 状态 |
|--------|--------|------|
| `.rail-card` | 第 178 行 | ❌ **未使用**（0 vue 文件引用） |
| `.rail-kpi-value` / `.rail-kpi-label` | 第 274/282 行 | ❌ **未使用**（0 vue 文件引用） |
| `.rail-table` / `.rail-table-*` | 多行 | ❌ **未使用**（0 vue 文件引用） |
| `.rail-label` | 多行 | ❌ **未使用**（0 vue 文件引用） |

**可用但被各 view 覆盖或绕过的全局类：**

| 选择器 | 状态 |
|--------|------|
| `.rail-input` | ✅ 5 view 引用，但 ComponentDetailView 和 TemplatesView 各自覆盖了定义 |
| `.rail-select` | ⚠️ 4 view 引用，类似覆盖问题 |
| `.rail-btn` | ✅ 13 view 引用，但 AppDeployView/AppCIView/AppMonitorView 各自重新定义了 `.rail-btn` |

**预计清理收益：** 移除上述废弃代码约可减少 `style.scss` 60+ 行（~13%）。

---

### 7.5 汇总

| # | 问题 | 严重程度 | 影响范围 |
|---|------|---------|---------|
| 1 | AppRegistryView 零 Carbon token | 🟡 P2 | 1 file |
| 2 | 106 个内联 SVG 无统一管理、9 处硬编码 fill、6 种尺寸 | 🟡 P2 | 10+ files |
| 3 | 过渡时间体系双轨（110ms vs 0.15s vs 0.1s vs 0.12s） | 🔴 P1 | 全项目 |
| 4 | `@keyframes spin` 在 8 个文件中重复定义 | 🟡 P2 | 8 files |
| 5 | style.scss 废弃全局类（rail-card, rail-kpi, rail-table, rail-label） | 🟢 P3 | 1 file |

**建议：**
1. **（P1）统一过渡速度**：所有 hover/focus 使用 `var(--cds-motion-duration-fast, 110ms)`，消除 0.15s/0.12s/0.1s 等自定义值
2. **（P2）抽离 `@keyframes spin` 到 style.scss**：全局定义一次，8 个文件删除重复
3. **（P2）引入 Carbon 图标系统**：`@carbon/icons-vue` 已安装，见 §15.2 迁移计划
4. **（P2）AppRegistryView 碳化**：替换硬编码色值为 `--cds-*`
5. **（P3）清理废弃全局类**：从 style.scss 删除 `.rail-card`、`.rail-kpi-*`、`.rail-table`、`.rail-label` 等无用定义

---

## 八、深层样式扫描第四期 (2026-06-26)

### 8.1 viewMarkup.test.ts CSS 测试覆盖度审计（P2）

`viewMarkup.test.ts`（1751 行，95 个 `it` 块）主要面向功能/行为测试，CSS 相关测试非常有限。

**CSS 测试现状：**

| 指标 | 值 |
|------|-----|
| 总 `it()` 数 | 95 |
| 使用 `cssRule()` 的测试 | 39 行断言 |
| 覆盖的 CSS 选择器 | 约 18 个 |
| 覆盖的视图文件 | 3 个（EnvDetailView, TemplatesView, ComponentConfigTemplateFields） |
| 覆盖的 workspace 组件 | **0** |
| 检查的 CSS 属性 | border-radius, border, background, color, box-shadow, font-family |

**已覆盖的 CSS（全部集中在新迁移的 modal 和 config template 区域）：**
- `.config-drawer` → `box-shadow: none`, `--cds-border-subtle-01`
- `.service-config-field` → `border-radius: 0`, `border: 0`, `--cds-layer-01`
- `.modal-container` → `border-radius: 0`, `box-shadow: none`
- `.rail-input` / `.rail-input:focus` → `--cds-field-01`, `inset 0 0 0 1px`
- `.component-template-*` → 各种 `--cds-*` 和 `border-radius: 0`
- 负面检测：`border-radius: 8px`、`border-radius: 999px`、`#3b82f6`、`#0f172a`、`%23687076`

**显著缺失（未测试 CSS 的视图）：**

| 视图 | 硬编码值 | 风险 |
|------|---------|------|
| AppCIView.vue | ❌ 未测试 | 21+ empty/loading 相关样式 |
| AppDeployView.vue | ❌ 未测试 | Tailwind 色值、rail-btn 覆盖 |
| AppMonitorView.vue | ❌ 未测试 | 自定义 link-card 样式 |
| AppRegistryView.vue | ❌ 未测试 | 零 Carbon token |
| CreateAppView.vue | ❌ 未测试 | form-input 自定样式 |
| PlatformUsersView.vue | ❌ 未测试 | 新平台管理页面 |
| **所有 workspace 组件** | ❌ 未测试 | ToolWorkspaceFrame 等 Tailwind 色值 |

**建议：**
- 为每个 `--cds-*` 已迁移的组件添加 CSS token 回归测试（使用 `cssRule`）
- 为 ComponentDetailView 和每个 workspace 组件添加基础 CSS token 测试
- 添加 border-radius 一致性正则测试（扫描所有 scoped style block 检查非 Carbon radius）

---

### 8.2 色值对比度合规性（P3）

使用 WCAG 2.1 AA 标准（4.5:1）对项目中频繁出现的色值组合进行计算：

| 前景 | 背景 | 所在场景 | 对比度 | WCAG 等级 |
|------|------|---------|--------|-----------|
| `#11181c` | `#ffffff` | text primary on white | 17.93:1 | ✅ AAA |
| `#687076` | `#ffffff` | text secondary on white | 5.04:1 | ✅ AA |
| `#687076` | `#f1f3f5` | muted text on panel bg | 4.53:1 | ⚠️ **AA 边界** |
| `#2563eb` | `#ffffff` | blue link on white | 5.17:1 | ✅ AA |
| `#166534` | `#f0fdf4` | green text on green bg | 6.81:1 | ✅ AA |
| `#b91c1c` | `#fef2f2` | red text on red bg | 5.91:1 | ✅ AA |
| `#161616` | `#ffffff` | Carbon text-primary on white | 18.10:1 | ✅ AAA |
| `#525252` | `#ffffff` | Carbon text-secondary on white | 7.81:1 | ✅ AAA |

**结论：** 当前项目中使用的色值组合整体对比度合规（均 ≥ 4.5:1）。**主要问题不是可读性，而是品牌色和语义一致性。**

**但需要注意的边界场景：**
- `#687076` 在 `#f1f3f5` 背景上为 **4.53:1**，刚好过 AA 门槛，对于小字体（<18px）或 thin weight 可能实际可读性不足
- Carbon 标准的 `--cds-text-secondary` 是 `#525252`（7.81:1），比 Tailwind `#687076` 更安全
- 如果未来设计稿要求使用更轻的灰色做次要文字，需特别注意 `#9ba1a6`（Tailwind gray-400，约 3.0:1）会被 AAA 和 AA 同时拒绝

---

### 8.3 空/加载/错误状态一致性（P2）

**结论：存在 3 种完全不同的空/加载/错误状态实现。**

**方案 A — style.scss 全局 `rail-empty` 系列：**
```css
.rail-empty { display: flex; flex-direction: column; align-items: center; justify-content: center; }
.rail-empty-title { font-size: 18px; font-weight: 600; color: var(--paap-text); }
.rail-empty-desc { font-size: 14px; color: var(--paap-muted); max-width: 420px; }
```
✅ 在 AppEnvironmentsView 和 TemplatesView 中使用。一致性好，但使用 `--paap-*` 而非 `--cds-*`。

**方案 B — EnvDetailView 的局部 `workspace-loading` 系列：**
```css
.workspace-loading { border: 1px solid var(--paap-border); border-radius: var(--paap-radius); }
```
用在 6 个不同 workspace 区域（inline-install、workspace、backup、logs、metrics），每个的文案和布局各不相同，但没有统一组件。
✅ 加载/错误/数据三态切换模式一致（`loading` → `error` → `data`）。

**方案 C — AppListView 的独有 `empty-panel`：**
```css
.empty-panel { /* 只有 AppListView 使用 */ }
```
没有使用 `rail-empty` 类，有自己的结构。

**关键差距：**

| 问题 | 细节 |
|------|------|
| 没有 `<RailEmpty>` 组件 | 每个 view 各自写 empty state |
| 没有 `<RailLoading>` 组件 | 靠 `workspace-loading` class + 文字 |
| 没有 `<RailError>` 组件 | EnvDetailView 用 `.modal-error` + `role="alert"`，但无统一组件 |
| loading 文案不统一 | "模板加载中..."/"工作台加载中..."/"正在读取当前卡片的真实日志..." 等 |
| empty 视觉不统一 | AppListView 用 `empty-panel`，EnvironmentsView 用 `rail-empty`，样式不同 |
| error 状态无重试按钮 | 所有 error 状态只显示文字，无 "重试" action |

---

### 8.4 汇总

| # | 问题 | 严重程度 | 影响范围 |
|---|------|---------|---------|
| 1 | viewMarkup.test.ts CSS 测试只覆盖 3 个视图、18 个选择器 | 🟡 P2 | 全项目 |
| 2 | 6 个视图 + 所有 workspace 零 CSS token 测试覆盖率 | 🟡 P2 | 10+ files |
| 3 | 色值对比度整体合规（4.5:1+），但 `#687076` on `#f1f3f5` 仅 4.53:1 | 🟢 P3 | 多个文件 |
| 4 | Empty/Loading/Error 状态 3 种不同实现 | 🟡 P2 | 全项目 |
| 5 | 无统一 Empty/Loading/Error 组件 | 🟡 P2 | 全项目 |

**建议：**
1. **（P2）扩展 CSS token 测试**：为每个已碳化迁移的文件添加 `cssRule` 回归测试，确保 `border-radius: 0`、`--cds-*` 不退化
2. **（P2）抽象 `<RailEmpty>` / `<RailLoading>` / `<RailError>` 组件**：消除 3 种不同实现，统一加载/空/错误状态的视觉表现和行为（含重试按钮）
3. **（P3）无障碍通行**：色值对比度当前合规，但未来添加新色值时应参考 WCAG 4.5:1 标准

---

## 九、深层样式扫描第五期 (2026-06-26)

### 9.1 IBM Plex 字体从未加载（P1）

**结论：CSS token 中声明了 IBM Plex 字体族，但字体文件从未部署到项目中。**

| 检查项 | 结果 |
|--------|------|
| `carbon-theme.css` 引用 `IBM Plex Sans` | ✅ 18 行、858 行 |
| `style.scss` 引用 `IBM Plex Sans` | ✅ 69 行（fallback 链） |
| 物理字体文件（`frontend/public/fonts/`） | ❌ 不存在 |
| `@font-face` 声明 | ❌ 不存在 |
| Google Fonts `<link>` 引用 | ❌ 不存在 |
| `npm` 字体包（`@fontsource/ibm-plex-sans` 等） | ❌ 不存在 |
| `index.html` 中任何字体引用 | ❌ 不存在 |
| `main.ts` 中任何字体引用 | ❌ 不存在 |

**用户实际上看到的是：**
```css
font-family: var(--cds-font-family-sans, 'IBM Plex Sans', system-ui, -apple-system,
             BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif);
```
⚠️ 浏览器找不到 IBM Plex Sans → 回退到 system-ui → 最终使用操作系统默认字体（Linux 上为 Noto Sans / DejaVu Sans）。

**影响：**
- IBM Plex Sans、Mono、Serif、Sans Condensed **四种字体均未加载**
- 页面渲染使用系统字体，与设计稿（Carbon / Railway 风格）字体外观完全不符
- 中文字体显示正常（fallback 到 PingFang/Microsoft YaHei），但英文字符、代码块字体不符预期
- 字体缺失在前端测试（`viewMarkup.test.ts`）中无任何捕获

**修复方案（3 选 1）：**

| 方案 | 操作 | 优点 | 缺点 |
|------|------|------|------|
| A — Google Fonts | `index.html` 加 `<link>` | 零配置 | 需外网访问 |
| B — npm fontsource | `npm install @fontsource/ibm-plex-sans` 等 | 自托管，内网友好 | 100KB+ 包体积 |
| C — 自托管 woff2 | 下载字体到 `public/fonts/` + `@font-face` | 离线可用 | 需定期更新 |

**建议：** 方案 B（fontsource），在 `main.ts` 中 import，无需手动写 `@font-face`。
```bash
npm install @fontsource/ibm-plex-sans @fontsource/ibm-plex-mono @fontsource/ibm-plex-serif
```
```ts
// main.ts
import '@fontsource/ibm-plex-sans/400.css'
import '@fontsource/ibm-plex-sans/600.css'
import '@fontsource/ibm-plex-mono/400.css'
```

---

### 9.2 `.rail-btn` 3 份 scoped 重复定义（P1）

**结论：AppCIView、AppDeployView、AppMonitorView 三份 `<style scoped>` 中各自定义了完全相同的 `.rail-btn` 和 `.rail-btn--primary`，且与全局 `style.scss` 的定义不一致。**

| 来源 | border-radius | font-weight | height | padding | border |
|------|--------------|-------------|--------|---------|--------|
| `style.scss`（全局） | 无设置 | 500 | 无设置 | 无设置 | `1px solid transparent` |
| AppCIView（scoped） | **6px** | **600** | **36px** | **0 16px** | `1px solid #e6e8eb` |
| AppDeployView（scoped） | **6px** | **600** | **36px** | **0 16px** | `1px solid #e6e8eb` |
| AppMonitorView（scoped） | **6px** | **600** | **36px** | **0 16px** | `1px solid #e6e8eb` |

**3 个 scoped 副本完全相同**，但都 **与全局 style.scss 不同**。这意味着：
- 按钮在这 3 个 view 中渲染为 Railway 风格（6px 圆角、600 字重），
- 而在其他 view 中渲染为另一种风格（无圆角、500 字重、透明边框）
- 由于是 `scoped`，复制 CSS 没有被去重，浪费约 2KB
- `background: #fff; color: #11181c` 硬编码 Tailwind 色值（非 `--cds-*`）

**同样的问题模式在 `.rail-btn--primary`：**
- 3 个 scoped 版本使用 `var(--cds-button-primary, var(--paap-accent))`（正确使用 Carbon token）
- 但全局 style.scss 的 `.rail-btn--primary` 使用 `var(--cds-button-primary, var(--paap-accent))`（相同）
- 差异在于 `.rail-btn--primary:hover`：scoped 版有 `border-color` 过渡而全局版没有

**修复：** 删除 3 个 scoped 文件中的 `.rail-btn` / `.rail-btn--primary` 定义，将 diff 合并到全局 `style.scss`。

---

### 9.3 选择器深度与特异性风险（P2）

**`!important` 使用数：0** ✅ 非常干净，全项目无一使用。

**选择器深度分布：**

| 深度 | 数量 | 说明 |
|------|------|------|
| 1 层（如 `.rail-btn`） | 多数 | 低特异性，易覆盖 |
| 2-3 层（如 `.component-preset-card:hover`） | 大量 | 正常水平 |
| 4 层（如 `.modal-container .modal-header .modal-label`） | **50** 个 | 较高特异性 |
| 5 层+ | **12** 个 | 高特异性，难维护 |

**5 层选择器示例：**
```css
/* EnvDetailView.vue:8464 */
.quick-icon.registry, .quick-icon.harbor { color: #d4b072; }

/* EnvDetailView.vue:9297 */
.node-status.error, .node-status.failed { background: var(--paap-danger); }

/* MinioWorkspace.vue:207 / KafkaWorkspace.vue:219 / MongoWorkspace.vue:215 */
.selectable.selected, .data-table tbody tr.selected { background: var(--paap-accent-soft); }
```

**风险：** 深层选择器意味着未来任何样式变更都需要同等或更高的特异性才能覆盖。50 个 4 层选择器、12 个 5 层选择器增加了未来 CSS 重构的难度。

---

### 9.4 内联动态样式生成模式（P2）

EnvDetailView.vue 中使用 JS 函数生成内联样式：

**`componentNodeStyle(node)` — 节点定位（第 3316 行）：**
```ts
const componentNodeStyle = (node:any) => ({
  left: `${node.x}px`,
  top: `${node.y}px`,
  width: `${node.width}px`,
  height: `${node.height}px`,
})
```
以 `:style` 绑定到画布节点 `<div>`。

**`componentTopologyZoneStyle(zone)` — 泳道定位（第 3322 行）：**
```ts
const componentTopologyZoneStyle = (zone:any) => ({
  left: `${zone.bounds.left}px`,
  top: `${zone.bounds.top}px`,
  width: `${zone.bounds.width}px`,
  height: `${zone.bounds.height}px`,
})
```

**`componentEdgeClasses(edge)` — 边样式（第 3373 行）：**
```ts
const componentEdgeClasses = (edge:any) => ({
  active: selectedManualEdge.value ? manualEdgeSelected(edge) : componentEdgeHighlighted(edge),
  'component-canvas-link--manual': isManualCanvasEdge(edge),
  'component-canvas-link--selected': manualEdgeSelected(edge),
})
```

**`runtimeMetricBarStyle(sample, kind)` — 指标条宽度（第 5777 行）：**
```ts
const runtimeMetricBarStyle = (sample:any, kind:'cpu' | 'memory') => {
  const raw = runtimeMetricNumericValue(sample, kind)
  const max = Math.max(...samples.map((item:any) => runtimeMetricNumericValue(item, kind)), raw, ...)
  const width = Math.max(raw > 0 ? 4 : 0, Math.min(100, (raw / max) * 100))
  return { width: `${width}%` }
}
```

**评估：** 这些是必要的动态样式（节点位置由数据驱动），无法用静态 CSS 替代。当前实现合理。但不加注释、使用 `any` 类型，可维护性有待提高。

**建议：** 不要将这些函数提取到 composable，因为它们与 EnvDetailView 的 canvas 布局高度耦合。只需添加 JSDoc 注释说明参数类型和返回格式。

---

### 9.5 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | IBM Plex 字体从未加载（4 种字体全部缺失） | 🔴 P1 | 全项目设计一致性 |
| 2 | `.rail-btn` 在 3 个文件 scoped 重复 + 与全局不一致 | 🟡 P2 | 3 files, ~2KB |
| 3 | 选择器深度 4 层 50 个 + 5 层 12 个 | 🟡 P2 | 全项目 |
| 4 | 动态内联样式使用 `any` 类型无注释 | 🟢 P3 | 1 file |

**建议：**
1. **（P1）安装 IBM Plex 字体**：推荐 fontsource 方案——`npm install @fontsource/ibm-plex-sans @fontsource/ibm-plex-mono @fontsource/ibm-plex-serif`，在 `main.ts` 中 import
2. **（P2）删除 3 个 scoped 的 .rail-btn 副本**：将差异合并到全局 style.scss，删除 AppCIView/AppDeployView/AppMonitorView 中的重复定义
3. **（P2）简化深度选择器**：将 `.quick-icon.registry, .quick-icon.harbor` 等 4+ 层选择器简化或改为类扩展
4. **（P3）添加动态样式函数类型注释**：为 `componentNodeStyle` 等添加 JSDoc

---

## 十、基础设施与性能扫描 (2026-06-26)

### 10.1 WebSocket 实时状态未接入 UI（P2）

**结论：`useWebSocket` composable 已实现但未被任何组件使用，实时状态推送功能形同虚设。**

| 检查项 | 状态 |
|--------|------|
| `useWebSocket()` composable 存在 | ✅ `frontend/src/composables/useWebSocket.ts`（61 行） |
| 被任何 `.vue` 文件 import 使用 | ❌ **0 处引用** |
| `connected` 状态在 UI 中展示 | ❌ 无连接状态指示器 |
| `lastMessage` 数据被消费 | ❌ 无 watch/consumer |
| 断线重连 | ⚠️ 有重连（3s 固定延迟），无指数退避 |
| 重连失败时通知用户 | ❌ 无通知 |
| WebSocket 错误时降级到 HTTP 轮询 | ❌ 无 fallback |

**具体的 dead code:** `useWebSocket()` 在 composable 中 `onMounted(connect)` 会在组件挂载时建立 WebSocket 连接，但由于没有组件 import 它，这个连接从未被建立。状态更新可能完全依赖 HTTP 请求/手动刷新。

**建议：**
1. **（P2）接入 WebSocket 到全局 store**：在 Pinia store 或 App.vue 中初始化 `useWebSocket()`，将 `lastMessage` 分发到相关状态
2. **（P2）添加连接状态指示器**：在导航栏或 canvas 角落显示 `● 已连接` / `○ 未连接` 状态点
3. **（P2）实现指数退避重连**：当前固定 3s 重连 → 改为 `min(2^n * 1000, 30000)` 退避
4. **（P2）实现 HTTP 轮询 fallback**：WebSocket 连接失败时，降级到 `setInterval` 轮询

---

### 10.2 前端工程化工具缺失（P2）

**结论：前端项目无 ESLint、Prettier、Stylelint 配置，CSS 质量完全依赖人工审查。**

| 工具 | 安装 | 配置文件 | 效果 |
|------|------|---------|------|
| ESLint | ❌ 未安装 | ❌ 无 | TypeScript 无 lint 校验 |
| Prettier | ❌ 未安装 | ❌ 无 | 代码格式无自动化 |
| Stylelint | ❌ 未安装 | ❌ 无 | CSS 无自动检查 |
| PostCSS | ✅ 已装 | ❌ 无 config | autoprefixer 无配置 → 不生效 |
| Tailwind CSS | ✅ 已装 | ❌ 无 config | v4 零配置模式，但项目中并未使用 Tailwind utility class |

**问题：**
- `autoprefixer` 安装在 `devDependencies` 中，但 `postcss.config.js` 不存在 → **autoprefixer 从未被激活**，CSS 没有自动添加浏览器前缀
- 无 CSS 规范检查 → 硬编码色值、非标准 border-radius、缺失 transition 等问题只能靠人工 code review 发现
- `.editorconfig` 不存在 → 团队成员编辑器缩进/编码设置无法统一
- 当前 TypeScript 配置未启用 `strict` 模式 → `any` 类型在 composable 和 handler 中自由使用

**建议：**
1. **（P2）安装 Prettier + Stylelint**：`npm install -D prettier stylelint stylelint-config-standard`
2. **（P2）配置 postcss.config.js**：激活 autoprefixer，指定 browserslist
3. **（P2）启用 TypeScript strict 模式**：在 `tsconfig.json` 中启用 `strict: true`
4. **（P2）添加 `.editorconfig`**：统一缩进为 2 spaces

---

### 10.3 画布渲染性能分析（P2）

**画布实现方式：** EnvDetailView.vue 使用自定义 DOM + SVG overlay 实现拓扑图，未使用 React Flow / Vue Flow 等库。

| 关注点 | 当前实现 | 评价 |
|--------|---------|------|
| 节点渲染 | `<div>` 绝对定位 | ✅ 简单高效 |
| 连线渲染 | `<svg>` overlay 层 | ✅ 独立层避免重排 |
| 缩放 | CSS `transform: scale(${canvasZoom})` | ✅ GPU 加速 |
| 平移 | `marquee` + pointer events | ⚠️ 未实现平移（pan） |
| GPU 加速 | `will-change: transform` | ❌ 无 |
| 渲染限制 | `content-visibility` / `contain` | ❌ 无 |
| 虚拟滚动 | 大量节点时懒加载 | ❌ 无 |
| 空画布提示 | `component-canvas-empty-hint` | ✅ 有 |

**风险：**
- 大型集群（30+ 节点 + 50+ 连线）时 DOM 数量可能达到数百个，没有渲染优化措施
- 缩放变换（0.5x ~ 2x）时无 `will-change` 提示，浏览器可能不做 GPU 合成层提升
- 节点拖拽 DnD 使用 `@pointerdown` 事件，无 `touch-action: none` 防止移动端手势冲突

**建议：**
1. **（P2）添加 GPU 加速**：在 `.component-canvas-stage` 上添加 `will-change: transform`
2. **（P2）节点容器添加 `contain: layout style`**：告诉浏览器每个节点是独立布局上下文
3. **（P2）连线 SVG 添加 `touch-action: none`**：防止触摸设备上的滚动/缩放手势冲突

---

### 10.4 画布缩放指数量级不一致

**结论：拓扑图缩放限制为 0.5x ~ 2x，但这两个值相差 4 倍，对于大画布不够用。**

| 指标 | 值 |
|------|-----|
| 最小缩放 | 0.5 (50%) |
| 最大缩放 | 2.0 (200%) |
| 步长 | 0.1 |
| 默认值 | 1.0 (100%) |

**问题：**
- 50% 对于大型拓扑图可能仍然太大（尤其是泳道分区时）
- zoom in 到 200% 对精细操作意义不大，且可能导致画布外溢
- 步长 0.1 意味着用户需要按 10 次才能从 50% 到 100%，交互效率低

**建议：** 调整缩放范围为 0.25x ~ 1.5x，步长改为 0.15 或增加缩放滑块直接选择。

---

### 10.5 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | WebSocket composable 存在但未被任何组件使用 | 🟡 P2 | 全项目实时状态 |
| 2 | 无 ESLint/Prettier/Stylelint/editorconfig | 🟡 P2 | 全项目代码质量 |
| 3 | autoprefixer 无 postcss.config.js → 不生效 | 🟡 P2 | CSS 浏览器兼容 |
| 4 | 画布无 GPU 加速/渲染优化 | 🟡 P2 | 大型节点性能 |
| 5 | 缩放范围与步长不合理 | 🟢 P3 | 用户体验 |

**建议：**
1. **（P2）接入 WebSocket**：在全局级别初始化 `useWebSocket`，展示连接状态，消费 `lastMessage` 更新组件状态
2. **（P2）安装 ESLint + Prettier + Stylelint + editorconfig**：建立基础代码质量门禁
3. **（P2）配置 postcss.config.js**：激活 autoprefixer，指定 `> 0.5%, last 2 versions, not dead`
4. **（P2）画布渲染优化**：添加 `will-change`、`contain`、`touch-action`
5. **（P3）调整缩放参数**：范围 0.25–1.5，步长 0.15，或增加缩放滑块

---

## 十一、全项目可维护性审计 (2026-06-26)

### 11.1 多语言 / i18n 缺失（P2）

**结论：1659 处中文文本全部硬编码在 Vue 模板中，无任何 i18n 准备。**

| 指标 | 值 |
|------|-----|
| `vue-i18n` 安装 | ❌ 未安装 |
| 任何国际化库 | ❌ 无 |
| 中文硬编码字符串 | **1659 处** |
| 占位符（placeholder）中文 | 41 处 |
| 单文件最集中 | EnvDetailView.vue — **809 处** |

**前 5 中文字符密集型文件：**
| 文件 | 中文行数 |
|------|---------|
| EnvDetailView.vue | 809 |
| TemplatesView.vue | 230 |
| ComponentDetailView.vue | 62 |
| GiteaWorkspace.vue | 61 |
| AppListView.vue | 39 |

**典型硬编码模式：**
```vue
<!-- 按钮文字 -->
<button>安装工具</button>
<!-- 状态显示 -->
<span>{{ statusText }}</span>  <!-- statusText 是 JS 中返回的中文字符串 -->
<!-- 错误消息 -->
<div>{{ '加载失败：' + error }}</div>
<!-- 提示消息 -->
<div class="config-empty">部署成功后这里会展示 keyspace、内存、过期键和对象级操作。</div>
```

**影响：**
- 无法支持多语言（英文、日语等）
- 模板中的中文文本无法被翻译工具提取（无 `$t()` 包装）
- JS 中的状态映射表（如 `serviceStatusText`）返回硬编码中文，无法切换语言
- 所有错误消息拼接格式 `XXX失败：${e?.message || '未知错误'}` 需全局替换

**建议：**
1. **（P2）安装 vue-i18n**：`npm install vue-i18n`
2. **（P2）从 EnvDetailView.vue 开始**：将 809 处中文字符串抽入 `locales/zh-CN.json`，模板中使用 `$t('key')`
3. **（P2）错误消息模板化**：统一错误消息格式为 `$t('error.format', { action: $t('action.load'), msg: e.message })`
4. **（P3）状态映射表外置**：将 `serviceStatusText`、`statusTagClass` 中的中文映射移至 locale 文件

---

### 11.2 可访问性 Accessiblity 细节缺失（P3）

**基础 aria 属性已覆盖部分场景，但关键领域缺失严重。**

**已有：**
| 属性 | 数量 | 说明 |
|------|------|------|
| `role` | 73 | 大部分是 `role="dialog"` 和 `role="alert"`，✅ 做得较好 |
| `aria-label` | 45 | 分布在 10 个文件中，✅ 关键交互元素有标签 |
| `aria-hidden` | 15 | 装饰性 SVG 图标标记为隐藏，✅ 正确 |
| `role="alert"` | 37 | 动态错误/状态提示，✅ 符合最佳实践 |

**缺失：**
| 属性 | 数量 | 说明 |
|------|------|------|
| `aria-labelledby` | **0** | modal 头部没有关联 dialog 的 aria 关系 |
| `aria-describedby` | **0** | modal 描述无关联 |
| `aria-modal` | **0** | 所有 dialog 缺少 `aria-modal="true"` |
| `aria-expanded` | **0** | 可折叠面板/下拉菜单缺少展开状态提示 |
| `aria-selected` | **0** | tab/列表选择状态无 aria 通知 |
| `aria-current` | **0** | 当前导航项无标记 |
| `tabindex` | **4** | 可交互元素缺少键盘焦点管理 |
| `role="img"` | **1** | 106 个 SVG 只有 1 个声明为图片 |

**具体问题：**
- **模态框**：6 个 `modal-overlay` 实现中，只有 1 个（AppListView）有关闭按钮的 `aria-label`。所有 modal 均缺少 `aria-modal`、`aria-labelledby`（关联标题）、`aria-describedby`（关联描述）。
- **SVG 图标**：106 个内联 SVG 中，只有 EnvDetailView 中的一部分使用了 `focusable="false"` 和 `aria-hidden="true"`。大部分图标无法被屏幕阅读器正确处理。
- **键盘导航**：整个项目只有 4 个 `tabindex` 属性。画布节点、tab、卡片均不可键盘导航。TemplatesView 的 tab 切换缺少键盘事件。
- **焦点管理**：模态框打开后焦点不会自动移动到第一个可聚焦元素，关闭后焦点不会回到触发元素。
- **颜色对比度**：已在 8.2 节评估，当前符合 WCAG AA 标准（≥ 4.5:1）。

**建议：**
1. **（P3）添加 `aria-labelledby` + `aria-modal`**：修改 6 个 modal-overlay 实现，关联到 `modal-heading` ID
2. **（P3）SVG 图标**：为所有 `fill="currentColor"` 的装饰性 SVG 添加 `aria-hidden="true"` + `focusable="false"`；为功能性图标添加 `role="img"` + `aria-label`
3. **（P3）键盘导航**：画布节点添加 `tabindex="0"` + `@keydown.enter`/`@keydown.delete` 处理器
4. **（P3）焦点陷阱**：抽象 `<RailModal>` 时，内置 `onMounted` 自动聚焦和 `onUnmounted` 焦点恢复

---

### 11.3 错误消息模板化与一致性（P2）

**结论：前端错误消息格式已经形成隐式约定（`XXX失败：消息`），但无标准化组件。**

**当前模式（见 EnvDetailView.vue）：**
```ts
// 第 2857 行 - 环境加载失败
pageError.value = '环境加载失败：' + (e?.message || '未知错误')

// 第 3999 行 - 画布布局保存失败
pageError.value = '保存画布布局失败：' + (e?.message || '未知错误')

// 第 4666 行 - 通用 action 失败
const message = '执行失败：' + (e?.message || '未知错误')
```

**发现：**
- 模式高度一致，格式为 `XXX失败：${msg || '未知错误'}` — 便于后续统一提取
- 后端返回的错误消息直接透传到用户界面，无安全过滤（可能存在内部信息泄漏风险）
- 无全局错误统一展示组件（目前每个 view 各自显示 `page-error` / `modal-error` / `config-empty`）
- 无错误类别区分（没有区分 4xx 用户错误、5xx 服务器错误、网络错误）

**建议：**
1. **（P2）抽象 `<RailError>` 组件**：统一渲染错误状态，支持 `retry` prop 添加重试按钮
2. **（P2）错误消息注册**：后端错误码标准化（如 `ERR_LOAD_ENV`），前端映射为翻译键
3. **（P3）错误分类与日志**：区分用户错误（toast 提示）vs 系统错误（控制台打印 + 可选 Sentry 报告）

---

### 11.4 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | 1659 处中文硬编码，无 i18n 框架 | 🟡 P2 | 全项目国际化 |
| 2 | EnvDetailView.vue 独占 809 中文字段 | 🟡 P2 | 1 file |
| 3 | 可访问性：所有 modal 缺 aria-labelledby/aria-modal | 🟢 P3 | 6 files |
| 4 | 键盘导航缺失（仅 4 处 tabindex） | 🟢 P3 | 全项目 |
| 5 | 错误消息格式一致但无统一组件 | 🟡 P2 | 全项目 |
| 6 | 后端错误透传无安全过滤 | 🟡 P2 | 多个 handler |

**建议：**
1. **（P2）引入 vue-i18n**：从 EnvDetailView.vue 开始抽取中文到 locale 文件
2. **（P2）抽象 `<RailError>` 组件**：统一错误展示模式（含重试按钮）
3. **（P3）Accessibility 深入清理**：从 modal 的 `aria-labelledby` + `aria-modal` 开始，逐步覆盖键盘导航和焦点管理

---

## 十二、后端安全与架构审计 (2026-06-26)

### 12.1 密码与认证（P2）

| 检查项 | 状态 |
|--------|------|
| 密码哈希算法 | ✅ bcrypt `DefaultCost`（`auth.go:27`） |
| 登录错误消息模糊化（"invalid credentials" 不区分用户是否存在） | ✅ `auth.go:86` |
| JWT 算法白名单验证（拒绝 alg: none） | ✅ `auth.go:111-113` |
| JWT `exp` 过期校验 | ✅ `auth.go:126` |
| WebSocket token 支持（子协议头 / 嵌入代理查询参数） | ✅ `auth.go:60-67` |
| 注册/登录请求无输入长度限制 | ⚠️ `LoginRequest` 和 `RegisterRequest` 没有 `min`/`max` binding 约束 |
| `init()` 种子函数仅用于 JWT 配置引用（无副作用） | ⚠️ 第 151-153 行 `init()` 函数仅引用 `config.Load().JWTSecret`，没有实际功能 |
| Session 管理（Token 吊销/刷新机制） | ❌ 无 refresh token、无 token 吊销列表 |
| Security Headers（CSP / HSTS / X-Frame-Options） | ❌ 无安全响应头 |

**关键发现：** 虽然登录认证基本实现正确，但缺少：
- **Rate limiting**：登录接口可被暴力穷举
- **Refresh token**：JWT 过期后用户只能重新登录
- **CSP / HSTS 响应头**：前端静态资源无内容安全策略保护

---

### 12.2 CSRF / XSS / API 安全（P2）

| 检查项 | 状态 |
|--------|------|
| CSRF Token 生成与验证 | ❌ 完全缺失 |
| SameSite Cookie | ❌ 无设置 |
| `v-html` / `innerHTML` 使用 | ✅ 0 处（前端） |
| 用户输入长度/模式校验 | ⚠️ 后端仅 `binding:"required"`，无更多验证规则 |
| API 错误信息泄漏内部细节 | ⚠️ `Register` 和 `GetCurrentUser` 返回原始 `err.Error()` 给客户端（`auth.go:46, 57, 62`） |
| 服务端 HTML 转义 | ❌ Gin 默认不自动转义响应中的 `gin.H` 值 |
| 管理 API 权限校验 | ✅ `RequirePlatformAdmin()` 中间件保护 admin 路由 |

**问题：**
- **CSRF 完全缺失**：如果用户通过 cookie 认证（暂未使用），存在 CSRF 攻击风险。即使现在使用 `Authorization: Bearer` header（CSRF 天然免疫），未来切换 cookie 方案时必须加入 CSRF 保护
- **错误消息泄漏**：`c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})` 将原始数据库/系统错误消息返回给客户端（如 `Duplicate entry 'xxx' for key 'users.username'`），存在信息泄漏风险
- **输入校验不足**：用户名/密码仅检查非空，无长度限制（`binding:"required"` 而非 `binding:"required,min=3,max=50"`）

---

### 12.3 GORM 模型索引缺失（P2）

**现有索引：** 所有模型都有 `DeletedAt` 软删除索引。外键字段大部分有单列索引。**缺少对业务唯一性的复合索引保障。**

| 模型 | 已有索引 | 应添加的复合索引 |
|------|---------|----------------|
| `Component` | `EnvironmentID`（单列） | `uniqueIndex:idx_env_component` on `(EnvironmentID, Name)` — 防止同一环境重复组件名 |
| `ServiceInstallation` | `EnvTemplateID`（单列） | `uniqueIndex:idx_env_service` on `(EnvironmentID, ServiceType)` — 禁止同一环境安装同类型服务 |
| `AppMember` | `ApplicationID`, `UserID`（各单列） | `uniqueIndex:idx_app_user` on `(ApplicationID, UserID)` — 防止重复邀请 |
| `Environment` | `ApplicationID`（单列） | `uniqueIndex:idx_app_env` on `(ApplicationID, Identifier)` — 应用内环境标识符唯一 |
| `ComponentConfigTemplate` | `DeletedAt` 仅索引 | 缺少 `(EnvironmentID, Type)` 索引 |

**风险：** 缺少复合唯一索引意味着业务层必须自行处理重复检测（存在并发竞态条件），且查询按 `(environment_id, name)` 过滤时无法使用索引覆盖扫描。

**建议（P2）：** 添加上述 5 个 `uniqueIndex`，同时在 `ServiceInstallation` 和 `Component` 上添加 `gorm:"check:name_length > 0"` 约束。

---

### 12.4 EnvDetailView.vue 超长文件（P1）

| 指标 | 值 | 评估 |
|------|-----|------|
| 总行数 | **11654** | 🔴 极危 — 大于 Vue 组件推荐的 500 行上限的 **23 倍** |
| 模板行数 | ~2200 | 包含 7 个模态框 + 2 个 canvas + 多个 tab panel |
| Script 行数 | ~5964 | 1365 个函数定义 |
| Style 行数 | ~3489 | 大量 scoped CSS |
| Computed | 157 | 分散在文件中，无逻辑分组 |
| `ref` / `reactive` | 1394 处 `.value` | 状态管理高度集中 |
| 内部组织注释 | **0** | 无 `// ===` 或 `// ---` 分组标记 |

**问题：**
- 文件覆盖了 10+ 个功能域（canvas 渲染、服务管理、组件管理、权限校验、模态框、抽屉面板、WebSocket、拖拽、缩放、右键菜单），全部在同一个 SFC 中
- 任意修改都可能引起不相关的功能域回归
- 157 个 computed 属性互相引用，数据流难以追踪
- 任何 lint 规则的 TypeScript strict 模式都会产生数百个错误（大量 `any` 类型）

**建议（P1）：** 将 EnvDetailView 拆分为：
1. **`composables/useCanvasTopology.ts`** — 画布节点/边/缩放/拖拽逻辑（当前 ~2000 行）
2. **`composables/useServiceDrawer.ts`** — 服务抽屉面板逻辑（当前 ~1500 行）
3. **`components/EnvServiceDrawer.vue`** — 服务抽屉 SFC（模板 ~800 行）
4. **`components/EnvCanvasTopology.vue`** — 画布拓扑组件（模板 ~1200 行）
5. **`composables/useEnvCapabilities.ts`** — 能力/资源管理逻辑

**拆分后收益（见 §15.1 bundle 分析）：** 首屏 JS 减少 400KB（48%），CSS 减少 160KB。
6. 剩余核心状态 + 布局代码留在 `EnvDetailView.vue`（目标 < 3000 行）

---

### 12.5 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | 登录无 rate limiting | 🟡 P2 | 认证安全 |
| 2 | 无 refresh token / token 吊销 | 🟡 P2 | 会话管理 |
| 3 | CSRF 完全缺失 | 🟡 P2 | API 安全 |
| 4 | API 错误信息泄漏内部细节 | 🟡 P2 | 信息安全 |
| 5 | 输入校验不足（仅 `required`） | 🟢 P3 | 数据质量 |
| 6 | GORM 缺少 5 个复合唯一索引 | 🟡 P2 | 数据一致性 |
| 7 | **EnvDetailView.vue 11654 行 － bundle 400KB（见 §15.1）** | 🔴 **P1** | 可维护性 + 性能 |

**建议：**
1. **（P1）拆分 EnvDetailView.vue**：提取 canvas topology 和 service drawer 到独立 composable/组件
2. **（P2）添加登录 rate limiting**：使用 `github.com/ulule/limiter` 或 `gin-contrib/limiter`
3. **（P2）添加 refresh token**：`POST /api/v1/auth/refresh` 端点 + 短期 access token
4. **（P2）添加 GORM 复合唯一索引**：为 Component、ServiceInstallation、AppMember、Environment 添加 uniqueIndex
5. **（P2）修复错误信息泄漏**：将 `err.Error()` 替换为泛化错误消息（如 `internal error`），同时在日志中记录原始错误
6. **（P2）添加 CSP/HSTS 响应头**：`r.Use(gin.WrapH(securityHandler))`

---

## 十三、后端质量与基础设施审计 (2026-06-26)

### 13.1 Go 单元测试覆盖（P2）

见 **[§17.1 测试覆盖率分析](#十七最终扫描测试覆盖--错误类型--docker--k8s-清单--依赖-2026-06-26)**（完整 per-package 覆盖 % 数据）。

| 简览 | 值 |
|------|-----|
| 测试函数总数 | 473 |
| middleware 测试 | 2 (0.4%) — 🔴 缺口 |
| database 测试 | 1 (0.2%) — 🔴 缺口 |
| migration 测试 | **0** — 🔴 缺口 |

**建议（P2）：**
1. 为 middleware（特别是 `RequirePlatformAdmin`）补充单元测试
2. 为 migration 添加集成测试（apply → rollback → re-apply 场景）
3. 为 controller 补充异常场景测试（网络错误、CRD 冲突、资源不存在）

---

### 13.2 Docker 镜像构建（P3）

见 **[§17.3 Docker 镜像分析](#十七最终扫描测试覆盖--错误类型--docker--k8s-清单--依赖-2026-06-26)**（含 CGO/.dockerignore/BuildKit 缓存）。

**本节要点（不重复展开）：**
- Server `CGO_ENABLED=1` 可能不必要（评估改为 `CGO_ENABLED=0`）
- 添加 `.dockerignore` 排除 `.git`/`node_modules`
- 利用 BuildKit `--mount=type=cache` 加速构建

---

### 13.3 数据库迁移策略（P2）

**当前架构 — GORM AutoMigrate + SQL 迁移文件双轨制：**

```go
// database.go:39-60
func autoMigrate() error {
    deduplicateServiceInstallations()  // 数据清理
    DB.AutoMigrate(&model.User{}, ...) // 模型→表自动同步
}
```

```go
// migrations.go:22-68
func RunSQLMigrationsWithFS(db, migrations) // SQL 文件按序执行
    schema_migrations 表 → 跟踪已应用的迁移
    SQL 文件来自 embed.FS（migration.Files）
    事务性：SQL + 记录在同个事务中
```

**评估：**

| 方面 | 状态 |
|------|------|
| schema 自动迁移 | ✅ GORM AutoMigrate 自动建表/加列 |
| DDL/DML 版本化 | ✅ SQL 文件 + `schema_migrations` 跟踪表 |
| 迁移事务性 | ✅ SQL 和记录在同一事务中 |
| 回滚支持 | ❌ 无回滚机制 |
| 迁移前数据校验 | ⚠️ 仅 `deduplicateServiceInstallations()` 处理数据清理 |
| 迁移文件组织 | ⚠️ 需要确认 `migration/` 目录存在 |
| 并行迁移安全 | ❌ 无锁机制，多副本同时启动时可能竞争 |

**风险：**
- GORM AutoMigrate 只加列不删列：表字段无法通过 AutoMigrate 删除，废弃字段会留在表中
- 无回滚：如果 SQL 迁移失败，数据可能处于部分迁移状态（虽然有事务保护单个文件，但多个文件间无原子性）
- 多副本竞争：同时启动的多个 PAAP Server 实例可能同时执行迁移

**建议（P2）：**
1. 在 `schema_migrations` 中添加 `checksum` 列，检测已应用迁移的文件是否被修改
2. 使用 `GET_LOCK()`（MySQL）或 `pg_advisory_lock()`（PostgreSQL）实现迁移锁
3. 为关键迁移添加回滚 SQL（文件名约定 `V001__name.up.sql` / `V001__name.down.sql`）

---

### 13.4 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | middleware 仅 2 个测试、migration 0 个测试 | 🟡 P2 | 测试覆盖率 |
| 2 | 迁移无回滚机制 | 🟡 P2 | 生产可靠性 |
| 3 | 迁移无多副本锁 | 🟡 P2 | 并发安全 |

**建议：**
1. **（P2）补充 middleware + migration 测试**（见 §17.1 完整覆盖率报告）
2. **（P2）迁移加 advisory lock**
3. **（P3）评估 CGO_ENABLED=0 可行性**（见 §17.3 Docker 分析）

---

## 十四、后端一致性与工程化审计 (2026-06-26)

### 14.1 前端视图组件测试覆盖（P2）

**结论：246 个前端测试，但集中在 composable 和 viewMarkup，15 个 `.vue` 页面组件零测试。**

| 层级 | 测试数 | 说明 |
|------|--------|------|
| `viewMarkup.test.ts` | 95 | 模板断言 + 行为测试 |
| Composable `.ts` 测试 | ~120 | componentTopology、envCapabilities 等 |
| Workspace 组件测试 | 2 | `giteaRepository.test.ts`, `argocdTopology.test.ts` |
| `.vue` 页面组件测试 | 1 | `EnvDetailView.test.ts` |

**15 个 Vue 视图零测试：**
| 视图 | 行数 | 风险 |
|------|------|------|
| AppCIView.vue | ~300 | CI 状态聚合页 |
| AppDeployView.vue | ~300 | 部署状态聚合页 |
| AppEnvironmentsView.vue | ~400 | 环境列表页 |
| AppListView.vue | ~500 | 应用列表首页 |
| AppMembersView.vue | ~200 | 成员管理页 |
| AppMonitorView.vue | ~300 | 监控聚合页 |
| AppOverviewView.vue | ~400 | 应用概览页 |
| AppRegistryView.vue | 196 | 镜像仓库页 |
| CatalogView.vue | ~500 | 服务目录页 |
| ComponentDetailView.vue | 959 ⚠️ | 组件详情页 |
| CreateAppView.vue | ~200 | 创建应用 |
| LoginView.vue | ~300 | 登录页 |
| PlatformSharedResourcesView.vue | ~150 | 共享资源管理 |
| PlatformUsersView.vue | ~200 | 用户管理 |
| TemplatesView.vue | ~2300 ⚠️ | 模板管理页 |

**建议（P2）：** 优先为 ComponentDetailView（959 行、高复杂度）和 TemplatesView（2300 行、零测试）添加 smoke test，然后为所有视图添加基础 render 测试。

---

### 14.2 environment.go — 第二个巨型文件（P2）

**结论：`internal/handler/environment.go` 8515 行，是仅次 EnvDetailView.vue 的第二大文件。**

| 指标 | 值 |
|------|-----|
| 行数 | **8515** |
| 测试文件 | `environment_test.go` 8096 行（测试覆盖好但文件同样庞大） |
| handler 函数数 | 40+（包含 CRUD、proxy、workspace action、canvas state 等） |
| N+1 查询检查 | 无法通过 grep 确认（文件太大） |
| Preload/Joins 使用 | 未发现 |

**问题：**
- 单个 handler 文件覆盖太多功能域（环境 CRUD、服务安装、画布状态、能力管理、外部访问、代理）
- 应当拆分：`environment.go`（CRUD）、`service_install.go`、`canvas_state.go`、`capability.go`
- 测试文件 8096 行同样过大，难以维护

**建议（P2）：** 按功能域拆分为 4-5 个文件：
1. `environment_crud.go` — 环境 CRUD handler
2. `environment_service.go` — 服务安装/升级/卸载
3. `environment_canvas.go` — 画布状态
4. `environment_capability.go` — 能力管理（已有单独文件但只有 699 行）

---

### 14.3 Go 依赖健康度（P3）

| 依赖 | 当前版本 | 评估 |
|------|---------|------|
| `gin` | v1.12.0 | ✅ 最新 |
| `controller-runtime` | v0.20.4 | ✅ 最新 |
| `k8s.io/client-go` | v0.32.3 | ✅ 最新 |
| `gorm.io/gorm` | v1.31.1 | ✅ 较新 |
| `gorm.io/driver/postgres` | v1.6.0 | ✅ 较新 |
| `helm.sh/helm/v3` | v3.16.4 | ✅ 较新（2025-03） |
| `gorilla/websocket` | v1.5.3 | ✅ 最新 |
| `golang.org/x/crypto` | v0.51.0 | ✅ 较新 |
| `minio/minio-go/v7` | **v7.2.0** | ⚠️ **过时（latest: v7.0.84+）** |
| `segmentio/kafka-go` | v0.4.48 | ✅ 较新 |
| `mongodb/mongo-driver/v2` | v2.5.0 | ✅ 较新 |
| `DATA-DOG/go-sqlmock` | v1.5.2 | ✅ 最新 |

**主要发现：**
- `minio-go/v7 v7.2.0` 是一个非常旧的版本（SemVer 被重置过，`v7.2.0` 对应 2021 年的代码），建议升级到 `v7.0.84+`（2025 年的实际最新版本）
- 其他依赖版本总体健康
- **未使用 `go vet` 或 `golangci-lint`**：Makefile 中缺少静态分析工具，无法自动发现潜在 bug

**建议（P3）：**
1. 升级 `minio-go/v7` 到最新版
2. 添加 `golangci-lint` 到 CI pipeline
3. 定期运行 `go mod tidy` 清理未使用的依赖

---

### 14.4 工程化工具缺失（延续 10.2）

**补充发现：**

| 工具 | 状态 |
|------|------|
| `go vet` | ❌ 不存在于 Makefile 中 |
| `golangci-lint` | ❌ 未安装 |
| `staticcheck` | ❌ 未安装 |
| pre-commit hooks | ❌ 无 |
| `.editorconfig` | ❌ 无 |
| Makefile `verify` 覆盖 | ⚠️ 仅 `go test ./...`，无 lint |
| 后端 error 类型 | ⚠️ 使用 `fmt.Errorf` + 字符串拼接，无 sentinel errors |

**后端错误处理模式：**
```go
// 当前：字符串拼接
return fmt.Errorf("failed to connect to database: %w", err)

// 更好的方式：定义 sentinel errors
var ErrDatabaseConnection = errors.New("database connection failed")
```

---

### 14.5 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | 15 个 Vue 视图零测试覆盖 | 🟡 P2 | 前端质量 |
| 2 | environment.go 8515 行（第二大文件） | 🟡 P2 | 后端可维护性 |
| 3 | minio-go 版本过旧 | 🟢 P3 | 功能兼容性 |
| 4 | Makefile 无 lint/vet 目标 | 🟢 P3 | 代码质量门禁 |

**建议：**
1. **（P2）为 ComponentDetailView 和 TemplatesView 添加 smoke test**
2. **（P2）拆分 environment.go** 为 4 个功能文件
3. **（P3）升级 minio-go/v7** + 添加 `golangci-lint` 到 Makefile

---

## 十五、打包与依赖审计 (2026-06-26)

### 15.1 前端打包体积分析（P2）

| 内容 | 大小 | 占比 |
|------|------|------|
| **EnvDetailView.js** | **400 KB** | **34%** 🔴 |
| 共享 vendor (index.js) | 104 KB | 9% |
| TemplatesView.js | 48 KB | 4% |
| ComponentDetailView.js | 24 KB | 2% |
| 其他视图 JS | ~80 KB | 7% |
| **EnvDetailView.css** | **160 KB** | **14%** 🟡 |
| 共享 CSS | 144 KB | 12% |
| 其他视图 CSS | ~200 KB | 18% |
| **总计** | **~1.2 MB** | 100% |

**关键发现：**
- **EnvDetailView 独占 34% JS + 14% CSS = 48% 的总 bundle** — 11654 行的 SFC 是打包体积最大的元凶
- 拆分 EnvDetailView 可减少首屏加载约 **400KB**（仅首次访问时加载）
- 共享 vendor 仅 104KB，说明三方库打包合理
- `framer-motion` + `motion`（12.40.0）两个动画库合计可能 50KB+，但项目实际使用 CSS transition 而非 JS 动画

**建议（P2）：**
1. 拆分 EnvDetailView 为懒加载子组件，首屏缩小 400KB
2. 评估 `framer-motion` + `motion` 是否必要（两个库功能重叠，至少可移除一个）

---

### 15.2 @carbon 包安装但未使用（P2）

| 包 | 版本 | 安装 | 实际使用 |
|----|------|------|---------|
| `@carbon/vue` | v3.0.30 | ✅ `package.json` | ❌ **未发现使用**（全部自建组件，未引用 Carbon Vue 组件） |
| `@carbon/styles` | v1.107.0 | ✅ `package.json` | ❌ **未发现使用**（项目使用自建 `carbon-theme.css`） |
| `@carbon/icons-vue` | v10.130.0 | ✅ `package.json` | ⚠️ **声明了但未使用**（106 个内联 SVG，0 个 `@carbon/icons-vue` import） |

**分析：**
- `@carbon/vue` 在 `vite.config.ts` 中声明为 `optimizeDeps.include`，但从未被任何 `.vue` 文件 import
- `@carbon/styles` 提供 SCSS 变量和 mixin，但项目维护了自己的 `carbon-theme.css`（898 行）
- `@carbon/icons-vue` 提供了 2000+ 个 Carbon 标准图标，但所有 106 个 SVG 都是手写的
- 这三个包合计约添加 300KB+ 到 `node_modules`，但不贡献任何运行时功能

**建议（P2）：**
1. 移除 `@carbon/vue` + `@carbon/styles`（如果确定不用 Carbon Vue 组件）
2. 或反之：**开始使用** `@carbon/icons-vue` 替换内联 SVG（106 个 SVG → Carbon 标准图标）
3. 清理 `vite.config.ts` 中的 `optimizeDeps.include: ['@carbon/vue']`（不再需要）

---

### 15.3 无 Vue 组件测试框架（P2）

**结论：`@vue/test-utils` 和 `jsdom` 均未安装，无法对 `.vue` 组件进行挂载/渲染测试。**

| 工具 | 安装 | 说明 |
|------|------|------|
| `@vue/test-utils` | ❌ | 无 `mount()`/`shallowMount()` 能力 |
| `jsdom` / `happy-dom` | ❌ | 无 DOM 环境模拟 |
| `vitest` | ✅ | 已安装，但只用于纯逻辑测试 |

**影响：**
- 所有 `.vue` 文件的 template 渲染逻辑无自动化测试
- `viewMarkup.test.ts` 是文本扫描测试（`toContain`），不是真正的组件渲染测试
- 组件交互（点击、输入、手势）无法测试
- 回归测试只能靠人工在浏览器中验证

**建议（P2）：**
```bash
npm install -D @vue/test-utils happy-dom
```
在 `vitest.config.ts` 中添加：
```ts
test: {
  environment: 'happy-dom',
}
```
然后为关键组件（尤其是 EnvDetailView 拆分后的子组件）添加 mount 测试。

---

### 15.4 配置管理风险（P2）

**当前实现 — `config/config.go`（22 行）：**
```go
func Load() *Config {
    return &Config{
        Port:        getEnv("PORT", "9090"),
        DatabaseURL: getEnv("DATABASE_URL", ""),
        JWTSecret:   getEnv("JWT_SECRET", "paap-dev-secret-change-in-prod"),
        // ...
    }
}
```

| 检查项 | 状态 |
|--------|------|
| 默认 JWT Secret（`paap-dev-secret-change-in-prod`） | ⚠️ 硬编码 dev 密钥，生产部署不更改则所有 JWT token 可被伪造 |
| `DATABASE_URL` 无默认值 | ✅ 正确（连接必须显式配置） |
| 配置值类型校验 | ❌ 无。`Port` 应为 int 但用 string |
| Required 字段验证 | ❌ 无。`JWTSecret` 和 `DatabaseURL` 启动时应检查非空 |
| 配置来源 | ⚠️ 仅 `os.Getenv`，不支持配置文件或 `.env` |

**建议（P2）：**
1. 在 `Load()` 中添加启动时校验：`if config.JWTSecret == "paap-dev-secret-change-in-prod" { log.Fatal("JWT_SECRET must be changed in production") }`
2. 添加 `config.Validate() error` 方法检查所有必需字段
3. 可选：支持 `.env` 文件（使用 `github.com/joho/godotenv`）

---

### 15.5 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | EnvDetailView 独占 48% bundle（400KB JS + 160KB CSS） | 🟡 P2 | 首屏加载 |
| 2 | `@carbon/vue` + `@carbon/styles` 安装但零使用 | 🟡 P2 | 依赖臃肿 |
| 3 | `@carbon/icons-vue` 安装但未使用（106 个手写 SVG） | 🟡 P2 | 图标一致性 |
| 4 | 无 `@vue/test-utils` — 无法测试组件渲染 | 🟡 P2 | 测试能力 |
| 5 | JWT_SECRET 硬编码 dev 默认值 | 🟡 P2 | 生产安全 |
| 6 | `framer-motion` + `motion` 双动画库重叠 | 🟢 P3 | 依赖冗余 |

**建议：**
1. **（P2）拆分 EnvDetailView**：减少首屏 400KB
2. **（P2）清理 Carbon 依赖**：移除不用的包或开始使用 `@carbon/icons-vue`
3. **（P2）安装 `@vue/test-utils` + `happy-dom`**：为 Vue 组件添加渲染测试能力
4. **（P2）生产部署防护**：`config.Load()` 中校验 `JWT_SECRET` 非默认值（见 §12.2 API 安全分析）
5. **（P2）迁移内联 SVG 到 Carbon 图标**：`@carbon/icons-vue` 已安装，替换 106 个手写 SVG（见 §7.2 SVG 审计）

---

## 十六、CRD 设计与 Operator 审计 (2026-06-26)

### 16.1 CRD API 版本设计（P2）

**现状：所有 CRD 使用单版本 `v1`，无版本化策略。**

| CRD | 文件 | Spec 类型 | Status 类型 | 行数 |
|-----|------|-----------|-------------|------|
| Application | `application_types.go` | `ApplicationSpec` | `ApplicationStatus` | 78 |
| Environment | `environment_types.go` | `EnvironmentSpec` | `EnvironmentStatus` | 108 |
| Component | `component_types.go` | `ComponentSpec` | `ComponentStatus` | 171 |
| ServiceInstance | `serviceinstance_types.go` | `ServiceInstanceSpec` | `ServiceInstanceStatus` | 334 |

**缺失项：**
- ❌ **无 `ValidateCreate()`/`ValidateUpdate()`/`ValidateDelete()`** — 无法在准入时校验必填字段、枚举值、镜像格式等
- ❌ **无 `Default()`** — 创建时无法设置默认值（如默认副本数、默认资源限制）
- ❌ **无多版本（`v1alpha1` → `v1beta1` → `v1`）** — 无 Conversion webhook，schema 更新必须断裂
- ❌ **无 `Hub()`/`ConvertTo()`/`ConvertFrom()`** — 无法支持存储版本与 API 版本分离

**建议（P2）：**
1. 至少添加 `Default()` webhook 为必填字段设置默认值（如副本数默认 1、资源限制默认值）
2. 添加 `ValidateCreate()` 校验 `Spec.Identifier` 合法性、镜像格式、端口范围
3. 后续考虑 `v1alpha1` → `v1` 版本演进支持

---

### 16.2 Operator Watch 模式（P2）

**所有 4 个 Controller 的 `SetupWithManager` 均过于简单，缺少对子资源的 Watch/Owns。**

| Controller | For | Owns | Watches | 说明 |
|-----------|-----|------|---------|------|
| Application | `Application` | ❌ | ❌ | 不跟踪 Namespace 变更 |
| Environment | `Environment` | ❌ | ❌ | 不跟踪 Namespace/NetworkPolicy/ResourceQuota 变更 |
| Component | `Component` | ❌ | ❌ | 不跟踪 Deployment/Service 变更 |
| ServiceInstance | `ServiceInstance` | ❌ | ✅ Namespace | 唯一有额外 Watch 的 |

**影响：**
- **用户手动修改 Deployment 副本数 → Operator 不会触发重新调谐**（除非有周期性 requeue）
- **Namespace 被误删 → Controller 无法立即恢复**（等下一次周期性 requeue）
- **Operator 完全依赖 `RequeueAfter` 轮询**，而非事件驱动

**建议（P2）：**
```go
// Component 应该 Watch 它创建的子资源
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&paapv1.Component{}).
        Owns(&appsv1.Deployment{}).   // 自动跟踪 Deployment 变更
        Owns(&corev1.Service{}).      // 自动跟踪 Service 变更
        Complete(r)
}
```

同样的模式应用到 Environment（Owns Namespace/NetworkPolicy/ResourceQuota）和 ServiceInstance。

---

### 16.3 RequeueAfter 一致性（P2）

**当前 9 个不同的 requeue 间隔值散布在 4 个 Controller 中，缺乏设计和文档：**

| 间隔 | 使用场景 | 所在 Controller |
|------|---------|----------------|
| **1s** | 工具就绪后立即重新检查 | ServiceInstance |
| **2s** | 等待子 CR 删除、等待 namespace 删除 | Application, Environment |
| **3s** | 等待 Helm 安装完成 | ServiceInstance |
| **5s** | 创建失败重试、获取失败重试 | Application, Component, ServiceInstance |
| **10s** | 等待 Helm 完成 | ServiceInstance |
| **30s** | 周期性状态刷新 | Application, Component |
| **60s** | 周期性状态刷新 | Environment |

**问题：**
- 1s 间隔在大量环境下可能造成 API server 压力
- 各 controller 无统一的 requeue 常量定义
- 不同 controller 对"重试"场景使用不同间隔（5s vs 10s）

**建议（P2）：**
1. 定义统一的 requeue 常量
   ```go
   const (
       RequeueImmediate = 1 * time.Second   // 立即重试（就绪轮询）
       RequeueRetry     = 5 * time.Second   // 操作失败重试
       RequeuePeriodic  = 30 * time.Second  // 周期性状态刷新
   )
   ```
2. 删除 2s/3s/10s/60s，统一映射到上述三档

---

### 16.4 k8s/client.go 基础设施风险（P1-P2）

**`internal/k8s/client.go`（401 行）使用 shell 式 kubectl/helm 而非 client-go SDK，存在多项问题：**

**🔴 硬编码凭据（P1 生产安全）：**
| 函数 | 凭据 | 问题 |
|------|------|------|
| `InstallPostgreSQL` | `auth.password: changeme123` | 所有 PostgreSQL 实例使用相同密码 |
| `InstallRabbitMQ` | `auth.password: changeme123` | 同上 |
| `InstallMinIO` | `auth.rootPassword: minioadmin123` | 同上 |
| `InstallRedis` | `auth.enabled: false` | 无认证，任何人可连接 |
| `DeployComponent` | 无 | 组件无认证 |

**🔴 exec.Command 替代 client-go（P2）：**
- kubectl `apply -f -` 的 YAML 通过字符串模板拼接，有 YAML injection 风险
- 无 client-go 的强类型保证（拼写错误在运行时才暴露）
- 每次操作都启动新的二进制进程，性能差
- 无法利用 `controller-runtime` 的缓存和重试机制

**🔴 生产部署硬编码 URL（P2）：**
```go
// ArgoCD 从 raw.githubusercontent.com 安装 — 版本固定但无校验
"https://raw.githubusercontent.com/argoproj/argo-cd/v2.13.3/manifests/namespace-install.yaml"

// Tekton — 版本 0.55.0 但 URL 可能有变动
"https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.55.0/release.yaml"

// Prometheus Operator — 版本 0.75.2
"https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.75.2/bundle.yaml"
```

这些函数在 `internal/handler/environment.go` 中由 HTTP handler 直接调用，存在 SSRF 风险和版本漂移问题。

**🟡 组件部署无健康检查（P2）：**
```go
func DeployComponent(...) {
    // Deployment 没有 livenessProbe/readinessProbe
    // 硬编码 containerPort: 8080
    // 不能自定义端口、环境变量、卷挂载
}
```

**🟡 无重试（P2）：** 所有 kubectl/helm 调用都不支持重试

**建议：**
1. **（P1）移除硬编码密码**：改为随机生成或从 Secret 引用
2. **（P2）逐步迁移到 client-go**：先迁移 Deployment/Service/Namespace 管理到 `controller-runtime` client
3. **（P2）用 Helm SDK 替代 helm exec**：`internal/helm/client.go` 已有 Helm SDK 封装，应该推广使用
4. **（P2）DeployComponent 支持自定义端口/探针/环境变量**

---

### 16.5 Go 并发安全分析（P3）

**goroutine 使用（仅 5 处）：**

| 位置 | 模式 | 是否安全 |
|------|------|---------|
| `websocket.go:97` | `go func()` — WebSocket 广播循环 | ✅ 使用 `sync.RWMutex` |
| `sync.go:39` | `go func()` — 定期集群同步 | ✅ 使用 `sync.Mutex` |
| `runtime_console.go:115` | `go func()` — 流式日志复制 | ✅ 使用 `sync.Mutex` |
| `environment.go:5704` | `go func()` — Gitea 缓存预热 | ⚠️ 使用 `sync.Mutex` |
| `environment.go:8439` | `go func()` — WebSocket 通知 | ✅ 无共享状态 |

**锁保护：**

| 位置 | 锁类型 | 保护对象 |
|------|--------|---------|
| `helm/client.go` | `sync.Map` + `sync.Mutex` | release 级别锁（并发 Helm 操作） |
| `websocket.go` | `sync.RWMutex` | WebSocket 连接池 |
| `sync.go` | `sync.Mutex` | 集群同步互斥 |
| `runtime_console.go` | `sync.Mutex` | 日志流 |
| `environment.go` | `sync.Mutex` | Gitea workspace 缓存 |

**发现：**
- ✅ goroutine 数量少且简单，没有复杂的并发模式
- ✅ `sync.Map` 用于 release 锁是合适的模式
- ✅ 所有 goroutine 都有对应的锁保护
- ❌ `k8s/client.go` 在 handler 中被并发调用时，可能有数据竞争（`exec.Command` 本身是并发安全的，但 `run()` 函数中的 `cmd.CombinedOutput()` 可能被多个 goroutine 调用同一个 Client 实例）
- ❌ `context.Background()` 在 `k8s/` 包中广泛使用，而非从请求传播 context，导致 goroutine 无法被取消

**建议（P3）：**
1. `k8s/client.go` 添加 `context.Context` 参数支持取消
2. 考虑用 `errgroup` 替代裸 `go func()` + `WaitGroup`

---

### 16.6 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | CRD 无 validating/mutating webhook（缺省填值+校验） | 🟡 P2 | 数据质量 |
| 2 | Controller 无子资源 Watch（依赖轮询而非事件驱动） | 🟡 P2 | 响应延迟 |
| 3 | RequeueAfter 间隔不一致（9 种值） | 🟢 P3 | 可维护性 |
| 4 | `k8s/client.go` 硬编码密码 (changeme123) | 🔴 P1 | 生产安全 |
| 5 | `k8s/client.go` 使用 exec.Command 而非 client-go | 🟡 P2 | 类型安全 |
| 6 | `k8s/client.go` 组件部署无健康检查 | 🟡 P2 | 可观测性 |
| 7 | `internal/helm/client.go` sync.Map 并发锁模式好 | ✅ 良好 | — |
| 8 | `context.Background()` 在 k8s/ 包中使用（无取消传播） | 🟡 P2 | 超时控制 |
| 9 | Operator 无最终一致性验证 | 🟢 P3 | 调试困难 |

**建议：**
1. **（P1）** 立即修复 `k8s/client.go` 中的硬编码密码
2. **（P2）** 为 CRD 添加 `Default()` 和 `ValidateCreate()` webhook
3. **（P2）** Controller 的 `SetupWithManager` 添加 `Owns()` 子资源 Watch
4. **（P2）** `k8s/client.go` 逐步迁移到 client-go/Helm SDK
5. **（P2）** 统一 `RequeueAfter` 常量（三档）
6. **（P2）** 用传播的 context 替换 `context.Background()`

---

## 十七、最终扫描：测试覆盖 · 错误类型 · Docker · K8s 清单 · 依赖 (2026-06-26)

### 17.1 Go 测试覆盖率分析（P2）

```
包                          覆盖率
internal/controller          49.5%    🟡
internal/database             6.2%    🔴
internal/handler             16.7%    🔴
internal/helm                43.9%    🟡
internal/k8s                 47.6%    🟡
internal/middleware          50.0%    🟡
internal/model               61.3%    ✅
internal/service             41.4%    🟡
api/v1                        0.0%    🔴
```

**关键发现：**
- **`api/v1` (CRD 类型) 0%** — 无任何测试，但 CRD 类型主要是结构体定义，测试价值不高
- **`internal/database` 仅 6.2%** — DB 迁移逻辑 Untested
- **`internal/handler` 仅 16.7%** — 90%+ 的 HTTP handler 无测试覆盖
- **整体平均 ~35%** — 项目初期合理水平，但 handler/service 层需要提升

**建议（P2）：**
1. 优先为 `internal/database/migrations.go` 添加测试（自动迁移正确性）
2. handler 层至少覆盖关键路径（创建环境、安装服务、部署组件）

---

### 17.2 Go 错误处理模式（P2）

| 统计项 | 数值 |
|--------|------|
| `errors.New()` | 9 处 |
| `fmt.Errorf()` | 365 处 |
| `fmt.Errorf(...%w)` (wrapped) | ~20 处 |
| Sentinel errors (var Err*) | **0 处** |

**发现：**
- 项目使用 `fmt.Errorf()` 占绝对主流（365:9），风格一致 ✅
- **无 sentinel errors** — 所有错误都是内联创建，调用方只能用字符串匹配来判断错误类型
- `%w` wrapping 使用比例约 **5%**（20/365），大部分错误不包含根因链
- 错误语言混用：`"missing token"`（英文）、`"releaseName is required"`（英文）、部分中文提示混在 handler 中
- 无自定义错误类型（业务错误码/分类）

**建议（P2）：**
1. 对业务关键错误使用 `fmt.Errorf("...: %w", err)` 保留根因链
2. 定义少量 sentinel errors：`var ErrNotFound = errors.New("not found")`、`var ErrConflict = errors.New("conflict")`
3. 统一错误语言（英文编程，中文用户提示在 handler 层转换）

---

### 17.3 Docker 镜像与构建分析（P2）

| 镜像 | 构建方式 | 基础镜像 | 阶段数 | 用户 |
|------|---------|---------|--------|------|
| Server | Multi-stage (3 阶段) | `node:26-alpine` → `golang:1.26-alpine` → `alpine:3.22` | 3 | `65532:65532` |
| Operator | Multi-stage (2 阶段) | `golang:1.26-alpine` → `alpine:3.22` | 2 | `65532:65532` |

**✅ 良好实践：**
- 都使用 multi-stage build（最小最终镜像）
- 都使用 non-root user（65532）
- Server 镜像包含 ca-certificates
- Server 镜像正确处理了 Chart 文件权限（data/charts）

**🔴 问题：**
- **Go 模块缓存不跨构建** — 每次 `COPY go.mod .` + `go mod download` 虽然使用了 Docker 缓存层，但未使用 `GOMODCACHE` 缓存加速
- **Server 镜像 3 阶段可优化为 2 阶段** — `frontend dist` 复制路径长
- **CGO_ENABLED=1** for Server（需要 gcc/musl-dev）— 增加构建复杂度和镜像大小
- Operator 未使用 `GOPROXY` 配置（Server 使用了 `goproxy.cn`）
- Operator 未指定 `GOMAXPROCS`（Server 已指定）
- 无 `.dockerignore`（未排除 /tmp、.git、node_modules 等）

**建议（P2）：**
1. 添加 `.dockerignore`：排除 `.git`、`node_modules`、`/tmp`、`*.md`
2. Server 也启用 `CGO_ENABLED=0`（如果没有 cgo 依赖）
3. Operator 添加 `GOPROXY` 和 `GOMAXPROCS` 与 Server 一致
4. 考虑 `--mount=type=cache` 加速 Go 构建（BuildKit）

---

### 17.4 K8s 部署清单安全分析（P2）

| 清单 | Namespace | SecurityContext | 资源限制 | 探针 |
|------|-----------|----------------|---------|------|
| `paap-server.yaml` | ✅ `paap-system` | ❌ 未设置 | ❌ 未设置 | ✅ readinessProbe |
| `paap-operator.yaml` | ✅ `paap-system` | ✅ `allowPrivilegeEscalation: false` | ✅ CPU/Memory 限制 | ✅ liveness+readiness |
| `postgres.yaml` | ✅ `paap-system` | ❌ 未设置 | ❌ 未设置 | ❌ |
| `minio.yaml` | ✅ `paap-system` | ❌ 未设置 | ❌ 未设置 | ❌ |
| `keycloak.yaml` | — | — | — | — |

**Server 缺失的安全项（对比 Operator）：**
```yaml
# paap-server.yaml 缺少:
securityContext:
  allowPrivilegeEscalation: false
  runAsNonRoot: true
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: true
resources:
  limits:
    cpu: "1"
    memory: 1Gi
  requests:
    cpu: 100m
    memory: 256Mi
```

**🔴 RBAC 过宽：** Operator 使用 `apiGroups: ["*"]` + `resources: ["*"]` + `verbs: ["*"]` — 理论需要安装任意 Helm Chart 但违背最小权限原则。建议添加注释说明为什么无法细分。（另见 §16.4 `k8s/client.go` 基础设施风险）

**建议（P2）：**
1. 为 `paap-server.yaml` 添加 `securityContext`（至少 `allowPrivilegeEscalation: false`）
2. 为 `paap-server.yaml` 添加 `resources` 限制
3. 为 `postgres.yaml`、`minio.yaml` 添加 `resources` 和探针
4. Operator 的 wildcard RBAC 添加文档注释解释必要性

---

### 17.5 Go 依赖与 License 简要审计（P3）

**依赖规模：** `go.sum` 686 行，约 250 个传递依赖。

**关键运行时依赖：**

| 依赖 | 用途 | 版本 | License |
|------|------|------|---------|
| `gin` v1.12.0 | HTTP 框架 | ✅ 最新 | MIT |
| `gorm` v1.31.1 | ORM | ⚠️ 落后（最新 ~v1.32） | MIT |
| `controller-runtime` v0.20.4 | Operator 框架 | ✅ 较新 | Apache-2.0 |
| `client-go` v0.32.3 | K8s 客户端 | ✅ 配合 K8s 1.32 | Apache-2.0 |
| `helm.sh/helm/v3` v3.16.4 | Helm SDK | ✅ 较新 | Apache-2.0 |
| `minio-go/v7` v7.2.0 | MinIO 客户端 | ⚠️ 落后（最新 ~v7.5） | AGPL-3.0 |
| `kafka-go` v0.4.48 | Kafka 客户端 | ✅ 较新 | MIT |
| `mongo-driver/v2` v2.5.0 | MongoDB 驱动 | ✅ 较新 | Apache-2.0 |

**License 提醒：**
- `minio-go/v7` 使用 **AGPL-3.0** — 如果项目是商业闭源产品，AGPL 可能不兼容。需要确认 minio-go 是否算 AGPL 传染
- 其他均为宽松许可证（MIT/Apache-2.0/BSD）

**建议（P3）：**
1. 升级 `gorm.io/gorm` 到 v1.32+（获取最新 bugfix）
2. 升级 `minio-go/v7`（v7.2.0 → v7.5+）
3. 法务确认 `minio-go` AGPL 是否适用于 PAAP 的发布方式

---

### 17.6 汇总

| # | 问题 | 严重程度 | 影响 |
|---|------|---------|------|
| 1 | Go 测试覆盖 16-35%（handler/database 最低） | 🟡 P2 | 回归风险 |
| 2 | 无 sentinel errors，%w wrapping 仅 5% | 🟡 P2 | 错误调试 |
| 3 | Server Docker 缺 .dockerignore / CGO 可禁用 | 🟢 P3 | 构建体积 |
| 4 | paap-server.yaml 缺 securityContext / resources | 🟡 P2 | 集群安全 |
| 5 | postgres/minio.yaml 缺探针/资源限制 | 🟡 P2 | 可观测性 |
| 6 | Operator RBAC wildcard 无注释说明（另见 §16.4） | 🟡 P2 | 最小权限 |
| 7 | minio-go AGPL-3.0 License | 🟢 P3 | 法务合规 |

---

## 附录：参考资源

- [Railway Design System](https://www.shadcn.io/design/railway) - 深色画布、4px 圆角、发丝边框
- [Vercel Web Interface Guidelines](https://vercel.com/design/guidelines) - 层叠阴影、嵌套圆角、半透明边框
- [React Flow Theming](https://reactflow.dev/learn/customization/theming) - 官方节点样式、CSS 变量
- [React Flow BaseNode](https://reactflow.dev/ui/components/base-node) - 官方节点组件库
- [kScratch](https://kscratch.app/) - K8s 可视化、健康状态颜色
- [InfraCanvas](https://github.com/bytestrix/InfraCanvas) - 实时拓扑图、状态指示器
