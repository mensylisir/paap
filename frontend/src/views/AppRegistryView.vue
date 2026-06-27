<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">镜像仓库</h1>
        <p class="page-desc">查看全平台各环境的镜像仓库概况与镜像分布</p>
      </div>
      <button class="rail-btn rail-btn--ghost" :disabled="loading" @click="load">{{ loading ? '加载中...' : '刷新' }}</button>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div class="kpi-section">
      <div class="kpi-card">
        <div class="kpi-number">{{ registries.length }}</div>
        <div class="kpi-label">仓库实例</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ totalProjects }}</div>
        <div class="kpi-label">项目总数</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-number">{{ totalArtifacts }}</div>
        <div class="kpi-label">制品总数</div>
      </div>
    </div>

    <section class="section-card">
      <div class="section-header">
        <div>
          <h2 class="rail-section-title">实例列表</h2>
          <p class="rail-section-desc">各环境中已安装的 Registry / Harbor 服务</p>
        </div>
      </div>

      <div v-if="loading" class="loading-mask"><div class="loading-spinner" /></div>
      <div v-else-if="!registries.length" class="rail-empty">
        <p class="rail-empty-desc">暂无已安装的镜像仓库服务。</p>
      </div>
      <div v-else class="registry-list">
        <div v-for="reg in registries" :key="(reg.envId || 0) + '-' + (reg.serviceId || 0)" class="registry-row">
          <div class="registry-body">
            <div class="registry-header">
              <div class="registry-name-group">
                <span class="registry-name">{{ reg.serviceName || reg.serviceType }}</span>
                <span class="tag" :class="reg.serviceType === 'harbor' ? 'harbor' : 'registry'">{{ typeLabel(reg.serviceType) }}</span>
                <span class="tag" :class="reg.status === 'running' ? 'green' : 'gray'">{{ statusText(reg.status) }}</span>
              </div>
              <div class="registry-actions">
                <a v-if="reg.accessUrl" :href="reg.accessUrl" target="_blank" rel="noreferrer" class="link external">访问</a>
                <router-link v-if="reg.appId && reg.envId" class="link" :to="`/apps/${reg.appId}/environments/${reg.envId}`">进入环境</router-link>
              </div>
            </div>
            <div class="registry-meta">
              <span>环境: {{ reg.envName || '-' }}</span>
              <span>命名空间: {{ reg.namespace || '-' }}</span>
              <span>Release: {{ reg.releaseName || '-' }}</span>
              <span v-if="reg.projectCount !== undefined">项目: {{ reg.projectCount }}</span>
              <span v-if="reg.artifactCount !== undefined">制品: {{ reg.artifactCount }}</span>
            </div>
            <div v-if="reg.projects?.length" class="project-chips">
              <span v-for="p in reg.projects" :key="p" class="chip">{{ p }}</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '../api/client'

interface RegistryInfo {
  appId?: number
  envId?: number
  envName?: string
  serviceId?: number
  serviceType: string
  serviceName?: string
  status?: string
  namespace?: string
  releaseName?: string
  accessUrl?: string
  projectCount?: number
  artifactCount?: number
  projects?: string[]
}

const registries = ref<RegistryInfo[]>([])
const loading = ref(false)
const pageError = ref('')

const totalProjects = computed(() => registries.value.reduce((sum, r) => sum + (r.projectCount || 0), 0))
const totalArtifacts = computed(() => registries.value.reduce((sum, r) => sum + (r.artifactCount || 0), 0))

onMounted(load)

async function load() {
  loading.value = true
  pageError.value = ''
  try {
    const appRes = await api.listApps()
    const apps = appRes.data?.applications || appRes.data || []
    const out: RegistryInfo[] = []
    for (const app of apps) {
      const envsRes = await api.listEnvs(app.id)
      const envs = envsRes.data || []
      for (const env of envs) {
        if (!env.installations) continue
        const regInstalls = env.installations.filter((i: any) => i.serviceType === 'registry' || i.serviceType === 'harbor')
        for (const inst of regInstalls) {
          const ns = `${app.identifier}-${env.identifier}-${inst.serviceType}`
          const accessUrl = `/api/v1/environments/${env.id}/services/${inst.id}/proxy/`
          out.push({
            appId: app.id,
            envId: env.id,
            envName: env.name,
            serviceId: inst.id,
            serviceType: inst.serviceType,
            serviceName: inst.serviceName || inst.serviceType,
            status: inst.status,
            namespace: ns,
            releaseName: inst.releaseName,
            accessUrl,
          })
        }
      }
    }
    registries.value = out
  } catch (e: any) {
    pageError.value = '加载失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

function typeLabel(type: string) {
  return type === 'harbor' ? 'Harbor' : 'Registry'
}
function statusText(status?: string) {
  return ({ running: '运行中', installing: '安装中', failed: '失败', deleting: '删除中', pending: '等待中' }[status || ''] || status || '未知')
}
</script>

<style scoped>
.rail-page { padding: 20px 20px 36px; max-width: none; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; gap: 16px; }
.header-text { display: flex; flex-direction: column; gap: 2px; }
.page-title { font-size: 24px; font-weight: 600; color: var(--cds-text-primary); line-height: 1.2; }
.page-desc { font-size: 14px; color: var(--cds-text-secondary); line-height: 1.4; }
.page-error { border: 1px solid var(--cds-red-20); background: var(--cds-red-10); color: var(--cds-text-error); border-radius: var(--paap-radius-sm); padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }

.kpi-section { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 12px; margin-bottom: 32px; }
.kpi-card { background: var(--cds-layer-02); border: 1px solid var(--cds-border-subtle-01); border-radius: var(--paap-radius); padding: 16px 18px; display: flex; flex-direction: column; gap: 8px; }
.kpi-number { font-size: 28px; font-weight: 600; color: var(--cds-text-primary); line-height: 1.2; }
.kpi-label { font-size: 12px; color: var(--cds-text-secondary); }

.section-card { background: var(--cds-layer-02); border: 1px solid var(--cds-border-subtle-01); border-radius: var(--paap-radius); padding: 24px; }
.section-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; }
.loading-mask { display: flex; align-items: center; justify-content: center; padding: 64px; }
.loading-spinner { width: 24px; height: 24px; border: 2px solid var(--cds-border-subtle-01); border-top-color: var(--cds-text-primary); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.registry-list { border-top: 1px solid var(--cds-gray-10); }
.registry-row { display: flex; border-bottom: 1px solid var(--cds-gray-10); transition: background-color var(--paap-transition-fast); }
.registry-row:last-child { border-bottom: none; }
.registry-row:hover { background: var(--cds-layer-hover-01); }
.registry-body { padding: 16px 20px; flex: 1; min-width: 0; }
.registry-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 6px; }
.registry-name-group { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.registry-name { font-weight: 600; font-size: 15px; color: var(--cds-text-primary); }
.registry-meta { display: flex; gap: 12px; flex-wrap: wrap; color: var(--cds-text-secondary); margin-bottom: 8px; font-size: 13px; }
.project-chips { display: flex; flex-wrap: wrap; gap: 6px; }

.registry-actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.link { color: var(--cds-link-primary); font-weight: 500; text-decoration: none; font-size: 13px; }
.link:hover { text-decoration: underline; }
.link.external { display: inline-flex; align-items: center; gap: 3px; }
.link.external::after { content: '↗'; font-size: 10px; opacity: .7; }

.chip { font-size: 11px; padding: 2px 8px; border-radius: var(--paap-radius-xs); background: var(--cds-gray-10); color: var(--cds-text-secondary); font-weight: 500; }
.tag { display: inline-flex; align-items: center; height: 20px; padding: 0 8px; font-size: 11px; font-weight: 500; border-radius: var(--paap-radius-xs); }
.tag.registry { background: var(--cds-green-10); color: var(--cds-green-60); }
.tag.harbor { background: var(--cds-blue-10); color: var(--cds-link-primary); }
.tag.green { background: var(--cds-green-10); color: var(--cds-green-60); }
.tag.gray { background: var(--cds-gray-10); color: var(--cds-text-secondary); }

@media (max-width: 672px) {
  .rail-page { padding: 20px 20px 32px; }
  .page-header { flex-direction: column; align-items: flex-start; gap: 12px; }
  .kpi-section { grid-template-columns: 1fr 1fr; }
  .registry-header { flex-direction: column; }
}
</style>
