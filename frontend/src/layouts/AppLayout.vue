<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-top">
        <router-link to="/apps" class="logo">
          <div class="logo-icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/>
              <path d="M2 17l10 5 10-5"/>
              <path d="M2 12l10 5 10-5"/>
            </svg>
          </div>
          <span class="logo-text">PAAP</span>
        </router-link>
        <div class="app-divider" />
        <nav v-if="!isEnvContext" class="nav">
          <router-link :to="`/apps/${appId}/overview`" class="nav-item" :class="{ active: active === 'overview' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="7" height="7" rx="1"/>
              <rect x="14" y="3" width="7" height="7" rx="1"/>
              <rect x="14" y="14" width="7" height="7" rx="1"/>
              <rect x="3" y="14" width="7" height="7" rx="1"/>
            </svg>
            <span>概览</span>
          </router-link>
          <router-link :to="`/apps/${appId}/environments`" class="nav-item" :class="{ active: active === 'environments' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/>
              <polyline points="9 22 9 12 15 12 15 22"/>
            </svg>
            <span>环境</span>
          </router-link>
          <router-link :to="`/apps/${appId}/members`" class="nav-item" :class="{ active: active === 'members' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M16 21v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2"/>
              <circle cx="9.5" cy="7" r="4"/>
              <path d="M22 21v-2a4 4 0 0 0-3-3.9"/>
              <path d="M16 3.1a4 4 0 0 1 0 7.8"/>
            </svg>
            <span>成员</span>
          </router-link>
        </nav>
        <nav v-else class="nav env-nav" aria-label="环境菜单">
          <router-link
            v-for="item in primaryEnvMenuItems"
            :key="item.key"
            :to="envMenuLink(item.key)"
            class="nav-item"
            :class="{ active: activeEnvMenuKey === item.key }"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path :d="envMenuIconPath(item.key)" />
            </svg>
            <span>{{ item.label }}</span>
            <span v-if="item.count > 0" class="nav-count">{{ item.count }}</span>
          </router-link>
        </nav>
      </div>
      <div class="sidebar-bottom">
        <router-link v-if="isEnvContext" :to="`/apps/${appId}/environments`" class="nav-item back">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12"/>
            <polyline points="12 19 5 12 12 5"/>
          </svg>
          <span>返回环境列表</span>
        </router-link>
        <router-link v-else to="/apps" class="nav-item back">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12"/>
            <polyline points="12 19 5 12 12 5"/>
          </svg>
          <span>返回应用列表</span>
        </router-link>
      </div>
    </aside>
    <main class="main">
      <div class="workspace-context context-bar workspace-switcher">
        <div class="context-menu-wrap">
          <button type="button" class="context-switcher-button app-context-switcher" @click="toggleAppSwitcher">
            <span class="context-avatar">{{ contextInitial(currentAppName) }}</span>
            <span class="context-copy">
              <span class="context-kicker">应用</span>
              <strong>{{ currentAppName }}</strong>
            </span>
            <svg class="context-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M6 9l6 6 6-6"/>
            </svg>
          </button>
          <div v-if="showAppSwitcher" class="context-popover app-context-popover">
            <button
              v-for="item in apps"
              :key="item.id"
              type="button"
              class="context-option"
              :class="{ active: Number(item.id) === appId }"
              @click="goToSelectedApp(Number(item.id))"
            >
              <span class="context-avatar small">{{ contextInitial(item.name) }}</span>
              <span class="context-option-copy">
                <strong>{{ item.name }}</strong>
                <small>{{ item.identifier || '未设置标识' }} · {{ appEnvironmentCount(item) }} 环境</small>
              </span>
            </button>
            <router-link to="/apps" class="context-option muted" @click="closeSwitchers">
              <span class="context-option-copy">
                <strong>查看全部应用</strong>
                <small>返回应用外菜单</small>
              </span>
            </router-link>
          </div>
        </div>

        <span v-if="isEnvContext || environments.length" class="context-divider">/</span>

        <div v-if="isEnvContext || environments.length" class="context-menu-wrap">
          <button type="button" class="context-switcher-button env-context-switcher" @click="toggleEnvSwitcher">
            <span class="env-status-dot" :class="currentEnvStatus"></span>
            <span class="context-copy">
              <span class="context-kicker">环境</span>
              <strong>{{ currentEnvName }}</strong>
            </span>
            <svg class="context-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M6 9l6 6 6-6"/>
            </svg>
          </button>
          <div v-if="showEnvSwitcher" class="context-popover env-context-popover">
            <button
              v-for="item in environments"
              :key="item.id"
              type="button"
              class="context-option"
              :class="{ active: Number(item.id) === envId }"
              @click="goToSelectedEnv(Number(item.id))"
            >
              <span class="env-status-dot" :class="effectiveEnvironmentStatus(item)"></span>
              <span class="context-option-copy">
                <strong>{{ item.name }}</strong>
                <small>{{ item.identifier || '未设置标识' }} · {{ envStatusText(effectiveEnvironmentStatus(item)) }}</small>
              </span>
            </button>
            <router-link :to="`/apps/${appId}/environments`" class="context-option muted" @click="closeSwitchers">
              <span class="context-option-copy">
                <strong>查看环境列表</strong>
                <small>管理当前应用的环境</small>
              </span>
            </router-link>
          </div>
        </div>
      </div>
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { effectiveEnvironmentStatus, environmentStatusLabel } from '../views/appSummary'

const route = useRoute()
const router = useRouter()
const appId = ref(0)
const appName = ref('')
const active = ref('overview')
const env = ref<any>(null)
const apps = ref<any[]>([])
const environments = ref<any[]>([])
const components = ref<any[]>([])

const envId = computed(() => Number(route.params.envId || 0))
const isEnvContext = computed(() => envId.value > 0)
const envName = computed(() => env.value?.name || env.value?.identifier || '')
const showAppSwitcher = ref(false)
const showEnvSwitcher = ref(false)
const currentApp = computed(() => apps.value.find((item:any) => Number(item.id) === appId.value) || null)
const currentEnv = computed(() => {
  const listed = environments.value.find((item:any) => Number(item.id) === envId.value)
  if (listed || env.value) return { ...(listed || {}), ...(env.value || {}) }
  return environments.value[0] || null
})
const currentAppName = computed(() => currentApp.value?.name || appName.value || '应用')
const currentEnvName = computed(() => currentEnv.value?.name || envName.value || '环境')
const currentEnvStatus = computed(() => effectiveEnvironmentStatus(currentEnv.value))
const primaryEnvMenuItems = computed(() => [
  { key: 'overview', label: '概览', count: 0 },
  { key: 'components', label: '组件', count: components.value.length },
])

const contextInitial = (value: string) => String(value || 'P').trim().slice(0, 1).toUpperCase()
const appEnvironmentCount = (app:any) => Number(app?.environmentCount ?? app?.environments?.length ?? 0)
const normalizeListPayload = (payload: any) => Array.isArray(payload) ? payload : Array.isArray(payload?.data) ? payload.data : []
const envStatusText = (status?: string) => environmentStatusLabel(status)
const closeSwitchers = () => {
  showAppSwitcher.value = false
  showEnvSwitcher.value = false
}
const toggleAppSwitcher = () => {
  showAppSwitcher.value = !showAppSwitcher.value
  showEnvSwitcher.value = false
}
const toggleEnvSwitcher = () => {
  showEnvSwitcher.value = !showEnvSwitcher.value
  showAppSwitcher.value = false
}

watch(() => route.params.id, async (id) => {
  if (id) {
    appId.value = Number(id)
    try {
      const [appRes, appsRes, envsRes] = await Promise.all([
        api.getApp(appId.value),
        api.listApps(),
        api.listEnvs(appId.value),
      ])
      const appPayload = appRes.data?.application || appRes.data
      const appEnvironments = appRes.data?.environments || appRes.environments || appPayload?.environments
      appName.value = appPayload?.name || '应用'
      apps.value = Array.isArray(appsRes.data) ? appsRes.data : appsRes.data?.applications || []
      const fallbackEnvironments = normalizeListPayload(envsRes.data)
      environments.value = Array.isArray(appEnvironments) && appEnvironments.length ? appEnvironments : fallbackEnvironments
    }
    catch (e) {}
  }
}, { immediate: true })

const loadEnvContext = async () => {
  if (!envId.value) {
    env.value = null
    components.value = []
    return
  }
  try {
    const envRes = await api.getEnv(envId.value)
    const envPayload = envRes.data?.environment || envRes.data
    components.value = envRes.data?.components || []
    const serviceList = envRes.data?.services || []
    env.value = {
      ...envPayload,
      componentCount: components.value.length,
      services: serviceList,
      toolCount: serviceList.length,
    }
  } catch (e) {
    console.warn('environment menu fallback:', e)
  }
}

watch(() => route.params.envId, () => {
  void loadEnvContext()
}, { immediate: true })

const handleEnvUpdated = (event: Event) => {
  const detail = (event as CustomEvent).detail || {}
  if (!detail.envId || Number(detail.envId) === envId.value) {
    void loadEnvContext()
  }
  if (appId.value) {
    api.getApp(appId.value)
      .then((res:any) => {
        const appPayload = res.data?.application || res.data
        const appEnvironments = res.data?.environments || res.environments || appPayload?.environments
        environments.value = Array.isArray(appEnvironments) ? appEnvironments : []
      })
      .catch(() => {})
  }
}

onMounted(() => window.addEventListener('paap-env-updated', handleEnvUpdated))
onUnmounted(() => window.removeEventListener('paap-env-updated', handleEnvUpdated))

watch(() => route.path, (path) => {
  if (path.includes('/members')) active.value = 'members'
  else if (path.includes('/environments')) active.value = 'environments'
  else active.value = 'overview'
  closeSwitchers()
}, { immediate: true })

const envMenuLink = (key: string) => {
  const path = `/apps/${appId.value}/environments/${envId.value}`
  return key === 'overview' ? path : { path, query: { tab: key } }
}

const goToSelectedApp = (nextAppId: number) => {
  if (!nextAppId) return
  closeSwitchers()
  router.push(`/apps/${nextAppId}/overview`)
}

const goToSelectedEnv = (nextEnvId: number) => {
  closeSwitchers()
  if (nextEnvId) router.push(`/apps/${appId.value}/environments/${nextEnvId}`)
}

const normalizeEnvMenuKey = (key: string) => {
  if (key === 'overview' || key === 'components') return key
  return ''
}

const activeEnvMenuKey = computed(() => {
  if (route.params.compId) return 'components'
  return normalizeEnvMenuKey(String(route.query.tab || 'overview'))
})

const envMenuIconPath = (key: string) => {
  const paths: Record<string, string> = {
    overview: 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z',
    components: 'M4 4h7v7H4V4zm9 0h7v7h-7V4zM4 13h7v7H4v-7zm9 0h7v7h-7v-7z',
    tools: 'M4 6h16v3H4V6zm2 5h12v3H6v-3zm3 5h6v3H9v-3z',
    'code-repository': 'M6 6a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm0 4v6a2 2 0 0 0 2 2h2a2 2 0 1 1 0 2H8a4 4 0 0 1-4-4v-6h2zm8-6a2 2 0 1 1 0 4 2 2 0 0 1 0-4z',
    'image-registry': 'M5 4h14v5H5V4zm0 7h14v9H5v-9zm3 3v2h8v-2H8z',
    'continuous-integration': 'M12 3l2 4 4 .5-3 3 .8 4.2L12 13l-3.8 1.7.8-4.2-3-3 4-.5 2-4zm-7 16h14v2H5v-2z',
    'continuous-deployment': 'M12 3l8 6-8 6-8-6 8-6zm-6 11l6 4 6-4v3l-6 4-6-4v-3z',
    'monitoring-center': 'M3 18h18v2H3v-2zm2-2V8h3v8H5zm6 0V4h3v12h-3zm6 0v-6h3v6h-3z',
    'logging-center': 'M5 4h14v16H5V4zm3 4h8V6H8v2zm0 4h10v-2H8v2zm0 4h10v-2H8v2z',
    databases: 'M12 3c4.4 0 8 1.3 8 3s-3.6 3-8 3-8-1.3-8-3 3.6-3 8-3zm-8 6c0 1.7 3.6 3 8 3s8-1.3 8-3v4c0 1.7-3.6 3-8 3s-8-1.3-8-3V9zm0 6c0 1.7 3.6 3 8 3s8-1.3 8-3v3c0 1.7-3.6 3-8 3s-8-1.3-8-3v-3z',
    cache: 'M12 3 3 7l9 4 9-4-9-4zm-7 8 7 3 7-3v2l-7 3-7-3v-2zm0 4 7 3 7-3v2l-7 3-7-3v-2z',
    'message-queue': 'M4 6h16v12H8l-4 4V6zm2 2v10l1.2-1.2H18V8H6zm3 2h2v2H9v-2zm4 0h2v2h-2v-2z',
    middleware: 'M4 6h16v4H4V6zm0 7h7v7H4v-7zm9 0h7v7h-7v-7z',
    'object-storage': 'M6 5h12l3 5v9H3v-9l3-5zm0 5h12l-1.2-2H7.2L6 10zm1 4h10v2H7v-2z',
  }
  return paths[key] || 'M12 3L4 7v10l8 4 8-4V7l-8-4z'
}
</script>

<style scoped>
.layout {
  display: flex;
  min-height: 100vh;
  min-width: 0;
  background: var(--paap-bg);
}

.sidebar {
  width: var(--paap-sidebar);
  flex-shrink: 0;
  background: var(--paap-panel);
  border-right: 1px solid var(--paap-border);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: var(--paap-space-5) 0;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 100;
}

.sidebar-top {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-2);
}

.logo {
  padding: 0 var(--paap-space-4) var(--paap-space-4);
  text-decoration: none;
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
}

.logo-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--cds-background-brand, var(--paap-accent));
  color: var(--cds-icon-on-color, #fff);
  border-radius: var(--paap-radius-xs);
}

.logo-text {
  font-size: 16px;
  font-weight: 700;
  color: var(--paap-accent);
  letter-spacing: -0.02em;
  font-family: var(--paap-mono);
}

.app-divider {
  margin: var(--paap-space-1) var(--paap-space-4) var(--paap-space-3);
  height: 1px;
  background: var(--paap-border);
}

.context-bar {
  display: flex;
  gap: var(--paap-space-2);
}

.workspace-context {
  width: 100%;
  max-width: var(--paap-content-max);
  margin: 0;
  padding: var(--paap-space-4) var(--paap-space-5) 0;
  align-items: center;
}

.workspace-switcher {
  min-height: 44px;
}

.context-menu-wrap {
  position: relative;
  min-width: 0;
}

.context-switcher-button {
  display: inline-flex;
  align-items: center;
  gap: var(--paap-space-2);
  min-width: 180px;
  max-width: 320px;
  height: 40px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.context-switcher-button:hover {
  border-color: var(--paap-border-strong);
  background: var(--paap-panel-subtle);
}

.context-switcher-button:focus-visible {
  outline: none;
  border-color: var(--paap-accent);
  box-shadow: 0 0 0 3px rgba(37,99,235,0.1);
}

.context-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border-radius: var(--paap-radius-xs);
  background: var(--cds-background-brand, var(--paap-accent));
  color: var(--cds-icon-on-color, #fff);
  font-size: 12px;
  font-weight: 700;
}

.context-avatar.small {
  width: 22px;
  height: 22px;
  font-size: 11px;
}

.context-copy,
.context-option-copy {
  display: grid;
  min-width: 0;
  text-align: left;
}

.context-kicker {
  color: var(--paap-muted-2);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  text-transform: uppercase;
}

.context-copy strong,
.context-option-copy strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: 13px;
  font-weight: 650;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.context-option-copy small {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-muted);
  font-size: 11px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.context-chevron {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--paap-muted-2);
}

.context-divider {
  color: var(--paap-muted-2);
  font-size: 18px;
  line-height: 1;
}

.context-popover {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 500;
  display: grid;
  gap: 2px;
  width: min(320px, calc(100vw - 32px));
  max-height: 360px;
  overflow-y: auto;
  padding: var(--paap-space-2);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  box-shadow: none;
}

.context-option {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  width: 100%;
  min-height: 42px;
  padding: 8px 10px;
  border: 0;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  color: inherit;
  font-family: inherit;
  text-decoration: none;
  cursor: pointer;
}

.context-option:hover,
.context-option.active {
  background: var(--paap-panel-subtle);
}

.context-option.active .context-option-copy strong {
  color: var(--paap-accent);
}

.context-option.muted {
  border-top: 1px solid var(--paap-border);
  border-radius: 0 0 var(--paap-radius-xs) var(--paap-radius-xs);
  margin-top: 4px;
}

.env-status-dot {
  width: 9px;
  height: 9px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--paap-border-strong);
}

.env-status-dot.running { background: var(--paap-success); }
.env-status-dot.creating { background: var(--paap-accent); }
.env-status-dot.failed,
.env-status-dot.error { background: var(--paap-danger); }
.env-status-dot.empty,
.env-status-dot.stopped { background: var(--paap-muted-2); }

.app-name {
  display: block;
  padding: 0 var(--paap-space-4) var(--paap-space-3);
  font-size: 13px;
  font-weight: 600;
  color: var(--paap-muted);
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color 0.15s;
}
.app-name:hover {
  color: var(--paap-text);
}

.env-name {
  display: block;
  margin: 0 var(--paap-space-4) var(--paap-space-3);
  padding: var(--paap-space-2) var(--paap-space-3);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav {
  display: flex;
  flex-direction: column;
  padding: 0 var(--paap-space-3);
  gap: 2px;
}

.env-nav {
  gap: var(--paap-space-2);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  height: 40px;
  padding: 0 var(--paap-space-3);
  border-radius: var(--paap-radius-sm);
  font-size: 14px;
  font-weight: 500;
  color: var(--paap-muted);
  text-decoration: none;
  transition: all 0.15s ease;
  cursor: pointer;
  background: transparent;
  border: none;
  width: 100%;
}

.nav-item span:first-of-type {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nav-count {
  margin-left: auto;
  min-width: 18px;
  height: 18px;
  padding: 0 6px;
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: 11px;
  line-height: 18px;
  text-align: center;
}

.nav-item.active .nav-count {
  background: #fff;
  color: var(--paap-text);
}

.nav-item:hover {
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
}

.nav-item.active {
  background: var(--paap-panel-subtle);
  color: var(--cds-icon-interactive, var(--paap-accent));
  font-weight: 600;
}

.sidebar-bottom {
  padding: 0 var(--paap-space-3);
}

.nav-item.back {
  color: var(--paap-muted-2);
  font-size: 13px;
}
.nav-item.back:hover {
  color: var(--paap-muted);
}

.main {
  flex: 1;
  margin-left: var(--paap-sidebar);
  width: calc(100% - var(--paap-sidebar));
  min-width: 0;
  background: transparent;
  min-height: 100vh;
}

@media (max-width: 768px) {
  .layout { display: block; }
  .sidebar {
    position: sticky;
    width: 100%;
    height: auto;
    bottom: auto;
    padding: var(--paap-space-3) var(--paap-space-4);
  }
  .sidebar-top { gap: var(--paap-space-2); }
  .app-divider, .sidebar-bottom { display: none; }
  .logo { padding: 0; }
  .workspace-context {
    width: 100%;
    padding: var(--paap-space-3) var(--paap-space-4) 0;
    overflow-x: auto;
  }
  .context-switcher-button { min-width: 160px; max-width: 240px; }
  .context-popover { position: fixed; left: var(--paap-space-4); right: var(--paap-space-4); width: auto; }
  .app-name { padding: 0 0 var(--paap-space-2); }
  .env-name { margin: 0; }
  .nav {
    flex-direction: row;
    padding: 0;
    overflow-x: auto;
    gap: var(--paap-space-1);
  }
  .nav-item { width: auto; white-space: nowrap; height: 36px; }
  .main {
    width: 100%;
    margin-left: 0;
  }
}
</style>
