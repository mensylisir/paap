<template>
  <div class="rail-page shared-resource-page">
    <header class="page-header">
      <div>
        <h1 class="page-title">共享资源</h1>
        <p class="page-desc">集中维护平台公共工具和中间件，业务环境只引用这里提供的资源。</p>
      </div>
      <button type="button" class="rail-btn rail-btn--primary" :disabled="!poolEnvId" @click="openPoolWorkspace">
        进入共享资源画布
      </button>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <section class="pool-summary">
      <div class="summary-item">
        <span>系统应用</span>
        <strong>{{ poolAppName }}</strong>
      </div>
      <div class="summary-item">
        <span>共享环境</span>
        <strong>{{ poolEnvName }}</strong>
      </div>
      <div class="summary-item">
        <span>工具</span>
        <strong>{{ toolCount }}</strong>
      </div>
      <div class="summary-item">
        <span>中间件</span>
        <strong>{{ infraCount }}</strong>
      </div>
    </section>

    <section class="resource-section">
      <div class="section-heading">
        <h2>公共资源</h2>
        <button type="button" class="rail-btn rail-btn--ghost rail-btn--sm" :disabled="loading" @click="loadPage">
          {{ loading ? '刷新中...' : '刷新' }}
        </button>
      </div>

      <div v-if="loading" class="empty-state">加载中...</div>
      <div v-else-if="services.length === 0" class="empty-state">
        暂无公共工具或中间件。进入共享资源画布后可以安装 Registry、Gitea、Redis、PostgreSQL 等资源。
      </div>
      <div v-else class="resource-grid">
        <article v-for="service in services" :key="service.id" class="resource-card">
          <div class="resource-main">
            <span class="resource-kind">{{ serviceKindLabel(service.serviceType) }}</span>
            <h3>{{ service.serviceName || service.serviceType }}</h3>
            <p>{{ service.namespace || '未分配运行空间' }}</p>
          </div>
          <span class="status-pill" :class="'status--' + normalizedStatus(service.status)">
            {{ statusLabel(service.status) }}
          </span>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'

interface SharedService {
  id: number
  serviceType: string
  serviceName: string
  namespace: string
  status: string
}

const router = useRouter()
const loading = ref(false)
const pageError = ref('')
const poolAppId = ref(0)
const poolEnvId = ref(0)
const poolAppName = ref('共享资源池')
const poolEnvName = ref('共享环境')
const services = ref<SharedService[]>([])

const toolTypes = new Set(['git', 'registry', 'harbor', 'ci', 'deploy', 'monitor', 'log'])

const toolCount = computed(() => services.value.filter((service) => toolTypes.has(service.serviceType)).length)
const infraCount = computed(() => services.value.length - toolCount.value)

const normalizeService = (raw: any): SharedService => ({
  id: Number(raw?.id || 0),
  serviceType: String(raw?.serviceType || raw?.type || ''),
  serviceName: String(raw?.serviceName || raw?.name || ''),
  namespace: String(raw?.namespace || ''),
  status: String(raw?.status || ''),
})

const serviceKindLabel = (type: string) => toolTypes.has(type) ? '工具' : '中间件'

const normalizedStatus = (status: string) => {
  const value = String(status || '').toLowerCase()
  if (value === 'running') return 'running'
  if (value === 'failed' || value === 'error') return 'failed'
  if (value === 'installing' || value === 'creating') return 'pending'
  return 'unknown'
}

const statusLabel = (status: string) => {
  const labels: Record<string, string> = {
    running: '运行中',
    installing: '安装中',
    creating: '创建中',
    failed: '失败',
    error: '异常',
  }
  return labels[String(status || '').toLowerCase()] || '未知'
}

const loadPage = async () => {
  loading.value = true
  pageError.value = ''
  try {
    const pool = await api.getSharedResourcePool()
    const app = pool.data?.application || {}
    const env = pool.data?.environment || {}
    poolAppId.value = Number(app.id || 0)
    poolEnvId.value = Number(env.id || 0)
    poolAppName.value = String(app.name || '共享资源池')
    poolEnvName.value = String(env.name || '共享环境')
    if (!poolEnvId.value) throw new Error('共享资源池尚未初始化')

    const serviceRes = await api.listServices(poolEnvId.value)
    services.value = Array.isArray(serviceRes.data)
      ? serviceRes.data.map(normalizeService).filter((service: SharedService) => service.id > 0)
      : []
  } catch (e: any) {
    pageError.value = '加载共享资源池失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

const openPoolWorkspace = () => {
  if (!poolAppId.value || !poolEnvId.value) return
  router.push(`/apps/${poolAppId.value}/environments/${poolEnvId.value}`)
}

onMounted(loadPage)
</script>

<style scoped>
.shared-resource-page {
  padding: 20px 20px 36px;
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.page-title {
  color: var(--paap-text);
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.page-desc {
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
}

.page-error {
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  font-size: var(--paap-fs-compact);
}

.pool-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.summary-item {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  display: grid;
  gap: 6px;
  padding: 16px;
}

.summary-item span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.summary-item strong {
  color: var(--paap-text);
  font-size: 22px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.resource-section {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  padding: 20px;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.section-heading h2 {
  color: var(--paap-text);
  font-size: 16px;
  font-weight: 600;
}

.resource-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 12px;
}

.resource-card {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  display: flex;
  justify-content: space-between;
  gap: 12px;
  min-height: 100px;
  padding: 16px;
  transition: box-shadow 0.15s;
}
.resource-card:hover {
  box-shadow: var(--paap-shadow-sm);
}

.resource-main {
  display: grid;
  align-content: start;
  gap: 5px;
  min-width: 0;
}

.resource-kind {
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.resource-card h3 {
  color: var(--paap-text);
  font-size: 15px;
  font-weight: 600;
  margin: 0;
}

.resource-card p {
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
  overflow-wrap: anywhere;
  margin: 0;
}

.status-pill {
  align-self: flex-start;
  flex-shrink: 0;
  height: 24px;
  padding: 3px 8px;
  border-radius: var(--paap-radius-xs);
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  white-space: nowrap;
}

.status--running {
  color: var(--paap-success);
  background: var(--paap-success-soft);
}

.status--pending {
  color: var(--paap-warning);
  background: var(--paap-warning-soft);
}

.status--failed {
  color: var(--paap-danger);
  background: var(--paap-danger-soft);
}

.empty-state {
  padding: 32px 16px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  text-align: center;
}

@media (max-width: 900px) {
  .pool-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 672px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .pool-summary {
    grid-template-columns: 1fr;
  }
}
</style>
