<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">模板管理</h1>
        <p class="page-desc">管理工具模板、中间件模板和组件运行配置模板</p>
      </div>
      <div class="header-actions">
        <button v-if="activeTemplateTab !== 'config'" class="rail-btn rail-btn--ghost" :disabled="syncing" @click="syncBuiltinTemplates">
          {{ syncing ? '同步中...' : '同步内置模板' }}
        </button>
        <button v-else class="rail-btn rail-btn--ghost" :disabled="syncing" @click="syncBuiltinConfigTemplates">
          {{ syncing ? '同步中...' : '同步内置配置模板' }}
        </button>
        <button v-if="activeTemplateTab === 'config'" class="rail-btn rail-btn--primary" @click="openConfigTemplateImportModal">
          导入配置模板
        </button>
        <button v-if="activeTemplateTab !== 'config'" class="rail-btn rail-btn--primary" @click="openUploadModal">
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M11 18l-7 7 7 7v-5h12v-4H11v-5zM21 14V9H9v4h12v5l7-7-7-7v5z"/></svg>
          上传模板
        </button>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <nav class="template-tabs" aria-label="模板分类">
      <button
        v-for="tab in templateTabs"
        :key="tab.key"
        type="button"
        :class="{ active: activeTemplateTab === tab.key }"
        @click="activeTemplateTab = tab.key"
      >
        <span>{{ tab.label }}</span>
        <strong>{{ tab.count }}</strong>
      </button>
    </nav>

    <!-- KPI -->
    <div class="kpi-section">
      <div class="kpi-card">
        <div class="kpi-number">{{ templates.length }}</div>
        <div class="kpi-label">部署模板总数</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ toolTemplates.length }}</div>
        <div class="kpi-label">工具模板</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ middlewareTemplates.length }}</div>
        <div class="kpi-label">中间件模板</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ configTemplates.length }}</div>
        <div class="kpi-label">配置模板</div>
      </div>
    </div>

    <!-- Template list -->
    <section class="section-card">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">{{ activeTemplateTitle }}</h2>
          <p class="rail-section-desc">{{ activeTemplateDescription }}</p>
        </div>
        <button class="rail-btn rail-btn--ghost" @click="refreshActiveTemplates">
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M26 12H6V10h20v2zm0 4H6v2h20v-2zm0 6H6v2h20v-2z"/></svg>
          刷新
        </button>
      </div>

      <div v-if="loading" class="loading-mask">
        <div class="loading-spinner" />
      </div>

      <div v-else-if="activeTemplateTab !== 'config' && activeServiceTemplates.length === 0" class="rail-empty">
        <p class="rail-empty-desc">暂无{{ activeTemplateTitle }}，点击「同步内置模板」初始化。</p>
      </div>

      <div v-else-if="activeTemplateTab !== 'config'" class="template-list">
        <div v-for="tmpl in activeServiceTemplates" :key="tmpl.id || tmpl.type" class="template-row">
          <div class="template-body">
            <div class="template-header">
              <div class="template-name-group">
                <span class="template-name">{{ tmpl.name }}</span>
                <span :class="['tag', tmpl.isCustom ? 'custom' : 'builtin']">{{ tmpl.isCustom ? '自定义' : '内置' }}</span>
                <span v-if="isHeavyTemplate(tmpl)" class="tag heavy">重型</span>
              </div>
              <span class="policy">{{ permissionSummary(tmpl) }}</span>
            </div>
            <div class="template-meta">
              <span>{{ tmpl.type }}</span>
              <span>{{ categoryLabel(tmpl.category) }}</span>
              <span :class="{ uploaded: tmpl.s3Key || tmpl.chartArchivePath }">{{ (tmpl.s3Key || tmpl.chartArchivePath) ? '已上传' : '未上传' }}</span>
            </div>
            <p class="template-desc">{{ tmpl.description || '无描述' }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="configTemplates.length === 0" class="rail-empty">
        <p class="rail-empty-desc">暂无配置模板。点击「同步内置配置模板」初始化常用框架配置。</p>
      </div>

      <div v-else class="template-list config-template-list">
        <div v-for="tmpl in configTemplates" :key="tmpl.id || tmpl.name" class="template-row config-template-row">
          <div class="template-body">
            <div class="template-header">
              <div class="template-name-group">
                <span class="template-name">{{ tmpl.name }}</span>
                <span :class="['tag', tmpl.isBuiltin ? 'builtin' : 'custom']">{{ tmpl.isBuiltin ? '内置' : '自定义' }}</span>
                <span class="tag config">{{ configTemplateFrameworkLabel(tmpl.framework) }}</span>
              </div>
              <span class="policy">{{ configTemplateBindingLabel(tmpl.bindingMode) }}</span>
            </div>
            <p class="template-desc">{{ configTemplateDisplayText(tmpl.description) || '无描述' }}</p>
            <div class="config-template-plain-summary">
              <span>{{ configTemplateComponentTypeSummary(tmpl.componentTypes) }}</span>
              <span>{{ configTemplateNativeBlockCount(tmpl) }}</span>
              <span>{{ configTemplateEditableFieldCount(tmpl.fields) }}</span>
            </div>
            <div class="template-row-actions">
              <button type="button" class="rail-btn rail-btn--ghost rail-btn--sm" @click="openConfigTemplateDetail(tmpl)">
                查看模板
              </button>
              <button v-if="!tmpl.isBuiltin" type="button" class="rail-btn rail-btn--ghost rail-btn--sm danger" @click="deleteConfigTemplate(tmpl.id)">
                删除
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Upload Modal -->
    <Teleport to="body">
      <div v-if="showUploadModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showUploadModal = false">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">上传自定义模板</p>
              <p class="modal-heading">上传包含 Helm Chart 的 .tar.gz 包</p>
            </div>
            <button class="modal-close" @click="showUploadModal = false">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-item">
              <label class="form-label">模板类型</label>
              <input v-model.trim="uploadForm.type" class="rail-input" placeholder="例如：custom-prometheus" />
            </div>
            <div class="form-item">
              <label class="form-label">显示名称</label>
              <input v-model.trim="uploadForm.name" class="rail-input" placeholder="例如：自定义 Prometheus" />
            </div>
            <div class="form-item">
              <label class="form-label">分类</label>
              <select v-model="uploadForm.category" class="rail-select">
                <option value="tool">工具</option>
                <option value="infra">基础设施</option>
                <option value="middleware">中间件</option>
              </select>
            </div>
            <div class="form-item">
              <label class="form-label">描述</label>
              <textarea v-model.trim="uploadForm.description" class="rail-textarea" rows="4" placeholder="说明模板用途"></textarea>
            </div>
            <div class="form-item">
              <label class="form-label">模板包</label>
              <input class="rail-input" type="file" accept=".tar.gz,.tgz" @change="onFileChange" />
              <div class="form-helper">上传包含 chart/、platform-manifest.yaml、preset-values.yaml 的 .tar.gz 或 .tgz 包。</div>
            </div>
            <div v-if="uploadError" class="form-error" role="alert">{{ uploadError }}</div>
          </div>
          <div class="modal-footer">
            <button class="rail-btn rail-btn--ghost" @click="showUploadModal = false">取消</button>
            <button class="rail-btn rail-btn--primary" :disabled="uploading" @click="submitUpload">
              {{ uploading ? '上传中...' : '上传模板' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showConfigTemplateImportModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showConfigTemplateImportModal = false">
        <div class="modal-container modal-container--wide">
          <div class="modal-header">
            <div>
              <p class="modal-label">导入配置模板</p>
              <p class="modal-heading">{{ configImportMode === 'native' ? '从原生配置创建模板' : '导入高级模板 JSON' }}</p>
            </div>
            <button class="modal-close" @click="showConfigTemplateImportModal = false">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body config-import-shell--carbon">
            <div class="config-import-grid">
              <div class="form-item config-import-mode">
                <label class="form-label">导入模式</label>
                <div class="template-mode-switch" role="group" aria-label="导入模式">
                  <button type="button" class="config-import-mode-card" :class="{ active: configImportMode === 'native' }" @click="configImportMode = 'native'">
                    <span>普通配置</span>
                    <small>普通配置适合从现有配置文件快速生成模板</small>
                  </button>
                  <button type="button" class="config-import-mode-card" :class="{ active: configImportMode === 'advanced' }" @click="configImportMode = 'advanced'">
                    <span>高级模板 JSON</span>
                    <small>平台工程师可导入完整 schema/template JSON</small>
                  </button>
                </div>
              </div>
              <div class="form-item">
                <label class="form-label">模板名称</label>
                <input v-model.trim="configImportForm.name" class="rail-input" placeholder="例如：Spring Boot + PostgreSQL + Redis" />
              </div>
              <div class="form-item">
                <label class="form-label">模板 Key（可选）</label>
                <input v-model.trim="configImportForm.key" class="rail-input" placeholder="留空则后台自动生成" />
              </div>
              <div class="form-item">
                <label class="form-label">应用框架</label>
                <select v-model="configImportForm.framework" class="rail-select">
                  <option value="auto">自动</option>
                  <option value="springboot">Spring Boot</option>
                  <option value="node">Node.js</option>
                  <option value="go">Go</option>
                  <option value="python">Python</option>
                  <option value="nginx">Nginx</option>
                  <option value="custom">自定义</option>
                </select>
              </div>
              <div class="form-item">
                <label class="form-label">适用组件</label>
                <select id="config-template-component-type-select" v-model="configImportForm.componentTypes" class="rail-select">
                  <option v-for="option in configTemplateComponentTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
                <div class="form-helper">选择后会影响组件配置 Tab 的模板候选范围。</div>
              </div>
            </div>
            <div class="form-item">
              <label class="form-label">描述</label>
              <textarea v-model.trim="configImportForm.description" class="rail-textarea" rows="2" placeholder="说明模板会生成哪些环境变量、配置文件和敏感配置"></textarea>
            </div>
            <div class="form-item">
              <label class="form-label">{{ configImportMode === 'native' ? '原生配置文件' : '高级模板 JSON 文件' }}</label>
              <input class="rail-input" type="file" :accept="configImportMode === 'native' ? '.yml,.yaml,.json,.toml,.conf,.ini,.properties,.env,.txt,application/json,text/*' : '.json,application/json'" @change="onConfigTemplateFileChange" />
              <div class="form-helper">{{ configImportMode === 'native' ? '普通模式直接使用 __TEMPLATE__KEY__显示名__ 标记变量；挂载路径作为推荐值，具体组件可在右侧栏调整。' : '高级模式可上传完整 PAAP template.json，或包含 template/schema 的 JSON。' }}</div>
            </div>
            <div class="form-item">
              <div class="form-label-row">
                <label class="form-label">{{ configImportMode === 'native' ? '原生配置内容' : '高级模板 JSON' }}</label>
                <button type="button" class="text-btn" @click="fillConfigTemplateSample">{{ configImportMode === 'native' ? '填入示例' : '填入高级示例' }}</button>
              </div>
              <textarea v-model.trim="configTemplateImportText" class="rail-textarea code-textarea" rows="14" spellcheck="false" :placeholder="configTemplateImportPlaceholder"></textarea>
              <div class="form-helper">{{ configImportMode === 'native' ? '普通用户只需要在原生配置中写 __TEMPLATE__KEY__显示名__；FOR/IF 会自动解析成字段。' : '高级 JSON / schema 模式适合平台工程师，可直接定义字段、环境变量、普通配置、敏感配置和文件。' }}</div>
            </div>
            <div v-if="configUploadError" class="form-error" role="alert">{{ configUploadError }}</div>
          </div>
          <div class="modal-footer">
            <button class="rail-btn rail-btn--ghost" @click="showConfigTemplateImportModal = false">取消</button>
            <button class="rail-btn rail-btn--primary" :disabled="configUploading" @click="submitConfigTemplateImport">
              {{ configUploading ? '导入中...' : '导入配置模板' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="selectedConfigTemplate" class="modal-overlay" role="dialog" aria-modal="true" @click.self="selectedConfigTemplate = null">
        <div class="modal-container modal-container--template">
          <div class="modal-header">
            <div>
              <p class="modal-label">配置模板预览</p>
              <p class="modal-heading">{{ selectedConfigTemplate.name }}</p>
            </div>
            <button class="modal-close" @click="selectedConfigTemplate = null">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body template-preview">
            <p class="template-preview-desc">{{ configTemplateDisplayText(selectedConfigTemplate.description) || '无描述' }}</p>
            <div class="template-preview-meta">
              <span>{{ configTemplateFrameworkLabel(selectedConfigTemplate.framework) }}</span>
              <span>{{ configTemplateBindingLabel(selectedConfigTemplate.bindingMode) }}</span>
              <span>{{ selectedConfigTemplate.isBuiltin ? '内置模板' : '自定义模板' }}</span>
            </div>
            <div class="template-preview-summary" aria-label="配置模板影响摘要">
              <div v-for="item in configTemplatePreviewSummary(selectedConfigTemplate)" :key="item.label" class="template-preview-summary-item">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>
            <div class="template-preview-tabs">
              <button type="button" :class="{ active: configTemplatePreviewTab === 'native' }" @click="configTemplatePreviewTab = 'native'">原生配置预览</button>
              <button type="button" :class="{ active: configTemplatePreviewTab === 'advanced' }" @click="configTemplatePreviewTab = 'advanced'">高级 JSON / schema</button>
            </div>

            <section v-if="configTemplatePreviewTab === 'native'" class="template-preview-section">
              <div v-if="configTemplateNativePreviewBlocks(selectedConfigTemplate).length" class="preview-block-list">
                <div v-for="block in configTemplateNativePreviewBlocks(selectedConfigTemplate)" :key="block.name" class="preview-block">
                  <div class="preview-block-title">{{ block.name }}</div>
                  <pre><code>{{ block.content }}</code></pre>
                </div>
              </div>
              <div v-else class="preview-empty">这个模板没有原生配置文件内容，只会生成运行变量。</div>
            </section>

            <section v-else class="template-preview-section template-preview-advanced">
              <div class="preview-block">
                <div class="preview-block-title">schema.json</div>
                <pre><code>{{ configTemplateSchemaJSON(selectedConfigTemplate) }}</code></pre>
              </div>
              <div class="preview-block">
                <div class="preview-block-title">template.json</div>
                <pre><code>{{ configTemplateRawJSON(selectedConfigTemplate) }}</code></pre>
              </div>
            </section>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="pendingConfigTemplateDelete" class="modal-overlay" role="dialog" aria-modal="true" @click.self="pendingConfigTemplateDelete = null">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">删除配置模板</p>
              <p class="modal-heading">{{ pendingConfigTemplateDelete.name }}</p>
            </div>
            <button class="modal-close" @click="pendingConfigTemplateDelete = null">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="confirm-text">删除后，这个自定义配置模板将不再出现在组件右侧栏中。已应用到组件的运行配置不会被自动回滚。</p>
            <div v-if="pageError" class="form-error" role="alert">{{ pageError }}</div>
          </div>
          <div class="modal-footer">
            <button class="rail-btn rail-btn--ghost" @click="pendingConfigTemplateDelete = null">取消</button>
            <button class="rail-btn rail-btn--primary danger" @click="confirmDeleteConfigTemplate">删除</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '../api/client'
import { nativeConfigTemplateSyntax, parseNativeConfigTemplate } from './configTemplateSyntax'
import { normalizeComponentTemplateFiles } from './componentConfigTemplateFiles'

const templates = ref<any[]>([])
const configTemplates = ref<any[]>([])
const loading = ref(false)
const uploading = ref(false)
const configUploading = ref(false)
const syncing = ref(false)
const showUploadModal = ref(false)
const showConfigTemplateImportModal = ref(false)
const uploadFile = ref<File | null>(null)
const pageError = ref('')
const uploadError = ref('')
const configUploadError = ref('')
const selectedConfigTemplate = ref<any | null>(null)
const configTemplatePreviewTab = ref<'native' | 'advanced'>('native')
const configImportMode = ref<'native' | 'advanced'>('native')
const pendingConfigTemplateDelete = ref<any | null>(null)
const activeTemplateTab = ref<'tool' | 'middleware' | 'config'>('tool')
const uploadForm = ref({
  type: '',
  name: '',
  category: 'tool',
  description: '',
})
const configImportForm = ref({
  key: '',
  name: '',
  description: '',
  framework: 'auto',
  bindingMode: 'recommended',
  componentTypes: 'backend',
})
const configTemplateImportText = ref('')
const configTemplateComponentTypeOptions = [
  { value: 'all', label: '所有组件' },
  { value: 'frontend', label: '前端组件' },
  { value: 'backend', label: '后端组件' },
  { value: 'frontend-backend', label: '前端 + 后端' },
  { value: 'worker', label: 'Worker / 任务组件' },
  { value: 'custom', label: '自定义组件' },
]
const configTemplateImportPlaceholder = computed(() => configImportMode.value === 'native'
  ? 'spring:\n  datasource:\n    url: __TEMPLATE__JDBC_URL__数据库地址__'
  : '{\n  "template": { "name": "...", "fields": [] },\n  "schema": { "fields": [] }\n}'
)

const middlewareTemplateTypes = new Set([
  'mysql',
  'mysql-galera',
  'postgresql',
  'postgresql-ha',
  'mongodb',
  'redis',
  'redis-cluster',
  'rabbitmq',
  'kafka',
  'minio',
])
const sortedTemplates = computed(() =>
  [...templates.value].sort((a, b) => {
    const aOrder = Number(a.installOrder ?? 999)
    const bOrder = Number(b.installOrder ?? 999)
    if (aOrder !== bOrder) return aOrder - bOrder
    return String(a.name || a.type).localeCompare(String(b.name || b.type), 'zh-Hans-CN')
  })
)
const middlewareTemplates = computed(() => sortedTemplates.value.filter(isMiddlewareTemplate))
const toolTemplates = computed(() => sortedTemplates.value.filter((tmpl) => !isMiddlewareTemplate(tmpl)))
const activeServiceTemplates = computed(() => activeTemplateTab.value === 'middleware' ? middlewareTemplates.value : toolTemplates.value)
const templateTabs = computed(() => [
  { key: 'tool' as const, label: '工具模板', count: toolTemplates.value.length },
  { key: 'middleware' as const, label: '中间件模板', count: middlewareTemplates.value.length },
  { key: 'config' as const, label: '配置模板', count: configTemplates.value.length },
])
const activeTemplateTitle = computed(() => templateTabs.value.find((tab) => tab.key === activeTemplateTab.value)?.label || '模板列表')
const activeTemplateDescription = computed(() => {
  if (activeTemplateTab.value === 'config') return '组件运行配置模板，包含环境变量、普通配置、敏感配置、配置文件和启动参数。'
  if (activeTemplateTab.value === 'middleware') return '数据库、缓存、消息队列和对象存储等中间件的 Helm 部署模板。'
  return '代码仓库、镜像仓库、部署、监控、日志和 CI 等平台工具模板。'
})

onMounted(async () => {
  await Promise.all([loadTemplates(), loadConfigTemplates()])
})

async function loadTemplates() {
  loading.value = true
  pageError.value = ''
  try {
    const res = await api.listServiceTemplates()
    templates.value = res.data || []
  } catch (e: any) {
    pageError.value = '加载模板失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

async function loadConfigTemplates() {
  pageError.value = ''
  try {
    const res = await api.listComponentConfigTemplates()
    const parsed = res.data || []
    configTemplates.value = Array.isArray(parsed) ? parsed.map(normalizeConfigTemplate).filter(Boolean) : []
  } catch (e: any) {
    configTemplates.value = []
    pageError.value = '加载配置模板失败：' + (e?.message || '未知错误')
  }
}

async function refreshActiveTemplates() {
  if (activeTemplateTab.value === 'config') {
    await loadConfigTemplates()
    return
  }
  void loadTemplates()
}

function normalizeConfigTemplate(raw: any) {
  const name = String(raw?.name || '').trim()
  if (!name) return null
  return {
    id: String(raw?.id || name),
    name,
    description: String(raw?.description || '').trim(),
    framework: String(raw?.framework || 'auto'),
    bindingMode: String(raw?.bindingMode || 'recommended'),
    componentTypes: Array.isArray(raw?.componentTypes) ? raw.componentTypes.map((item: any) => String(item).trim()).filter(Boolean) : [],
    fields: Array.isArray(raw?.fields) ? raw.fields : [],
    nativeConfigs: normalizeConfigTemplateNativeConfigs(raw?.nativeConfigs),
    syntax: String(raw?.syntax || ''),
    isBuiltin: Boolean(raw?.isBuiltin),
    env: Array.isArray(raw?.env) ? raw.env.map(normalizeConfigTemplateEnv).filter((item: any) => item.name) : [],
    configMaps: normalizeConfigTemplateObjects(raw?.configMaps),
    secrets: normalizeConfigTemplateObjects(raw?.secrets),
    files: normalizeConfigTemplateFiles(raw?.files),
    command: Array.isArray(raw?.command) ? raw.command.map((item: any) => String(item).trim()).filter(Boolean) : [],
    args: Array.isArray(raw?.args) ? raw.args.map((item: any) => String(item).trim()).filter(Boolean) : [],
  }
}

function normalizeConfigTemplateObjects(items: any) {
  if (!Array.isArray(items)) return []
  return items.map((item: any) => ({
    name: String(item?.name || '').trim(),
    data: Object.fromEntries(Object.entries(item?.data || {}).map(([key, value]) => [String(key).trim(), String(value ?? '')]).filter(([key]) => key)),
  })).filter((item: any) => Object.keys(item.data).length)
}

function normalizeConfigTemplateNativeConfigs(items: any) {
  if (!Array.isArray(items)) return []
  return items.map((item: any) => ({
    name: String(item?.name || '').trim(),
    content: String(item?.content ?? ''),
  })).filter((item: any) => item.name && item.content)
}

function normalizeConfigTemplateFiles(items: any) {
  return normalizeComponentTemplateFiles(Array.isArray(items) ? items : [])
}

function normalizeConfigTemplateEnv(item: any) {
  return {
    name: String(item?.name || '').trim(),
    source: String(item?.source || 'value'),
    value: String(item?.value || ''),
    refName: String(item?.refName || ''),
    refKey: String(item?.refKey || ''),
  }
}

function openConfigTemplateDetail(tmpl: any) {
  selectedConfigTemplate.value = tmpl
  configTemplatePreviewTab.value = 'native'
}

function openUploadModal() {
  uploadError.value = ''
  showUploadModal.value = true
}

function openConfigTemplateImportModal() {
  configUploadError.value = ''
  showConfigTemplateImportModal.value = true
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  uploadFile.value = input.files?.[0] || null
  uploadError.value = ''
}

async function submitUpload() {
  uploadError.value = ''
  if (!uploadForm.value.type || !uploadForm.value.name || !uploadFile.value) {
    uploadError.value = '请填写模板类型、显示名称并选择模板包'
    return
  }
  const formData = new FormData()
  formData.append('type', uploadForm.value.type)
  formData.append('name', uploadForm.value.name)
  formData.append('category', uploadForm.value.category)
  formData.append('description', uploadForm.value.description)
  formData.append('file', uploadFile.value)

  uploading.value = true
  try {
    await api.uploadServiceTemplate(formData)
    uploadForm.value = { type: '', name: '', category: 'tool', description: '' }
    uploadFile.value = null
    showUploadModal.value = false
    await loadTemplates()
  } catch (e: any) {
    uploadError.value = '上传模板失败：' + (e?.message || '未知错误')
  } finally {
    uploading.value = false
  }
}

async function onConfigTemplateFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  configUploadError.value = ''
  if (!file) return
  try {
    configTemplateImportText.value = await file.text()
  } catch (e: any) {
    configUploadError.value = '读取模板文件失败：' + (e?.message || '未知错误')
  }
}

function fillConfigTemplateSample() {
  if (configImportMode.value === 'advanced') {
    if (!configImportForm.value.name.trim()) {
      configImportForm.value.name = '自定义高级配置模板'
      configImportForm.value.framework = 'custom'
      configImportForm.value.componentTypes = 'backend'
    }
    configTemplateImportText.value = JSON.stringify({
      template: {
        name: '自定义高级配置模板',
        framework: 'custom',
        componentTypes: ['backend'],
        fields: [
          { key: 'APP_PORT', label: '应用端口', type: 'number', default: '8080' },
        ],
        env: [
          { name: 'APP_PORT', source: 'value', value: '[[paap:APP_PORT default=8080]]' },
        ],
      },
      schema: {
        fields: [
          { key: 'APP_PORT', label: '应用端口', type: 'number', default: '8080' },
        ],
      },
    }, null, 2)
    return
  }
  if (!configImportForm.value.name.trim()) {
    configImportForm.value.name = '自定义 Spring Boot 配置'
    configImportForm.value.framework = 'springboot'
    configImportForm.value.componentTypes = 'backend'
  }
  configTemplateImportText.value = [
    'spring:',
    '  datasource:',
    '    url: __TEMPLATE__JDBC_URL__数据库地址__DEFAULT__jdbc:postgresql://postgresql:5432/postgres__',
    '    username: __TEMPLATE__JDBC_USER__数据库用户__DEFAULT__postgres__',
    '    password: __TEMPLATE__JDBC_PASSWORD__数据库密码__',
    '  data:',
    '    redis:',
    '      host: __TEMPLATE__REDIS_HOST__Redis地址__DEFAULT__redis-master__',
    '      port: __TEMPLATE__REDIS_PORT__Redis端口__DEFAULT__6379__',
    '',
  ].join('\n')
}

async function submitConfigTemplateImport() {
  configUploadError.value = ''
  let raw: any = {}
  const text = configTemplateImportText.value.trim()
  if (text) {
    if (configImportMode.value === 'advanced') {
      try {
        raw = normalizeAdvancedConfigTemplateImport(JSON.parse(text))
      } catch (e: any) {
        configUploadError.value = '高级模板 JSON 解析失败：' + (e?.message || 'JSON 格式错误')
        return
      }
    } else {
      raw = parseNativeConfigTemplate(text, {
        framework: configImportForm.value.framework,
      })
    }
  }
  const payload = configTemplatePayloadFromImport(raw)
  if (!payload.name) {
    configUploadError.value = '请填写模板名称，或在 JSON 中提供 name。'
    return
  }
  configUploading.value = true
  try {
    await api.createComponentConfigTemplate(payload)
    showConfigTemplateImportModal.value = false
    resetConfigTemplateImportForm()
    await loadConfigTemplates()
  } catch (e: any) {
    configUploadError.value = '导入配置模板失败：' + (e?.message || '未知错误')
  } finally {
    configUploading.value = false
  }
}

function resetConfigTemplateImportForm() {
  configImportForm.value = {
    key: '',
    name: '',
    description: '',
    framework: 'auto',
    bindingMode: 'recommended',
    componentTypes: 'backend',
  }
  configImportMode.value = 'native'
  configTemplateImportText.value = ''
}

function selectedConfigImportComponentTypes() {
  const value = String(configImportForm.value.componentTypes || 'all')
  if (value === 'all') return []
  if (value === 'frontend-backend') return ['frontend', 'backend']
  return [value].filter(Boolean)
}

function normalizeAdvancedConfigTemplateImport(input: any) {
  const root = input?.data && typeof input.data === 'object' ? input.data : input
  const template = root?.template && typeof root.template === 'object' ? root.template : root
  const schema = root?.schema && typeof root.schema === 'object' ? root.schema : {}
  const fields = Array.isArray(template?.fields)
    ? template.fields
    : Array.isArray(schema?.fields)
      ? schema.fields
      : []
  return {
    ...template,
    fields,
  }
}

function configTemplatePayloadFromImport(raw: any) {
  const componentTypes = selectedConfigImportComponentTypes()
  const rawComponentTypes = Array.isArray(raw?.componentTypes) ? raw.componentTypes.map((item: any) => String(item).trim()).filter(Boolean) : []
  return {
    key: configImportForm.value.key || String(raw?.key || '').trim(),
    name: configImportForm.value.name || String(raw?.name || '').trim(),
    description: configImportForm.value.description || String(raw?.description || '').trim(),
    framework: configImportForm.value.framework || String(raw?.framework || 'auto'),
    bindingMode: configImportForm.value.bindingMode || String(raw?.bindingMode || 'recommended'),
    componentTypes: componentTypes.length ? componentTypes : rawComponentTypes,
    syntax: String(raw?.syntax || nativeConfigTemplateSyntax),
    nativeConfigs: normalizeConfigTemplateNativeConfigs(raw?.nativeConfigs),
    fields: Array.isArray(raw?.fields) ? raw.fields : [],
    env: Array.isArray(raw?.env) ? raw.env.map(normalizeConfigTemplateEnv).filter((item: any) => item.name) : [],
    configMaps: normalizeConfigTemplateObjects(raw?.configMaps),
    secrets: normalizeConfigTemplateObjects(raw?.secrets),
    files: normalizeConfigTemplateFiles(raw?.files),
    command: Array.isArray(raw?.command) ? raw.command.map((item: any) => String(item).trim()).filter(Boolean) : [],
    args: Array.isArray(raw?.args) ? raw.args.map((item: any) => String(item).trim()).filter(Boolean) : [],
    enabled: raw?.enabled !== false,
  }
}

async function deleteConfigTemplate(id: number | string) {
  const tmpl = configTemplates.value.find((item) => String(item.id) === String(id))
  if (!tmpl || tmpl.isBuiltin) return
  pendingConfigTemplateDelete.value = tmpl
}

async function confirmDeleteConfigTemplate() {
  const tmpl = pendingConfigTemplateDelete.value
  if (!tmpl || tmpl.isBuiltin) return
  pageError.value = ''
  try {
    await api.deleteComponentConfigTemplate(tmpl.id)
    configTemplates.value = configTemplates.value.filter((item) => String(item.id) !== String(tmpl.id))
    pendingConfigTemplateDelete.value = null
  } catch (e: any) {
    pageError.value = '删除配置模板失败：' + (e?.message || '未知错误')
  }
}

async function syncBuiltinTemplates() {
  syncing.value = true
  pageError.value = ''
  try {
    await api.syncBuiltinServiceTemplates()
    await loadTemplates()
  } catch (e: any) {
    pageError.value = '同步失败：' + (e?.message || '未知错误')
  } finally {
    syncing.value = false
  }
}

async function syncBuiltinConfigTemplates() {
  syncing.value = true
  pageError.value = ''
  try {
    await api.syncBuiltinComponentConfigTemplates()
    await loadConfigTemplates()
  } catch (e: any) {
    pageError.value = '同步配置模板失败：' + (e?.message || '未知错误')
  } finally {
    syncing.value = false
  }
}

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    tool: '工具',
    infra: '基础设施',
    middleware: '中间件',
  }
  return labels[category] || category || '未分类'
}

function isMiddlewareTemplate(tmpl: any) {
  return String(tmpl?.category || '') === 'middleware' || middlewareTemplateTypes.has(String(tmpl?.type || ''))
}

function configTemplateFrameworkLabel(value: string) {
  const labels: Record<string, string> = {
    auto: '自动',
    spring: 'Spring Boot',
    springboot: 'Spring Boot',
    node: 'Node.js',
    python: 'Python',
    go: 'Go',
    nginx: 'Nginx',
    custom: '自定义',
  }
  return labels[value] || value || '自动'
}

function configTemplateBindingLabel(value: string) {
  const labels: Record<string, string> = {
    recommended: '推荐方式',
    env: '环境变量 + 敏感配置',
    spring: 'Spring 配置文件',
  }
  return labels[value] || value || '推荐方式'
}

function configTemplateComponentTypeSummary(items: any[]) {
  if (!Array.isArray(items) || !items.length) return '适用所有组件'
  return `适用 ${items.join(' / ')}`
}

function configTemplateNativeBlockCount(tmpl: any) {
  const count = configTemplateNativePreviewBlocks(tmpl).length
  return count ? `${count} 个原生配置片段` : '运行变量模板'
}

function configTemplateEditableFieldCount(items: any[]) {
  const count = Array.isArray(items) ? items.length : 0
  return count ? `${count} 个可填写项` : '无需填写'
}

function configTemplatePreviewSummary(tmpl: any) {
  return [
    { label: '适用组件', value: configTemplateComponentTypeSummary(tmpl?.componentTypes) },
    { label: '可填写项', value: configTemplateEditableFieldCount(tmpl?.fields) },
    { label: '敏感配置', value: configTemplateSecretConfigCount(tmpl) },
    { label: '生成文件', value: configTemplateGeneratedFileCount(tmpl) },
  ]
}

function configTemplateSecretConfigCount(tmpl: any) {
  const keys = new Set<string>()
  for (const item of tmpl?.secrets || []) {
    const objectName = String(item?.name || 'secret')
    for (const key of Object.keys(item?.data || {})) {
      keys.add(`${objectName}:${key}`)
    }
  }
  for (const item of tmpl?.env || []) {
    const source = String(item?.source || '')
    const refName = String(item?.refName || '')
    const refKey = String(item?.refKey || item?.name || '')
    if (source === 'secret' || refName || refKey.toLowerCase().includes('password') || refKey.toLowerCase().includes('secret')) {
      keys.add(`${refName || 'env'}:${refKey}`)
    }
  }
  for (const field of tmpl?.fields || []) {
    const key = String(field?.key || '')
    const target = String(field?.target || field?.output || field?.type || '')
    if (key && /secret|password|token|credential/i.test(`${key} ${target}`)) {
      keys.add(`field:${key}`)
    }
  }
  return keys.size ? `${keys.size} 项敏感配置` : '无敏感配置'
}

function configTemplateGeneratedFileCount(tmpl: any) {
  const files = new Set<string>()
  for (const item of tmpl?.nativeConfigs || []) {
    const name = String(item?.name || '').trim()
    if (name) files.add(name)
  }
  for (const item of tmpl?.files || []) {
    const key = String(item?.key || item?.name || '').trim()
    if (key) files.add(key)
  }
  for (const item of tmpl?.configMaps || []) {
    for (const key of Object.keys(item?.data || {})) {
      files.add(key)
    }
  }
  return files.size ? `${files.size} 个生成文件` : '无文件输出'
}

function configTemplateNativePreviewBlocks(tmpl: any) {
  const blocks: Array<{ name: string; content: string }> = []
  for (const item of tmpl?.nativeConfigs || []) {
    const name = String(item?.name || '').trim()
    const content = String(item?.content || '')
    if (name && content) blocks.push({ name, content })
  }
  if (blocks.length) return blocks
  for (const item of tmpl?.configMaps || []) {
    for (const [key, value] of Object.entries(item?.data || {})) {
      blocks.push({ name: String(key || 'config'), content: configTemplateReadableNativeContent(String(value ?? ''), tmpl) })
    }
  }
  if (!blocks.length && Array.isArray(tmpl?.env) && tmpl.env.length) {
    blocks.push({
      name: '.env',
      content: tmpl.env.map((item:any) => `${item.name}=${item.value || (item.refKey ? `[[${item.refKey}]]` : '')}`).join('\n'),
    })
  }
  return blocks
}

function configTemplateReadableNativeContent(value: string, tmpl: any) {
  const fieldLabels = new Map<string, string>()
  for (const field of tmpl?.fields || []) {
    if (field?.key) fieldLabels.set(String(field.key), String(field.label || field.key))
    for (const itemField of field?.itemFields || []) {
      if (itemField?.key) fieldLabels.set(String(itemField.key), String(itemField.label || itemField.key))
    }
  }
  return String(value || '')
    .replace(/\[\[paap:for\s+([^\]\s]+)\]\]/g, (_match, key) => `__TEMPLATE__FOR__${key}__${fieldLabels.get(String(key)) || key}__`)
    .replace(/\[\[paap:if\s+([^\]\s]+)\]\]/g, (_match, key) => `__TEMPLATE__IF__${key}__${fieldLabels.get(String(key)) || key}__`)
    .replace(/\[\[paap:end\s+([^\]\s]+)\]\]/g, (_match, key) => `__TEMPLATE__END__${key}__`)
    .replace(/\[\[paap:item\.([^\]\s]+)([^\]]*)\]\]/g, (_match, key, options) => {
      const defaultValue = configTemplateDefaultFromTokenOptions(String(options || ''))
      return `__TEMPLATE__ITEM_${key}__${fieldLabels.get(String(key)) || key}__${defaultValue ? `DEFAULT__${defaultValue}__` : ''}`
    })
    .replace(/\[\[paap:([^\]\s]+)([^\]]*)\]\]/g, (_match, key, options) => {
      const defaultValue = configTemplateDefaultFromTokenOptions(String(options || ''))
      return `__TEMPLATE__${key}__${fieldLabels.get(String(key)) || key}__${defaultValue ? `DEFAULT__${defaultValue}__` : ''}`
    })
}

function configTemplateDefaultFromTokenOptions(options: string) {
  const match = options.match(/\bdefault=("[^"]*"|'[^']*'|[^\s\]]+)/)
  return match ? match[1].replace(/^['"]|['"]$/g, '') : ''
}

function configTemplateSchemaJSON(tmpl: any) {
  return JSON.stringify({
    fields: tmpl?.fields || [],
    files: tmpl?.files || [],
    env: tmpl?.env || [],
  }, null, 2)
}

function configTemplateRawJSON(tmpl: any) {
  if (!tmpl) return ''
  return JSON.stringify({
    key: tmpl.key,
    name: tmpl.name,
    description: tmpl.description,
    framework: tmpl.framework,
    bindingMode: tmpl.bindingMode,
    componentTypes: tmpl.componentTypes,
    syntax: tmpl.syntax,
    nativeConfigs: tmpl.nativeConfigs,
    fields: tmpl.fields,
    env: tmpl.env,
    configMaps: tmpl.configMaps,
    secrets: tmpl.secrets,
    files: tmpl.files,
    command: tmpl.command,
    args: tmpl.args,
  }, null, 2)
}

function configTemplateDisplayText(value: string) {
  return String(value || '')
    .replace(/ConfigMap/g, '普通配置')
    .replace(/Secret keys/gi, '敏感配置项')
    .replace(/Secret/g, '敏感配置')
    .replace(/Kubernetes/g, '平台底层')
}

function permissionSummary(tmpl: any) {
  try {
    const manifest = JSON.parse(tmpl.platformManifestJSON || '{}')
    const scope = manifest.permissions?.workloadNamespaces?.scope
    return scope === 'environment-wide' ? '环境命名空间权限' : '仅工具命名空间'
  } catch {
    return '未解析权限'
  }
}

function isHeavyTemplate(tmpl: any) {
  return tmpl.type === 'registry' || tmpl.type === 'harbor'
}
</script>

<style scoped>
.rail-page {
  padding: 20px 20px 36px;
  max-width: none;
}

/* ===== Page header ===== */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 16px;
}
.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--cds-text-primary, #161616);
  letter-spacing: 0;
  line-height: 1.2;
}
.page-desc {
  font-size: 14px;
  color: var(--cds-text-secondary, #525252);
  line-height: 1.4;
}
.page-error {
  border: 1px solid var(--cds-red-60, #da1e28);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-red-60, #da1e28);
  border-radius: 0;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
  margin-bottom: 16px;
}
.header-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.template-tabs {
  display: flex;
  gap: 0;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.template-tabs button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
  padding: 0 12px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--cds-text-secondary, #525252);
  font-size: 13px;
  cursor: pointer;
}
.template-tabs button.active {
  border-bottom-color: var(--cds-blue-60, #0f62fe);
  color: var(--cds-text-primary, #161616);
}
.template-tabs strong {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: var(--cds-border-radius-md, 2px);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
  font-size: 11px;
  line-height: 20px;
  text-align: center;
}

/* ===== KPI ===== */
.kpi-section {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
  margin-bottom: 32px;
}
.kpi-card {
  background: var(--cds-layer-01, #ffffff);
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kpi-number {
  font-size: 28px;
  font-weight: 600;
  color: var(--cds-text-primary, #161616);
  letter-spacing: 0;
  line-height: 1.2;
}
.kpi-label {
  font-size: 12px;
  color: var(--cds-text-secondary, #525252);
}

/* ===== Section ===== */
.section-card {
  background: var(--cds-layer-01, #ffffff);
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  padding: 24px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

/* ===== Loading ===== */
.loading-mask {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 64px;
}
.loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--cds-border-subtle-01, #e0e0e0);
  border-top-color: var(--cds-text-primary, #161616);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* ===== Template list ===== */
.template-list {
  border-top: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.template-row {
  display: flex;
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  transition: background-color 110ms;
  overflow: hidden;
  cursor: default;
}
.template-row:last-child {
  border-bottom: none;
}
.template-row:hover {
  background: var(--cds-background-hover, rgba(141, 141, 141, 0.12));
}
.template-body {
  padding: 16px 20px;
  flex: 1;
}
.template-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 6px;
}
.template-name-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.template-name {
  font-weight: 600;
  font-size: 15px;
  line-height: 1.4;
  color: var(--cds-text-primary, #161616);
}
.template-meta {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--cds-text-secondary, #525252);
  margin-bottom: 6px;
  font-size: 13px;
}
.template-meta .uploaded {
  color: var(--cds-green-60, #198038);
}
.template-desc {
  color: var(--cds-text-secondary, #525252);
  line-height: 1.4;
  font-size: 13px;
}
.config-template-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}
.config-template-details div {
  display: grid;
  gap: 4px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  background: var(--cds-layer-01, #ffffff);
}
.config-template-details span {
  color: var(--cds-text-secondary, #525252);
  font-size: 11px;
}
.config-template-details code {
  min-width: 0;
  color: var(--cds-text-primary, #161616);
  font-family: var(--cds-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  font-size: 12px;
  white-space: normal;
  overflow-wrap: anywhere;
}
.template-row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
  flex-wrap: wrap;
}

/* ===== Tags ===== */
.tag {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0.2px;
  border-radius: var(--cds-border-radius-md, 2px);
}
.tag.builtin {
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
}
.tag.custom {
  background: var(--cds-blue-10, #edf5ff);
  color: var(--cds-blue-70, #0043ce);
}
.tag.heavy {
  background: var(--cds-red-10, #fff1f1);
  color: var(--cds-red-60, #da1e28);
}
.tag.config {
  background: var(--cds-green-10, #defbe6);
  color: var(--cds-green-60, #198038);
}

/* Meta info: uploaded state */
.template-meta .uploaded {
  color: var(--cds-green-60, #198038);
}

.policy {
  color: var(--cds-text-placeholder, rgba(22, 22, 22, 0.4));
  white-space: nowrap;
  font-size: 12px;
  letter-spacing: 0.2px;
}
.rail-btn.danger {
  color: var(--cds-red-60, #da1e28);
}
.text-btn {
  border: 0;
  background: transparent;
  color: var(--cds-blue-60, #0f62fe);
  cursor: pointer;
  padding: 0;
  font-size: 12px;
}
.text-btn:hover {
  text-decoration: underline;
}

/* ===== Responsive ===== */
@media (max-width: 672px) {
  .rail-page {
    padding: 20px 20px 32px;
  }
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .kpi-section {
    grid-template-columns: 1fr 1fr;
  }
  .template-tabs {
    overflow-x: auto;
  }
  .template-header {
    flex-direction: column;
    align-items: flex-start;
  }
  .config-template-details {
    grid-template-columns: 1fr;
  }
}
</style>

<style>
/* ===== Modal styles ===== */
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9000;
  background: rgba(22, 22, 22, 0.48);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.modal-container {
  background: var(--cds-layer-01, #ffffff);
  width: 480px;
  max-height: 90vh;
  overflow-y: auto;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  box-shadow: none;
}
.modal-container--wide {
  width: min(760px, 96vw);
}
.modal-container--template {
  width: min(980px, 96vw);
}
.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px 24px;
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.modal-label {
  font-size: 11px;
  color: var(--cds-text-secondary, #525252);
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}
.modal-heading {
  font-size: 18px;
  font-weight: 600;
  color: var(--cds-text-primary, #161616);
  margin: 0;
  line-height: 1.3;
}
.modal-close {
  background: none;
  border: none;
  color: var(--cds-text-secondary, #525252);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0;
  transition: background 110ms, color 110ms;
}
.modal-close:hover {
  background: var(--cds-background-hover, rgba(141, 141, 141, 0.12));
  color: var(--cds-text-primary, #161616);
}
.modal-body {
  padding: 24px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px;
  border-top: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}

/* ===== Form elements ===== */
.form-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 20px;
}
.form-item:last-child {
  margin-bottom: 0;
}
.form-label {
  font-size: 12px;
  color: var(--cds-text-secondary, #525252);
  font-weight: 500;
}
.form-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.form-helper {
  font-size: 12px;
  color: var(--cds-text-helper, #6f6f6f);
  margin-top: 2px;
  line-height: 1.4;
}
.form-error {
  border: 1px solid var(--cds-red-60, #da1e28);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-red-60, #da1e28);
  border-radius: 0;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
}

.rail-input {
  width: 100%;
  padding: 9px 12px;
  font-size: 14px;
  border: 1px solid var(--cds-border-strong-01, #8d8d8d);
  border-radius: 0;
  background: var(--cds-field-01, #f4f4f4);
  color: var(--cds-text-primary, #161616);
  outline: none;
  font-family: inherit;
  transition: border-color 110ms, box-shadow 110ms;
}
.rail-input:focus {
  border-color: var(--cds-border-interactive, #0f62fe);
  box-shadow: inset 0 0 0 1px var(--cds-border-interactive, #0f62fe);
}
.rail-input::placeholder {
  color: var(--cds-text-placeholder, rgba(22, 22, 22, 0.4));
}

.rail-select {
  width: 100%;
  padding: 9px 36px 9px 12px;
  font-size: 14px;
  border: 1px solid var(--cds-border-strong-01, #8d8d8d);
  border-radius: 0;
  background: var(--cds-field-01, #f4f4f4);
  color: var(--cds-text-primary, #161616);
  outline: none;
  appearance: none;
  cursor: pointer;
  font-family: inherit;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='M6 8L1 3h10z' fill='%23525252'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  transition: border-color 110ms, box-shadow 110ms;
}
.rail-select:focus {
  border-color: var(--cds-border-interactive, #0f62fe);
  box-shadow: inset 0 0 0 1px var(--cds-border-interactive, #0f62fe);
}

.rail-textarea {
  width: 100%;
  resize: vertical;
  padding: 10px 12px;
  font-size: 14px;
  border: 1px solid var(--cds-border-strong-01, #8d8d8d);
  border-radius: 0;
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-primary, #161616);
  outline: none;
  font-family: inherit;
  line-height: 1.5;
  transition: border-color 110ms, box-shadow 110ms;
}
.rail-input,
.rail-select {
  background: var(--cds-layer-01, #ffffff);
}
.config-import-shell--carbon {
  background: var(--cds-layer-01, #ffffff);
}
.template-mode-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-01, #ffffff);
}
.template-mode-switch button {
  min-height: 72px;
  border: 0;
  border-right: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  text-align: left;
  padding: 12px 14px;
  transition: background-color 110ms, box-shadow 110ms, color 110ms;
}
.template-mode-switch button:last-child {
  border-right: 0;
}
.template-mode-switch button:hover {
  background: var(--cds-background-hover, rgba(141, 141, 141, 0.12));
  color: var(--cds-text-primary, #161616);
}
.template-mode-switch button.active {
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-primary, #161616);
  box-shadow: inset 0 -3px 0 var(--cds-border-interactive, #0f62fe);
}
.config-import-mode-card {
  display: grid;
  align-content: center;
  gap: 4px;
}
.config-import-mode-card span {
  font-size: var(--cds-heading-01-font-size, 14px);
  font-weight: var(--cds-heading-01-font-weight, 600);
  line-height: var(--cds-heading-01-line-height, 1.42857);
  letter-spacing: var(--cds-heading-01-letter-spacing, 0.16px);
}
.config-import-mode-card small {
  color: var(--cds-text-helper, #6f6f6f);
  font-size: var(--cds-helper-text-01-font-size, 12px);
  line-height: var(--cds-helper-text-01-line-height, 1.33333);
  letter-spacing: var(--cds-helper-text-01-letter-spacing, 0.32px);
}
.config-import-mode {
  grid-column: 1 / -1;
}
.rail-textarea:focus {
  border-color: var(--cds-border-interactive, #0f62fe);
  box-shadow: inset 0 0 0 1px var(--cds-border-interactive, #0f62fe);
}
.code-textarea {
  font-family: var(--cds-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  font-size: 12px;
  line-height: 1.45;
}
.config-import-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.template-preview {
  display: grid;
  gap: 16px;
}
.template-preview-desc {
  margin: 0;
  color: var(--cds-text-secondary, #525252);
  line-height: 1.5;
  font-size: 13px;
}
.template-preview-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.template-preview-meta span,
.preview-chip {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: var(--cds-border-radius-md, 2px);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
  font-size: 12px;
}
.template-preview-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-01, #ffffff);
}
.template-preview-summary-item {
  display: grid;
  gap: 4px;
  min-height: 64px;
  padding: 10px 12px;
  border-right: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  align-content: center;
  min-width: 0;
}
.template-preview-summary-item:last-child {
  border-right: 0;
}
.template-preview-summary-item span {
  color: var(--cds-text-secondary, #525252);
  font-size: var(--cds-label-01-font-size, 12px);
  line-height: var(--cds-label-01-line-height, 1.33333);
  letter-spacing: var(--cds-label-01-letter-spacing, 0.32px);
}
.template-preview-summary-item strong {
  color: var(--cds-text-primary, #161616);
  font-size: var(--cds-body-01-font-size, 14px);
  font-weight: var(--cds-heading-01-font-weight, 600);
  line-height: var(--cds-body-01-line-height, 1.42857);
  letter-spacing: var(--cds-body-01-letter-spacing, 0.16px);
  overflow-wrap: anywhere;
}
.template-preview-section {
  display: grid;
  gap: 12px;
}
.template-preview-tabs {
  display: flex;
  align-items: flex-end;
  gap: 0;
  min-height: 44px;
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-01, #ffffff);
}
.template-preview-tabs button {
  position: relative;
  min-height: 44px;
  padding: 0 18px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
  font-family: inherit;
  font-size: var(--cds-body-01-font-size, 14px);
  font-weight: var(--cds-body-01-font-weight, 400);
  line-height: 44px;
  cursor: pointer;
}
.template-preview-tabs button:hover {
  color: var(--cds-text-primary, #161616);
  background: var(--cds-field-01, #f4f4f4);
}
.template-preview-tabs button.active {
  border-bottom-color: var(--cds-blue-60, #0f62fe);
  color: var(--cds-text-primary, #161616);
  background: var(--cds-layer-01, #ffffff);
  font-weight: var(--cds-heading-01-font-weight, 600);
}
.template-preview-section h3 {
  margin: 0;
  font-size: 14px;
  line-height: 1.3;
  color: var(--cds-text-primary, #161616);
}
.preview-table {
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  overflow: hidden;
}
.preview-row {
  display: grid;
  grid-template-columns: 1.4fr 0.8fr 1.2fr 1.2fr;
  gap: 10px;
  align-items: start;
  padding: 10px 12px;
  border-top: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  font-size: 12px;
  color: var(--cds-text-primary, #161616);
}
.preview-row:first-child {
  border-top: 0;
}
.preview-row--head {
  background: var(--cds-field-01, #f4f4f4);
  color: var(--cds-text-secondary, #525252);
  font-weight: 600;
}
.preview-row small {
  display: block;
  margin-top: 2px;
  color: var(--cds-text-helper, #6f6f6f);
  overflow-wrap: anywhere;
}
.preview-block-list {
  display: grid;
  gap: 12px;
}
.preview-block {
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  overflow: hidden;
  background: var(--cds-layer-01, #ffffff);
}
.preview-block-title {
  display: flex;
  align-items: center;
  min-height: 40px;
  padding: 0 14px;
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-primary, #161616);
  font-size: var(--cds-heading-01-font-size, 14px);
  font-weight: var(--cds-heading-01-font-weight, 600);
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.template-preview pre,
.preview-block pre {
  margin: 0;
  padding: 14px 16px;
  background: var(--cds-field-01, #f4f4f4);
  color: var(--cds-text-primary, #161616);
  overflow: auto;
  max-height: 360px;
  font-size: 12px;
  line-height: 1.55;
  tab-size: 2;
}
.preview-block pre + pre {
  border-top: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.template-preview code,
.preview-block code {
  font-family: var(--cds-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
}
.preview-chip-list {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.preview-empty {
  padding: 10px 12px;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  border-radius: 0;
  color: var(--cds-text-helper, #6f6f6f);
  font-size: 12px;
}

@media (max-width: 672px) {
  .config-import-grid,
  .template-preview-summary,
  .preview-row {
    grid-template-columns: 1fr;
  }
  .template-preview-summary-item {
    border-right: 0;
    border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  }
  .template-preview-summary-item:last-child {
    border-bottom: 0;
  }
}
</style>
