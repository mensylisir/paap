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
            <div class="config-template-details">
              <div>
                <span>用户填写</span>
                <code>{{ configTemplateFieldSummary(tmpl.fields) }}</code>
              </div>
              <div>
                <span>环境变量</span>
                <code>{{ configTemplateEnvSummary(tmpl) }}</code>
              </div>
              <div>
                <span>普通配置模板</span>
                <code>{{ configTemplateObjectSummary(tmpl.configMaps) }}</code>
              </div>
              <div>
                <span>敏感配置项</span>
                <code>{{ configTemplateSecretSummary(tmpl.secrets) }}</code>
              </div>
              <div>
                <span>配置文件</span>
                <code>{{ configTemplateFileSummary(tmpl.files) }}</code>
              </div>
              <div v-if="(tmpl.command || []).length">
                <span>启动命令</span>
                <code>{{ tmpl.command.join(' ') }}</code>
              </div>
              <div v-if="(tmpl.args || []).length">
                <span>启动参数</span>
                <code>{{ tmpl.args.join(' ') }}</code>
              </div>
              <div>
                <span>占位规则</span>
                <code>{{ configTemplateSyntaxSummary(tmpl.syntax) }}</code>
              </div>
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
              <p class="modal-heading">上传或粘贴组件运行配置模板 JSON</p>
            </div>
            <button class="modal-close" @click="showConfigTemplateImportModal = false">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="config-import-grid">
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
                <input v-model.trim="configImportForm.componentTypes" class="rail-input" placeholder="backend, frontend" />
              </div>
            </div>
            <div class="form-item">
              <label class="form-label">描述</label>
              <textarea v-model.trim="configImportForm.description" class="rail-textarea" rows="2" placeholder="说明模板会生成哪些环境变量、配置文件和挂载"></textarea>
            </div>
            <div class="form-item">
              <label class="form-label">模板 JSON 文件</label>
              <input class="rail-input" type="file" accept=".json,application/json" @change="onConfigTemplateFileChange" />
              <div class="form-helper">模板只需要描述用户可填写字段、环境变量、普通配置内容、敏感配置项、文件 key 和应用内路径；底层配置对象由平台自动生成。</div>
            </div>
            <div class="form-item">
              <div class="form-label-row">
                <label class="form-label">模板 JSON</label>
                <button type="button" class="text-btn" @click="fillConfigTemplateSample">填入示例</button>
              </div>
              <textarea v-model.trim="configTemplateImportText" class="rail-textarea code-textarea" rows="14" spellcheck="false" placeholder='{"fields":[],"env":[],"files":[]}'></textarea>
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

            <section class="template-preview-section">
              <h3>用户填写字段</h3>
              <div v-if="selectedConfigTemplate.fields.length" class="preview-table">
                <div class="preview-row preview-row--head">
                  <span>字段</span><span>类型</span><span>默认值</span><span>用途</span>
                </div>
                <div v-for="field in selectedConfigTemplate.fields" :key="field.key || field.label" class="preview-row">
                  <span><strong>{{ configTemplateDisplayText(field.label || field.key) }}</strong><small>{{ field.key }}</small></span>
                  <span>{{ field.type || 'text' }}</span>
                  <span>{{ field.default || '-' }}</span>
                  <span>{{ configTemplateFieldUsage(field) }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">这个模板不需要用户填写字段。</div>
            </section>

            <section class="template-preview-section">
              <h3>环境变量</h3>
              <div v-if="selectedConfigTemplate.env.length" class="preview-table">
                <div class="preview-row preview-row--head">
                  <span>变量名</span><span>来源</span><span>值 / Key</span><span>管理方式</span>
                </div>
                <div v-for="item in selectedConfigTemplate.env" :key="`${item.name}:${item.refKey}:${item.source}`" class="preview-row">
                  <span><strong>{{ item.name }}</strong></span>
                  <span>{{ configTemplateEnvSourceLabel(item.source) }}</span>
                  <span>{{ configTemplateEnvValueLabel(item) }}</span>
                  <span>{{ configTemplateGeneratedObjectLabel(item.source) }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">不会生成环境变量。</div>
            </section>

            <section class="template-preview-section">
              <h3>普通配置模板</h3>
              <div v-if="selectedConfigTemplate.configMaps.length" class="preview-block-list">
                <div v-for="(item, idx) in selectedConfigTemplate.configMaps" :key="`${item.name || 'config'}:${idx}`" class="preview-block">
                  <div class="preview-block-title">{{ configTemplatePlatformObjectName(item.name, '普通配置') }}</div>
                  <pre v-for="(value, key) in item.data" :key="key"><code>{{ key }}:
{{ value }}</code></pre>
                </div>
              </div>
              <div v-else class="preview-empty">不会生成普通配置。</div>
            </section>

            <section class="template-preview-section">
              <h3>敏感配置项</h3>
              <div v-if="selectedConfigTemplate.secrets.length" class="preview-chip-list">
                <span v-for="key in configTemplateSecretKeys(selectedConfigTemplate.secrets)" :key="key" class="preview-chip">{{ key }}</span>
              </div>
              <div v-else class="preview-empty">不会生成敏感配置。</div>
            </section>

            <section class="template-preview-section">
              <h3>配置文件</h3>
              <div v-if="selectedConfigTemplate.files.length" class="preview-table">
                <div class="preview-row preview-row--head">
                  <span>应用内路径</span><span>配置文件 Key</span><span>来源</span><span>权限</span>
                </div>
                <div v-for="item in selectedConfigTemplate.files" :key="`${item.mountPath}:${item.key}`" class="preview-row">
                  <span><strong>{{ item.mountPath }}</strong></span>
                  <span>{{ item.key }}</span>
                  <span>平台自动生成</span>
                  <span>{{ item.readOnly === false ? '可写' : '只读' }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">不会挂载配置文件。</div>
            </section>

            <section v-if="selectedConfigTemplate.command.length || selectedConfigTemplate.args.length" class="template-preview-section">
              <h3>启动命令</h3>
              <pre v-if="selectedConfigTemplate.command.length"><code>command:
{{ selectedConfigTemplate.command.join('\n') }}</code></pre>
              <pre v-if="selectedConfigTemplate.args.length"><code>args:
{{ selectedConfigTemplate.args.join('\n') }}</code></pre>
            </section>

            <details class="template-preview-section template-preview-advanced">
              <summary>模板源码 JSON（面向模板作者）</summary>
              <pre><code>{{ configTemplateRawJSON(selectedConfigTemplate) }}</code></pre>
            </details>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '../api/client'

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
  componentTypes: 'backend, frontend',
})
const configTemplateImportText = ref('')

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

function normalizeConfigTemplateFiles(items: any) {
  if (!Array.isArray(items)) return []
  return items.map((item: any) => ({
    name: String(item?.name || '').trim(),
    configMapName: String(item?.configMapName || '').trim(),
    key: String(item?.key || '').trim(),
    mountPath: String(item?.mountPath || '').trim(),
    readOnly: item?.readOnly !== false,
  })).filter((item: any) => item.key && item.mountPath)
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
  if (!configImportForm.value.name.trim()) {
    configImportForm.value.name = '自定义 Spring Boot 配置'
    configImportForm.value.framework = 'springboot'
    configImportForm.value.componentTypes = 'backend'
  }
  configTemplateImportText.value = JSON.stringify({
    fields: [
      { key: 'database.jdbcUrl', label: '数据库 JDBC URL', type: 'serviceRef', target: 'postgresql|mysql', output: 'configMap', default: 'jdbc:postgresql://postgresql:5432/postgres', required: true },
      { key: 'database.username', label: '数据库用户名', type: 'text', output: 'configMap', default: 'postgres', required: true },
      { key: 'database.password', label: '数据库密码', type: 'password', output: 'secret', required: true },
      { key: 'redis.host', label: 'Redis 地址', type: 'serviceRef', target: 'redis', output: 'configMap', default: 'redis-master' },
    ],
    env: [
      { name: 'SPRING_CONFIG_ADDITIONAL_LOCATION', source: 'value', value: 'file:/etc/paap/' },
      { name: 'SPRING_DATASOURCE_PASSWORD', source: 'secret', refKey: 'SPRING_DATASOURCE_PASSWORD' },
    ],
    configMaps: [
      {
        data: {
          'application-paap.yml': 'spring:\n  datasource:\n    url: [[paap:database.jdbcUrl default=jdbc:postgresql://postgresql:5432/postgres]]\n    username: [[paap:database.username default=postgres]]\n    password: ${SPRING_DATASOURCE_PASSWORD}\n  data:\n    redis:\n      host: [[paap:redis.host default=redis-master]]\n      port: [[paap:redis.port default=6379]]\n',
        },
      },
    ],
    secrets: [
      { data: { SPRING_DATASOURCE_PASSWORD: '' } },
    ],
    files: [
      { key: 'application-paap.yml', mountPath: '/etc/paap/application-paap.yml', readOnly: true },
    ],
  }, null, 2)
}

async function submitConfigTemplateImport() {
  configUploadError.value = ''
  let raw: any = {}
  const text = configTemplateImportText.value.trim()
  if (text) {
    try {
      const parsed = JSON.parse(text)
      raw = parsed?.data && typeof parsed.data === 'object' ? parsed.data : parsed
    } catch (e: any) {
      configUploadError.value = '模板 JSON 解析失败：' + (e?.message || '未知错误')
      return
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
    componentTypes: 'backend, frontend',
  }
  configTemplateImportText.value = ''
}

function configTemplatePayloadFromImport(raw: any) {
  const componentTypes = String(configImportForm.value.componentTypes || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  const rawComponentTypes = Array.isArray(raw?.componentTypes) ? raw.componentTypes.map((item: any) => String(item).trim()).filter(Boolean) : []
  return {
    key: configImportForm.value.key || String(raw?.key || '').trim(),
    name: configImportForm.value.name || String(raw?.name || '').trim(),
    description: configImportForm.value.description || String(raw?.description || '').trim(),
    framework: configImportForm.value.framework || String(raw?.framework || 'auto'),
    bindingMode: configImportForm.value.bindingMode || String(raw?.bindingMode || 'recommended'),
    componentTypes: componentTypes.length ? componentTypes : rawComponentTypes,
    syntax: String(raw?.syntax || '使用 [[paap:<field>]] 占位符描述用户填写字段；底层配置对象由平台按组件自动生成。'),
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
  if (!window.confirm(`删除配置模板「${tmpl.name}」？`)) return
  pageError.value = ''
  try {
    await api.deleteComponentConfigTemplate(id)
    configTemplates.value = configTemplates.value.filter((item) => String(item.id) !== String(id))
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

function configTemplateEnvSummary(tmpl: any) {
  const env = Array.isArray(tmpl?.env) ? tmpl.env : []
  if (!env.length) return '-'
  return env.map((item: any) => {
    const name = item.name || '-'
    if (item.source === 'secret') return `${name}=敏感配置:${item.refKey || name}`
    if (item.source === 'configMap') return `${name}=普通配置:${item.refKey || name}`
    return `${name}=value`
  }).join(', ')
}

function configTemplateFieldSummary(items: any[]) {
  if (!Array.isArray(items) || !items.length) return '-'
  return items.map((item: any) => {
    const label = configTemplateDisplayText(item?.label || item?.key || '-')
    const target = item?.target ? ` → ${item.target}` : ''
    return `${label}${target}`
  }).join('；')
}

function configTemplateObjectSummary(items: any[]) {
  if (!Array.isArray(items) || !items.length) return '-'
  return items.map((item: any) => {
    const keys = Object.keys(item?.data || {})
    return `${configTemplatePlatformObjectName(item?.name, '普通配置')}${keys.length ? ` (${keys.join(', ')})` : ''}`
  }).join('；')
}

function configTemplateSecretSummary(items: any[]) {
  if (!Array.isArray(items) || !items.length) return '-'
  const keys = items.flatMap((item: any) => Object.keys(item?.data || {})).filter(Boolean)
  return keys.length ? keys.join(', ') : `${items.length} 组敏感配置`
}

function configTemplateFileSummary(items: any[]) {
  if (!Array.isArray(items) || !items.length) return '-'
  return items.map((item: any) => `${item.mountPath || '-'} ← ${item.key || item.name || 'config'}`).join('；')
}

function configTemplatePlatformObjectName(value: string, kind: string) {
  const raw = String(value || '').trim()
  if (!raw || /\{\{\s*(configMapName|secretName)\s*\}\}/.test(raw)) return `平台自动生成${kind}`
  return raw
}

function configTemplateFieldUsage(field: any) {
  const parts = []
  if (field?.required) parts.push('必填')
  if (field?.target) parts.push(`连接 ${field.target}`)
  if (field?.output === 'secret') parts.push('写入敏感配置')
  else if (field?.output === 'configMap') parts.push('写入普通配置')
  return parts.join(' / ') || '-'
}

function configTemplateEnvSourceLabel(source: string) {
  if (source === 'secret') return '敏感配置'
  if (source === 'configMap') return '普通配置'
  return '固定值'
}

function configTemplateEnvValueLabel(item: any) {
  if (item?.source === 'secret') return item.refKey || item.name || '-'
  if (item?.source === 'configMap') return item.refKey || item.name || '-'
  return item?.value || '-'
}

function configTemplateGeneratedObjectLabel(source: string) {
  if (source === 'secret') return '平台自动管理'
  if (source === 'configMap') return '平台自动管理'
  return '-'
}

function configTemplateSecretKeys(items: any[]) {
  if (!Array.isArray(items)) return []
  return Array.from(new Set(items.flatMap((item: any) => Object.keys(item?.data || {})).filter(Boolean)))
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
    fields: tmpl.fields,
    env: tmpl.env,
    configMaps: tmpl.configMaps,
    secrets: tmpl.secrets,
    files: tmpl.files,
    command: tmpl.command,
    args: tmpl.args,
  }, null, 2)
}

function configTemplateSyntaxSummary(_value: string) {
  return '使用 [[paap:字段名]] 标记需要用户填写的位置，可设置默认值；底层配置对象由平台自动生成。'
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
  color: #11181c;
  letter-spacing: 0;
  line-height: 1.2;
}
.page-desc {
  font-size: 14px;
  color: #687076;
  line-height: 1.4;
}
.page-error {
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #991b1b;
  border-radius: 6px;
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
  gap: 8px;
  margin-bottom: 16px;
  border-bottom: 1px solid #e6e8eb;
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
  color: #687076;
  font-size: 13px;
  cursor: pointer;
}
.template-tabs button.active {
  border-bottom-color: #11181c;
  color: #11181c;
}
.template-tabs strong {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 999px;
  background: #f1f3f5;
  color: #687076;
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
  background: #ffffff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kpi-number {
  font-size: 28px;
  font-weight: 600;
  color: #11181c;
  letter-spacing: 0;
  line-height: 1.2;
}
.kpi-label {
  font-size: 12px;
  color: #687076;
}

/* ===== Section ===== */
.section-card {
  background: #ffffff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
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
  border: 2px solid #e6e8eb;
  border-top-color: #11181c;
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
  border-top: 1px solid #f1f3f5;
}
.template-row {
  display: flex;
  border-bottom: 1px solid #f1f3f5;
  transition: background-color 0.15s;
  overflow: hidden;
  cursor: default;
}
.template-row:last-child {
  border-bottom: none;
}
.template-row:hover {
  background: #f9fafb;
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
  color: #11181c;
}
.template-meta {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: #687076;
  margin-bottom: 6px;
  font-size: 13px;
}
.template-meta .uploaded {
  color: #22c55e;
}
.template-desc {
  color: #687076;
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
  border: 1px solid #f1f3f5;
  border-radius: 6px;
  background: #fbfcfd;
}
.config-template-details span {
  color: #687076;
  font-size: 11px;
}
.config-template-details code {
  min-width: 0;
  color: #11181c;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
  border-radius: 4px;
}
.tag.builtin {
  background: #f1f3f5;
  color: #687076;
}
.tag.custom {
  background: #eef4ff;
  color: #5a7fc2;
}
.tag.heavy {
  background: #fdf2f2;
  color: #c07373;
}
.tag.config {
  background: #ecfdf5;
  color: #047857;
}

/* Meta info: uploaded state */
.template-meta .uploaded {
  color: #7aa87a;
}

.policy {
  color: #a8afb5;
  white-space: nowrap;
  font-size: 12px;
  letter-spacing: 0.2px;
}
.rail-btn.danger {
  color: #b42318;
}
.text-btn {
  border: 0;
  background: transparent;
  color: #11181c;
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
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
.modal-container {
  background: #ffffff;
  width: 480px;
  max-height: 90vh;
  overflow-y: auto;
  border-radius: 8px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.15);
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
  border-bottom: 1px solid #f1f3f5;
}
.modal-label {
  font-size: 11px;
  color: #9ba1a6;
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}
.modal-heading {
  font-size: 18px;
  font-weight: 600;
  color: #11181c;
  margin: 0;
  line-height: 1.3;
}
.modal-close {
  background: none;
  border: none;
  color: #9ba1a6;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
}
.modal-close:hover {
  background: #f1f3f5;
  color: #11181c;
}
.modal-body {
  padding: 24px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px;
  border-top: 1px solid #f1f3f5;
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
  color: #687076;
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
  color: #9ba1a6;
  margin-top: 2px;
  line-height: 1.4;
}
.form-error {
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #991b1b;
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
}

.rail-input {
  width: 100%;
  padding: 9px 12px;
  font-size: 14px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #ffffff;
  color: #11181c;
  outline: none;
  font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.rail-input:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}
.rail-input::placeholder {
  color: #9ba1a6;
}

.rail-select {
  width: 100%;
  padding: 9px 36px 9px 12px;
  font-size: 14px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #ffffff;
  color: #11181c;
  outline: none;
  appearance: none;
  cursor: pointer;
  font-family: inherit;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='M6 8L1 3h10z' fill='%23687076'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.rail-select:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.rail-textarea {
  width: 100%;
  resize: vertical;
  padding: 10px 12px;
  font-size: 14px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #ffffff;
  color: #11181c;
  outline: none;
  font-family: inherit;
  line-height: 1.5;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.rail-textarea:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}
.code-textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
  gap: 18px;
}
.template-preview-desc {
  margin: 0;
  color: #687076;
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
  border-radius: 4px;
  background: #f1f3f5;
  color: #687076;
  font-size: 12px;
}
.template-preview-section {
  display: grid;
  gap: 10px;
}
.template-preview-section h3 {
  margin: 0;
  font-size: 14px;
  line-height: 1.3;
  color: #11181c;
}
.preview-table {
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  overflow: hidden;
}
.preview-row {
  display: grid;
  grid-template-columns: 1.4fr 0.8fr 1.2fr 1.2fr;
  gap: 10px;
  align-items: start;
  padding: 10px 12px;
  border-top: 1px solid #f1f3f5;
  font-size: 12px;
  color: #3f464d;
}
.preview-row:first-child {
  border-top: 0;
}
.preview-row--head {
  background: #fbfcfd;
  color: #687076;
  font-weight: 600;
}
.preview-row small {
  display: block;
  margin-top: 2px;
  color: #9ba1a6;
  overflow-wrap: anywhere;
}
.preview-block-list {
  display: grid;
  gap: 12px;
}
.preview-block {
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  overflow: hidden;
}
.preview-block-title {
  padding: 9px 12px;
  background: #fbfcfd;
  color: #687076;
  font-size: 12px;
  border-bottom: 1px solid #f1f3f5;
}
.template-preview pre,
.preview-block pre {
  margin: 0;
  padding: 12px;
  background: #0f172a;
  color: #e5e7eb;
  overflow: auto;
  max-height: 320px;
  font-size: 12px;
  line-height: 1.45;
}
.preview-block pre + pre {
  border-top: 1px solid rgba(255, 255, 255, 0.12);
}
.template-preview code,
.preview-block code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.preview-chip-list {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.preview-empty {
  padding: 10px 12px;
  border: 1px dashed #d7dbdf;
  border-radius: 6px;
  color: #9ba1a6;
  font-size: 12px;
}

@media (max-width: 672px) {
  .config-import-grid,
  .preview-row {
    grid-template-columns: 1fr;
  }
}
</style>
