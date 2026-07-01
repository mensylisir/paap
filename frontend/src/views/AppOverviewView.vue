<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">应用概览</h1>
        <p class="page-desc">管理应用的运行环境和部署状态</p>
      </div>
      <cv-button v-if="!isSystemApp" kind="primary" @click="openCreateEnvironmentModal">
        <template #icon>
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M17 15V7h-2v8H7v2h8v8h2v-8h8v-2z"/></svg>
        </template>
        创建环境
      </cv-button>
    </header>

    <!-- KPI -->
    <section class="kpi-section slide-up">
      <div v-for="kpi in kpis" :key="kpi.label" class="kpi-card">
        <div class="kpi-number">{{ kpi.value }}</div>
        <div class="kpi-label-row">
          <span v-if="kpi.dotClass" class="rail-status-dot" :class="kpi.dotClass" />
          {{ kpi.label }}
        </div>
      </div>
    </section>

    <!-- 环境状态 -->
    <section class="section-card slide-up">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">环境</h2>
          <p class="rail-section-desc">管理应用的运行环境</p>
        </div>
      </div>

      <div v-if="environments.length === 0" class="rail-empty minimal">
        <svg class="env-empty-icon" width="48" height="48" viewBox="0 0 32 32" fill="none" style="margin-bottom:12px">
          <rect x="4" y="8" width="24" height="16" rx="2" stroke="var(--paap-border-03)" stroke-width="1.5"/>
          <line x1="4" y1="14" x2="28" y2="14" stroke="var(--paap-border-03)" stroke-width="1.5"/>
          <rect x="8" y="18" width="6" height="3" rx="1" fill="var(--paap-border)"/>
          <rect x="18" y="18" width="8" height="3" rx="1" fill="var(--paap-border)"/>
        </svg>
        <p class="rail-empty-desc" style="max-width:360px">暂无环境。创建第一个环境来开始部署服务。</p>
        <cv-button v-if="!isSystemApp" kind="primary" style="margin-top:12px" @click="openCreateEnvironmentModal">
          创建第一个环境
        </cv-button>
      </div>

      <div v-else class="env-grid">
        <div v-for="env in environments" :key="env.id" class="env-card" @click="goToEnv(env.id)">
          <div class="env-card-top">
            <div class="env-name-group">
              <h3 class="env-name">{{ env.name }}</h3>
              <span class="env-id">{{ env.identifier }}</span>
            </div>
            <div class="env-card-actions">
              <span class="status-badge" :class="effectiveEnvironmentStatus(env)">
                <span class="rail-status-dot" :class="`rail-status-dot--${environmentStatusDotClass(effectiveEnvironmentStatus(env))}`" />
                {{ environmentStatusLabel(effectiveEnvironmentStatus(env)) }}
              </span>
              <cv-button
                v-if="!env.isSystem"
                kind="danger"
                size="sm"
                :disabled="deletingEnvId === Number(env.id)"
                @click.stop="openDeleteEnvironmentDialog(env)"
              >
                {{ deletingEnvId === Number(env.id) ? '删除中...' : '删除环境' }}
              </cv-button>
            </div>
          </div>
          <div class="env-meta">
            <span v-if="environmentResourceSummary(env).toolCount" class="rail-tag rail-tag--blue">{{ environmentResourceSummary(env).toolCount }} 工具</span>
            <span v-if="environmentResourceSummary(env).middlewareCount" class="rail-tag rail-tag--purple">{{ environmentResourceSummary(env).middlewareCount }} 中间件</span>
            <span v-if="environmentResourceSummary(env).componentCount" class="rail-tag rail-tag--green">{{ environmentResourceSummary(env).componentCount }} 组件</span>
            <span v-if="!environmentResourceSummary(env).toolCount && !environmentResourceSummary(env).middlewareCount && !environmentResourceSummary(env).componentCount" class="rail-tag rail-tag--gray">基座未安装</span>
          </div>
        </div>
      </div>
      <div v-if="deleteError" class="form-error env-list-error" role="alert">{{ deleteError }}</div>
    </section>

    <!-- 最近事件 -->
    <section class="section-card slide-up">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">最近事件</h2>
          <p class="rail-section-desc">关键操作记录</p>
        </div>
      </div>
      <div v-if="recentEvents.length > 0" class="event-list">
        <div v-for="(evt, i) in recentEvents" :key="i" class="event-item" :class="{first: i===0}">
          <span class="event-dot" :class="{active: i===0}" />
          <div class="event-text">{{ evt }}</div>
        </div>
      </div>
      <div v-else class="rail-empty minimal">
        <p class="rail-empty-desc">暂无事件记录</p>
      </div>
    </section>

    <CreateEnvironmentModal
      :visible="showCreateEnvModal"
      :creating="creatingEnv"
      :error="modalError"
      :templates="templates"
      :shared-resources="sharedResources"
      :shared-resources-loading="sharedResourcesLoading"
      :shared-resources-error="sharedResourcesError"
      :form="envForm"
      dialog-id-prefix="overview"
      @update:form="envForm = $event"
      @close="closeCreateEnvironmentModal"
      @submit="submitEnvironment"
    />

    <cv-modal
      v-if="pendingDeleteEnv"
      kind="danger"
      :visible="!!pendingDeleteEnv"
      :primary-button-disabled="deletingEnvId !== null"
      @primary-click="performDeleteEnvironment"
      @secondary-click="closeDeleteEnvironmentDialog"
      @modal-hidden="closeDeleteEnvironmentDialog"
      :close-aria-label="'关闭'"
    >
      <template #title>删除环境</template>
      <template #content>
        <p>确认删除 {{ pendingDeleteEnv.name || pendingDeleteEnv.id }}</p>
        <p style="margin-top:8px;color:var(--paap-muted);font-size:var(--paap-fs-body)">这会删除环境记录和关联资源，请确认后继续。</p>
        <div v-if="deleteError" class="form-error" role="alert" style="margin-top:8px">{{ deleteError }}</div>
      </template>
      <template #secondary-button>取消</template>
      <template #primary-button>{{ deletingEnvId !== null ? '删除中...' : '确认删除' }}</template>
    </cv-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import CreateEnvironmentModal from '../components/CreateEnvironmentModal.vue'
import { toIdentifier } from '../utils/identifier'
import { buildRecentEvents, countServiceIssues, effectiveEnvironmentStatus, environmentResourceSummary, environmentStatusDotClass, environmentStatusLabel, sumApplicationResources } from './appSummary'
import { buildSharedCapabilityPayload, emptyEnvironmentForm, type SharedCapabilityResource } from './createEnvironmentSharedServices'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)
const app = ref<any>(null)
const environments = ref<any[]>([])
const recentEvents = ref<string[]>([])
const templates = ref<any[]>([])
const showCreateEnvModal = ref(false)
const creatingEnv = ref(false)
const deletingEnvId = ref<number | null>(null)
const modalError = ref('')
const deleteError = ref('')
const sharedResources = ref<SharedCapabilityResource[]>([])
const sharedResourcesLoading = ref(false)
const sharedResourcesError = ref('')
const envForm = ref(emptyEnvironmentForm())
const pendingDeleteEnv = ref<any | null>(null)

const runningCount = computed(() => environments.value.filter(e => effectiveEnvironmentStatus(e) === 'running').length)
const resourceTotals = computed(() => sumApplicationResources(environments.value))
const totalTools = computed(() => resourceTotals.value.toolCount)
const totalMiddleware = computed(() => resourceTotals.value.middlewareCount)
const totalComponents = computed(() => resourceTotals.value.componentCount)
const serviceIssues = computed(() => countServiceIssues(environments.value))
const isSystemApp = computed(() => Boolean(app.value?.isSystem))

const kpis = computed(() => [
  { label: '环境总数', value: environments.value.length, dotClass: '' },
  { label: '运行中', value: runningCount.value, dotClass: 'rail-status-dot--running' },
  { label: '已安装工具', value: totalTools.value, dotClass: 'rail-status-dot--creating' },
  { label: '中间件', value: totalMiddleware.value, dotClass: 'rail-status-dot--warning' },
  { label: '业务组件', value: totalComponents.value, dotClass: 'rail-status-dot--empty' },
  { label: '待处理', value: serviceIssues.value, dotClass: serviceIssues.value > 0 ? 'rail-status-dot--error' : 'rail-status-dot--empty' },
])

async function loadAppOverview() {
  try {
    const res = await api.getApp(appId)
    const appPayload = res.data?.application || res.data
    app.value = appPayload
    const appEnvironments = res.data?.environments || res.environments || appPayload?.environments || []
    environments.value = await hydrateEnvironmentSummaries(appEnvironments)
    recentEvents.value = buildRecentEvents(environments.value)
    templates.value = (await api.templates()).data || []
  } catch (e) { console.error(e) }
}

async function hydrateEnvironmentSummaries(items: any[]) {
  return Promise.all((Array.isArray(items) ? items : []).map(async (env: any) => {
    const [componentsRes, servicesRes] = await Promise.allSettled([
      api.listComponents(env.id),
      api.listServices(env.id),
    ])
    const envComponents = componentsRes.status === 'fulfilled' ? (componentsRes.value.data || []) : []
    const envServices = servicesRes.status === 'fulfilled' ? (servicesRes.value.data || []) : []
    return {
      ...env,
      services: envServices.length ? envServices : (env.services || []),
      componentCount: envComponents.length || Number(env.componentCount || 0),
    }
  }))
}

onMounted(async () => {
  await loadAppOverview()
  if (route.query.createEnvironment === 'true') openCreateEnvironmentModal()
})

const goToEnv = (envId: number) => router.push(`/apps/${appId}/environments/${envId}`)
const openCreateEnvironmentModal = () => {
  if (isSystemApp.value) return
  envForm.value = emptyEnvironmentForm(templates.value[0]?.id || 1)
  modalError.value = ''
  showCreateEnvModal.value = true
  loadSharedResourcesForEnvironmentCreate()
}
const closeCreateEnvironmentModal = () => {
  if (creatingEnv.value) return
  showCreateEnvModal.value = false
}
const submitEnvironment = async () => {
  if (!envForm.value.name || creatingEnv.value) return
  creatingEnv.value = true
  modalError.value = ''
  try {
    const res = await api.createEnv(appId, {
      name: envForm.value.name,
      identifier: envForm.value.identifier,
      templateId: envForm.value.mode === 'template' ? Number(envForm.value.templateId) : 0,
      fromEmpty: envForm.value.mode === 'empty' || envForm.value.mode === 'blank',
      blank: envForm.value.mode === 'blank',
      additionalNamespaces: parseAdditionalNamespacesInput(envForm.value.additionalNamespacesInput),
      capabilities: buildSharedCapabilityPayload(envForm.value.sharedResourceIds, sharedResources.value),
    })
    showCreateEnvModal.value = false
    const envId = res.data?.id || res.data?.environment?.id
    if (envId) router.push(`/apps/${appId}/environments/${envId}`)
    else await loadAppOverview()
  } catch (e: any) {
    modalError.value = '创建失败：' + (e?.message || '未知错误')
  } finally {
    creatingEnv.value = false
  }
}

async function loadSharedResourcesForEnvironmentCreate() {
  sharedResourcesLoading.value = true
  sharedResourcesError.value = ''
  try {
    const res = await api.listSharedCapabilityResources()
    sharedResources.value = Array.isArray(res.data) ? res.data : []
  } catch (e: any) {
    sharedResources.value = []
    sharedResourcesError.value = '公共服务读取失败，创建后可在环境画布中添加。'
  } finally {
    sharedResourcesLoading.value = false
  }
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
    await loadAppOverview()
  } catch (e: any) {
    deleteError.value = '删除环境失败：' + (e?.message || '未知错误')
  } finally {
    deletingEnvId.value = null
  }
}
</script>

<style scoped>
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
  max-width: none;
}
.form-error {
  color: var(--paap-danger);
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-danger-soft);
  font-size: var(--paap-fs-compact);
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: var(--paap-space-8);
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
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.page-desc {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  line-height: 1.4;
  margin-top: var(--paap-space-1);
}

/* KPI */
.kpi-section {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--paap-space-3);
  margin-bottom: var(--paap-space-8);
}
.kpi-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5) var(--paap-space-6);
}
.kpi-number {
  font-size: 32px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.kpi-label-row {
  font-size: var(--paap-fs-compact);
  color: var(--paap-muted);
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: var(--paap-space-2);
}

/* Section cards */
.section-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-6);
  margin-bottom: var(--paap-space-4);
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--paap-space-5);
}

/* Env cards */
.env-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: var(--paap-space-3);
}
.env-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-3);
}
.env-card:hover {
  border-color: var(--paap-border-strong);
  box-shadow: var(--paap-shadow-lg);
}
.env-card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--paap-space-3);
}
.env-card-actions {
  display: flex;
  align-items: flex-start;
  gap: var(--paap-space-2);
  flex-shrink: 0;
}
.env-name-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.env-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--paap-text);
  margin: 0;
  line-height: 1.3;
}
.env-id {
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
  color: var(--paap-muted);
  letter-spacing: 0.02em;
}
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: var(--paap-fs-small);
  font-weight: 500;
  padding: 2px 10px;
  border-radius: var(--paap-radius-full);
  white-space: nowrap;
  flex-shrink: 0;
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
.status-badge.running { background: var(--paap-success-soft); color: var(--paap-success); }
.status-badge.error { background: var(--paap-danger-soft); color: var(--paap-danger); }
.status-badge.creating { background: var(--paap-accent-soft); color: var(--paap-accent); }
.status-badge.stopped { background: var(--paap-panel-subtle); color: var(--paap-muted); }
.status-badge.empty { background: var(--paap-panel-subtle); color: var(--paap-text); }

.env-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* Events */
.event-list {
  display: flex;
  flex-direction: column;
}
.event-item {
  display: flex;
  align-items: flex-start;
  gap: var(--paap-space-3);
  padding: 10px 0;
  border-bottom: 1px solid var(--paap-border);
}
.event-item:last-child { border-bottom: none; }
.event-item.first { padding-top: 0; }
.event-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--paap-border-strong);
  flex-shrink: 0;
  margin-top: 7px;
}
.event-dot.active { background: var(--paap-accent); }
.event-text {
  font-size: var(--paap-fs-body);
  color: var(--paap-text);
  line-height: 1.5;
}

/* Empty */
.minimal { padding: var(--paap-space-8); }

/* Responsive */
@media (max-width: 960px) {
  .kpi-section { grid-template-columns: repeat(3, 1fr); }
  .env-card-top { flex-direction: column; }
  .env-card-actions { width: 100%; justify-content: space-between; }
}
@media (max-width: 672px) {
  .rail-page { padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10); }
  .page-header { flex-direction: column; align-items: flex-start; gap: var(--paap-space-4); }
  .section-header { flex-direction: column; gap: var(--paap-space-3); }
  .env-grid { grid-template-columns: 1fr; }
  .kpi-section { grid-template-columns: repeat(2, 1fr); }
}
</style>
