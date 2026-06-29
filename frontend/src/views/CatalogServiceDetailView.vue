<template>
  <div class="rail-page">
    <!-- Breadcrumb -->
    <nav class="breadcrumb">
      <router-link class="breadcrumb-link" to="/catalog">服务目录</router-link>
      <span class="breadcrumb-sep">/</span>
      <span class="breadcrumb-current">{{ product?.name || '服务详情' }}</span>
    </nav>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <!-- Loading -->
    <div v-if="loading" class="loading-section">
      <div class="loading-skeleton">
        <div class="skeleton-heading"></div>
        <div class="skeleton-desc"></div>
        <div class="skeleton-desc short"></div>
      </div>
      <div class="skeleton-blocks">
        <div v-for="n in 3" :key="n" class="skeleton-block"></div>
      </div>
    </div>

    <template v-else-if="product">
      <!-- ── Header ── -->
      <header class="detail-header">
        <div class="detail-header-left">
          <div class="detail-header-row">
            <span class="category-badge" :style="{ background: categoryColor(product.category) }">{{ categoryLabel(product.category) }}</span>
            <code class="type-code">{{ product.type }}</code>
          </div>
          <h1 class="detail-title">{{ product.name }}</h1>
        </div>
      </header>

      <div class="detail-body">
        <!-- ── Main column ── -->
        <div class="detail-main">
          <!-- Overview -->
          <section class="detail-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
              概览
            </h2>
            <p class="section-desc">{{ product.description || '暂无服务说明' }}</p>

            <div v-if="product.features.length" class="feature-list">
              <span
                v-for="f in product.features"
                :key="f.key"
                class="feature-chip"
                :class="{ disabled: !f.enabled }"
              >{{ f.label }}</span>
            </div>
          </section>

          <!-- Deploy -->
          <section class="detail-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="3" y1="3" x2="9" y2="9"/></svg>
              部署
            </h2>
            <div class="deploy-paths">
              <div class="deploy-path-card">
                <div class="deploy-path-header">
                  <span class="deploy-path-icon env-icon">📦</span>
                  <strong>环境内创建</strong>
                </div>
                <p>在业务环境中选择版本和参数，由平台通过 Helm 安装并维护该服务实例。支持配置持久卷、资源限制、网络策略等。</p>
              </div>
              <div v-if="catalogFeatureEnabled('shared')" class="deploy-path-card">
                <div class="deploy-path-header">
                  <span class="deploy-path-icon shared-icon">🔗</span>
                  <strong>引用公共服务</strong>
                </div>
                <p>从共享资源池引用平台预装实例，业务环境只保存引用和连接关系，无需单独安装和维护。</p>
              </div>
              <div v-if="catalogFeatureEnabled('external')" class="deploy-path-card">
                <div class="deploy-path-header">
                  <span class="deploy-path-icon external-icon">🌐</span>
                  <strong>接入外部连接</strong>
                </div>
                <p>录入外部 endpoint 和凭据，平台只验证连接并管理当前环境的访问配置，不负责实例生命周期。</p>
              </div>
              <div v-if="catalogFeatureEnabled('kubevirt')" class="deploy-path-card">
                <div class="deploy-path-header">
                  <span class="deploy-path-icon kubevirt-icon">🖥️</span>
                  <strong>KubeVirt 模板交付</strong>
                </div>
                <p>通过平台维护的 KubeVirt 服务模板交付数据库、缓存等服务实例，兼顾性能与平台管理能力。</p>
              </div>
            </div>

            <!-- Versions -->
            <div class="versions-block">
              <h3>可用版本</h3>
              <div v-if="product.versions.length" class="version-tags">
                <span v-for="v in product.versions" :key="v" class="version-tag">v{{ stripV(v) }}</span>
              </div>
              <p v-else class="muted-text">暂无可安装版本</p>
            </div>
          </section>

          <!-- Configuration -->
          <section class="detail-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
              配置
            </h2>
            <p class="section-desc">该服务的部署参数由平台模板管理。安装时可根据环境需求自定义资源配置、连接信息等参数。</p>
            <div class="config-note">
              <p>服务安装后可在环境画布中通过抽屉面板修改运行配置。</p>
            </div>
          </section>

          <!-- Monitoring -->
          <section class="detail-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
              监控
            </h2>
            <div class="monitor-grid">
              <div class="monitor-card">
                <div class="monitor-card-title">指标</div>
                <p>平台自动采集服务实例的 CPU、内存、网络等基础指标。可在环境画布中查看实时监控面板。</p>
              </div>
              <div class="monitor-card">
                <div class="monitor-card-title">日志</div>
                <p>服务运行日志自动汇聚到平台日志中心，支持按时间、级别、关键词检索。</p>
              </div>
            </div>
          </section>

          <!-- Environment Usage -->
          <section v-if="instances.length > 0" class="detail-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
              环境使用情况（{{ instances.length }}）
            </h2>
            <div class="table-wrap">
              <table class="env-table">
                <thead>
                  <tr>
                    <th>环境</th>
                    <th>应用</th>
                    <th>来源</th>
                    <th>状态</th>
                    <th>Endpoint</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="inst in instances" :key="inst.id">
                    <td class="env-cell">
                      <router-link v-if="inst.applicationId && inst.environmentId" :to="`/apps/${inst.applicationId}/environments/${inst.environmentId}`" class="env-link">
                        {{ inst.environmentName || inst.environmentIdentifier }}
                      </router-link>
                      <span v-else>{{ inst.environmentName || inst.environmentIdentifier }}</span>
                    </td>
                    <td>{{ inst.applicationName || '-' }}</td>
                    <td>
                      <span class="source-tag" :class="'source--' + inst.source">{{ sourceLabel(inst.source) }}{{ inst.provisionMode === 'kubevirt' ? ' (KubeVirt)' : '' }}</span>
                    </td>
                    <td>
                      <span class="status-dot" :class="'status--' + inst.status"></span>
                      <span class="status-text">{{ statusLabel(inst.status) }}</span>
                    </td>
                    <td class="endpoint-cell">
                      <code v-if="inst.endpoint" class="endpoint-code">{{ inst.endpoint }}</code>
                      <span v-else class="muted-text">-</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <!-- ── Sidebar stats ── -->
        <aside class="detail-sidebar">
          <section class="stats-panel">
            <h3 class="stats-panel-title">使用统计</h3>
            <div class="stat-row">
              <span class="stat-value">{{ product.managedInstances }}</span>
              <span class="stat-label">环境内实例</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.publicInstances }}</span>
              <span class="stat-label">公共实例</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.sharedReferences }}</span>
              <span class="stat-label">公共引用</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.externalConnections }}</span>
              <span class="stat-label">外部连接</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.kubevirtInstances }}</span>
              <span class="stat-label">KubeVirt 交付</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-row highlight">
              <span class="stat-value">{{ product.runningInstances }}</span>
              <span class="stat-label">运行中</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.environmentCount }}</span>
              <span class="stat-label">使用环境</span>
            </div>
            <div class="stat-row">
              <span class="stat-value">{{ product.applicationCount }}</span>
              <span class="stat-label">使用应用</span>
            </div>
          </section>

          <section class="versions-mini">
            <h3 class="stats-panel-title">版本</h3>
            <div v-if="product.versions.length" class="version-list">
              <span v-for="v in product.versions" :key="v" class="version-badge">v{{ stripV(v) }}</span>
            </div>
            <p v-else class="muted-text">暂无版本</p>
          </section>

          <section class="actions-mini">
            <router-link class="rail-btn rail-btn--primary" to="/catalog">← 返回目录</router-link>
          </section>
        </aside>
      </div>
    </template>

    <!-- Not found -->
    <div v-else-if="!loading && !pageError" class="not-found">
      <h2>未找到服务</h2>
      <p>指定类型的服务在目录中不存在。</p>
      <router-link class="rail-btn rail-btn--primary" to="/catalog">返回目录</router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import { usePermissionStore } from '../stores/permission'
import { stripCatalogVersionPrefix } from '../utils/catalogVersions'

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
  catalogSource?: string
  versions: string[]
  features: CatalogFeature[]
  preinstalled: boolean
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

interface ServiceInstance {
  id: string
  serviceType: string
  serviceName: string
  source: string
  provisionMode?: string
  status: string
  applicationId: number
  applicationName: string
  environmentId: number
  environmentName: string
  environmentIdentifier: string
  namespace: string
  endpoint: string
  monitoringTarget: string
  monitoringUrl: string
}

const route = useRoute()
const permissionStore = usePermissionStore()
const stripV = stripCatalogVersionPrefix

const loading = ref(true)
const pageError = ref('')
const product = ref<CatalogProduct | null>(null)
const instances = ref<ServiceInstance[]>([])

// ── Helpers ──

const categoryLabel = (cat: string) => {
  const labels: Record<string, string> = {
    ci: 'CI/CD', cd: 'CI/CD',
    monitor: '监控', log: '日志',
    database: '数据库', middleware: '中间件',
    environment: '环境', virtualMachine: '虚拟机',
    git: '代码', registry: '镜像',
    other: '其他',
  }
  return labels[cat] || cat
}

const categoryColor = (cat: string) => {
  const colors: Record<string, string> = {
    ci: '#d0e2ff', cd: '#d0e2ff',
    monitor: '#b9e6b9', log: '#a7f0e8',
    database: '#e8daff', middleware: '#fdd5a0',
    environment: '#d1d1d1', virtualMachine: '#ffb3b8',
    git: '#b4e6f0', registry: '#f5e3a0',
  }
  return colors[cat] || '#d1d1d1'
}

const sourceLabel = (source: string) => {
  const labels: Record<string, string> = {
    self: '环境内', shared: '公共', external: '外部', deferred: '待配置',
  }
  return labels[source] || source
}

const statusLabel = (s: string) => {
  const labels: Record<string, string> = {
    running: '运行中', pending: '等待', installing: '安装中',
    failed: '失败', deleting: '删除中', stopped: '已停止',
  }
  return labels[s] || s
}

const catalogFeatureEnabled = (key: string) => {
  if (!product.value) return false
  return product.value.features.some(f => f.key === key && f.enabled)
}

// ── Load data ──

const catalogFeatureItems = (raw: unknown): CatalogFeature[] => {
  const fallback = [
    { key: 'managed', label: '环境内创建', enabled: true },
    { key: 'shared', label: '公共服务', enabled: true },
  ]
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
    try { return catalogFeatureItems(JSON.parse(raw)) } catch { return fallback }
  }
  return fallback
}

const findProduct = (products: any[], type: string): CatalogProduct | null => {
  const items = new Map<string, CatalogProduct>()

  for (const t of products) {
    if (!t.type) continue
    const key = String(t.type)

    if (!items.has(key)) {
      items.set(key, {
        type: t.type,
        name: t.name || t.type,
        description: t.description || '',
        category: t.category || '',
        catalogSource: t.catalogSource || '',
        versions: Array.isArray(t.versions) ? t.versions.map((v: any) => String(v)).filter(Boolean) : [],
        features: catalogFeatureItems(t.features),
        preinstalled: Number(t.publicInstances || 0) > 0,
        managedInstances: 0,
        kubevirtInstances: 0,
        publicInstances: 0,
        sharedReferences: 0,
        externalConnections: 0,
        deferredReferences: 0,
        runningInstances: 0,
        applicationCount: 0,
        environmentCount: 0,
      })
    }

    const item = items.get(key)!
    item.preinstalled = item.preinstalled || Number(t.publicInstances || 0) > 0
    item.managedInstances += Number(t.managedInstances || 0)
    item.kubevirtInstances += Number(t.kubevirtInstances || 0)
    item.publicInstances += Number(t.publicInstances || 0)
    item.sharedReferences += Number(t.sharedReferences || 0)
    item.externalConnections += Number(t.externalConnections || 0)
    item.deferredReferences += Number(t.deferredReferences || 0)
    item.runningInstances += Number(t.runningInstances || 0)
    item.applicationCount += Number(t.applicationCount || 0)
    item.environmentCount += Number(t.environmentCount || 0)
    if (t.appVersion && !item.versions.includes(t.appVersion)) {
      item.versions.push(t.appVersion)
    }
  }

  return items.get(type) || null
}

onMounted(async () => {
  const type = String(route.params.type || '')
  if (!type) {
    pageError.value = '未指定服务类型'
    loading.value = false
    return
  }

  loading.value = true
  pageError.value = ''
  try {
    const catalogResult = await api.listCatalogServices()
    const serviceItems = Array.isArray(catalogResult) ? catalogResult : (catalogResult?.data ? (Array.isArray(catalogResult.data) ? catalogResult.data : []) : [])
    product.value = findProduct(serviceItems, type)

    // Load instances if permission allows
    if (permissionStore.has('system.shared_pool.manage')) {
      try {
        const instancesResult = await api.platformServiceInstances(type)
        instances.value = Array.isArray(instancesResult.data) ? instancesResult.data : []
      } catch {
        // Non-critical — silently skip
      }
    }
  } catch (e: any) {
    pageError.value = '加载服务详情失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
/* ── Layout ── */
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-12);
}

/* ── Breadcrumb ── */
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 20px;
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
.breadcrumb-sep {
  color: var(--paap-muted);
}
.breadcrumb-current {
  color: var(--paap-text);
  font-weight: 600;
}

/* ── Error ── */
.page-error {
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  margin-bottom: 16px;
}

/* ── Loading ── */
.loading-section {
  display: grid;
  gap: 20px;
}
.loading-skeleton {
  display: grid;
  gap: 10px;
}
.loading-skeleton .skeleton-heading {
  height: 28px;
  width: 240px;
  border-radius: var(--paap-radius-xs);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}
.loading-skeleton .skeleton-desc {
  height: 14px;
  width: 80%;
  border-radius: var(--paap-radius-xs);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}
.loading-skeleton .skeleton-desc.short { width: 50%; }
.skeleton-blocks {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.skeleton-block {
  height: 100px;
  border-radius: var(--paap-radius);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
}
@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

/* ── Header ── */
.detail-header {
  margin-bottom: 24px;
}
.detail-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.category-badge {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: var(--paap-radius-xs, 4px);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  color: var(--paap-text);
}
.type-code {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
  padding: 2px 6px;
  border-radius: var(--paap-radius-xs);
  font-family: var(--paap-mono);
}
.detail-title {
  font-size: var(--paap-fs-heading-2xl);
  font-weight: 700;
  color: var(--paap-text);
  line-height: 1.2;
  margin: 0;
}

/* ── Body layout ── */
.detail-body {
  display: grid;
  grid-template-columns: 1fr 280px;
  gap: 24px;
  align-items: start;
}
.detail-main {
  display: grid;
  gap: 28px;
  min-width: 0;
}

/* ── Sections ── */
.detail-section {
  display: grid;
  gap: 12px;
}
.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--paap-text);
  margin: 0;
  line-height: 1.3;
}
.section-title svg {
  color: var(--paap-muted);
  flex-shrink: 0;
}
.section-desc {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  line-height: 1.6;
}

/* ── Features ── */
.feature-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.feature-chip {
  display: inline-flex;
  align-items: center;
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 500;
}
.feature-chip.disabled {
  opacity: 0.45;
}

/* ── Deploy ── */
.deploy-paths {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 10px;
}
.deploy-path-card {
  padding: 16px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  transition: box-shadow 0.15s;
}
.deploy-path-card:hover {
  box-shadow: var(--paap-shadow-sm);
}
.deploy-path-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  font-size: var(--paap-fs-body);
}
.deploy-path-icon { font-size: 18px; }
.deploy-path-card p {
  margin: 0;
  font-size: var(--paap-fs-compact);
  color: var(--paap-muted);
  line-height: 1.5;
}

.versions-block {
  display: grid;
  gap: 8px;
}
.versions-block h3 {
  margin: 0;
  font-size: var(--paap-fs-body);
  font-weight: 600;
  color: var(--paap-text);
}
.version-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.version-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: var(--paap-radius-xs, 4px);
  font-size: var(--paap-fs-label);
  font-weight: 500;
  background: var(--paap-accent-soft, rgba(15,98,254,0.08));
  color: var(--paap-accent, #0f62fe);
}
.muted-text {
  margin: 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  font-style: italic;
}

/* ── Config note ── */
.config-note {
  padding: 12px 14px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-info-bg, #edf5ff);
}
.config-note p {
  margin: 0;
  font-size: var(--paap-fs-compact);
  color: var(--paap-muted);
  line-height: 1.5;
}

/* ── Monitoring ── */
.monitor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
}
.monitor-card {
  padding: 14px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.monitor-card-title {
  font-weight: 600;
  font-size: var(--paap-fs-body);
  color: var(--paap-text);
  margin-bottom: 4px;
}
.monitor-card p {
  margin: 0;
  font-size: var(--paap-fs-compact);
  color: var(--paap-muted);
  line-height: 1.5;
}

/* ── Environment table ── */
.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
}
.env-table {
  width: 100%;
  border-collapse: collapse;
}
.env-table {
  width: 100%;
  border-collapse: collapse;
}
.env-table th,
.env-table td {
  padding: 10px 14px;
  text-align: left;
  vertical-align: middle;
  border-bottom: 1px solid var(--paap-border);
  font-size: var(--paap-fs-compact);
}
.env-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.env-table tbody tr:hover {
  background: var(--paap-accent-soft, rgba(15,98,254,0.04));
}
.env-link {
  color: var(--paap-accent, #0f62fe);
  text-decoration: none;
  font-weight: 500;
}
.env-link:hover {
  text-decoration: underline;
}
.source-tag {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--paap-radius-xs, 4px);
  font-size: var(--paap-fs-small);
  font-weight: 600;
}
.source--self {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.source--shared {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}
.source--external {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}
.status-dot.status--running { background: var(--paap-success); }
.status-dot.status--pending { background: var(--paap-warning); }
.status-dot.status--failed { background: var(--paap-danger); }
.status-text {
  vertical-align: middle;
  color: var(--paap-text);
}
.endpoint-code {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  word-break: break-all;
}

/* ── Sidebar ── */
.detail-sidebar {
  display: grid;
  gap: 16px;
  position: sticky;
  top: 20px;
}
.stats-panel,
.versions-mini,
.actions-mini {
  padding: 16px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}
.stats-panel-title {
  margin: 0 0 12px;
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  color: var(--paap-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 7px 0;
}
.stat-row.highlight {
  padding: 10px 0;
}
.stat-value {
  font-size: 18px;
  font-weight: 700;
  color: var(--paap-text);
  line-height: 1;
  font-variant-numeric: tabular-nums;
}
.stat-row.highlight .stat-value {
  color: var(--paap-accent);
}
.stat-label {
  font-size: var(--paap-fs-compact);
  color: var(--paap-muted);
}
.stat-divider {
  height: 1px;
  background: var(--paap-border);
  margin: 6px 0;
}

.version-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.version-badge {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--paap-radius-xs, 4px);
  font-size: var(--paap-fs-small);
  background: var(--paap-accent-soft, rgba(15,98,254,0.08));
  color: var(--paap-accent, #0f62fe);
  font-weight: 500;
}

.actions-mini {
  text-align: center;
}
.actions-mini .rail-btn {
  width: 100%;
}

/* ── Not found ── */
.not-found {
  text-align: center;
  padding: 60px 20px;
}
.not-found h2 {
  font-size: 20px;
  font-weight: 600;
  color: var(--paap-text);
  margin-bottom: 8px;
}
.not-found p {
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  margin-bottom: 20px;
}

/* ── Responsive ── */
@media (max-width: 720px) {
  .rail-page {
    padding: var(--paap-space-4) var(--paap-space-4) var(--paap-space-8);
  }
  .detail-body {
    grid-template-columns: 1fr;
  }
  .detail-sidebar {
    position: static;
    order: -1;
  }
  .skeleton-blocks {
    grid-template-columns: 1fr;
  }
}
</style>
