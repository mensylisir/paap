<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">模板管理</h1>
        <p class="page-desc">管理平台的工具模板与基础设施模板</p>
      </div>
      <div class="header-actions">
        <button class="rail-btn rail-btn--ghost" :disabled="syncing" @click="syncBuiltinTemplates">
          {{ syncing ? '同步中...' : '同步内置模板' }}
        </button>
        <button class="rail-btn rail-btn--primary" @click="openUploadModal">
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M11 18l-7 7 7 7v-5h12v-4H11v-5zM21 14V9H9v4h12v5l7-7-7-7v5z"/></svg>
          上传模板
        </button>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <!-- KPI -->
    <div class="kpi-section">
      <div class="kpi-card">
        <div class="kpi-number">{{ templates.length }}</div>
        <div class="kpi-label">模板总数</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ builtinCount }}</div>
        <div class="kpi-label">内置模板</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ customCount }}</div>
        <div class="kpi-label">自定义模板</div>
      </div>
    </div>

    <!-- Template list -->
    <section class="section-card">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">模板列表</h2>
          <p class="rail-section-desc">当前平台上所有可用模板的索引</p>
        </div>
        <button class="rail-btn rail-btn--ghost" @click="loadTemplates">
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M26 12H6V10h20v2zm0 4H6v2h20v-2zm0 6H6v2h20v-2z"/></svg>
          刷新
        </button>
      </div>

      <div v-if="loading" class="loading-mask">
        <div class="loading-spinner" />
      </div>

      <div v-else-if="templates.length === 0" class="rail-empty">
        <p class="rail-empty-desc">暂无模板，点击「同步内置模板」初始化。</p>
      </div>

      <div v-else class="template-list">
        <div v-for="tmpl in sortedTemplates" :key="tmpl.id || tmpl.type" class="template-row">
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
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '../api/client'

const templates = ref<any[]>([])
const loading = ref(false)
const uploading = ref(false)
const syncing = ref(false)
const showUploadModal = ref(false)
const uploadFile = ref<File | null>(null)
const pageError = ref('')
const uploadError = ref('')
const uploadForm = ref({
  type: '',
  name: '',
  category: 'tool',
  description: '',
})

const builtinCount = computed(() => templates.value.filter((t) => !t.isCustom).length)
const customCount = computed(() => templates.value.filter((t) => t.isCustom).length)
const sortedTemplates = computed(() =>
  [...templates.value].sort((a, b) => {
    const aOrder = Number(a.installOrder ?? 999)
    const bOrder = Number(b.installOrder ?? 999)
    if (aOrder !== bOrder) return aOrder - bOrder
    return String(a.name || a.type).localeCompare(String(b.name || b.type), 'zh-Hans-CN')
  })
)

onMounted(loadTemplates)

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

function openUploadModal() {
  uploadError.value = ''
  showUploadModal.value = true
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

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    tool: '工具',
    infra: '基础设施',
    middleware: '中间件',
  }
  return labels[category] || category || '未分类'
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
  .template-header {
    flex-direction: column;
    align-items: flex-start;
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
</style>
