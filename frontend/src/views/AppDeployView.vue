<template>
  <div class="rail-page">
    <div class="page-title-bar">
      <div class="title-group">
        <nav class="breadcrumb">
          <router-link to="/apps" class="breadcrumb-link">我的应用</router-link>
          <span class="breadcrumb-sep">/</span>
          <span class="breadcrumb-current">部署总览</span>
        </nav>
        <h1 class="page-title">部署总览</h1>
        <p class="page-desc">汇总所有环境的 ArgoCD Application 同步与健康状态</p>
      </div>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">去环境管理</button>
    </div>

    <div v-if="loading" class="loading-wrap">
      <div class="loading-spinner" />
      <p class="loading-text">加载中...</p>
    </div>

    <div v-else-if="environments.length === 0" class="empty-card">
      <h3 class="empty-title">暂无环境</h3>
      <p class="empty-desc">创建环境并安装 deploy 工具后可查看部署状态。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">创建环境</button>
    </div>

    <div v-else-if="!deployEnvs.length" class="empty-card">
      <h3 class="empty-title">未安装部署工具</h3>
      <p class="empty-desc">在环境中安装 ArgoCD 后可查看部署状态。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">安装 deploy</button>
    </div>

    <template v-else>
      <div class="env-tabs">
        <button v-for="e in deployEnvs" :key="e.env.id" class="env-tab" :class="{ active: activeEnvId === e.env.id }" @click="activeEnvId = e.env.id">
          {{ e.env.name }}
          <span v-if="e.unsyncedCount > 0" class="tab-badge red">{{ e.unsyncedCount }}</span>
        </button>
      </div>

      <div v-if="activeEnv" class="workspace-content">
        <div class="summary-bar" v-if="apps.length">
          <div class="sum-item"><div class="sum-num">{{ apps.length }}</div><div class="sum-label">应用</div></div>
          <div class="sum-item"><div class="sum-num green">{{ syncedCount }}</div><div class="sum-label">已同步</div></div>
          <div class="sum-item"><div class="sum-num red">{{ unhealthyCount }}</div><div class="sum-label">异常</div></div>
        </div>

        <div v-if="apps.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">🚀</span> ArgoCD Applications</h4>
            <span class="count">{{ apps.length }}</span>
          </div>
          <div class="app-list">
            <div v-for="app in apps" :key="app.name" class="app-card">
              <div class="app-header">
                <div class="app-title">{{ app.name }}</div>
                <div class="app-badges">
                  <span class="badge" :class="syncBadge(app.status)">{{ syncPart(app.status) }}</span>
                  <span class="badge" :class="healthBadge(app.status)">{{ healthPart(app.status) }}</span>
                </div>
              </div>
              <div class="app-desc">{{ app.description || '-' }}</div>
              <div v-if="app.children && app.children.length" class="app-topo">
                <div class="topo-header" @click="toggleTopo(app.name)">
                  <span>资源拓扑 ({{ app.children.length }})</span>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :style="{ transform: topoOpen[app.name] ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform .2s' }"><polyline points="6 9 12 15 18 9"/></svg>
                </div>
                <div v-show="topoOpen[app.name]" class="topo-body">
                  <div class="topo-children">
                    <div v-for="(r, idx) in app.children" :key="r.name + r.type" class="topo-row">
                      <div class="topo-conn" :class="{ last: idx === app.children.length - 1 }"><div class="tc-v" /><div class="tc-h" /></div>
                      <div class="topo-node child">
                        <span class="node-kind">{{ r.type }}</span>
                        <span class="node-name">{{ r.name }}</span>
                        <span class="node-health" :class="healthClass(r.annotations?.health)">{{ r.annotations?.health || r.status || 'Unknown' }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="ok-box">该环境暂无 ArgoCD Application</div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { WorkspaceResource } from './serviceWorkspace'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)

const environments = ref<any[]>([])
const deployEnvs = ref<any[]>([])
const loading = ref(true)
const activeEnvId = ref<number | null>(null)

const activeEnv = computed(() => deployEnvs.value.find(e => e.env.id === activeEnvId.value))
const apps = computed(() => {
  const r = activeEnv.value?.workspace?.resources || []
  return r.filter((x: any) => x.type === 'Application')
})
const syncedCount = computed(() => apps.value.filter((a: any) => String(a.status).toLowerCase().includes('synced')).length)
const unhealthyCount = computed(() => apps.value.filter((a: any) => {
  const s = String(a.status).toLowerCase()
  return s.includes('degraded') || s.includes('error') || s.includes('unknown')
}).length)

const syncPart = (s?: string) => {
  const parts = String(s || '').split('/')
  return parts[0] || '-'
}
const healthPart = (s?: string) => {
  const parts = String(s || '').split('/')
  return parts[1] || '-'
}
const syncBadge = (s?: string) => {
  const v = syncPart(s).toLowerCase()
  if (v.includes('synced')) return 'green'
  if (v.includes('outofsync') || v.includes('error')) return 'red'
  return 'gray'
}
const healthBadge = (s?: string) => {
  const v = healthPart(s).toLowerCase()
  if (v.includes('healthy')) return 'green'
  if (v.includes('degraded') || v.includes('error')) return 'red'
  return 'gray'
}
const healthClass = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('healthy')) return 'green'
  if (v.includes('degraded') || v.includes('error')) return 'red'
  return 'gray'
}

const topoOpen = ref<Record<string, boolean>>({})
const toggleTopo = (name: string) => {
  topoOpen.value[name] = !topoOpen.value[name]
}

onMounted(async () => {
  try {
    const res = await api.listEnvs(appId)
    environments.value = res.data || []
    const list = []
    for (const env of environments.value) {
      try {
        const services = (await api.listServices(env.id)).data || []
        const deploy = services.find((s: any) => s.serviceType === 'deploy')
        if (deploy) {
          const wsRes = await api.getServiceWorkspace(env.id, deploy.id)
          const resources: WorkspaceResource[] = wsRes.data?.resources || []
          const apps = resources.filter((r: any) => r.type === 'Application')
          const unsynced = apps.filter((a: any) => !String(a.status).toLowerCase().includes('synced')).length
          list.push({ env, workspace: wsRes.data, unsyncedCount: unsynced })
        }
      } catch { }
    }
    deployEnvs.value = list
    if (list.length > 0) activeEnvId.value = list[0].env.id
  } finally {
    loading.value = false
  }
})

const goEnvs = () => router.push(`/apps/${appId}/environments`)
</script>

<style scoped>
.rail-page { padding: 20px 20px 36px; max-width: none; }
.page-title-bar { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; gap: 16px; }
.title-group { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.breadcrumb { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--paap-text-02); margin-bottom: 2px; }
.breadcrumb-link { color: var(--paap-accent-01); text-decoration: none; }
.breadcrumb-sep { color: var(--paap-border-03); }
.breadcrumb-current { color: var(--paap-text-02); }
.page-title { font-size: 22px; font-weight: 600; color: var(--paap-text-01); line-height: 1.2; letter-spacing: 0; margin: 0; }
.page-desc { font-size: 14px; color: var(--paap-text-02); line-height: 1.4; }

.loading-wrap { display: flex; flex-direction: column; align-items: center; padding: 80px 0; gap: 12px; }
.loading-spinner { width: 28px; height: 28px; border: 2px solid var(--paap-border-01); border-top-color: var(--paap-text-01); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.loading-text { color: var(--paap-text-02); font-size: 14px; }

.empty-card { background: var(--paap-white); border: 1px solid var(--paap-border-01); border-radius: 8px; padding: 64px 32px; text-align: center; }
.empty-title { font-size: 18px; font-weight: 600; color: var(--paap-text-01); margin-bottom: 8px; }
.empty-desc { color: var(--paap-text-02); max-width: 520px; margin: 0 auto 20px; line-height: 1.5; font-size: 14px; }

.env-tabs { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
.env-tab { padding: 8px 16px; border-radius: 8px; border: 1px solid var(--paap-border-01); background: #fff; font-size: 13px; font-weight: 600; color: var(--paap-text-02); cursor: pointer; display: flex; align-items: center; gap: 6px; }
.env-tab:hover { background: var(--paap-bg-02); }
.env-tab.active { background: var(--paap-text-01); color: #fff; border-color: var(--paap-text-01); }
.tab-badge { font-size: 10px; padding: 1px 5px; border-radius: 10px; background: var(--paap-bg-01); color: var(--paap-text-02); }
.tab-badge.red { background: var(--paap-danger-bg); color: var(--paap-danger-text); }

.workspace-content { display: flex; flex-direction: column; gap: 24px; }
.summary-bar { display: flex; gap: 24px; background: #fff; border: 1px solid var(--paap-border-01); border-radius: 8px; padding: 18px 20px; }
.sum-item { text-align: center; min-width: 80px; }
.sum-num { font-size: 22px; font-weight: 700; color: var(--paap-text-01); }
.sum-num.green { color: var(--paap-success-text); }
.sum-num.red { color: var(--paap-danger-text); }
.sum-label { font-size: 12px; color: var(--paap-text-02); margin-top: 2px; }

.section { background: #fff; border: 1px solid var(--paap-border-01); border-radius: 8px; padding: 18px 20px; }
.section-head { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.section-title { font-size: 15px; font-weight: 600; color: var(--paap-text-01); margin: 0; display: flex; align-items: center; gap: 6px; }
.icon { font-size: 16px; }
.count { font-size: 12px; font-weight: 600; color: #fff; background: var(--paap-text-02); padding: 1px 7px; border-radius: 10px; }

.table-wrap { overflow: hidden; }
.data-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.data-table thead { background: var(--paap-bg-03); }
.data-table th { text-align: left; padding: 10px 14px; font-size: 11px; font-weight: 600; color: var(--paap-text-03); text-transform: uppercase; letter-spacing: 0.4px; border-bottom: 1px solid var(--paap-border-01); }
.data-table td { padding: 10px 14px; border-bottom: 1px solid var(--paap-bg-01); color: var(--paap-text-01); }
.cell-name { font-weight: 500; }
.cell-desc { color: var(--paap-text-02); font-size: 12px; }

.badge { display: inline-flex; align-items: center; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; background: var(--paap-bg-01); color: var(--paap-text-02); }
.badge.green { background: var(--paap-success-bg); color: var(--paap-success-text); }
.badge.red { background: var(--paap-danger-bg); color: var(--paap-danger-text); }
.badge.gray { background: var(--paap-bg-01); color: var(--paap-text-02); }

.ok-box { padding: 14px 16px; background: var(--paap-bg-03); color: var(--paap-text-02); font-size: 13px; border-radius: 8px; border: 1px solid var(--paap-border-01); }

.app-list { display: flex; flex-direction: column; gap: 12px; }
.app-card { background: #fff; border: 1px solid var(--paap-border-01); border-radius: 8px; padding: 16px; }
.app-header { display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 6px; }
.app-title { font-size: 15px; font-weight: 600; color: var(--paap-text-01); }
.app-badges { display: flex; gap: 6px; }
.app-desc { font-size: 12px; color: var(--paap-text-02); margin-bottom: 10px; }

.app-topo { border: 1px solid var(--paap-bg-01); border-radius: 6px; overflow: hidden; }
.topo-header { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; background: var(--paap-bg-03); font-size: 12px; font-weight: 600; color: var(--paap-text-02); cursor: pointer; }
.topo-body { padding: 12px; }
.topo-children { display: flex; flex-direction: column; padding-left: 20px; }
.topo-row { display: flex; align-items: flex-start; }
.topo-conn { position: relative; width: 24px; height: 36px; flex-shrink: 0; }
.tc-v { position: absolute; left: 0; top: 0; width: 1px; height: 100%; background: var(--paap-border-01); }
.tc-h { position: absolute; left: 0; top: 18px; width: 16px; height: 1px; background: var(--paap-border-01); }
.topo-conn.last .tc-v { height: 18px; }
.topo-node.child { display: inline-flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 6px; border: 1px solid var(--paap-border-01); background: #fff; font-size: 12px; margin-top: 2px; }
.node-kind { font-size: 10px; text-transform: uppercase; letter-spacing: 0.4px; padding: 1px 5px; border-radius: 4px; background: var(--paap-bg-01); color: var(--paap-text-02); font-weight: 600; flex-shrink: 0; }
.node-name { font-weight: 500; color: var(--paap-text-01); }
.node-health { font-size: 11px; padding: 1px 5px; border-radius: 4px; background: var(--paap-bg-01); color: var(--paap-text-02); font-weight: 600; }
.node-health.green { background: var(--paap-success-bg); color: var(--paap-success-text); }
.node-health.red { background: var(--paap-danger-bg); color: var(--paap-danger-text); }

.rail-btn { display: inline-flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 500; height: 36px; padding: 0 16px; border-radius: var(--paap-radius-sm); border: 1px solid var(--paap-border-01); background: #fff; color: var(--paap-text-01); cursor: pointer; }
.rail-btn--primary { background: var(--cds-button-primary, var(--paap-accent)); color: var(--cds-text-on-color, #fff); border-color: var(--cds-button-primary, var(--paap-accent)); }
.rail-btn--primary:hover { background: var(--cds-button-primary-hover, var(--paap-accent-hover)); border-color: var(--cds-button-primary-hover, var(--paap-accent-hover)); }
</style>
