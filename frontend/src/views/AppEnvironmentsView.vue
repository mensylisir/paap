<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">环境管理</h1>
        <p class="page-desc">管理应用的所有运行环境</p>
      </div>
      <button v-if="!isSystemApp" class="rail-btn rail-btn--primary" @click="openModal">
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
        <button v-if="!isSystemApp" class="rail-btn rail-btn--primary" style="margin-top:8px" @click="openModal">创建第一个环境</button>
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
                v-if="!env.isSystem"
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

    <CreateEnvironmentModal
      :visible="showModal"
      :creating="creating"
      :error="modalError"
      :templates="templates"
      :form="envForm"
      dialog-id-prefix="environments"
      @update:form="envForm = $event"
      @close="closeCreateEnvironmentModal"
      @submit="submitEnv"
    />

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
import CreateEnvironmentModal from '../components/CreateEnvironmentModal.vue'
import { toIdentifier } from '../utils/identifier'
import { effectiveEnvironmentStatus, environmentResourceSummary, environmentStatusDotClass, environmentStatusLabel } from './appSummary'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)

const app = ref<any>(null)
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
const envForm = ref({ name: '', identifier: '', mode: 'empty' as string, templateId: '1', additionalNamespacesInput: '' })
const pendingDeleteEnv = ref<any | null>(null)

const runningCount = computed(() => environments.value.filter(e => environmentCardStatus(e) === 'running').length)
const totalTools = computed(() => environments.value.reduce((s, e) => s + environmentCardResources(e).toolCount, 0))
const totalComponents = computed(() => environments.value.reduce((s, e) => s + ((components.value[e.id] || []).length), 0))
const isSystemApp = computed(() => Boolean(app.value?.isSystem))

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
  if (!isSystemApp.value && (route.query.create === 'true' || environments.value.length === 0)) {
    autoOpenCreateEnvironment()
    return
  }
  if (route.query.auto === 'true') goToDefaultEnvironment()
})

async function loadEnvs() {
  loading.value = true
  try {
    const [appRes, envRes] = await Promise.allSettled([api.getApp(appId), api.listEnvs(appId)])
    if (appRes.status === 'fulfilled') app.value = appRes.value.data?.application || appRes.value.data
    environments.value = envRes.status === 'fulfilled' ? (envRes.value.data || []) : []
    for (const env of environments.value) {
      try { components.value[env.id] = (await api.listComponents(env.id)).data || [] } catch (e) {}
      try { services.value[env.id] = (await api.listServices(env.id)).data || [] } catch (e) {}
    }
  } catch (e) { console.error(e) }
  finally { loading.value = false }
}

function openModal() {
  if (isSystemApp.value) return
  envForm.value = { name: '', identifier: '', mode: 'empty', templateId: String(templates.value[0]?.id || 1), additionalNamespacesInput: '' }
  modalError.value = ''
  showModal.value = true
}

function autoOpenCreateEnvironment() {
  openModal()
}

function closeCreateEnvironmentModal() {
  if (creating.value) return
  showModal.value = false
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
      fromEmpty: envForm.value.mode === 'empty' || envForm.value.mode === 'blank',
      blank: envForm.value.mode === 'blank',
      additionalNamespaces: parseAdditionalNamespacesInput(envForm.value.additionalNamespacesInput),
    })
    showModal.value = false
    const envId = res.data?.id
    if (envId) router.push(`/apps/${appId}/environments/${envId}`)
    else await loadEnvs()
  } catch (e: any) { modalError.value = '创建失败：' + (e?.message || '未知错误') }
  finally { creating.value = false }
}

function parseAdditionalNamespacesInput(value: string) {
  const seen = new Set<string>()
  return String(value || '')
    .split(/[\n,，]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      const [rawSuffix, rawPurpose] = item.split(/[:：]/)
      const suffix = toIdentifier(rawSuffix || '', 'ns')
      const purpose = toIdentifier(rawPurpose || rawSuffix || '', 'workload')
      return { suffix, purpose }
    })
    .filter((item) => {
      if (!item.suffix || seen.has(item.suffix)) return false
      seen.add(item.suffix)
      return true
    })
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
.modal-overlay { position: fixed; inset: 0; z-index: 9000; background: rgba(17,19,24,0.46); backdrop-filter: blur(10px); display: flex; align-items: center; justify-content: center; padding: var(--paap-space-6); }
.modal-container { background: var(--cds-layer-01, var(--paap-panel)); width: min(520px, 100%); max-height: 90vh; overflow-y: auto; border: 1px solid var(--cds-border-subtle-01, var(--paap-border)); border-radius: 0; box-shadow: none; }
.modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: var(--paap-space-5) var(--paap-space-6); border-bottom: 1px solid var(--cds-border-subtle-01, var(--paap-border)); }
.modal-label { font-size: var(--cds-label-01-font-size, 12px); color: var(--cds-text-secondary, var(--paap-muted)); margin-bottom: var(--paap-space-2); text-transform: uppercase; letter-spacing: var(--cds-label-01-letter-spacing, 0.32px); font-weight: var(--cds-font-weight-semibold, 600); }
.modal-heading { font-size: var(--cds-heading-03-font-size, 20px); font-weight: var(--cds-heading-03-font-weight, 400); color: var(--cds-text-primary, var(--paap-text)); margin: 0; line-height: var(--cds-heading-03-line-height, 1.4); }
.modal-close { background: var(--cds-layer-01, var(--paap-panel)); border: 1px solid var(--cds-border-subtle-01, var(--paap-border)); color: var(--cds-text-secondary, var(--paap-muted)); cursor: pointer; padding: 4px; display: flex; align-items: center; justify-content: center; border-radius: 0; transition: background 110ms, color 110ms, border-color 110ms; width: 32px; height: 32px; }
.modal-close:hover { background: var(--cds-layer-hover-01, var(--paap-panel-subtle)); color: var(--cds-text-primary, var(--paap-text)); }
.modal-body { padding: var(--paap-space-6); }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--paap-space-2); padding: var(--paap-space-4) var(--paap-space-6); border-top: 1px solid var(--cds-border-subtle-01, var(--paap-border)); }

.confirm-text { color: var(--cds-text-primary, var(--paap-text)); font-size: var(--cds-body-compact-01-font-size, 14px); line-height: 1.6; margin: 0; }
.confirm-text + .form-error { margin-top: 16px; }
.form-error {
  border: 1px solid var(--cds-border-error, var(--cds-red-60, var(--paap-danger)));
  background: var(--cds-layer-01, var(--paap-panel));
  color: var(--cds-text-error, var(--paap-danger));
  border-radius: 0;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.4;
}
</style>
