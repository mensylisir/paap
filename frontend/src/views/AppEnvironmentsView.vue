<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">环境管理</h1>
        <p class="page-desc">管理应用的所有运行环境</p>
      </div>
      <button class="rail-btn rail-btn--primary" @click="openModal">
        <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M17 15V7h-2v8H7v2h8v8h2v-8h8v-2z"/></svg>
        创建环境
      </button>
    </header>

    <!-- KPI -->
    <div class="kpi-section">
      <div class="kpi-card">
        <div class="kpi-number">{{ environments.length }}</div>
        <div class="kpi-label">环境总数</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number" :class="{ 'text-green': runningCount > 0 }">{{ runningCount }}</div>
        <div class="kpi-label">运行中</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ totalTools }}</div>
        <div class="kpi-label">已安装工具</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ totalComponents }}</div>
        <div class="kpi-label">业务组件</div>
      </div>
    </div>

    <!-- Environment list -->
    <section class="section-card">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">环境列表</h2>
          <p class="rail-section-desc">当前应用下的所有环境</p>
        </div>
      </div>

      <div v-if="loading" class="loading-mask">
        <div class="loading-spinner" />
      </div>

      <div v-else-if="environments.length === 0" class="rail-empty">
        <svg width="48" height="48" viewBox="0 0 32 32" fill="none" style="margin-bottom:12px">
          <rect x="4" y="8" width="24" height="16" rx="2" stroke="#d1d5db" stroke-width="1.5"/>
          <line x1="4" y1="14" x2="28" y2="14" stroke="#d1d5db" stroke-width="1.5"/>
          <rect x="8" y="18" width="6" height="3" rx="1" fill="#e6e8eb"/>
          <rect x="18" y="18" width="8" height="3" rx="1" fill="#e6e8eb"/>
        </svg>
        <h3 class="rail-empty-title">暂无环境</h3>
        <p class="rail-empty-desc">创建第一个环境来部署服务。</p>
        <button class="rail-btn rail-btn--primary" style="margin-top:8px" @click="openModal">创建第一个环境</button>
      </div>

      <div v-else class="env-grid">
        <div
          v-for="env in environments"
          :key="env.id"
          class="env-card"
          role="button"
          tabindex="0"
          @click="goToEnv(env.id)"
          @keydown.enter="goToEnv(env.id)"
          @keydown.space.prevent="goToEnv(env.id)"
        >
          <div class="env-card-top">
            <div class="env-name-group">
              <h3 class="env-name">{{ env.name }}</h3>
              <span class="env-id">{{ env.identifier }}</span>
            </div>
            <div class="env-card-actions">
              <span class="status-badge" :class="environmentCardStatus(env)">
                <span class="rail-status-dot" :class="`rail-status-dot--${environmentStatusDotClass(environmentCardStatus(env))}`" />
                {{ environmentStatusLabel(environmentCardStatus(env)) }}
              </span>
              <button
                type="button"
                class="rail-btn rail-btn--danger rail-btn--sm env-delete-btn"
                :disabled="deletingEnvId === Number(env.id)"
                @click.stop="openDeleteEnvironmentDialog(env)"
              >
                {{ deletingEnvId === Number(env.id) ? '删除中...' : '删除环境' }}
              </button>
            </div>
          </div>
          <div class="env-meta">
            <span v-if="environmentCardResources(env).toolCount" class="rail-tag rail-tag--blue">{{ environmentCardResources(env).toolCount }} 工具</span>
            <span v-if="environmentCardResources(env).middlewareCount" class="rail-tag rail-tag--purple">{{ environmentCardResources(env).middlewareCount }} 中间件</span>
            <span v-if="(components[env.id] || []).length" class="rail-tag rail-tag--green">{{ (components[env.id] || []).length }} 组件</span>
            <span v-if="!environmentCardResources(env).toolCount && !environmentCardResources(env).middlewareCount && !(components[env.id] || []).length" class="rail-tag rail-tag--gray">基座未安装</span>
          </div>
        </div>
      </div>
      <div v-if="deleteError" class="form-error env-list-error" role="alert">{{ deleteError }}</div>
    </section>

    <!-- Create Modal -->
    <Teleport to="body">
      <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">创建环境</p>
              <p class="modal-heading">新建环境</p>
            </div>
            <button class="modal-close" @click="showModal = false">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-item">
              <label class="form-label">环境名称 <span class="required">*</span></label>
              <input v-model.trim="envForm.name" class="rail-input" placeholder="例如：测试环境" />
            </div>
            <div class="form-item">
              <label class="form-label">环境标识</label>
              <input v-model.trim="envForm.identifier" class="rail-input" placeholder="留空由后台生成" />
              <div class="form-helper">当前预览：{{ identifierPreview }}</div>
            </div>
            <div class="form-item">
              <label class="form-label">创建方式</label>
              <div class="radio-group">
                <label class="radio-item" :class="{ active: envForm.mode === 'template' }">
                  <input type="radio" value="template" v-model="envForm.mode" />
                  <span>从模板创建</span>
                </label>
                <label class="radio-item" :class="{ active: envForm.mode === 'empty' }">
                  <input type="radio" value="empty" v-model="envForm.mode" />
                  <span>创建基础环境</span>
                </label>
              </div>
            </div>
            <div v-if="envForm.mode === 'template'" class="form-item">
              <label class="form-label">选择模板</label>
              <select v-model="envForm.templateId" class="rail-select">
                <option v-for="t in templates" :key="t.id" :value="String(t.id)">{{ t.name }}</option>
              </select>
            </div>
            <div class="form-item">
              <label class="form-label" for="environments-environment-ip-pool">网络地址池</label>
              <input id="environments-environment-ip-pool" class="rail-input environment-ip-pool-state" value="暂未启用" readonly disabled aria-describedby="environments-environment-ip-pool-helper" />
              <div id="environments-environment-ip-pool-helper" class="form-helper">当前环境创建使用平台默认网络规划，自定义 IP 池将在后续版本启用。</div>
            </div>
            <div v-if="modalError" class="form-error" role="alert">{{ modalError }}</div>
          </div>
          <div class="modal-footer">
            <button class="rail-btn rail-btn--ghost" @click="showModal = false">取消</button>
            <button class="rail-btn rail-btn--primary" :disabled="!envForm.name || creating" @click="submitEnv">
              {{ creating ? '创建中...' : '创建' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="pendingDeleteEnv" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeDeleteEnvironmentDialog">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">删除环境</p>
              <p class="modal-heading">确认删除 {{ pendingDeleteEnv.name || pendingDeleteEnv.id }}</p>
            </div>
            <button class="modal-close" type="button" aria-label="关闭" :disabled="deletingEnvId !== null" @click="closeDeleteEnvironmentDialog">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="confirm-text">这会删除环境记录和关联资源，请确认后继续。</p>
            <div v-if="deleteError" class="form-error" role="alert">{{ deleteError }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="rail-btn rail-btn--ghost" :disabled="deletingEnvId !== null" @click="closeDeleteEnvironmentDialog">取消</button>
            <button type="button" class="rail-btn rail-btn--danger" :disabled="deletingEnvId !== null" @click="performDeleteEnvironment">
              {{ deletingEnvId !== null ? '删除中...' : '确认删除' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { toIdentifier } from '../utils/identifier'
import { effectiveEnvironmentStatus, environmentResourceSummary, environmentStatusDotClass, environmentStatusLabel } from './appSummary'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)

const environments = ref<any[]>([])
const components = ref<Record<number, any[]>>({})
const services = ref<Record<number, any[]>>({})
const templates = ref<any[]>([])
const showModal = ref(false)
const loading = ref(false)
const creating = ref(false)
const deletingEnvId = ref<number | null>(null)
const modalError = ref('')
const deleteError = ref('')
const envForm = ref({ name: '', identifier: '', mode: 'empty' as string, templateId: '1' })
const pendingDeleteEnv = ref<any | null>(null)

const runningCount = computed(() => environments.value.filter(e => environmentCardStatus(e) === 'running').length)
const totalTools = computed(() => environments.value.reduce((s, e) => s + environmentCardResources(e).toolCount, 0))
const totalComponents = computed(() => environments.value.reduce((s, e) => s + ((components.value[e.id] || []).length), 0))
const identifierPreview = computed(() => toIdentifier(envForm.value.identifier || envForm.value.name, 'env'))

function environmentCardSummary(env: any) {
  return {
    ...env,
    services: services.value[env.id] || env.services || [],
    componentCount: (components.value[env.id] || []).length || Number(env.componentCount || 0),
  }
}

function environmentCardResources(env: any) {
  return environmentResourceSummary(environmentCardSummary(env))
}

function environmentCardStatus(env: any) {
  return effectiveEnvironmentStatus(environmentCardSummary(env))
}

onMounted(async () => {
  await loadEnvs()
  try { templates.value = (await api.templates()).data || [] } catch (e) {}
  if (route.query.create === 'true' || environments.value.length === 0) {
    autoOpenCreateEnvironment()
    return
  }
  if (route.query.auto === 'true') goToDefaultEnvironment()
})

async function loadEnvs() {
  loading.value = true
  try {
    const res = await api.listEnvs(appId)
    environments.value = res.data || []
    for (const env of environments.value) {
      try { components.value[env.id] = (await api.listComponents(env.id)).data || [] } catch (e) {}
      try { services.value[env.id] = (await api.listServices(env.id)).data || [] } catch (e) {}
    }
  } catch (e) { console.error(e) }
  finally { loading.value = false }
}

function openModal() {
  envForm.value = { name: '', identifier: '', mode: 'empty', templateId: String(templates.value[0]?.id || 1) }
  modalError.value = ''
  showModal.value = true
}

function autoOpenCreateEnvironment() {
  openModal()
}

function goToDefaultEnvironment() {
  const first = environments.value[0]
  if (first?.id) router.replace(`/apps/${appId}/environments/${first.id}`)
}

const submitEnv = async () => {
  if (!envForm.value.name) return
  creating.value = true
  modalError.value = ''
  try {
    const res = await api.createEnv(appId, {
      name: envForm.value.name,
      identifier: envForm.value.identifier,
      templateId: envForm.value.mode === 'template' ? Number(envForm.value.templateId) : 0,
      fromEmpty: envForm.value.mode === 'empty',
    })
    showModal.value = false
    const envId = res.data?.id
    if (envId) router.push(`/apps/${appId}/environments/${envId}`)
    else await loadEnvs()
  } catch (e: any) { modalError.value = '创建失败：' + (e?.message || '未知错误') }
  finally { creating.value = false }
}

const goToEnv = (envId: number) => router.push(`/apps/${appId}/environments/${envId}`)

const openDeleteEnvironmentDialog = (env: any) => {
  if (!env?.id || deletingEnvId.value !== null) return
  deleteError.value = ''
  pendingDeleteEnv.value = env
}

const closeDeleteEnvironmentDialog = () => {
  if (deletingEnvId.value !== null) return
  pendingDeleteEnv.value = null
}

const performDeleteEnvironment = async () => {
  const env = pendingDeleteEnv.value
  if (!env?.id || deletingEnvId.value !== null) return
  deletingEnvId.value = Number(env.id)
  deleteError.value = ''
  try {
    await api.deleteEnv(Number(env.id))
    pendingDeleteEnv.value = null
    await loadEnvs()
  } catch (e: any) {
    deleteError.value = '删除环境失败：' + (e?.message || '未知错误')
  } finally {
    deletingEnvId.value = null
  }
}
</script>

<style scoped>
.rail-page {
  padding: 20px 20px 36px;
  max-width: none;
}

/* Header */
.page-header {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 20px;
}
.header-text { display: flex; flex-direction: column; gap: 2px; }
.page-title { font-size: 24px; font-weight: 600; color: #11181c; letter-spacing: 0; line-height: 1.2; }
.page-desc { font-size: 14px; color: #687076; line-height: 1.4; }

/* KPI */
.kpi-section {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px; margin-bottom: 32px;
}
.kpi-card {
  background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px;
  padding: 16px 18px; display: flex; flex-direction: column; gap: 8px;
}
.kpi-number {
  font-size: 28px; font-weight: 600; color: #11181c;
  letter-spacing: 0; line-height: 1.2;
}
.kpi-number.text-green { color: #22c55e; }
.kpi-label { font-size: 12px; color: #687076; }

/* Section */
.section-card { background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px; padding: 24px; }
.section-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; }

/* Loading */
.loading-mask { display: flex; align-items: center; justify-content: center; padding: 64px; }
.loading-spinner { width: 24px; height: 24px; border: 2px solid #e6e8eb; border-top-color: #11181c; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Env cards */
.env-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px; }
.env-card {
  background: #ffffff; border: 1px solid #e6e8eb; border-radius: 8px;
  padding: 18px 20px; cursor: pointer; transition: all 0.15s ease;
  display: flex; flex-direction: column; gap: 12px;
}
.env-card:hover { border-color: #d1d5db; box-shadow: 0 4px 12px rgba(0,0,0,0.04); }
.env-card-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.env-name-group { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.env-name { font-size: 16px; font-weight: 600; color: #11181c; margin: 0; line-height: 1.3; }
.env-id { font-family: 'IBM Plex Mono', monospace; font-size: 11px; color: #9ba1a6; letter-spacing: 0.3px; }
.env-card-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.env-delete-btn { white-space: nowrap; }
.env-list-error { margin-top: 16px; }
.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  font-size: 11px; font-weight: 500; padding: 3px 8px; border-radius: 4px;
  background: #f1f3f5; color: #687076; white-space: nowrap; flex-shrink: 0;
}
.status-badge.running { background: #f0fdf4; color: #16a34a; }
.status-badge.error { background: #fef2f2; color: #dc2626; }
.status-badge.creating { background: #eff6ff; color: #2563eb; }
.status-badge.empty { background: #f1f3f5; color: #687076; }
.env-meta { display: flex; flex-wrap: wrap; gap: 6px; }

/* Responsive */
@media (max-width: 672px) {
  .rail-page { padding: 20px 20px 32px; }
  .page-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .kpi-section { grid-template-columns: 1fr 1fr; }
  .env-grid { grid-template-columns: 1fr; }
}
</style>

<style>
.modal-overlay { position: fixed; inset: 0; z-index: 9000; background: rgba(0,0,0,0.45); display: flex; align-items: center; justify-content: center; padding: 24px; }
.modal-container { background: #ffffff; width: 520px; max-height: 90vh; overflow-y: auto; border-radius: 8px; box-shadow: 0 20px 40px rgba(0,0,0,0.15); }
.modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: 20px 24px; border-bottom: 1px solid #f1f3f5; }
.modal-label { font-size: 11px; color: #9ba1a6; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px; font-weight: 600; }
.modal-heading { font-size: 18px; font-weight: 600; color: #11181c; margin: 0; line-height: 1.3; }
.modal-close { background: none; border: none; color: #9ba1a6; cursor: pointer; padding: 4px; display: flex; align-items: center; justify-content: center; border-radius: 4px; transition: all 0.15s; }
.modal-close:hover { background: #f1f3f5; color: #11181c; }
.modal-body { padding: 24px; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; padding: 16px 24px; border-top: 1px solid #f1f3f5; }

.form-item { display: flex; flex-direction: column; gap: 6px; margin-bottom: 20px; }
.form-item:last-child { margin-bottom: 0; }
.form-label { font-size: 12px; color: #687076; font-weight: 500; }
.required { color: #ef4444; }
.form-helper { font-size: 12px; color: #9ba1a6; margin-top: 2px; }
.confirm-text { color: #11181c; font-size: 14px; line-height: 1.6; margin: 0; }
.confirm-text + .form-error { margin-top: 16px; }
.form-error {
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #991b1b;
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
}

.radio-group { display: flex; gap: 12px; }
.radio-item {
  display: flex; align-items: center; gap: 8px; padding: 10px 16px;
  border: 1px solid #e6e8eb; border-radius: 6px; cursor: pointer;
  font-size: 14px; color: #11181c; transition: all 0.15s;
}
.radio-item.active { border-color: #3b82f6; background: #f5f9ff; }
.radio-item:hover { border-color: #d1d5db; }
</style>
