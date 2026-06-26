<template>
  <div class="rail-page">
    <div class="page-title-bar">
      <div class="title-group">
        <nav class="breadcrumb">
          <router-link to="/apps" class="breadcrumb-link">我的应用</router-link>
          <span class="breadcrumb-sep">/</span>
          <span class="breadcrumb-current">CI 总览</span>
        </nav>
        <h1 class="page-title">CI 总览</h1>
        <p class="page-desc">汇总所有环境的 Jenkins 流水线与构建状态</p>
      </div>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">去环境管理</button>
    </div>

    <div v-if="loading" class="loading-wrap">
      <div class="loading-spinner" />
      <p class="loading-text">加载中...</p>
    </div>

    <div v-else-if="environments.length === 0" class="empty-card">
      <h3 class="empty-title">暂无环境</h3>
      <p class="empty-desc">创建环境并安装 CI 工具后可查看流水线。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">创建环境</button>
    </div>

    <div v-else-if="!ciEnvs.length" class="empty-card">
      <h3 class="empty-title">未安装 CI 工具</h3>
      <p class="empty-desc">在环境中安装 Jenkins 后可查看流水线状态。</p>
      <button class="rail-btn rail-btn--primary" @click="goEnvs">安装 CI</button>
    </div>

    <template v-else>
      <div class="env-tabs">
        <button v-for="e in ciEnvs" :key="e.env.id" class="env-tab" :class="{ active: activeEnvId === e.env.id }" @click="activeEnvId = e.env.id">
          {{ e.env.name }}
          <span v-if="e.jobCount > 0" class="tab-badge">{{ e.jobCount }}</span>
        </button>
      </div>

      <div v-if="activeEnv" class="workspace-content">
        <div v-if="triggerError" class="error-box" role="alert">{{ triggerError }}</div>
        <div v-if="jobs.length" class="section">
          <div class="section-head">
            <h4 class="section-title"><span class="icon">🔧</span> 流水线</h4>
            <span class="count">{{ jobs.length }}</span>
          </div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>名称</th><th>状态</th><th>说明</th><th style="width: 90px;">操作</th></tr></thead>
              <tbody>
                <tr v-for="job in jobs" :key="job.name">
                  <td class="cell-name">{{ job.name }}</td>
                  <td><span class="badge" :class="jobBadge(job.status)">{{ job.status }}</span></td>
                  <td class="cell-desc">{{ job.description }}</td>
                  <td>
                    <button class="act-btn primary" :disabled="triggering[job.name]" @click="triggerBuild(job.name)">
                      {{ triggering[job.name] ? '触发中...' : '构建' }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-else class="ok-box">该环境暂无 Jenkins Job</div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)

const environments = ref<any[]>([])
const ciEnvs = ref<any[]>([])
const loading = ref(true)
const activeEnvId = ref<number | null>(null)

const activeEnv = computed(() => ciEnvs.value.find(e => e.env.id === activeEnvId.value))
const jobs = computed(() => {
  const r = activeEnv.value?.workspace?.resources || []
  return r.filter((x: any) => x.type === 'Job' || x.type === 'Pipeline')
})

const jobBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('success') || v.includes('ready')) return 'green'
  if (v.includes('fail') || v.includes('error')) return 'red'
  if (v.includes('run') || v.includes('progress')) return 'blue'
  return 'gray'
}

const triggering = ref<Record<string, boolean>>({})
const triggerError = ref('')

async function triggerBuild(jobName: string) {
  const env = activeEnv.value
  if (!env || !env.ciServiceId) return
  triggerError.value = ''
  triggering.value[jobName] = true
  try {
    await api.runServiceWorkspaceAction(env.env.id, env.ciServiceId, 'trigger_jenkins_build', jobName)
    // refresh workspace after trigger
    const wsRes = await api.getServiceWorkspace(env.env.id, env.ciServiceId)
    env.workspace = wsRes.data
    const resources = wsRes.data?.resources || []
    env.jobCount = resources.filter((r: any) => r.type === 'Job' || r.type === 'Pipeline').length
  } catch (e: any) {
    triggerError.value = '触发失败：' + (e?.message || '未知错误')
  } finally {
    triggering.value[jobName] = false
  }
}

onMounted(async () => {
  try {
    const res = await api.listEnvs(appId)
    environments.value = res.data || []
    const list = []
    for (const env of environments.value) {
      try {
        const services = (await api.listServices(env.id)).data || []
        const ci = services.find((s: any) => s.serviceType === 'ci')
        if (ci) {
          const wsRes = await api.getServiceWorkspace(env.id, ci.id)
          const resources = wsRes.data?.resources || []
          const jc = resources.filter((r: any) => r.type === 'Job' || r.type === 'Pipeline').length
          list.push({ env, workspace: wsRes.data, jobCount: jc, ciServiceId: ci.id })
        }
      } catch { }
    }
    ciEnvs.value = list
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

.workspace-content { display: flex; flex-direction: column; gap: 24px; }
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
.badge.blue { background: var(--paap-info-bg); color: var(--paap-info-text); }
.badge.gray { background: var(--paap-bg-01); color: var(--paap-text-02); }

.ok-box { padding: 14px 16px; background: var(--paap-bg-03); color: var(--paap-text-02); font-size: 13px; border-radius: 8px; border: 1px solid var(--paap-border-01); }
.error-box { border: 1px solid var(--paap-danger-border); background: var(--paap-danger-bg); color: var(--paap-danger-text-strong); border-radius: 6px; padding: 10px 12px; font-size: 13px; line-height: 1.4; margin-bottom: 14px; }

.act-btn { height: 28px; padding: 0 10px; border-radius: 6px; border: 1px solid var(--paap-border-02); background: #fff; font-size: 12px; font-weight: 600; cursor: pointer; }
.act-btn:hover:not(:disabled) { background: var(--paap-bg-02); }
.act-btn.primary { background: var(--cds-button-primary, var(--paap-accent)); color: var(--cds-text-on-color, #fff); border-color: var(--cds-button-primary, var(--paap-accent)); }
.act-btn:disabled { opacity: 0.55; cursor: not-allowed; }

.rail-btn { display: inline-flex; align-items: center; justify-content: center; font-size: 13px; font-weight: 600; height: 36px; padding: 0 16px; border-radius: 6px; border: 1px solid var(--paap-border-01); background: #fff; color: var(--paap-text-01); cursor: pointer; }
.rail-btn--primary { background: var(--cds-button-primary, var(--paap-accent)); color: var(--cds-text-on-color, #fff); border-color: var(--cds-button-primary, var(--paap-accent)); }
.rail-btn--primary:hover { background: var(--cds-button-primary-hover, var(--paap-accent-hover)); border-color: var(--cds-button-primary-hover, var(--paap-accent-hover)); }
</style>
