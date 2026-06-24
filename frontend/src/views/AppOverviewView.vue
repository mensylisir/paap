<template>
  <div class="rail-page">
    <!-- Page header -->
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">应用概览</h1>
        <p class="page-desc">管理应用的运行环境和部署状态</p>
      </div>
      <button class="rail-btn rail-btn--primary" @click="openCreateEnvironmentModal">
        <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M17 15V7h-2v8H7v2h8v8h2v-8h8v-2z"/></svg>
        创建环境
      </button>
    </header>

    <!-- KPI -->
    <section class="kpi-section">
      <div v-for="kpi in kpis" :key="kpi.label" class="kpi-card">
        <div class="kpi-number">{{ kpi.value }}</div>
        <div class="kpi-label-row">
          <span v-if="kpi.dotClass" class="rail-status-dot" :class="kpi.dotClass" />
          {{ kpi.label }}
        </div>
      </div>
    </section>

    <!-- 环境状态 -->
    <section class="section-card">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">环境</h2>
          <p class="rail-section-desc">管理应用的运行环境</p>
        </div>
      </div>

      <div v-if="environments.length === 0" class="rail-empty minimal">
        <svg width="48" height="48" viewBox="0 0 32 32" fill="none" style="margin-bottom:12px">
          <rect x="4" y="8" width="24" height="16" rx="2" stroke="#d1d5db" stroke-width="1.5"/>
          <line x1="4" y1="14" x2="28" y2="14" stroke="#d1d5db" stroke-width="1.5"/>
          <rect x="8" y="18" width="6" height="3" rx="1" fill="#e6e8eb"/>
          <rect x="18" y="18" width="8" height="3" rx="1" fill="#e6e8eb"/>
        </svg>
        <p class="rail-empty-desc" style="max-width:360px">暂无环境。创建第一个环境来开始部署服务。</p>
        <button class="rail-btn rail-btn--primary" style="margin-top:12px" @click="openCreateEnvironmentModal">
          创建第一个环境
        </button>
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
    <section class="section-card">
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

    <Teleport to="body">
      <div v-if="showCreateEnvModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeCreateEnvironmentModal">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">创建环境</p>
              <p class="modal-heading">新建环境</p>
            </div>
            <button class="modal-close" type="button" aria-label="关闭" :disabled="creatingEnv" @click="closeCreateEnvironmentModal">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-item">
              <label class="form-label">环境名称 <span class="required">*</span></label>
              <input v-model.trim="envForm.name" class="rail-input" placeholder="例如：开发环境" @keyup.enter="submitEnvironment" />
            </div>
            <div class="form-item">
              <label class="form-label">环境标识</label>
              <input v-model.trim="envForm.identifier" class="rail-input" placeholder="留空由后台生成" />
              <div class="form-helper">当前预览：{{ identifierPreview }}</div>
            </div>
            <div class="form-item">
              <label class="form-label">创建方式</label>
              <div class="radio-group">
                <label class="radio-item" :class="{ active: envForm.mode === 'empty' }">
                  <input type="radio" value="empty" v-model="envForm.mode" />
                  <span>创建基础环境</span>
                </label>
                <label class="radio-item" :class="{ active: envForm.mode === 'template' }">
                  <input type="radio" value="template" v-model="envForm.mode" />
                  <span>从模板创建</span>
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
              <label class="form-label" for="overview-environment-ip-pool">网络地址池</label>
              <input id="overview-environment-ip-pool" class="rail-input environment-ip-pool-state" value="暂未启用" readonly disabled aria-describedby="overview-environment-ip-pool-helper" />
              <div id="overview-environment-ip-pool-helper" class="form-helper">当前环境创建使用平台默认网络规划，自定义 IP 池将在后续版本启用。</div>
            </div>
            <div v-if="modalError" class="form-error" role="alert">{{ modalError }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="rail-btn rail-btn--ghost" :disabled="creatingEnv" @click="closeCreateEnvironmentModal">取消</button>
            <button type="button" class="rail-btn rail-btn--primary" :disabled="!envForm.name || creatingEnv" @click="submitEnvironment">
              {{ creatingEnv ? '创建中...' : '创建' }}
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
import { buildRecentEvents, countServiceIssues, effectiveEnvironmentStatus, environmentResourceSummary, environmentStatusDotClass, environmentStatusLabel, sumApplicationResources } from './appSummary'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)
const environments = ref<any[]>([])
const recentEvents = ref<string[]>([])
const templates = ref<any[]>([])
const showCreateEnvModal = ref(false)
const creatingEnv = ref(false)
const deletingEnvId = ref<number | null>(null)
const modalError = ref('')
const deleteError = ref('')
const envForm = ref({ name: '', identifier: '', mode: 'empty' as string, templateId: '1' })
const pendingDeleteEnv = ref<any | null>(null)

const runningCount = computed(() => environments.value.filter(e => effectiveEnvironmentStatus(e) === 'running').length)
const resourceTotals = computed(() => sumApplicationResources(environments.value))
const totalTools = computed(() => resourceTotals.value.toolCount)
const totalMiddleware = computed(() => resourceTotals.value.middlewareCount)
const totalComponents = computed(() => resourceTotals.value.componentCount)
const serviceIssues = computed(() => countServiceIssues(environments.value))
const identifierPreview = computed(() => toIdentifier(envForm.value.identifier || envForm.value.name, 'env'))

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
  envForm.value = { name: '', identifier: '', mode: 'empty', templateId: String(templates.value[0]?.id || 1) }
  modalError.value = ''
  showCreateEnvModal.value = true
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
      fromEmpty: envForm.value.mode === 'empty',
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
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--paap-space-6);
  background: rgba(17,19,24,0.46);
  backdrop-filter: blur(10px);
}
.modal-container {
  width: min(520px, 100%);
  max-height: 90vh;
  overflow-y: auto;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
}
.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-4);
  padding: var(--paap-space-5) var(--paap-space-6);
  border-bottom: 1px solid var(--paap-border);
}
.modal-label {
  margin: 0 0 4px;
  color: var(--paap-muted);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.modal-heading {
  margin: 0;
  color: var(--paap-text);
  font-size: 18px;
  font-weight: 600;
}
.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: transparent;
  color: var(--paap-muted);
  cursor: pointer;
}
.modal-body { padding: var(--paap-space-6); }
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  padding: var(--paap-space-4) var(--paap-space-6);
  border-top: 1px solid var(--paap-border);
}
.form-item {
  display: grid;
  gap: 6px;
  margin-bottom: var(--paap-space-5);
}
.form-item:last-child { margin-bottom: 0; }
.form-label {
  color: var(--paap-muted);
  font-size: 12px;
  font-weight: 500;
}
.required,
.form-error { color: var(--paap-danger); }
.form-helper {
  color: var(--paap-muted-2);
  font-size: 12px;
}
.form-error {
  padding: 10px 12px;
  border: 1px solid #fecaca;
  border-radius: var(--paap-radius-sm);
  background: var(--paap-danger-soft);
  font-size: 13px;
}
.radio-group {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--paap-space-2);
}
.radio-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 38px;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-muted);
  cursor: pointer;
}
.radio-item.active {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
  color: var(--paap-text);
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
  font-size: 28px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.page-desc {
  font-size: 14px;
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
  font-size: 13px;
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
  transition: border-color 0.15s;
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-3);
}
.env-card:hover {
  border-color: var(--paap-border-strong);
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
  font-size: 11px;
  color: var(--paap-muted-2);
  letter-spacing: 0.02em;
}
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 500;
  padding: 2px 10px;
  border-radius: var(--paap-radius-full);
  white-space: nowrap;
  flex-shrink: 0;
  background: var(--cds-tag-background-gray, var(--cds-gray-20, #e0e0e0));
  color: var(--paap-muted);
}
.status-badge.running { background: var(--cds-tag-background-green, var(--paap-success-soft)); color: var(--cds-tag-color-green, var(--paap-success)); }
.status-badge.error { background: var(--cds-tag-background-red, var(--paap-danger-soft)); color: var(--cds-tag-color-red, var(--paap-danger)); }
.status-badge.creating { background: var(--paap-accent-soft); color: var(--paap-accent); }
.status-badge.stopped { background: var(--cds-tag-background-teal, var(--cds-teal-20, #9ef0f0)); color: var(--cds-tag-color-teal, var(--cds-teal-70, #005d5d)); }
.status-badge.empty { background: var(--cds-tag-background-gray, var(--cds-gray-20, #e0e0e0)); color: var(--cds-tag-color-gray, var(--paap-text)); }

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
  border-bottom: 1px solid var(--cds-border-subtle-02, var(--paap-border));
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
  font-size: 14px;
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
@media (max-width: 640px) {
  .rail-page { padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10); }
  .page-header { flex-direction: column; align-items: flex-start; gap: var(--paap-space-4); }
  .section-header { flex-direction: column; gap: var(--paap-space-3); }
  .env-grid { grid-template-columns: 1fr; }
  .kpi-section { grid-template-columns: repeat(2, 1fr); }
}
</style>
