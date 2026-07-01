<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">配置模板</h1>
        <p class="page-desc">管理组件运行配置模板，服务产品与环境服务统一在服务目录查看。</p>
      </div>
      <div class="header-actions">
        <cv-button v-has-perm="'system.template.manage'" kind="primary" @click="openConfigTemplateImportModal">
          导入配置模板
        </cv-button>
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
    <div class="kpi-section slide-up">
      <div class="kpi-card">
        <div class="kpi-number">{{ configTemplates.length }}</div>
        <div class="kpi-label">配置模板</div>
      </div>
    </div>

    <!-- Template list -->
    <section class="section-card slide-up">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">{{ activeTemplateTitle }}</h2>
          <p class="rail-section-desc">{{ activeTemplateDescription }}</p>
        </div>
        <cv-button kind="ghost" @click="refreshActiveTemplates">
          <template #icon>
            <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M26 12H6V10h20v2zm0 4H6v2h20v-2zm0 6H6v2h20v-2z"/></svg>
          </template>
          刷新
        </cv-button>
      </div>

      <div v-if="loading" class="loading-mask">
        <div class="loading-spinner" />
      </div>

      <div v-else-if="configTemplates.length === 0" class="rail-empty">
        <p class="rail-empty-desc">暂无配置模板。点击「导入配置模板」上传普通配置文件或高级模板包。</p>
      </div>

      <div v-else class="template-list config-template-list">
        <div v-for="tmpl in configTemplates" :key="tmpl.id || tmpl.name" class="template-row config-template-row">
          <div class="template-body">
            <div class="template-header">
              <div class="template-name-group">
                <span class="template-name">{{ tmpl.name }}</span>
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
              <cv-button kind="ghost" size="sm" @click="openConfigTemplateDetail(tmpl)">
                查看模板
              </cv-button>
              <cv-button kind="danger--ghost" size="sm" @click="deleteConfigTemplate(tmpl.id)">
                删除
              </cv-button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <Teleport to="body">
      <div v-if="showConfigTemplateImportModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showConfigTemplateImportModal = false">
        <div class="modal-container modal-container--wide">
          <div class="modal-header">
            <div>
              <p class="modal-label">导入配置模板</p>
              <p class="modal-heading">{{ configImportMode === 'native' ? '上传普通配置文件' : '导入高级模板包' }}</p>
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
                    <small>上传自己的配置文件，只把可变字段换成模板语法</small>
                  </button>
                  <button type="button" class="config-import-mode-card" :class="{ active: configImportMode === 'advanced' }" @click="configImportMode = 'advanced'">
                    <span>高级模板包</span>
                    <small>高级用户导入 template.json/schema.json</small>
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
              <label class="form-label">{{ configImportMode === 'native' ? '普通配置文件' : '高级模板包' }}</label>
              <input class="rail-input" type="file" :multiple="configImportMode === 'native'" :accept="configImportMode === 'native' ? '.yml,.yaml,.json,.toml,.conf,.ini,.properties,.env,.txt,application/json,text/*' : '.json,.tar.gz,application/json,application/gzip'" @change="onConfigTemplateFileChange" />
              <div class="form-helper">{{ configImportMode === 'native' ? '普通模式直接上传用户自己的配置文件，可一次选择多个文件；文件里用 __TEMPLATE__KEY__显示名__ 标记需要填写的字段。' : '高级模式仅面向高级用户，可上传完整 PAAP template.json，或包含 template.json/schema.json 的 tar.gz 包。' }}</div>
            </div>
            <div class="form-item">
              <div class="form-label-row">
                <label class="form-label">{{ configImportMode === 'native' ? '普通配置内容' : '高级模板 JSON' }}</label>
                <button type="button" class="text-btn" @click="fillConfigTemplateSample">{{ configImportMode === 'native' ? '填入示例' : '填入高级示例' }}</button>
              </div>
              <textarea v-model.trim="configTemplateImportText" class="rail-textarea code-textarea" rows="14" spellcheck="false" :placeholder="configTemplateImportPlaceholder"></textarea>
              <div class="form-helper">{{ configImportMode === 'native' ? '普通用户只需要在自己的配置文件中写 __TEMPLATE__KEY__显示名__；FOR/IF 会自动解析成字段。' : '高级 JSON / schema 模式适合高级用户，可直接定义字段、环境变量、普通配置、敏感配置和文件。' }}</div>
            </div>
            <div v-if="configUploadError" class="form-error" role="alert">{{ configUploadError }}</div>
          </div>
          <div class="modal-footer">
            <cv-button kind="ghost" @click="showConfigTemplateImportModal = false">取消</cv-button>
            <cv-button kind="primary" :disabled="configUploading" @click="submitConfigTemplateImport">
              {{ configUploading ? '导入中...' : '导入配置模板' }}
            </cv-button>
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
            </div>
            <div class="template-preview-summary" aria-label="配置模板影响摘要">
              <div v-for="item in configTemplatePreviewSummary(selectedConfigTemplate)" :key="item.label" class="template-preview-summary-item">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </div>
            </div>
            <section class="template-preview-section template-preview-fields" aria-label="配置模板抽取字段">
              <h3>抽取字段</h3>
              <div v-if="configTemplatePreviewFields(selectedConfigTemplate).length" class="preview-table">
                <div class="preview-row preview-row--head">
                  <span>字段键</span>
                  <span>类型</span>
                  <span>默认值</span>
                  <span>来源</span>
                </div>
                <div v-for="field in configTemplatePreviewFields(selectedConfigTemplate)" :key="field.key" class="preview-row">
                  <span>
                    <strong>{{ field.key }}</strong>
                    <small>{{ field.label }}</small>
                  </span>
                  <span>{{ field.type }}</span>
                  <span>{{ field.defaultValue }}</span>
                  <span>{{ field.source }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">这个模板没有抽取出可填写字段。</div>
            </section>
            <section class="template-preview-section template-preview-files" aria-label="配置模板生成文件明细">
              <h3>生成文件明细</h3>
              <div v-if="configTemplatePreviewFiles(selectedConfigTemplate).length" class="preview-table">
                <div class="preview-row preview-row--files preview-row--head">
                  <span>文件名</span>
                  <span>来源</span>
                  <span>推荐挂载路径</span>
                  <span>访问方式</span>
                </div>
                <div v-for="file in configTemplatePreviewFiles(selectedConfigTemplate)" :key="file.identity" class="preview-row preview-row--files">
                  <span>
                    <strong>{{ file.name }}</strong>
                    <small>{{ file.key }}</small>
                  </span>
                  <span>{{ file.source }}</span>
                  <span>{{ file.mountPath }}</span>
                  <span>{{ file.readMode }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">这个模板不会生成配置文件。</div>
            </section>
            <section class="template-preview-section template-preview-validation" aria-label="配置模板校验提示">
              <h3>校验提示</h3>
              <div v-if="configTemplatePreviewValidationItems(selectedConfigTemplate).length" class="preview-validation-list">
                <div v-for="item in configTemplatePreviewValidationItems(selectedConfigTemplate)" :key="item.message" :class="['preview-validation-item', item.level]">
                  <strong>{{ item.label }}</strong>
                  <span>{{ item.message }}</span>
                </div>
              </div>
              <div v-else class="preview-empty">未发现预览层面的配置问题。</div>
            </section>
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

    <cv-modal
      kind="danger"
      :visible="pendingConfigTemplateDelete !== null"
      @primary-click="confirmDeleteConfigTemplate"
      @secondary-click="pendingConfigTemplateDelete = null"
      @modal-hidden="pendingConfigTemplateDelete = null"
    >
      <template #title>删除配置模板</template>
      <template #content>
        <p><strong>{{ pendingConfigTemplateDelete?.name }}</strong></p>
        <p>删除后，这个自定义配置模板将不再出现在组件右侧栏中。已应用到组件的运行配置不会被自动回滚。</p>
        <div v-if="pageError" class="form-error" role="alert">{{ pageError }}</div>
      </template>
      <template #secondary-button>取消</template>
      <template #primary-button>删除</template>
    </cv-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '../api/client'
import { nativeConfigTemplateSyntax, parseNativeConfigTemplate } from './configTemplateSyntax'
import { normalizeComponentTemplateFiles } from './componentConfigTemplateFiles'

const configTemplates = ref<any[]>([])
const loading = ref(false)
const configUploading = ref(false)
const showConfigTemplateImportModal = ref(false)
const pageError = ref('')
const configUploadError = ref('')
const selectedConfigTemplate = ref<any | null>(null)
const configTemplatePreviewTab = ref<'native' | 'advanced'>('native')
const configImportMode = ref<'native' | 'advanced'>('native')
const pendingConfigTemplateDelete = ref<any | null>(null)
const activeTemplateTab = ref<'config'>('config')
const configImportForm = ref({
  key: '',
  name: '',
  description: '',
  framework: 'auto',
  bindingMode: 'recommended',
  componentTypes: 'backend',
})
const selectedConfigTemplateUploadFiles = ref<File[]>([])
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

const templateTabs = computed(() => [
  { key: 'config' as const, label: '配置模板', count: configTemplates.value.length },
])
const activeTemplateTitle = computed(() => templateTabs.value.find((tab) => tab.key === activeTemplateTab.value)?.label || '模板列表')
const activeTemplateDescription = computed(() => '组件运行配置模板，包含环境变量、普通配置、敏感配置、配置文件和启动参数。')

onMounted(async () => {
  await loadConfigTemplates()
})

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
  await loadConfigTemplates()
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

function openConfigTemplateImportModal() {
  configUploadError.value = ''
  showConfigTemplateImportModal.value = true
}

async function onConfigTemplateFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  selectedConfigTemplateUploadFiles.value = files
  configUploadError.value = ''
  if (!files.length) return
  try {
    if (files.length === 1) {
      configTemplateImportText.value = await files[0].text()
      return
    }
    const blocks = await Promise.all(files.map(async (file) => {
      const text = await file.text()
      return `# ${file.name}\n${text}`
    }))
    configTemplateImportText.value = blocks.join('\n\n')
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
  const formData = new FormData()
  let raw: any = {}
  const text = configTemplateImportText.value.trim()
  let uploadFiles = selectedConfigTemplateUploadFiles.value
  if (!uploadFiles.length && text) {
    if (configImportMode.value === 'advanced') {
      try {
        raw = normalizeAdvancedConfigTemplateImport(JSON.parse(text))
      } catch (e: any) {
        configUploadError.value = '高级模板 JSON 解析失败：' + (e?.message || 'JSON 格式错误')
        return
      }
      uploadFiles = [new File([text], 'template.json', { type: 'application/json' })]
    } else {
      raw = parseNativeConfigTemplate(text, {
        framework: configImportForm.value.framework,
      })
      uploadFiles = [new File([text], defaultConfigTemplateUploadFileName(configImportForm.value.framework), { type: 'text/plain' })]
    }
  }
  const payload = configTemplatePayloadFromImport(raw)
  if (!uploadFiles.length) {
    configUploadError.value = '请选择配置文件或高级模板包。'
    return
  }
  if (!selectedConfigTemplateUploadFiles.value.length && !payload.name) {
    configUploadError.value = '请填写模板名称，或在 JSON 中提供 name。'
    return
  }
  uploadFiles.forEach((file) => {
    if (uploadFiles.length === 1) {
      formData.set('file', file)
    } else {
      formData.append('files', file)
    }
  })
  formData.set('mode', configImportMode.value)
  formData.set('key', configImportForm.value.key || payload.key || '')
  formData.set('name', configImportForm.value.name || payload.name || '')
  formData.set('description', configImportForm.value.description || payload.description || '')
  formData.set('framework', payload.framework || configImportForm.value.framework || 'auto')
  formData.set('bindingMode', payload.bindingMode || configImportForm.value.bindingMode || 'recommended')
  formData.set('componentTypes', (payload.componentTypes || selectedConfigImportComponentTypes()).join(','))
  configUploading.value = true
  try {
    await api.uploadComponentConfigTemplate(formData)
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
  selectedConfigTemplateUploadFiles.value = []
  configTemplateImportText.value = ''
}

function defaultConfigTemplateUploadFileName(framework: string) {
  const normalized = String(framework || '').toLowerCase()
  if (normalized === 'nginx') return 'default.conf'
  if (normalized === 'springboot' || normalized === 'spring') return 'application.yml'
  if (normalized === 'node') return '.env'
  return 'config.txt'
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
  const rawFramework = String(raw?.framework || '').trim()
  const rawBindingMode = String(raw?.bindingMode || '').trim()
  return {
    key: configImportForm.value.key || String(raw?.key || '').trim(),
    name: configImportForm.value.name || String(raw?.name || '').trim(),
    description: configImportForm.value.description || String(raw?.description || '').trim(),
    framework: rawFramework || configImportForm.value.framework || 'auto',
    bindingMode: rawBindingMode || configImportForm.value.bindingMode || 'recommended',
    componentTypes: rawComponentTypes.length ? rawComponentTypes : componentTypes,
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
  if (!tmpl) return
  pendingConfigTemplateDelete.value = tmpl
}

async function confirmDeleteConfigTemplate() {
  const tmpl = pendingConfigTemplateDelete.value
  if (!tmpl) return
  pageError.value = ''
  try {
    await api.deleteComponentConfigTemplate(tmpl.id)
    configTemplates.value = configTemplates.value.filter((item) => String(item.id) !== String(tmpl.id))
    pendingConfigTemplateDelete.value = null
  } catch (e: any) {
    pageError.value = '删除配置模板失败：' + (e?.message || '未知错误')
  }
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

function configTemplatePreviewFields(tmpl: any) {
  const fields: Array<{ key: string; label: string; type: string; defaultValue: string; source: string }> = []
  for (const field of tmpl?.fields || []) {
    const key = String(field?.key || '').trim()
    if (!key) continue
    fields.push(configTemplatePreviewFieldRow(field, key))
    for (const child of field?.itemFields || []) {
      const childKey = String(child?.key || '').trim()
      if (!childKey) continue
      fields.push(configTemplatePreviewFieldRow(child, `${key}.${childKey}`))
    }
  }
  return fields
}

function configTemplatePreviewFieldRow(field: any, key: string) {
  return {
    key,
    label: String(field?.label || key),
    type: configTemplateFieldTypeLabel(field?.type),
    defaultValue: configTemplateFieldDefaultLabel(field),
    source: configTemplateFieldSourceLabel(field),
  }
}

function configTemplateFieldTypeLabel(value: string) {
  const labels: Record<string, string> = {
    text: '文本',
    string: '文本',
    number: '数字',
    boolean: '开关',
    password: '敏感文本',
    secret: '敏感文本',
    list: '列表',
    serviceRef: '服务引用',
  }
  return labels[String(value || '')] || String(value || '文本')
}

function configTemplateFieldDefaultLabel(field: any) {
  const value = field?.default ?? field?.defaultValue ?? ''
  const text = String(value ?? '').trim()
  return text || '无默认值'
}

function configTemplateFieldSourceLabel(field: any) {
  const hint = String(field?.target || field?.output || field?.source || field?.type || '').toLowerCase()
  if (hint.includes('secret') || hint.includes('password') || hint.includes('token')) return '敏感配置'
  if (hint.includes('config') || hint.includes('file')) return '配置文件'
  if (hint.includes('env')) return '运行变量'
  if (hint.includes('service')) return '服务引用'
  if (hint.includes('backend') || hint.includes('frontend') || hint.includes('worker')) return '组件引用'
  return '模板字段'
}

function configTemplatePreviewFiles(tmpl: any) {
  const rows: Array<{ identity: string; name: string; key: string; source: string; mountPath: string; readMode: string }> = []
  const seen = new Set<string>()
  const add = (input: { name?: string; key?: string; source: string; mountPath?: string; readOnly?: boolean }) => {
    const name = String(input.name || input.key || '').trim()
    const key = String(input.key || input.name || '').trim()
    if (!name && !key) return
    const mountPath = String(input.mountPath || '').trim()
    const identity = [name, key].join(':')
    if (seen.has(identity)) return
    seen.add(identity)
    rows.push({
      identity,
      name: name || key,
      key: key || name,
      source: input.source,
      mountPath: mountPath || '组件配置时选择',
      readMode: input.readOnly === false ? '可写入' : '只读挂载',
    })
  }

  for (const item of tmpl?.files || []) {
    add({
      name: item?.name,
      key: item?.key,
      source: '模板文件',
      mountPath: item?.recommendedMountPath || item?.mountPath,
      readOnly: item?.readOnly,
    })
  }
  for (const item of tmpl?.nativeConfigs || []) {
    add({
      name: item?.name,
      key: item?.name,
      source: '原生配置',
      mountPath: item?.recommendedMountPath || item?.mountPath,
      readOnly: true,
    })
  }
  for (const item of tmpl?.configMaps || []) {
    const objectName = String(item?.name || '普通配置').trim()
    for (const key of Object.keys(item?.data || {})) {
      add({
        name: key,
        key,
        source: objectName,
        mountPath: item?.recommendedMountPath || item?.mountPath,
        readOnly: true,
      })
    }
  }
  return rows
}

function configTemplatePreviewValidationItems(tmpl: any) {
  const items: Array<{ level: 'info' | 'warning' | 'danger'; label: string; message: string }> = []
  const fields = Array.isArray(tmpl?.fields) ? tmpl.fields : []
  const files = configTemplatePreviewFiles(tmpl)
  const fieldKeys = new Set<string>()

  if (!fields.length) {
    items.push({ level: 'info', label: '提示', message: '没有可填写项，应用该模板时不会打开字段表单。' })
  }
  if (!files.length && !arrayLength(tmpl?.env) && !arrayLength(tmpl?.secrets)) {
    items.push({ level: 'warning', label: '提示', message: '没有生成文件或运行配置，导入后可能不会产生配置内容。' })
  }
  for (const field of fields) {
    const key = String(field?.key || '').trim()
    if (!key) {
      items.push({ level: 'danger', label: '错误', message: '存在未填写字段键的可填写项。' })
      continue
    }
    if (fieldKeys.has(key)) {
      items.push({ level: 'warning', label: '提示', message: `字段 ${key} 重复出现，应用时可能互相覆盖。` })
    }
    fieldKeys.add(key)
    if (!String(field?.label || '').trim()) {
      items.push({ level: 'info', label: '提示', message: `字段 ${key} 缺少显示名，将直接使用字段键展示。` })
    }
    if (String(field?.type || '').toLowerCase() === 'list' && !Array.isArray(field?.itemFields)) {
      items.push({ level: 'warning', label: '提示', message: `列表字段 ${key} 没有子字段定义，批量项预览可能不完整。` })
    }
  }
  for (const file of files) {
    if (file.mountPath === '组件配置时选择') {
      items.push({ level: 'info', label: '提示', message: `${file.name} 没有推荐挂载路径，应用到组件时需要手动选择。` })
    }
  }
  return items
}

function arrayLength(value: any) {
  return Array.isArray(value) ? value.length : 0
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

</script>

<style scoped>
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
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
  color: var(--paap-text);
  letter-spacing: 0;
  line-height: 1.2;
}
.page-desc {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  line-height: 1.4;
}
.page-error {
  border: 1px solid var(--paap-danger);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  border-radius: var(--paap-radius-sm);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
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
  border-bottom: 1px solid var(--paap-border);
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
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  cursor: pointer;
}
.template-tabs button.active {
  border-bottom-color: var(--paap-accent);
  color: var(--paap-text);
}
.template-tabs strong {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
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
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kpi-number {
  font-size: var(--paap-fs-heading-2xl);
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: 0;
  line-height: 1.2;
}
.kpi-label {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
}

/* ===== Section ===== */
.section-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
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
  border: 2px solid var(--paap-border);
  border-top-color: var(--paap-text);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

/* ===== Template list ===== */
.template-list {
  border-top: 1px solid var(--paap-border);
}
.template-row {
  display: flex;
  border-bottom: 1px solid var(--paap-border);
  transition: background-color 110ms;
  overflow: hidden;
  cursor: default;
}
.template-row:last-child {
  border-bottom: none;
}
.template-row:hover {
  background: var(--paap-accent-fill);
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
  color: var(--paap-text);
}
.template-meta {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--paap-muted);
  margin-bottom: 6px;
  font-size: var(--paap-fs-compact);
}
.template-meta .uploaded {
  color: var(--paap-success);
}
.template-desc {
  color: var(--paap-muted);
  line-height: 1.4;
  font-size: var(--paap-fs-compact);
}
.config-template-plain-summary,
.environment-template-summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 10px;
}
.config-template-plain-summary span,
.environment-template-summary span {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.3;
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
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
}
.config-template-details span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.config-template-details code {
  min-width: 0;
  color: var(--paap-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--paap-fs-label);
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
  font-size: var(--paap-fs-small);
  font-weight: 500;
  letter-spacing: 0.2px;
  border-radius: var(--paap-radius-xs);
}
.tag.builtin {
  background: var(--paap-panel);
  color: var(--paap-muted);
}
.tag.custom {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.tag.heavy {
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
}
.tag.config {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}

/* Meta info: uploaded state */
.template-meta .uploaded {
  color: var(--paap-success);
}

.policy {
  color: var(--paap-muted);
  white-space: nowrap;
  font-size: var(--paap-fs-label);
  letter-spacing: 0.2px;
}
.text-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: var(--paap-radius-sm);
  background: transparent;
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  text-decoration: none;
  transition: background-color var(--paap-transition-fast), border-color var(--paap-transition-fast), color var(--paap-transition-fast);
}
.text-btn:hover {
  background: var(--paap-accent-soft);
  color: var(--paap-accent-hover);
  text-decoration: none;
}
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--paap-z-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--paap-overlay);
}
.modal-container {
  width: 480px;
  max-height: 90vh;
  overflow-y: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-lg);
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
  gap: 16px;
  padding: 20px 24px;
  border-bottom: 1px solid var(--paap-border);
}
.modal-label {
  margin: 0 0 4px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 600;
  line-height: 1.3;
}
.modal-heading {
  margin: 0;
  color: var(--paap-text);
  font-size: 18px;
  font-weight: 600;
  line-height: 1.3;
}
.modal-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 1px solid transparent;
  border-radius: var(--paap-radius-sm);
  background: transparent;
  color: var(--paap-muted);
  cursor: pointer;
  transition: background-color var(--paap-transition-fast), border-color var(--paap-transition-fast), color var(--paap-transition-fast);
}
.modal-close:hover {
  border-color: var(--paap-border);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
}
.modal-body {
  display: grid;
  gap: 16px;
  padding: 20px 24px;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px 20px;
  border-top: 1px solid var(--paap-border);
}
.rail-input,
.rail-select,
.rail-textarea {
  width: 100%;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  font-size: var(--paap-fs-body);
  outline: none;
  transition: border-color var(--paap-transition-fast), box-shadow var(--paap-transition-fast);
}
.rail-input,
.rail-select {
  min-height: 40px;
  padding: 0 12px;
}
.rail-input {
  background: var(--paap-panel);
  border-radius: var(--paap-radius-sm);
}
.rail-textarea {
  min-height: 96px;
  padding: 10px 12px;
  resize: vertical;
}
.rail-input:focus,
.rail-select:focus,
.rail-textarea:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-focus-ring);
}
.rail-input:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-focus-ring);
}
.form-item {
  display: grid;
  gap: 6px;
}
.form-label,
.form-label-row {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}
.form-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.form-helper {
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  line-height: 1.4;
}
.form-error {
  color: var(--paap-danger);
  font-size: var(--paap-fs-compact);
  line-height: 1.4;
}
.config-import-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.config-import-mode {
  grid-column: 1 / -1;
}
.template-mode-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}
.config-import-mode-card {
  min-height: 76px;
  padding: 12px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  color: var(--paap-text);
  text-align: left;
  cursor: pointer;
}
.config-import-mode-card.active {
  border-color: var(--paap-accent);
  box-shadow: inset 0 0 0 1px var(--paap-accent);
}
.config-import-mode-card span,
.config-import-mode-card small {
  display: block;
}
.config-import-mode-card small {
  margin-top: 6px;
  color: var(--paap-muted);
  line-height: 1.4;
}
.config-import-shell--carbon {
  background: var(--paap-panel);
}
.code-textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: var(--paap-fs-code);
  line-height: 1.5;
}
.template-preview {
  gap: 18px;
}
.template-preview-meta,
.template-preview-summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.template-preview-meta span,
.template-preview-summary-item {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  padding: 8px 10px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}
.template-preview-summary-item {
  display: grid;
  gap: 4px;
}
.template-preview-summary-item strong {
  color: var(--paap-text);
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
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.template-preview-tabs button {
  min-height: 44px;
  padding: 0 18px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: inherit;
  cursor: pointer;
}
.template-preview-tabs button.active {
  border-bottom-color: var(--paap-accent);
  color: var(--paap-text);
}
.preview-table {
  border: 1px solid var(--paap-border);
}
.preview-row {
  display: grid;
  grid-template-columns: 1.4fr 0.8fr 1.2fr 1.2fr;
  gap: 10px;
  padding: 10px 12px;
  border-top: 1px solid var(--paap-border);
  font-size: var(--paap-fs-label);
}
.preview-row:first-child {
  border-top: 0;
}
.preview-row--head {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-weight: 600;
}
.preview-row--files {
  grid-template-columns: 1.3fr 1fr 1.4fr 0.8fr;
}
.preview-row strong,
.preview-row small {
  display: block;
  overflow-wrap: anywhere;
}
.preview-row small {
  margin-top: 2px;
  color: var(--paap-muted);
}
.preview-block-list {
  display: grid;
  gap: 12px;
}
.preview-block {
  border: 1px solid var(--paap-border);
  border-radius: 0;
  overflow: hidden;
  background: var(--paap-panel);
}
.preview-block-title {
  min-height: 40px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-weight: 600;
}
.preview-block pre {
  margin: 0;
  padding: 12px 14px;
  max-height: 360px;
  overflow: auto;
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  line-height: 1.55;
}
.preview-block code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.preview-empty {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}
.preview-validation-list {
  display: grid;
  gap: 8px;
}
.preview-validation-item {
  display: grid;
  gap: 4px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  padding: 10px 12px;
}

/* ===== Responsive ===== */
@media (max-width: 672px) {
  .rail-page {
    padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10);
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
  .config-import-grid,
  .template-mode-switch {
    grid-template-columns: 1fr;
  }
}
</style>

<style>
/* Global shared styles have been moved to src/style.scss (Shared UI Patterns section).
   This empty global block must remain to prevent Vue from removing it —
   the styles are available project-wide via style.scss. */
</style>
