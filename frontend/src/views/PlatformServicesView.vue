<template>
  <div class="rail-page platform-services-page">
    <header class="page-header">
      <div>
        <h1 class="page-title">平台服务</h1>
        <p class="page-subtitle">跨应用查看服务实例、共享引用和外部连接</p>
      </div>
      <button class="rail-btn rail-btn--secondary" :disabled="loading" @click="loadStats">
        {{ loading ? '刷新中...' : '刷新' }}
      </button>
    </header>

    <div v-if="error" class="form-error" role="alert">{{ error }}</div>

    <section v-if="loading" class="section-card platform-services-loading">
      <div class="loading-spinner" aria-hidden="true" />
      <span>正在加载平台服务统计...</span>
    </section>

    <div v-else-if="stats.length" class="platform-services-layout">
      <section class="platform-services-table-shell">
        <table class="platform-services-table">
          <thead>
            <tr>
              <th>服务</th>
              <th>能力</th>
              <th>环境内</th>
              <th>KubeVirt</th>
              <th>公共引用</th>
              <th>外部连接</th>
              <th>应用</th>
              <th>环境</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in stats" :key="item.type" :class="{ selected: selectedType === item.type }">
              <td>
                <strong>{{ item.name || item.type }}</strong>
                <code>{{ item.type }}</code>
              </td>
              <td>
                <div class="feature-row">
                  <span
                    v-for="feature in featureItems(item.features)"
                    :key="feature.key"
                    class="feature-chip"
                    :class="{ disabled: !feature.enabled }"
                  >
                    {{ feature.label }}
                  </span>
                </div>
              </td>
              <td>{{ item.managedInstances }}</td>
              <td>{{ item.kubevirtInstances }}</td>
              <td>{{ item.sharedReferences }}</td>
              <td>{{ item.externalConnections }}</td>
              <td>{{ item.applicationCount }}</td>
              <td>{{ item.environmentCount }}</td>
              <td>
                <span class="status-pill" :class="{ running: item.runningInstances > 0 }">
                  {{ item.runningInstances > 0 ? `${item.runningInstances} 运行中` : '暂无运行实例' }}
                </span>
              </td>
              <td>
                <button class="row-action" type="button" @click="selectService(item)">
                  查看
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <aside v-if="selectedService" class="platform-service-detail" aria-label="服务详情右侧栏">
        <header class="detail-header">
          <div>
            <h2>{{ selectedService.name || selectedService.type }}</h2>
            <p>{{ selectedService.type }} · {{ selectedService.category || '未分类' }}</p>
          </div>
          <button class="rail-btn rail-btn--ghost" type="button" @click="selectedType = ''">关闭</button>
        </header>

        <div class="detail-summary-grid">
          <section class="detail-summary-item">
            <h3>创建方式</h3>
            <ul class="detail-list">
              <li v-for="item in selectedUsagePaths" :key="item.key" :class="{ muted: !item.enabled }">
                <span>{{ item.label }}</span>
                <strong>{{ item.enabled ? '可用' : '暂不可用' }}</strong>
              </li>
            </ul>
          </section>
          <section class="detail-summary-item">
            <h3>连接方式</h3>
            <p>{{ connectionSummary }}</p>
          </section>
          <section class="detail-summary-item">
            <h3>支持能力</h3>
            <div class="feature-row">
              <span
                v-for="feature in selectedFeatureItems"
                :key="feature.key"
                class="feature-chip"
                :class="{ disabled: !feature.enabled }"
              >
                {{ feature.label }}
              </span>
            </div>
          </section>
          <section class="detail-summary-item">
            <h3>最近使用</h3>
            <p>{{ recentUsageSummary }}</p>
          </section>
          <section class="detail-summary-item">
            <h3>告警 / 指标</h3>
            <p>{{ monitoringSummary }}</p>
            <a
              v-if="monitoringLinks.length"
              class="detail-link"
              :href="monitoringLinks[0].url"
              target="_blank"
              rel="noreferrer"
            >
              打开 Grafana
            </a>
          </section>
        </div>

        <div v-if="detailError" class="form-error" role="alert">{{ detailError }}</div>
        <div v-if="detailLoading" class="detail-loading">
          <div class="loading-spinner" aria-hidden="true" />
          <span>正在加载服务实例...</span>
        </div>

        <div v-else class="detail-grid">
          <section class="detail-section">
            <h3>实例</h3>
            <table class="detail-table">
              <thead>
                <tr>
                  <th>实例</th>
                  <th>来源</th>
                  <th>交付方式</th>
                  <th>状态</th>
                  <th>环境</th>
                  <th>使用数</th>
                  <th>监控目标</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in instances" :key="item.id">
                  <td>
                    <strong>{{ item.serviceName || item.serviceType }}</strong>
                    <code>{{ item.id }}</code>
                  </td>
                  <td>{{ sourceLabel(item.source) }}</td>
                  <td>{{ provisionModeLabel(item.provisionMode || item.source) }}</td>
                  <td>{{ item.status || '-' }}</td>
                  <td>{{ item.applicationName || '-' }} / {{ item.environmentName || '-' }}</td>
                  <td>{{ item.usageCount }}</td>
                  <td>
                    <a v-if="item.monitoringUrl" class="detail-link" :href="item.monitoringUrl" target="_blank" rel="noreferrer">
                      打开 Grafana
                    </a>
                    <code>{{ item.monitoringTarget || '-' }}</code>
                  </td>
                </tr>
                <tr v-if="!instances.length">
                  <td colspan="7" class="empty-cell">暂无实例</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section class="detail-section">
            <h3>使用方</h3>
            <table class="detail-table">
              <thead>
                <tr>
                  <th>应用 / 环境</th>
                  <th>来源</th>
                  <th>交付方式</th>
                  <th>能力</th>
                  <th>实例</th>
                  <th>连接</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in usage" :key="item.id">
                  <td>{{ item.applicationName || '-' }} / {{ item.environmentName || '-' }}</td>
                  <td>{{ sourceLabel(item.source) }}</td>
                  <td>{{ provisionModeLabel(item.provisionMode || item.source) }}</td>
                  <td>{{ item.capability || '-' }}</td>
                  <td><code>{{ item.serviceInstanceId || '-' }}</code></td>
                  <td>
                    <a v-if="item.monitoringUrl" class="detail-link" :href="item.monitoringUrl" target="_blank" rel="noreferrer">
                      打开 Grafana
                    </a>
                    <code>{{ item.endpoint || item.monitoringTarget || '-' }}</code>
                  </td>
                </tr>
                <tr v-if="!usage.length">
                  <td colspan="6" class="empty-cell">暂无使用方</td>
                </tr>
              </tbody>
            </table>
          </section>
        </div>
      </aside>
    </div>

    <section v-if="!loading && !stats.length" class="empty-panel">
      <h2>暂无平台服务数据</h2>
      <p>服务安装、共享引用或外部连接创建后会出现在这里。</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'

type PlatformServiceStat = {
  type: string
  name: string
  category: string
  features?: string | FeatureItem[]
  managedInstances: number
  kubevirtInstances: number
  sharedReferences: number
  externalConnections: number
  runningInstances: number
  applicationCount: number
  environmentCount: number
}

type FeatureItem = {
  key: string
  label: string
  enabled: boolean
}

type PlatformServiceInstance = {
  id: string
  serviceType: string
  serviceName: string
  source: string
  provisionMode?: string
  status: string
  applicationName?: string
  environmentName?: string
  monitoringTarget?: string
  monitoringUrl?: string
  usageCount: number
}

type PlatformServiceUsage = {
  id: string
  serviceType: string
  serviceName: string
  source: string
  provisionMode?: string
  status: string
  applicationName?: string
  environmentName?: string
  capability?: string
  serviceInstanceId?: string
  endpoint?: string
  monitoringTarget?: string
  monitoringUrl?: string
}

const loading = ref(false)
const error = ref('')
const stats = ref<PlatformServiceStat[]>([])
const selectedType = ref('')
const detailLoading = ref(false)
const detailError = ref('')
const instances = ref<PlatformServiceInstance[]>([])
const usage = ref<PlatformServiceUsage[]>([])

const selectedService = computed(() => stats.value.find(item => item.type === selectedType.value))
const selectedFeatureItems = computed(() => featureItems(selectedService.value?.features))
const selectedUsagePaths = computed(() => selectedFeatureItems.value.map(item => ({
  ...item,
  label: usagePathLabel(item.key, item.label),
})))
const connectionSummary = computed(() => {
  const endpointCount = new Set(usage.value.map(item => item.endpoint).filter(Boolean)).size
  const instanceEndpointCount = new Set(instances.value.map(item => item.monitoringTarget).filter(Boolean)).size
  if (endpointCount > 0) return `${endpointCount} 个连接入口来自使用关系`
  if (instanceEndpointCount > 0) return `${instanceEndpointCount} 个监控目标可作为入口线索`
  return selectedService.value?.runningInstances ? '运行实例已存在，连接信息等待实例工作区补齐' : '暂无可用连接'
})
const recentUsageSummary = computed(() => {
  const latest = usage.value[0]
  if (latest) {
    const app = latest.applicationName || '未命名应用'
    const env = latest.environmentName || '未命名环境'
    return `${app} / ${env} · ${sourceLabel(latest.source)}`
  }
  if (selectedService.value?.environmentCount) return `${selectedService.value.environmentCount} 个环境已使用`
  return '暂无使用记录'
})
const monitoringSummary = computed(() => {
  const monitoringTargets = new Set([
    ...instances.value.map(item => item.monitoringTarget).filter(Boolean),
    ...usage.value.map(item => item.monitoringTarget).filter(Boolean),
  ])
  if (monitoringLinks.value.length > 0) return `${monitoringTargets.size || monitoringLinks.value.length} 个指标目标，已发现 Grafana 入口`
  if (monitoringTargets.size > 0) return `${monitoringTargets.size} 个指标目标`
  if (selectedService.value?.runningInstances) return '运行实例存在，指标目标待发现'
  return '暂无指标目标'
})
const monitoringLinks = computed(() => {
  const seen = new Set<string>()
  return [...instances.value, ...usage.value]
    .map(item => ({
      url: String(item.monitoringUrl || '').trim(),
      label: item.serviceName || item.serviceType || 'Grafana',
    }))
    .filter(item => {
      if (!item.url || seen.has(item.url)) return false
      seen.add(item.url)
      return true
    })
})

const featureItems = (raw: PlatformServiceStat['features']): FeatureItem[] => {
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
    try {
      return featureItems(JSON.parse(raw))
    } catch {
      return []
    }
  }
  return []
}

const loadStats = async () => {
  loading.value = true
  error.value = ''
  try {
    const res = await api.platformServiceStats()
    stats.value = Array.isArray(res?.data) ? res.data : []
  } catch (e: any) {
    error.value = '加载平台服务统计失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

const sourceLabel = (source: string) => ({
  managed: '环境内',
  shared: '公共服务',
  external: '外部连接',
  deferred: '待接入',
}[source] || source || '-')

const usagePathLabel = (key: string, fallback: string) => ({
  managed: '环境内创建',
  shared: '引用公共服务',
  external: '接入外部连接',
  kubevirt: 'KubeVirt 模板交付',
}[key] || fallback || key)

const provisionModeLabel = (mode: string) => ({
  managed: '环境内托管',
  kubevirt: 'KubeVirt 模板',
  shared: '公共服务',
  external: '外部连接',
  deferred: '待接入',
}[mode] || mode || '-')

const selectService = async (item: PlatformServiceStat) => {
  selectedType.value = item.type
  detailLoading.value = true
  detailError.value = ''
  try {
    const [instancesRes, usageRes] = await Promise.all([
      api.platformServiceInstances(item.type),
      api.platformServiceUsage(item.type),
    ])
    instances.value = Array.isArray(instancesRes?.data) ? instancesRes.data : []
    usage.value = Array.isArray(usageRes?.data) ? usageRes.data : []
  } catch (e: any) {
    instances.value = []
    usage.value = []
    detailError.value = '加载服务详情失败：' + (e?.message || '未知错误')
  } finally {
    detailLoading.value = false
  }
}

onMounted(() => {
  void loadStats()
})
</script>

<style scoped>
.platform-services-page {
  min-width: 0;
}

.platform-services-loading {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--paap-muted);
}

.platform-services-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(340px, 420px);
  align-items: start;
  gap: 24px;
}

.platform-services-table-shell {
  overflow-x: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}

.platform-services-table {
  width: 100%;
  min-width: 920px;
  border-collapse: collapse;
}

.platform-services-table th,
.platform-services-table td {
  padding: 12px 14px;
  border-bottom: 1px solid var(--paap-border);
  text-align: left;
  vertical-align: top;
  font-size: var(--paap-fs-compact);
}

.platform-services-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-weight: 600;
}

.platform-services-table td strong,
.platform-services-table td code {
  display: block;
}

.platform-services-table td code {
  margin-top: 3px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.platform-services-table tr.selected td {
  background: var(--paap-accent-soft);
}

.platform-services-table th:first-child,
.platform-services-table td:first-child {
  position: sticky;
  left: 0;
  z-index: 1;
  min-width: 150px;
  background: var(--paap-panel);
}

.platform-services-table th:first-child {
  z-index: 2;
  background: var(--paap-panel-subtle);
}

.platform-services-table tr.selected td:first-child {
  background: var(--paap-accent-soft);
}

.row-action {
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  cursor: pointer;
  font-size: var(--paap-fs-compact);
  transition: border-color var(--paap-transition-fast), color var(--paap-transition-fast);
}

.row-action:hover {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
}

.platform-service-detail {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  min-width: 0;
  max-height: calc(100vh - 150px);
  overflow: auto;
  position: sticky;
  top: 16px;
}

.detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--paap-border);
}

.detail-header h2,
.detail-summary-item h3,
.detail-section h3 {
  margin: 0;
  font-size: 16px;
}

.detail-header p {
  margin: 4px 0 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

.detail-loading {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  color: var(--paap-muted);
}

.detail-summary-grid {
  display: grid;
  gap: 10px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--paap-border);
}

.detail-summary-item {
  min-width: 0;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--paap-border);
}

.detail-summary-item:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}

.detail-summary-item h3 {
  margin-bottom: 8px;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}

.detail-summary-item p {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
}

.detail-list {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.detail-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
}

.detail-list li span {
  min-width: 0;
  overflow-wrap: anywhere;
}

.detail-list li strong {
  color: var(--paap-success);
  font-size: var(--paap-fs-small);
}

.detail-list li.muted {
  color: var(--paap-muted);
}

.detail-list li.muted strong {
  color: var(--paap-muted);
}

.detail-grid {
  display: grid;
  gap: 20px;
  padding: 16px 20px;
}

.detail-section {
  min-width: 0;
  overflow-x: auto;
}

.detail-table {
  width: 100%;
  min-width: 760px;
  border-collapse: collapse;
}

.detail-table th,
.detail-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--paap-border);
  text-align: left;
  vertical-align: top;
  font-size: var(--paap-fs-label);
}

.detail-table th {
  color: var(--paap-muted);
  font-weight: 600;
  background: var(--paap-panel-subtle);
}

.detail-table td strong,
.detail-table td code {
  display: block;
}

.detail-table td code {
  margin-top: 3px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}

.detail-link {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  margin-top: 6px;
  color: var(--paap-accent);
  font-size: var(--paap-fs-small);
  font-weight: 600;
  text-decoration: none;
}

.detail-link:hover {
  text-decoration: underline;
}

.empty-cell {
  color: var(--paap-muted);
  text-align: center;
  padding: 32px 14px !important;
}

.feature-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.feature-chip {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 7px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
  font-weight: 500;
}

.feature-chip.disabled {
  opacity: 0.45;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.status-pill.running {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}

@media (max-width: 1180px) {
  .platform-services-layout {
    grid-template-columns: 1fr;
  }

  .platform-service-detail {
    position: static;
    max-height: none;
  }
}
</style>
