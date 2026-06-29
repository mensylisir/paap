<template>
  <div class="rail-page">
    <div class="page-title-bar">
      <div class="title-group">
        <nav class="breadcrumb">
          <router-link to="/apps" class="breadcrumb-link">我的应用</router-link>
          <span class="breadcrumb-sep">/</span>
          <span class="breadcrumb-current">监控总览</span>
        </nav>
        <h1 class="page-title">监控总览</h1>
        <p class="page-desc">汇总所有环境的指标大盘、告警、目标和规则</p>
      </div>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">去环境管理</button>
    </div>

    <div v-if="loading" class="loading-wrap">
      <div class="loading-spinner" />
      <p class="loading-text">加载中...</p>
    </div>

    <div v-else-if="environments.length === 0" class="empty-card">
      <h3 class="empty-title">暂无环境</h3>
      <p class="empty-desc">创建环境并安装监控工具后可在这里查看监控数据。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">创建环境</button>
    </div>

    <div v-else-if="!monitorEnvs.length" class="empty-card">
      <h3 class="empty-title">未安装监控工具</h3>
      <p class="empty-desc">在环境中安装 Grafana + Prometheus 后可查看监控数据。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">安装监控</button>
    </div>

    <template v-else>
      <!-- Env tabs -->
      <div class="env-tabs">
        <button
          v-for="e in monitorEnvs"
          :key="e.env.id"
          class="env-tab"
          :class="{ active: activeEnvId === e.env.id }"
          @click="activeEnvId = e.env.id"
        >
          {{ e.env.name }}
          <span v-if="e.alertCount > 0" class="tab-badge" :class="e.alertCount > 0 ? 'red' : 'green'">{{ e.alertCount }}</span>
        </button>
      </div>

      <div v-if="activeEnv" class="workspace-content">
        <div v-if="activeEnv.grafanaUrl" class="access-bar">
          <a :href="activeEnv.grafanaUrl" target="_blank" rel="noreferrer" class="link external">打开 Grafana ↗</a>
        </div>
        <!-- Dashboards -->
        <div v-if="dashboards.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">📊</span> Grafana 大盘</h4>
            <span class="count">{{ dashboards.length }}</span>
          </div>
          <div class="card-grid">
            <a v-for="d in dashboards" :key="d.name" :href="d.externalUrl || '#'" target="_blank" rel="noreferrer" class="dash-card link-card">
              <div class="dash-name">{{ d.name }}</div>
              <div class="dash-meta">{{ d.description }}</div>
              <span class="badge green">{{ d.status }}</span>
            </a>
          </div>
        </div>

        <!-- Alerts -->
        <div v-if="alerts.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">🔔</span> 告警</h4>
            <span class="count" :class="alerts.length > 0 ? 'red' : ''">{{ alerts.length }}</span>
          </div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="a in alerts" :key="a.name">
                  <td class="cell-name">{{ a.name }}</td>
                  <td><span class="badge" :class="alertBadge(a.status)">{{ a.status }}</span></td>
                  <td class="cell-desc">{{ a.description }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-else class="section">
          <div class="section-head"><h4 class="section-title"><span class="icon">🔔</span> 告警</h4></div>
          <div class="ok-box">当前无活跃告警</div>
        </div>

        <!-- Targets -->
        <div v-if="targets.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">🎯</span> Prometheus 目标</h4>
            <span class="count">{{ targets.length }}</span>
          </div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="t in targets" :key="t.name">
                  <td class="cell-name">{{ t.name }}</td>
                  <td><span class="badge" :class="targetBadge(t.status)">{{ t.status }}</span></td>
                  <td class="cell-desc">{{ t.description }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Rules -->
        <div v-if="rules.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">📐</span> 规则</h4>
            <span class="count">{{ rules.length }}</span>
          </div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="r in rules" :key="r.name">
                  <td class="cell-name">{{ r.name }}</td>
                  <td><span class="badge" :class="targetBadge(r.status)">{{ r.status }}</span></td>
                  <td class="cell-desc">{{ r.description }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
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
const monitorEnvs = ref<any[]>([])
const loading = ref(true)
const activeEnvId = ref<number | null>(null)

const activeEnv = computed(() => monitorEnvs.value.find(e => e.env.id === activeEnvId.value))
const dashboards = computed(() => filterType(activeEnv.value?.workspace?.resources, 'Dashboard'))
const targets = computed(() => filterType(activeEnv.value?.workspace?.resources, 'Prometheus Target'))
const alerts = computed(() => filterType(activeEnv.value?.workspace?.resources, 'Alert'))
const rules = computed(() => filterType(activeEnv.value?.workspace?.resources, 'Rule'))

function filterType(resources: any[] | undefined, type: string): WorkspaceResource[] {
  if (!resources) return []
  return resources.filter((r: any) => r.type === type)
}

const alertBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('critical') || v.includes('firing')) return 'red'
  if (v.includes('warning') || v.includes('pending')) return 'orange'
  if (v.includes('resolved')) return 'green'
  return 'gray'
}
const targetBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('up') || v.includes('healthy')) return 'green'
  if (v.includes('down') || v.includes('error')) return 'red'
  return 'gray'
}

onMounted(async () => {
  try {
    const res = await api.listEnvs(appId)
    environments.value = res.data || []
    const envWorkspaces = []
    for (const env of environments.value) {
      try {
        const services = (await api.listServices(env.id)).data || []
        const monitor = services.find((s: any) => s.serviceType === 'monitor')
        if (monitor) {
          const wsRes = await api.getServiceWorkspace(env.id, monitor.id)
          const resources = wsRes.data?.resources || []
          const alertCount = resources.filter((r: any) => r.type === 'Alert' && String(r.status).toLowerCase().includes('firing')).length
          const grafanaUrl = `/api/v1/environments/${env.id}/services/${monitor.id}/proxy/`
          envWorkspaces.push({ env, workspace: wsRes.data, alertCount, grafanaUrl })
        }
      } catch { /* ignore per-env errors */ }
    }
    monitorEnvs.value = envWorkspaces
    if (envWorkspaces.length > 0) activeEnvId.value = envWorkspaces[0].env.id
  } finally {
    loading.value = false
  }
})

const goEnvs = () => router.push(`/apps/${appId}/environments`)
</script>

<style scoped>
.rail-page { padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10); max-width: none; }
.page-title-bar { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; gap: 16px; }
.title-group { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.breadcrumb { display: flex; align-items: center; gap: 6px; font-size: var(--paap-fs-compact); color: var(--paap-muted); margin-bottom: 2px; }
.breadcrumb-link { color: var(--paap-accent-01); text-decoration: none; }
.breadcrumb-sep { color: var(--paap-border-03); }
.breadcrumb-current { color: var(--paap-muted); }
.page-title { font-size: 24px; font-weight: 600; color: var(--paap-text); line-height: 1.2; letter-spacing: 0; margin: 0; }
.page-desc { font-size: var(--paap-fs-body); color: var(--paap-muted); line-height: 1.4; }

.loading-wrap { display: flex; flex-direction: column; align-items: center; padding: 80px 0; gap: 12px; }
.loading-spinner { width: 28px; height: 28px; border: 2px solid var(--paap-border); border-top-color: var(--paap-text); border-radius: 50%; animation: spin 0.8s linear infinite; }
.loading-text { color: var(--paap-muted); font-size: var(--paap-fs-body); }

.env-tabs { display: flex; gap: var(--paap-space-2); margin-bottom: var(--paap-space-5); flex-wrap: wrap; }
.env-tab { padding: var(--paap-space-2) var(--paap-space-4); border-radius: var(--paap-radius); border: 1px solid var(--paap-border); background: var(--paap-panel); font-size: var(--paap-fs-compact); font-weight: 600; color: var(--paap-muted); cursor: pointer; display: flex; align-items: center; gap: 6px; transition: background 0.15s, color 0.15s, border-color 0.15s; }
.env-tab:hover { background: var(--paap-bg-02); }
.env-tab.active { background: var(--paap-text); color: #fff; border-color: var(--paap-text); }
.tab-badge { font-size: 10px; padding: 1px 5px; border-radius: var(--paap-radius-full); background: var(--paap-panel-subtle); color: var(--paap-muted); }
.tab-badge.red { background: var(--paap-danger-bg); color: var(--paap-danger-text); }
.tab-badge.green { background: var(--paap-success-bg); color: var(--paap-success-text); }

.workspace-content { display: flex; flex-direction: column; gap: var(--paap-space-6); }
.section { background: var(--paap-panel); border: 1px solid var(--paap-border); border-radius: var(--paap-radius); padding: var(--paap-space-4) var(--paap-space-5); }
.section-head { display: flex; align-items: center; gap: 10px; margin-bottom: 14px; }
.section-title { font-size: 15px; font-weight: 600; color: var(--paap-text); margin: 0; display: flex; align-items: center; gap: 6px; }
.icon { font-size: 16px; }
.count { font-size: var(--paap-fs-label); font-weight: 600; color: #fff; background: var(--paap-muted); padding: 1px 7px; border-radius: var(--paap-radius-full); }
.count.red { background: var(--paap-red-bright); }

.access-bar { display: flex; justify-content: flex-end; }
.access-bar a { font-size: var(--paap-fs-compact); font-weight: 500; color: var(--paap-info-text); text-decoration: none; }
.access-bar a:hover { text-decoration: underline; }
.card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.dash-card { background: var(--paap-panel); border: 1px solid var(--paap-panel-subtle); border-radius: var(--paap-radius); padding: 14px 16px; }
.link-card { display: block; text-decoration: none; color: inherit; transition: box-shadow .15s, border-color .15s; }
.link-card:hover { border-color: var(--paap-border-03); box-shadow: var(--paap-shadow-md); }
.dash-name { font-weight: 600; font-size: var(--paap-fs-body); color: var(--paap-text); margin-bottom: 4px; }
.dash-meta { font-size: var(--paap-fs-label); color: var(--paap-muted); margin-bottom: 8px; }

.ok-box { padding: 14px 16px; background: var(--paap-success-bg); color: var(--paap-success-text); font-size: var(--paap-fs-compact); border-radius: var(--paap-radius); }

.table-wrap { overflow: hidden; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--paap-fs-compact); }
.data-table thead { background: var(--paap-panel); }
.data-table th { text-align: left; padding: 10px 14px; font-size: var(--paap-fs-small); font-weight: 600; color: var(--paap-muted); text-transform: uppercase; letter-spacing: 0.4px; border-bottom: 1px solid var(--paap-border); }
.data-table td { padding: 10px 14px; border-bottom: 1px solid var(--paap-panel-subtle); color: var(--paap-text); }
.cell-name { font-weight: 500; }
.cell-desc { color: var(--paap-muted); font-size: var(--paap-fs-label); }

.badge { display: inline-flex; align-items: center; padding: 2px 8px; border-radius: var(--paap-radius-xs); font-size: var(--paap-fs-small); font-weight: 600; background: var(--paap-panel-subtle); color: var(--paap-muted); }
.badge.green { background: var(--paap-success-bg); color: var(--paap-success-text); }
.badge.red { background: var(--paap-danger-bg); color: var(--paap-danger-text); }
.badge.orange { background: var(--paap-orange-bg); color: var(--paap-orange-text); }
.badge.gray { background: var(--paap-panel-subtle); color: var(--paap-muted); }

.rail-btn { display: inline-flex; align-items: center; justify-content: center; font-size: var(--paap-fs-compact); font-weight: 500; height: 36px; padding: 0 16px; border-radius: var(--paap-radius-sm); border: 1px solid var(--paap-border); background: var(--paap-panel); color: var(--paap-text); cursor: pointer; }
.rail-btn--primary { background: var(--paap-accent); color: #fff; border-color: var(--paap-accent); }
.rail-btn--primary:hover { background: var(--paap-accent-hover); border-color: var(--paap-accent-hover); }
</style>
