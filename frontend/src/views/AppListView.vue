<template>
  <div class="rail-page">
    <div v-if="loading" class="loading-mask">
      <div class="loading-spinner" />
    </div>

    <template v-else>
      <header class="page-header">
        <div>
          <h1 class="page-title">应用列表</h1>
          <p class="page-subtitle">选择应用后进入应用菜单，再管理概览和环境</p>
        </div>
        <button v-has-perm="'app.create'" class="rail-btn rail-btn--primary" @click="openCreateAppModal">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          创建应用
        </button>
      </header>

      <section v-if="listedApps.length" class="app-list">
        <div
          v-for="app in listedApps"
          :key="app.id"
          class="app-card"
          role="button"
          tabindex="0"
          @click="goToAppHome(app)"
          @keydown.enter="goToAppHome(app)"
          @keydown.space.prevent="goToAppHome(app)"
        >
          <div class="app-card-main">
            <div class="app-card-header">
              <h3 class="app-card-name">{{ appDisplayName(app) }}</h3>
              <span v-if="app.isSystem" class="rail-tag rail-tag--gray">系统应用</span>
              <span v-if="app.environmentCount" class="rail-tag rail-tag--blue">{{ app.environmentCount }} 环境</span>
              <span v-else class="rail-tag rail-tag--gray">无环境</span>
            </div>
            <code class="app-card-id">{{ app.identifier }}</code>
            <p class="app-card-desc">{{ app.description || '暂无描述' }}</p>
          </div>
          <div class="app-card-meta">
            <span class="app-stat"><strong>{{ appResourceSummary(app).toolCount }}</strong><em>工具</em></span>
            <span class="app-stat"><strong>{{ appResourceSummary(app).middlewareCount }}</strong><em>中间件</em></span>
            <span class="app-stat"><strong>{{ appComponentCount(app) }}</strong><em>组件</em></span>
            <button
              v-if="!app.isSystem"
              type="button"
              class="rail-btn rail-btn--danger rail-btn--sm app-delete-btn"
              :disabled="deletingAppId === Number(app.id)"
              @click.stop="openDeleteApplicationDialog(app)"
            >
              {{ deletingAppId === Number(app.id) ? '删除中...' : '删除应用' }}
            </button>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="9 18 15 12 9 6"/>
            </svg>
          </div>
        </div>
      </section>
      <div v-if="deleteError" class="form-error app-list-error" role="alert">{{ deleteError }}</div>
      <section v-if="!listedApps.length && loadError" class="empty-panel">
        <h2>应用加载失败</h2>
        <p>{{ loadError }}</p>
        <button class="rail-btn rail-btn--primary" @click="loadApps">重新加载</button>
      </section>

      <section v-else-if="!listedApps.length" class="empty-panel">
        <div class="empty-icon">
          <svg width="44" height="44" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <h2>还没有应用</h2>
        <p>创建应用后会继续创建第一个环境，并直接进入环境工作台。</p>
        <button v-has-perm="'app.create'" class="rail-btn rail-btn--primary" @click="openCreateAppModal">创建第一个应用</button>
      </section>
    </template>

    <Teleport to="body">
      <div v-if="showCreateAppModal" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeCreateAppModal">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">创建应用</p>
              <p class="modal-heading">新建应用</p>
            </div>
            <button class="modal-close" type="button" aria-label="关闭" @click="closeCreateAppModal">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-item">
              <label class="form-label">应用名称 <span class="required">*</span></label>
              <input v-model.trim="appForm.name" class="rail-input" placeholder="例如：订单服务" @keyup.enter="submitApp" />
            </div>
            <div class="form-item">
              <label class="form-label">应用标识</label>
              <input v-model.trim="appForm.identifier" class="rail-input" placeholder="留空由后台生成" />
              <div class="form-helper">当前预览：{{ identifierPreview }}</div>
            </div>
            <div class="form-item">
              <label class="form-label">应用描述</label>
              <textarea v-model.trim="appForm.description" class="rail-textarea" rows="3" placeholder="简要描述应用用途"></textarea>
            </div>
            <div v-if="formError" class="form-error" role="alert">{{ formError }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="rail-btn rail-btn--ghost" :disabled="submitting" @click="closeCreateAppModal">取消</button>
            <button type="button" class="rail-btn rail-btn--primary" :disabled="!appForm.name || submitting" @click="submitApp">
              {{ submitting ? '创建中...' : '创建应用' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="pendingDeleteApp" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeDeleteApplicationDialog">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">删除应用</p>
              <p class="modal-heading">确认删除 {{ pendingDeleteApp.name || pendingDeleteApp.id }}</p>
            </div>
            <button class="modal-close" type="button" aria-label="关闭" :disabled="deletingAppId !== null" @click="closeDeleteApplicationDialog">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="confirm-text">这会删除应用下的环境记录和关联资源，请确认后继续。</p>
            <div v-if="deleteError" class="form-error" role="alert">{{ deleteError }}</div>
          </div>
          <div class="modal-footer">
            <button type="button" class="rail-btn rail-btn--ghost" :disabled="deletingAppId !== null" @click="closeDeleteApplicationDialog">取消</button>
            <button type="button" class="rail-btn rail-btn--danger" :disabled="deletingAppId !== null" @click="performDeleteApplication">
              {{ deletingAppId !== null ? '删除中...' : '确认删除' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { toIdentifier } from '../utils/identifier'
import { sumApplicationResources } from './appSummary'

const router = useRouter()
const route = useRoute()
const apps = ref<any[]>([])
const loading = ref(true)
const showCreateAppModal = ref(false)
const submitting = ref(false)
const deletingAppId = ref<number | null>(null)
const formError = ref('')
const deleteError = ref('')
const loadError = ref('')
const appForm = ref({ name: '', identifier: '', description: '' })
const pendingDeleteApp = ref<any | null>(null)

const identifierPreview = computed(() => toIdentifier(appForm.value.identifier || appForm.value.name, 'app'))

const appResourceSummary = (app: any) => sumApplicationResources(app)

const appComponentCount = (app: any) =>
  appResourceSummary(app).componentCount

const isSystemSharedResourcePool = (app: any) =>
  Boolean(app?.isSystem) && String(app?.identifier || '') === 'default'

const listedApps = computed(() =>
  apps.value.filter((app: any) => !isSystemSharedResourcePool(app))
)

const appDisplayName = (app: any) =>
  app?.name || app?.identifier || '未命名应用'

const firstBusinessApp = () =>
  listedApps.value.find((app: any) => !app?.isSystem) || listedApps.value[0]

const normalizeApps = (payload: any) => {
  if (Array.isArray(payload?.data)) return payload.data
  if (Array.isArray(payload?.data?.applications)) return payload.data.applications
  if (Array.isArray(payload?.data?.data)) return payload.data.data
  if (Array.isArray(payload?.data?.items)) return payload.data.items
  if (Array.isArray(payload?.applications)) return payload.applications
  if (Array.isArray(payload?.items)) return payload.items
  if (Array.isArray(payload)) return payload
  return []
}

async function loadApps() {
  loading.value = true
  loadError.value = ''
  try {
    apps.value = normalizeApps(await api.listApps())
    if (listedApps.value.length === 0) openCreateAppModal()
    else if (route.query.auto === 'true') goToDefaultWorkspace(firstBusinessApp())
  } catch (e) {
    console.error(e)
    loadError.value = '无法读取应用列表，请稍后重试。'
  } finally {
    loading.value = false
  }
}

function openCreateAppModal() {
  appForm.value = { name: '', identifier: '', description: '' }
  formError.value = ''
  showCreateAppModal.value = true
}

function closeCreateAppModal() {
  if (submitting.value) return
  showCreateAppModal.value = false
}

function goToDefaultWorkspace(app: any) {
  const firstEnv = app.environments?.[0]
  if (firstEnv?.id) router.push(`/apps/${app.id}/environments/${firstEnv.id}`)
  else router.push(`/apps/${app.id}/overview?createEnvironment=true`)
}

function goToAppHome(app: any) {
  router.push(`/apps/${app.id}/overview`)
}

async function submitApp() {
  if (!appForm.value.name || submitting.value) return
  submitting.value = true
  formError.value = ''
  try {
    const res = await api.createApp({
      name: appForm.value.name,
      identifier: appForm.value.identifier,
      description: appForm.value.description,
    })
    const appId = res.data?.id || res.data?.application?.id
    showCreateAppModal.value = false
    if (appId) router.push(`/apps/${appId}/overview?createEnvironment=true`)
    else await loadApps()
  } catch (e: any) {
    formError.value = '创建失败：' + (e?.message || '未知错误')
  } finally {
    submitting.value = false
  }
}

function openDeleteApplicationDialog(app: any) {
  if (!app?.id || deletingAppId.value !== null) return
  deleteError.value = ''
  pendingDeleteApp.value = app
}

function closeDeleteApplicationDialog() {
  if (deletingAppId.value !== null) return
  pendingDeleteApp.value = null
}

async function performDeleteApplication() {
  const app = pendingDeleteApp.value
  if (!app?.id || deletingAppId.value !== null) return
  deletingAppId.value = Number(app.id)
  deleteError.value = ''
  try {
    await api.deleteApp(Number(app.id))
    pendingDeleteApp.value = null
    await loadApps()
  } catch (e: any) {
    deleteError.value = '删除应用失败：' + (e?.message || '未知错误')
  } finally {
    deletingAppId.value = null
  }
}

onMounted(loadApps)
</script>

<style scoped>
.rail-page { padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10); max-width: none; }
.loading-mask { display: flex; align-items: center; justify-content: center; min-height: calc(100vh - 48px); }
.loading-spinner { width: 24px; height: 24px; border: 2px solid var(--paap-border); border-top-color: var(--paap-text); border-radius: 50%; animation: spin 0.8s linear infinite; }
.page-header { display: flex; justify-content: space-between; align-items: flex-end; margin-bottom: var(--paap-space-8); gap: var(--paap-space-4); }
.page-title { font-size: 24px; font-weight: 600; color: var(--paap-text); line-height: 1.2; }
.page-subtitle { font-size: var(--paap-fs-body); color: var(--paap-muted); margin-top: var(--paap-space-1); }
.app-list { display: grid; gap: var(--paap-space-3); }
.app-card {
  display: flex; align-items: center; justify-content: space-between; gap: var(--paap-space-6);
  width: 100%; text-align: left; background: var(--paap-panel); color: inherit;
  border: 1px solid var(--paap-border); border-radius: var(--paap-radius);
  padding: var(--paap-space-5) var(--paap-space-6); cursor: pointer; transition: all 0.15s;
}
.app-card:hover { border-color: var(--paap-border-strong); box-shadow: var(--paap-shadow-lg); }
.app-card-main { flex: 1; min-width: 0; }
.app-card-header { display: flex; align-items: center; gap: var(--paap-space-3); margin-bottom: var(--paap-space-1); flex-wrap: wrap; }
.app-card-name { font-size: var(--paap-fs-heading-lg); font-weight: 600; color: var(--paap-text); line-height: 1.3; }
.app-card-id { display: block; font-family: var(--paap-mono); font-size: var(--paap-fs-code); color: var(--paap-muted); margin-bottom: var(--paap-space-1); }
.app-card-desc { color: var(--paap-muted); font-size: var(--paap-fs-compact); line-height: 1.4; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 680px; }
.app-card-meta { display: flex; align-items: center; gap: var(--paap-space-5); color: var(--paap-muted); flex-shrink: 0; }
.app-delete-btn { white-space: nowrap; }
.app-list-error { margin-top: var(--paap-space-4); }
.app-stat { display: grid; gap: 2px; text-align: right; }
.app-stat strong { color: var(--paap-text); font-size: 15px; line-height: 1.2; }
.app-stat em { color: var(--paap-muted); font-style: normal; font-size: var(--paap-fs-small); }
.empty-panel {
  min-height: calc(100vh - 260px); display: flex; flex-direction: column; align-items: center; justify-content: center;
  text-align: center; gap: var(--paap-space-4); color: var(--paap-muted);
}
.empty-panel h2 { color: var(--paap-text); font-size: 24px; font-weight: 600; }
.empty-panel p { max-width: 460px; line-height: 1.6; }
.empty-icon { width: 76px; height: 76px; display: flex; align-items: center; justify-content: center; border: 1px solid var(--paap-border); border-radius: var(--paap-radius); background: var(--paap-panel); color: var(--paap-muted); }
.modal-container {   background: var(--paap-panel); width: min(520px, 100%); max-height: 90vh; overflow-y: auto; border-radius: var(--paap-radius); border: 1px solid var(--paap-border); box-shadow: var(--paap-shadow-lg); }
.modal-header { display: flex; justify-content: space-between; align-items: flex-start; padding: var(--paap-space-5) var(--paap-space-6); border-bottom: 1px solid var(--paap-border); }
.modal-label { font-size: var(--paap-fs-small); color: var(--paap-muted); text-transform: uppercase; letter-spacing: 0.04em; font-weight: 600; margin-bottom: 4px; }
.modal-heading { color: var(--paap-text); font-size: 18px; font-weight: 600; }
.modal-close { border: 1px solid var(--paap-border); background: var(--paap-panel); color: var(--paap-muted); border-radius: var(--paap-radius-sm); width: 28px; height: 28px; display: inline-flex; align-items: center; justify-content: center; cursor: pointer; }
.modal-close:hover { background: var(--paap-panel-subtle); color: var(--paap-text); }
.modal-body { padding: var(--paap-space-6); }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--paap-space-2); padding: var(--paap-space-4) var(--paap-space-6); border-top: 1px solid var(--paap-border); }
.form-item { display: grid; gap: 6px; margin-bottom: var(--paap-space-5); }
.form-item:last-child { margin-bottom: 0; }
.form-label { font-size: var(--paap-fs-label); color: var(--paap-muted); font-weight: 500; }
.required { color: var(--paap-danger); }
.form-helper { font-size: var(--paap-fs-label); color: var(--paap-muted); }
.confirm-text { color: var(--paap-text); font-size: var(--paap-fs-body); line-height: 1.6; margin: 0; }
.confirm-text + .form-error { margin-top: var(--paap-space-4); }
.rail-textarea { width: 100%; padding: 9px 12px; resize: vertical; border: 1px solid var(--paap-border); border-radius: var(--paap-radius-sm); background: var(--paap-panel); color: var(--paap-text); outline: none; font-family: inherit; font-size: var(--paap-fs-body); line-height: 1.5; }
.rail-textarea:focus { border-color: var(--paap-accent); box-shadow: var(--paap-focus-ring); }
.form-error { border: 1px solid var(--paap-danger); background: var(--paap-danger-soft); color: var(--paap-danger); border-radius: var(--paap-radius); padding: 10px 12px; font-size: var(--paap-fs-compact); line-height: 1.4; }
@media (max-width: 672px) {
  .rail-page { padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10); }
  .page-header, .app-card { flex-direction: column; align-items: flex-start; }
  .app-card-meta { width: 100%; justify-content: space-between; }
  .modal-footer { flex-direction: column; }
}
</style>
