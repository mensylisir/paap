<template>
  <div class="rail-page service-detail-portal">
    <nav class="breadcrumb">
      <router-link class="breadcrumb-link" to="/catalog">服务目录</router-link>
      <span class="breadcrumb-sep">/</span>
      <span class="breadcrumb-current">{{ product?.name || '服务详情' }}</span>
    </nav>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div v-if="loading" class="loading-section">
      <div class="skeleton-heading"></div>
      <div class="skeleton-grid">
        <div v-for="n in 6" :key="n" class="skeleton-block"></div>
      </div>
    </div>

    <template v-else-if="product">
      <header class="detail-header">
        <div class="header-main">
          <div class="header-kicker">
            <span class="category-badge" :style="{ background: categoryColor(product.category) }">{{ categoryLabel(product.category) }}</span>
            <code class="type-code">{{ product.type }}</code>
          </div>
          <h1 class="detail-title">{{ product.name }}</h1>
          <p class="detail-desc">{{ product.description || `${product.name} 服务` }}</p>
          <div class="feature-list" v-if="product.features.length">
            <span v-for="feature in product.features" :key="feature.key" class="feature-chip" :class="{ disabled: !feature.enabled }">
              {{ feature.label }}
            </span>
          </div>
        </div>
        <div class="header-stats">
          <div class="stat-tile">
            <span class="stat-value">{{ product.runningInstances }}</span>
            <span class="stat-label">运行中</span>
          </div>
          <div class="stat-tile">
            <span class="stat-value">{{ product.environmentCount }}</span>
            <span class="stat-label">环境</span>
          </div>
          <div class="stat-tile">
            <span class="stat-value">{{ product.applicationCount }}</span>
            <span class="stat-label">应用</span>
          </div>
        </div>
      </header>

      <div class="tabbar" role="tablist" aria-label="服务详情">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="tab-button"
          :class="{ active: activeTab === tab.key }"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.key"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <section v-if="activeTab === 'overview'" class="content-grid two-col">
        <article class="section-panel markdown-panel">
          <div class="section-heading">
            <h2>{{ detail?.docs.overview.title || '服务介绍' }}</h2>
          </div>
          <div class="markdown-body" v-html="renderMarkdown(detail?.docs.overview.markdown || '')"></div>
        </article>

        <aside class="section-panel">
          <div class="section-heading">
            <h2>生命周期入口</h2>
          </div>
          <div class="install-method-list">
            <div v-for="method in detail?.installMethods || []" :key="method.key" class="install-method" :class="{ disabled: !method.enabled }">
              <div>
                <strong>{{ method.label }}</strong>
                <p>{{ method.description || '-' }}</p>
              </div>
              <span>{{ method.enabled ? '可用' : '停用' }}</span>
            </div>
          </div>
          <div class="versions-block">
            <h3>版本</h3>
            <div v-if="product.versions.length" class="version-list">
              <span v-for="version in product.versions" :key="version" class="version-badge">v{{ stripV(version) }}</span>
            </div>
            <p v-else class="muted-text">暂无可安装版本</p>
          </div>
        </aside>
      </section>

      <section v-else-if="activeTab === 'install'" class="content-grid two-col">
        <article class="section-panel markdown-panel">
          <div class="section-heading">
            <h2>{{ detail?.docs.install.title || '安装方式' }}</h2>
          </div>
          <div class="markdown-body" v-html="renderMarkdown(detail?.docs.install.markdown || '')"></div>
        </article>
        <article class="section-panel markdown-panel">
          <div class="section-heading">
            <h2>{{ detail?.docs.quickstart.title || 'Quick Start' }}</h2>
          </div>
          <div class="markdown-body" v-html="renderMarkdown(detail?.docs.quickstart.markdown || '')"></div>
        </article>
      </section>

      <section v-else-if="activeTab === 'resources'" class="content-grid">
        <article class="section-panel">
          <div class="section-heading split">
            <h2>环境与资源统计</h2>
            <span class="muted-text">{{ resources?.instances?.length || 0 }} 个资源条目</span>
          </div>
          <div class="resource-summary">
            <div class="resource-tile">
              <span>{{ resourceTotal.instances }}</span>
              <label>实例</label>
            </div>
            <div class="resource-tile">
              <span>{{ formatCPU(resourceTotal.cpuRequestMillicores) }}</span>
              <label>CPU Request</label>
            </div>
            <div class="resource-tile">
              <span>{{ formatBytes(resourceTotal.memoryRequestBytes) }}</span>
              <label>内存 Request</label>
            </div>
            <div class="resource-tile">
              <span>{{ formatBytes(resourceTotal.storageRequestBytes) }}</span>
              <label>存储</label>
            </div>
          </div>

          <div class="group-grid">
            <div v-for="group in resources?.groups || []" :key="group.envType" class="group-row">
              <strong>{{ envTypeLabel(group.envType) }}</strong>
              <span>{{ group.instances }} 实例</span>
              <span>{{ formatCPU(group.cpuRequestMillicores) }} CPU</span>
              <span>{{ formatBytes(group.memoryRequestBytes) }} 内存</span>
              <span>{{ formatBytes(group.storageRequestBytes) }} 存储</span>
            </div>
          </div>

          <div class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>实例</th>
                  <th>环境</th>
                  <th>来源</th>
                  <th>状态</th>
                  <th>CPU</th>
                  <th>内存</th>
                  <th>存储</th>
                  <th>快照来源</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in resources?.instances || []" :key="item.id">
                  <td>
                    <strong>{{ item.serviceName }}</strong>
                    <small>{{ item.namespace || item.id }}</small>
                  </td>
                  <td>
                    <router-link v-if="item.applicationId && item.environmentId" :to="`/apps/${item.applicationId}/environments/${item.environmentId}`">
                      {{ item.environmentName || item.environmentIdentifier }}
                    </router-link>
                    <span v-else>{{ item.environmentName || item.environmentIdentifier || '-' }}</span>
                  </td>
                  <td><span class="source-tag" :class="'source--' + item.source">{{ sourceLabel(item.source) }}</span></td>
                  <td><span class="status-pill" :class="'status--' + item.status">{{ statusLabel(item.status) }}</span></td>
                  <td>{{ formatCPU(item.footprint.cpuRequestMillicores) }}</td>
                  <td>{{ formatBytes(item.footprint.memoryRequestBytes) }}</td>
                  <td>{{ formatBytes(item.footprint.storageRequestBytes) }}</td>
                  <td>{{ snapshotSourceLabel(item.snapshotSource) }}</td>
                </tr>
                <tr v-if="!resources?.instances?.length">
                  <td colspan="8" class="empty-cell">暂无资源数据</td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>
      </section>

      <section v-else-if="activeTab === 'monitor'" class="content-grid">
        <article class="section-panel">
          <div class="section-heading split">
            <h2>{{ observability?.dashboardTitle || '服务监控' }}</h2>
            <span class="muted-text">Grafana Dashboard</span>
          </div>
          <div v-if="monitorFrames.length" class="catalog-grafana-grid">
            <div v-for="frame in monitorFrames" :key="frame.key" class="catalog-frame-shell">
              <header class="catalog-frame-head">
                <div>
                  <strong>{{ frame.title }}</strong>
                  <span>{{ frame.subtitle }}</span>
                </div>
                <a class="rail-btn rail-btn--secondary" :href="frame.openUrl" target="_blank" rel="noopener noreferrer">打开 Grafana</a>
              </header>
              <iframe class="catalog-grafana-frame" :src="frame.url" :title="frame.title" loading="lazy" @load="compactGrafanaEmbed" />
            </div>
          </div>
          <div v-else class="empty-state">暂无 Grafana 大盘实例</div>
        </article>

        <article class="section-panel">
          <div class="section-heading">
            <h2>模板关键指标</h2>
          </div>
          <div v-if="observability?.metricCards?.length" class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th>指标</th>
                  <th>说明</th>
                  <th>PromQL</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="metric in observability?.metricCards || []" :key="metric.key">
                  <td>
                    <strong>{{ metric.title }}</strong>
                    <small>{{ metric.unit || metric.key }}</small>
                  </td>
                  <td>{{ metric.description || '-' }}</td>
                  <td><code>{{ metric.promql }}</code></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="empty-state">模板未声明专属指标</div>
        </article>
      </section>

      <section v-else-if="activeTab === 'logs'" class="content-grid">
        <article class="section-panel">
          <div class="section-heading split">
            <h2>日志</h2>
            <span class="muted-text">Loki 条件跳转</span>
          </div>
          <div v-if="logFrames.length" class="catalog-grafana-grid">
            <div v-for="frame in logFrames" :key="frame.key" class="catalog-frame-shell catalog-frame-shell--logs">
              <header class="catalog-frame-head">
                <div>
                  <strong>{{ frame.title }}</strong>
                  <span>{{ frame.subtitle }}</span>
                </div>
                <a class="rail-btn rail-btn--secondary" :href="frame.openUrl" target="_blank" rel="noopener noreferrer">打开 Loki</a>
              </header>
              <code class="catalog-log-query">{{ frame.query }}</code>
              <iframe class="catalog-grafana-frame catalog-grafana-frame--logs" :src="frame.url" :title="frame.title" loading="lazy" @load="compactGrafanaEmbed" />
            </div>
          </div>
          <div v-else class="empty-state">暂无 Loki 日志入口</div>
        </article>
      </section>
    </template>

    <div v-else class="empty-state large">
      <h2>未找到服务</h2>
      <router-link class="rail-btn rail-btn--primary" to="/catalog">返回目录</router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import { stripCatalogVersionPrefix } from '../utils/catalogVersions'
import { compactGrafanaEmbed } from '../components/workspaces/grafanaEmbed'
import { withEmbeddedProxyAuthToken } from './serviceWorkspace'

interface CatalogFeature {
  key: string
  label: string
  enabled: boolean
}

interface CatalogProduct {
  type: string
  name: string
  description: string
  category: string
  versions: string[]
  features: CatalogFeature[]
  managedInstances: number
  kubevirtInstances: number
  publicInstances: number
  sharedReferences: number
  externalConnections: number
  deferredReferences: number
  runningInstances: number
  applicationCount: number
  environmentCount: number
}

interface MarkdownDoc {
  title: string
  markdown: string
}

interface CatalogServiceDetail {
  product: CatalogProduct
  docs: {
    overview: MarkdownDoc
    install: MarkdownDoc
    quickstart: MarkdownDoc
  }
  installMethods: Array<{
    key: string
    label: string
    description: string
    enabled: boolean
  }>
}

interface ResourceFootprint {
  instances: number
  runningInstances: number
  cpuRequestMillicores: number
  cpuLimitMillicores: number
  memoryRequestBytes: number
  memoryLimitBytes: number
  storageRequestBytes: number
  estimatedCpuUsageMillicores: number
  estimatedMemoryUsageBytes: number
}

interface ResourceGroup extends ResourceFootprint {
  envType: string
  envName?: string
}

interface ResourceInstance {
  id: string
  serviceType: string
  serviceName: string
  source: string
  provisionMode?: string
  status: string
  applicationId?: number
  applicationName?: string
  environmentId?: number
  environmentName?: string
  environmentIdentifier?: string
  envType: string
  namespace?: string
  snapshotSource: string
  footprint: ResourceFootprint
}

interface ResourceSummary {
  serviceType: string
  total: ResourceFootprint
  groups: ResourceGroup[]
  instances: ResourceInstance[]
}

interface MetricCard {
  key: string
  title: string
  unit: string
  description: string
  promql: string
}

interface InstanceObservability {
  instanceId: string
  serviceName: string
  environmentName?: string
  namespace?: string
  monitoringTarget?: string
  dashboardUrl?: string
  errorLogsUrl?: string
  logQuery?: string
}

interface Observability {
  serviceType: string
  dashboardUid: string
  dashboardTitle: string
  metricCards: MetricCard[]
  logQueryTemplate: string
  instances: InstanceObservability[]
}

const route = useRoute()
const stripV = stripCatalogVersionPrefix

const loading = ref(true)
const pageError = ref('')
const activeTab = ref('overview')
const detail = ref<CatalogServiceDetail | null>(null)
const resources = ref<ResourceSummary | null>(null)
const observability = ref<Observability | null>(null)

const tabs = [
  { key: 'overview', label: '概览' },
  { key: 'install', label: '安装与 Quick Start' },
  { key: 'resources', label: '资源' },
  { key: 'monitor', label: '监控' },
  { key: 'logs', label: '日志' },
]

const emptyFootprint: ResourceFootprint = {
  instances: 0,
  runningInstances: 0,
  cpuRequestMillicores: 0,
  cpuLimitMillicores: 0,
  memoryRequestBytes: 0,
  memoryLimitBytes: 0,
  storageRequestBytes: 0,
  estimatedCpuUsageMillicores: 0,
  estimatedMemoryUsageBytes: 0,
}

const product = computed(() => detail.value?.product || null)
const resourceTotal = computed(() => resources.value?.total || emptyFootprint)
const monitorFrames = computed(() =>
  (observability.value?.instances || [])
    .filter(item => String(item.dashboardUrl || '').trim())
    .map(item => ({
      key: `monitor:${item.instanceId}`,
      title: item.serviceName || product.value?.name || '服务实例',
      subtitle: item.environmentName || item.namespace || '-',
      openUrl: item.dashboardUrl || '',
      url: embeddedGrafanaURL(item.dashboardUrl || '', item),
    }))
    .filter(item => item.url)
)
const logFrames = computed(() =>
  (observability.value?.instances || [])
    .filter(item => String(item.errorLogsUrl || '').trim())
    .map(item => ({
      key: `logs:${item.instanceId}`,
      title: item.serviceName || product.value?.name || '服务实例',
      subtitle: item.environmentName || item.namespace || '-',
      query: item.logQuery || observability.value?.logQueryTemplate || '',
      openUrl: item.errorLogsUrl || '',
      url: embeddedGrafanaURL(item.errorLogsUrl || '', item),
    }))
    .filter(item => item.url)
)

const embeddedGrafanaURL = (url: string, item?: InstanceObservability) => {
  if (!url) return ''
  try {
    const parsed = new URL(url, window.location.origin)
    parsed.searchParams.set('kiosk', '')
    parsed.searchParams.set('embed', 'true')
    parsed.searchParams.set('paap_embed', '1')
    parsed.searchParams.set('theme', 'light')
    parsed.searchParams.set('orgId', '1')
    const namespace = String(item?.namespace || '').trim()
    if (namespace) parsed.searchParams.set('var-namespace', namespace)
    return withEmbeddedProxyAuthToken(parsed.pathname + parsed.search + parsed.hash)
  } catch {
    return url
  }
}

const payloadData = (value: any) => value?.data ?? value

const catalogFeatureItems = (raw: unknown): CatalogFeature[] => {
  if (Array.isArray(raw)) {
    return raw
      .map((item: any) => ({
        key: String(item?.key || '').trim(),
        label: String(item?.label || item?.key || '').trim(),
        enabled: item?.enabled !== false,
      }))
      .filter(item => item.key && item.label)
  }
  if (typeof raw === 'string' && raw.trim()) {
    try { return catalogFeatureItems(JSON.parse(raw)) } catch { return [] }
  }
  return []
}

const normalizeProduct = (raw: any): CatalogProduct => ({
  type: String(raw?.type || ''),
  name: String(raw?.name || raw?.type || ''),
  description: String(raw?.description || ''),
  category: String(raw?.category || ''),
  versions: Array.isArray(raw?.versions) ? raw.versions.map((item: any) => String(item)).filter(Boolean) : [],
  features: catalogFeatureItems(raw?.features),
  managedInstances: Number(raw?.managedInstances || 0),
  kubevirtInstances: Number(raw?.kubevirtInstances || 0),
  publicInstances: Number(raw?.publicInstances || 0),
  sharedReferences: Number(raw?.sharedReferences || 0),
  externalConnections: Number(raw?.externalConnections || 0),
  deferredReferences: Number(raw?.deferredReferences || 0),
  runningInstances: Number(raw?.runningInstances || 0),
  applicationCount: Number(raw?.applicationCount || 0),
  environmentCount: Number(raw?.environmentCount || 0),
})

const normalizeDetail = (raw: any): CatalogServiceDetail => ({
  product: normalizeProduct(raw?.product || {}),
  docs: {
    overview: raw?.docs?.overview || { title: '服务介绍', markdown: '' },
    install: raw?.docs?.install || { title: '安装方式', markdown: '' },
    quickstart: raw?.docs?.quickstart || { title: 'Quick Start', markdown: '' },
  },
  installMethods: Array.isArray(raw?.installMethods) ? raw.installMethods : [],
})

const categoryLabel = (cat: string) => {
  const labels: Record<string, string> = {
    ci: 'CI 服务',
    cd: 'CD 服务',
    monitor: '监控服务',
    log: '日志服务',
    database: '数据库服务',
    middleware: '中间件服务',
    environment: '环境服务',
    virtualMachine: '虚拟机服务',
    git: '代码服务',
    registry: '镜像服务',
    other: '其他服务',
  }
  return labels[cat] || cat || '服务'
}

const categoryColor = (cat: string) => {
  const colors: Record<string, string> = {
    ci: '#d0e2ff',
    cd: '#d0e2ff',
    monitor: '#b9e6b9',
    log: '#a7f0e8',
    database: '#e8daff',
    middleware: '#fdd5a0',
    environment: '#d1d1d1',
    virtualMachine: '#ffb3b8',
    git: '#b4e6f0',
    registry: '#f5e3a0',
  }
  return colors[cat] || '#d1d1d1'
}

const sourceLabel = (source: string) => {
  const labels: Record<string, string> = {
    managed: '环境内',
    self: '环境内',
    shared: '共享',
    external: '外部',
    deferred: '待配置',
  }
  return labels[source] || source || '-'
}

const statusLabel = (status: string) => {
  const labels: Record<string, string> = {
    running: '运行中',
    pending: '等待',
    installing: '安装中',
    failed: '失败',
    deleting: '删除中',
    stopped: '已停止',
    linked: '已关联',
  }
  return labels[status] || status || '-'
}

const envTypeLabel = (value: string) => {
  const labels: Record<string, string> = {
    dev: 'Dev',
    test: 'Test',
    staging: 'Staging',
    prod: 'Prod',
    shared: '共享环境',
    unknown: '未分类',
  }
  return labels[value] || value
}

const snapshotSourceLabel = (value: string) => {
  const labels: Record<string, string> = {
    'install-values': '安装参数',
    'service-default': '服务默认值',
    'external-config': '外部配置',
  }
  return labels[value] || value || '-'
}

const formatCPU = (millicores?: number) => {
  const value = Number(millicores || 0)
  if (value <= 0) return '-'
  if (value < 1000) return `${value}m`
  return `${(value / 1000).toFixed(value % 1000 === 0 ? 0 : 1)} 核`
}

const formatBytes = (bytes?: number) => {
  const value = Number(bytes || 0)
  if (value <= 0) return '-'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let current = value
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

const escapeHtml = (value: string) => value
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#039;')

const inlineMarkdown = (value: string) => escapeHtml(value)
  .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  .replace(/`([^`]+)`/g, '<code>$1</code>')

const renderMarkdown = (markdown: string) => {
  const lines = String(markdown || '').split('\n')
  const html: string[] = []
  let inCode = false
  let inList = false
  let codeLines: string[] = []
  const closeList = () => {
    if (inList) {
      html.push('</ul>')
      inList = false
    }
  }
  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('```')) {
      closeList()
      if (inCode) {
        html.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`)
        codeLines = []
        inCode = false
      } else {
        inCode = true
      }
      continue
    }
    if (inCode) {
      codeLines.push(line)
      continue
    }
    if (!trimmed) {
      closeList()
      continue
    }
    if (trimmed.startsWith('### ')) {
      closeList()
      html.push(`<h3>${inlineMarkdown(trimmed.slice(4))}</h3>`)
      continue
    }
    if (trimmed.startsWith('## ')) {
      closeList()
      html.push(`<h2>${inlineMarkdown(trimmed.slice(3))}</h2>`)
      continue
    }
    if (trimmed.startsWith('# ')) {
      closeList()
      html.push(`<h1>${inlineMarkdown(trimmed.slice(2))}</h1>`)
      continue
    }
    if (trimmed.startsWith('- ')) {
      if (!inList) {
        html.push('<ul>')
        inList = true
      }
      html.push(`<li>${inlineMarkdown(trimmed.slice(2))}</li>`)
      continue
    }
    closeList()
    html.push(`<p>${inlineMarkdown(trimmed)}</p>`)
  }
  closeList()
  if (inCode) {
    html.push(`<pre><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`)
  }
  return html.join('')
}

const loadService = async () => {
  const type = String(route.params.type || '').trim()
  if (!type) {
    pageError.value = '未指定服务类型'
    loading.value = false
    return
  }
  loading.value = true
  pageError.value = ''
  detail.value = null
  resources.value = null
  observability.value = null
  try {
    const [detailResult, resourcesResult, observabilityResult] = await Promise.all([
      api.getCatalogServiceDetail(type),
      api.getCatalogServiceResources(type),
      api.getCatalogServiceObservability(type),
    ])
    detail.value = normalizeDetail(payloadData(detailResult))
    resources.value = payloadData(resourcesResult)
    observability.value = payloadData(observabilityResult)
    activeTab.value = 'overview'
  } catch (error: any) {
    pageError.value = '加载服务详情失败：' + (error?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

watch(() => route.params.type, loadService, { immediate: true })
</script>

<style scoped>
.service-detail-portal {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-12);
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 18px;
  font-size: var(--paap-fs-compact);
}

.breadcrumb-link {
  color: var(--paap-muted);
  text-decoration: none;
}

.breadcrumb-link:hover {
  color: var(--paap-text);
  text-decoration: underline;
}

.breadcrumb-sep,
.breadcrumb-current {
  color: var(--paap-muted);
}

.page-error {
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  padding: 10px 12px;
  margin-bottom: 16px;
  font-size: var(--paap-fs-compact);
}

.loading-section {
  display: grid;
  gap: 18px;
}

.skeleton-heading,
.skeleton-block {
  border-radius: var(--paap-radius-xs);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}

.skeleton-heading {
  width: 320px;
  height: 34px;
}

.skeleton-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.skeleton-block {
  height: 118px;
}

@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

.detail-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 20px;
  align-items: end;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--paap-border);
}

.header-main {
  display: grid;
  gap: 8px;
}

.header-kicker {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.category-badge,
.type-code,
.feature-chip,
.version-badge,
.source-tag,
.status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 2px 8px;
  border-radius: var(--paap-radius-xs);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  line-height: 1.2;
}

.type-code {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-family: var(--paap-mono);
}

.detail-title {
  margin: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-heading-2xl);
  line-height: 1.2;
}

.detail-desc {
  max-width: 900px;
  margin: 0;
  color: var(--paap-muted);
  line-height: 1.6;
}

.feature-list,
.version-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.feature-chip {
  border: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
}

.feature-chip.disabled {
  opacity: 0.45;
}

.header-stats {
  display: grid;
  grid-template-columns: repeat(3, 88px);
  gap: 8px;
}

.stat-tile {
  display: grid;
  gap: 4px;
  padding: 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  text-align: right;
}

.stat-value {
  color: var(--paap-text);
  font-size: 22px;
  font-weight: 700;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.stat-label {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.tabbar {
  display: flex;
  gap: 4px;
  margin: 18px 0;
  border-bottom: 1px solid var(--paap-border);
}

.tab-button {
  appearance: none;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--paap-muted);
  padding: 10px 14px;
  font: inherit;
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  cursor: pointer;
}

.tab-button:hover,
.tab-button.active {
  color: var(--paap-text);
  border-bottom-color: var(--paap-accent);
}

.content-grid {
  display: grid;
  gap: 16px;
}

.content-grid.two-col {
  grid-template-columns: minmax(0, 1.35fr) minmax(320px, 0.65fr);
}

.section-panel {
  display: grid;
  gap: 14px;
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}

.section-heading {
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-heading.split {
  justify-content: space-between;
}

.section-heading h2 {
  margin: 0;
  color: var(--paap-text);
  font-size: 18px;
  line-height: 1.3;
}

.markdown-body {
  color: var(--paap-text);
  line-height: 1.65;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 14px 0 8px;
  color: var(--paap-text);
  line-height: 1.3;
}

.markdown-body :deep(h1) { font-size: 20px; }
.markdown-body :deep(h2) { font-size: 16px; }
.markdown-body :deep(h3) { font-size: 14px; }

.markdown-body :deep(p) {
  margin: 8px 0;
}

.markdown-body :deep(ul) {
  margin: 8px 0;
  padding-left: 20px;
}

.markdown-body :deep(pre) {
  overflow: auto;
  padding: 12px;
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.markdown-body :deep(code) {
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
}

.install-method-list,
.observability-list {
  display: grid;
  gap: 10px;
}

.install-method {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
  padding: 12px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.install-method.disabled {
  opacity: 0.55;
}

.install-method p {
  margin: 4px 0 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
}

.versions-block {
  display: grid;
  gap: 8px;
  padding-top: 4px;
}

.versions-block h3 {
  margin: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
}

.version-badge {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}

.resource-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.resource-tile {
  display: grid;
  gap: 4px;
  padding: 14px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.resource-tile span {
  color: var(--paap-text);
  font-size: 20px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.resource-tile label {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.group-grid {
  display: grid;
  gap: 8px;
}

.group-row {
  display: grid;
  grid-template-columns: 1fr repeat(4, minmax(90px, auto));
  gap: 12px;
  align-items: center;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

.group-row strong {
  color: var(--paap-text);
}

.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
}

.data-table th,
.data-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--paap-border);
  text-align: left;
  vertical-align: top;
  font-size: var(--paap-fs-compact);
}

.data-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 700;
}

.data-table td small {
  display: block;
  margin-top: 2px;
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
}

.data-table a {
  color: var(--paap-accent);
  text-decoration: none;
}

.data-table a:hover {
  text-decoration: underline;
}

.empty-cell,
.empty-state {
  color: var(--paap-muted);
  text-align: center;
}

.empty-cell {
  padding: 28px 12px;
}

.empty-state {
  padding: 40px 16px;
}

.empty-state.large {
  display: grid;
  justify-items: center;
  gap: 12px;
  padding: 72px 16px;
}

.source--managed,
.source--self {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}

.source--shared {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}

.source--external,
.source--deferred {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.status-pill {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.status--running,
.status--linked {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}

.status--failed {
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
}

.status--installing,
.status--pending {
  background: var(--paap-warning-soft);
  color: var(--paap-warning);
}

.topology-canvas {
  overflow: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.topology-canvas svg {
  display: block;
  min-width: 720px;
  width: 100%;
  height: auto;
  color: var(--paap-muted);
}

.topology-edges line {
  stroke: currentColor;
  stroke-width: 1.4;
  opacity: 0.65;
}

.topology-node rect {
  fill: var(--paap-panel);
  stroke: var(--paap-border);
  stroke-width: 1;
}

.topology-node text {
  fill: var(--paap-text);
  font-size: 12px;
  font-weight: 700;
}

.topology-node .node-meta {
  fill: var(--paap-muted);
  font-size: 10px;
  font-weight: 500;
}

.node--product rect {
  stroke: var(--paap-accent);
}

.node--dependency rect {
  stroke: var(--paap-success);
}

.node--architecture rect {
  stroke-dasharray: 4 3;
}

.catalog-grafana-grid {
  display: grid;
  gap: var(--paap-space-3);
}

.catalog-frame-shell {
  display: grid;
  grid-template-rows: auto minmax(520px, calc(100vh - 300px));
  min-height: 620px;
  overflow: hidden;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.catalog-frame-shell--logs {
  grid-template-rows: auto auto minmax(520px, calc(100vh - 340px));
}

.catalog-frame-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel);
}

.catalog-frame-head div {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.catalog-frame-head strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
}

.catalog-frame-head span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.catalog-grafana-frame {
  width: 100%;
  height: 100%;
  min-height: 520px;
  border: 0;
  background: var(--paap-panel);
}

.catalog-grafana-frame--logs {
  min-height: 560px;
}

.catalog-log-query,
.data-table code {
  overflow: auto;
  padding: 8px;
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
  white-space: pre-wrap;
  word-break: break-word;
}

.catalog-log-query {
  margin: 10px 12px 0;
}

.muted-text {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

@media (max-width: 920px) {
  .detail-header,
  .content-grid.two-col {
    grid-template-columns: 1fr;
  }

  .header-stats,
  .resource-summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .service-detail-portal {
    padding: var(--paap-space-4) var(--paap-space-4) var(--paap-space-8);
  }

  .header-stats,
  .resource-summary,
  .skeleton-grid {
    grid-template-columns: 1fr;
  }

  .tabbar {
    overflow-x: auto;
  }

  .tab-button {
    white-space: nowrap;
  }

  .group-row {
    grid-template-columns: 1fr;
  }
}
</style>
