<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">服务目录</h1>
        <p class="page-desc">平台支持的服务与中间件一览<template v-if="hasCatalogItems">（共 {{ totalItems }} 个）</template></p>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <!-- Skeleton loading -->
    <div v-if="loading" class="catalog-skeleton">
      <div class="catalog-skeleton-bar"></div>
      <div class="catalog-skeleton-grid">
        <div v-for="n in 6" :key="n" class="catalog-skeleton-card">
          <div class="catalog-skeleton-row">
            <div class="catalog-skeleton-icon"></div>
            <div class="catalog-skeleton-texts">
              <div class="catalog-skeleton-line skeleton-line--long"></div>
              <div class="catalog-skeleton-line skeleton-line--short"></div>
            </div>
          </div>
          <div class="catalog-skeleton-desc">
            <div class="catalog-skeleton-line skeleton-line--full"></div>
            <div class="catalog-skeleton-line skeleton-line--medium"></div>
          </div>
          <div class="catalog-skeleton-tags">
            <div class="catalog-skeleton-tag"></div>
            <div class="catalog-skeleton-tag"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Search bar -->
    <div v-if="hasCatalogItems" class="catalog-search">
      <div class="catalog-search-icon">
        <svg width="14" height="14" viewBox="0 0 32 32" fill="currentColor"><path d="M30 28.6L22.4 21c1.2-1.5 1.9-3.4 1.9-5.4 0-4.8-3.9-8.7-8.7-8.7S6.9 10.9 6.9 15.7s3.9 8.7 8.7 8.7c2 0 3.9-.7 5.4-1.9l7.6 7.6 1.4-1.5zM9 15.7c0-3.7 3-6.7 6.7-6.7s6.7 3 6.7 6.7-3 6.7-6.7 6.7S9 19.4 9 15.7z"/></svg>
      </div>
      <input
        ref="searchInputRef"
        v-model="filterQuery"
        class="catalog-search-input"
        type="text"
        placeholder="搜索名称、类型、分组或描述..."
        @keydown.esc="clearCatalogSearch"
      />
      <button v-if="filterQuery" class="catalog-search-clear" @click="clearCatalogSearch" title="清除搜索">
        <svg width="12" height="12" viewBox="0 0 32 32" fill="currentColor"><path d="M24 9.4L22.6 8 16 14.6 9.4 8 8 9.4l6.6 6.6L8 22.6 9.4 24l6.6-6.6 6.6 6.6 1.4-1.4-6.6-6.6L24 9.4z"/></svg>
      </button>
    </div>

    <!-- Tab bar -->
    <div v-if="availableTabs.length" class="catalog-tabs">
      <button
        v-for="tab in availableTabs"
        :key="tab.key"
        class="catalog-tab"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        <span class="catalog-tab-icon">{{ tab.icon }}</span>
        <span class="catalog-tab-label">{{ tab.label }}</span>
        <span class="catalog-tab-count">{{ tab.count }}</span>
      </button>
    </div>

    <div v-if="availableTabs.length" class="catalog-content">
      <!-- Tab panels -->
      <div class="catalog-results">
        <template v-for="group in catalogGroups" :key="group.category">
          <section v-show="activeTab === group.category" class="catalog-section">
            <div v-if="group.items.length === 0" class="no-data">暂无数据</div>
            <div v-else class="catalog-grid">
              <article
                v-for="item in group.items"
                :key="item.type"
                class="catalog-card"
                role="link"
                tabindex="0"
                :aria-label="`查看 ${item.name} 服务详情`"
                @click="navigateToDetail(item.type)"
                @keydown.enter.prevent="navigateToDetail(item.type)"
                @keydown.space.prevent="navigateToDetail(item.type)"
              >
            <div class="catalog-card-header">
              <span class="catalog-card-icon">{{ group.icon }}</span>
              <div class="catalog-card-info">
                <strong class="catalog-card-name">{{ item.name }}</strong>
              </div>
            </div>
            <p class="catalog-card-desc">{{ item.description }}</p>
            <div v-if="item.features.length" class="catalog-feature-row" aria-label="支持能力">
              <span v-if="item.preinstalled" class="catalog-feature-chip catalog-feature-chip--preinstalled">
                平台已预装
              </span>
              <span
                v-for="feature in item.features"
                :key="feature.key"
                class="catalog-feature-chip"
                :class="{ disabled: !feature.enabled }"
              >
                {{ feature.label }}
              </span>
            </div>
            <div class="catalog-card-footer">
              <div class="catalog-versions">
                <span v-if="item.versions.length">
                  <span v-for="v in item.versions" :key="v" class="version-tag">v{{ stripV(v) }}</span>
                </span>
                <span v-else class="catalog-versions--empty">暂无版本</span>
              </div>
              <div class="card-stats">
                <span class="card-stat" title="使用环境">{{ item.environmentCount }} 环境</span>
                <span class="card-stat-dot"></span>
                <span class="card-stat" title="运行中实例">{{ item.runningInstances }} 运行中</span>
              </div>
            </div>
              </article>
            </div>
          </section>
        </template>
      </div>

      <!-- Detail moved to /catalog/:type detail page -->
    </div>

    <div v-if="!loading && hasCatalogItems && filterQuery.trim() && catalogGroups.length === 0" class="catalog-empty-search">
      <strong>没有匹配的服务或中间件</strong>
      <span>当前目录中没有包含“{{ filterQuery.trim() }}”的名称、类型、分组或描述。</span>
      <button type="button" class="catalog-empty-clear" @click="clearCatalogSearch">清除搜索</button>
    </div>

    <p v-if="!loading && !hasCatalogItems && !pageError" class="no-data">没有找到服务数据</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watchEffect, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import { catalogGroupForTemplate, catalogTemplateMatchesQuery, compareCatalogGroupMeta, type CatalogGroupMeta } from '../utils/catalogGroups'
import { compareCatalogVersions, stripCatalogVersionPrefix } from '../utils/catalogVersions'

interface CatalogItem {
  type: string
  name: string
  description: string
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
interface CatalogFeature {
  key: string
  label: string
  enabled: boolean
}
interface CatalogGroup extends CatalogGroupMeta {
  items: CatalogItem[]
}

const router = useRouter()
const loading = ref(false)
const pageError = ref('')
const catalogProducts = ref<any[]>([])
const activeTab = ref('')
const filterQuery = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)

const stripV = stripCatalogVersionPrefix

const availableTabs = computed(() =>
  catalogGroups.value.map(g => ({
    key: g.category,
    label: g.label,
    icon: g.icon,
    count: g.items.length,
  }))
)

const filteredTemplates = computed(() => {
  const q = filterQuery.value.trim().toLowerCase()
  if (!q) return catalogProducts.value
  return catalogProducts.value.filter((t: any) => catalogTemplateMatchesQuery(t, q))
})
const hasCatalogItems = computed(() => catalogProducts.value.length > 0)
const totalItems = computed(() => filteredTemplates.value.length)

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
    try {
      return catalogFeatureItems(JSON.parse(raw))
    } catch {
      return fallback
    }
  }
  return fallback
}

const catalogGroups = computed<CatalogGroup[]>(() => {
  const groups = new Map<string, CatalogGroup>()
  const items = new Map<string, CatalogItem>()
  const source = filteredTemplates.value

  for (const t of source) {
    if (!t.type) continue
    const key = String(t.type)

    if (!items.has(key)) {
      items.set(key, {
        type: t.type,
        name: t.name || t.type,
        description: t.description || '',
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

  for (const item of items.values()) {
    item.versions.sort(compareCatalogVersions)
    const t = source.find((x: any) => x.type === item.type)
    const groupMeta = catalogGroupForTemplate(t || item)
    const cat = groupMeta.category

    if (!groups.has(cat)) {
      groups.set(cat, { ...groupMeta, items: [] })
    }
    groups.get(cat)!.items.push(item)
  }

  return Array.from(groups.values()).sort(compareCatalogGroupMeta)
})

const navigateToDetail = (type: string) => {
  router.push(`/catalog/${encodeURIComponent(type)}`)
}

// Auto-select first tab when data loads, or if current tab becomes invalid
watchEffect(() => {
  const groups = catalogGroups.value
  if (groups.length > 0) {
    const current = activeTab.value
    if (!current || !groups.some(g => g.category === current)) {
      activeTab.value = groups[0].category
    }
  }
  // No auto-select needed — card click navigates to detail page
})

const clearCatalogSearch = () => {
  filterQuery.value = ''
  void nextTick(() => searchInputRef.value?.focus())
}

const catalogSearchShortcutHandler = (e: KeyboardEvent) => {
  if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes((e.target as HTMLElement)?.tagName)) {
    e.preventDefault()
    searchInputRef.value?.focus()
  }
}

onMounted(async () => {
  document.addEventListener('keydown', catalogSearchShortcutHandler)
  loading.value = true
  pageError.value = ''
  await nextTick() // ensure skeleton renders before api call
  try {
    const [catalogResult, envTemplateResult] = await Promise.all([
      api.listCatalogServices(),
      api.templates(),
    ])
    const serviceItems = Array.isArray(catalogResult) ? catalogResult : (catalogResult?.data ? (Array.isArray(catalogResult.data) ? catalogResult.data : []) : [])
    const envTemplates = Array.isArray(envTemplateResult?.data) ? envTemplateResult.data : []
    const environmentItems = envTemplates.map((tmpl: any) => ({
      type: `environment-template-${tmpl.id || tmpl.name}`,
      name: tmpl.name || '环境服务',
      description: tmpl.description || '环境服务产品',
      category: 'environment',
      catalogSource: 'environment-template',
      features: [
        { key: 'managed', label: '从环境服务创建', enabled: true },
      ],
      managedInstances: 0,
      kubevirtInstances: 0,
      publicInstances: 0,
      sharedReferences: 0,
      externalConnections: 0,
      deferredReferences: 0,
      runningInstances: 0,
      applicationCount: 0,
      environmentCount: 0,
      versions: [],
    }))
    catalogProducts.value = [...serviceItems, ...environmentItems]
  } catch (e: any) {
    pageError.value = '加载服务目录失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
})

onBeforeUnmount(() => document.removeEventListener('keydown', catalogSearchShortcutHandler))
</script>

<style scoped>
/* ===== Page layout ===== */
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
  max-width: none;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 16px;
}
.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: 0;
  line-height: 1.2;
}
.page-desc {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  line-height: 1.4;
}
.page-error {
  border: 1px solid var(--paap-danger);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  border-radius: var(--paap-radius-sm);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  line-height: 1.4;
  margin-bottom: 16px;
}

/* ===== Skeleton loading ===== */
@keyframes cds-skeleton-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
.catalog-skeleton {
  margin-bottom: 2rem;
}
.catalog-skeleton-bar {
  height: 40px;
  margin-bottom: 1rem;
  border-radius: var(--paap-radius);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}
.catalog-skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 0.75rem;
}
.catalog-skeleton-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.catalog-skeleton-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.catalog-skeleton-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--paap-radius-sm);
  flex-shrink: 0;
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}
.catalog-skeleton-texts {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.catalog-skeleton-line {
  height: 12px;
  border-radius: var(--paap-radius-xs);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}
.catalog-skeleton-desc {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.skeleton-line--long { width: 80%; }
.skeleton-line--short { width: 45%; }
.skeleton-line--full { width: 100%; }
.skeleton-line--medium { width: 65%; }
.catalog-skeleton-tags {
  display: flex;
  gap: 0.375rem;
  margin-top: auto;
}
.catalog-skeleton-tag {
  width: 56px;
  height: 20px;
  border-radius: var(--paap-radius-xs, 4px);
  background: linear-gradient(90deg, var(--paap-panel-subtle) 25%, var(--paap-border) 50%, var(--paap-panel-subtle) 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}

/* ===== Search bar ===== */
.catalog-search {
  display: flex;
  align-items: center;
  gap: 0;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: 0 0.75rem;
  margin-bottom: 0.75rem;
  background: var(--paap-panel);
  transition: border-color 0.15s;
  overflow: hidden;
}
.catalog-search:focus-within {
  border-color: var(--paap-accent);
  outline: 1px solid var(--paap-accent);
}
.catalog-search-icon {
  display: flex;
  align-items: center;
  color: var(--paap-muted);
  flex-shrink: 0;
}
.catalog-search-input {
  flex: 1;
  border: none;
  outline: none;
  background: none;
  font-size: 0.875rem;
  padding: 0.5rem 0.5rem;
  color: var(--paap-text);
  min-width: 0;
}
.catalog-search-input::placeholder {
  color: var(--paap-muted);
}
.catalog-search-clear {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--paap-muted);
  padding: 0.25rem;
  border-radius: var(--paap-radius-sm);
  flex-shrink: 0;
}
.catalog-search-clear:hover {
  color: var(--paap-text);
}

/* ===== Tab bar ===== */
.catalog-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--paap-border);
  margin-bottom: 1.25rem;
}
.catalog-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.625rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--paap-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
  white-space: nowrap;
}
.catalog-tab:hover {
  color: var(--paap-text);
  border-bottom-color: var(--paap-muted);
}
.catalog-tab.active {
  color: var(--paap-text);
  border-bottom-color: var(--paap-accent);
  font-weight: 600;
}
.catalog-tab-icon {
  font-size: 1rem;
  line-height: 1;
}
.catalog-tab-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.125rem;
  height: 1.125rem;
  padding: 0 0.3125rem;
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  font-size: 0.6875rem;
  font-weight: 600;
  color: var(--paap-muted);
  line-height: 1;
}
.catalog-tab.active .catalog-tab-count {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}

/* ===== Section / Grid / Card ===== */
.catalog-content {
  display: block;
}
.catalog-results {
  min-width: 0;
}
.catalog-section {
  margin-bottom: 2rem;
}
.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1rem;
}
.catalog-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: 1.125rem;
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
  transition: box-shadow 0.2s ease, transform 0.2s ease;
  will-change: transform;
  cursor: pointer;
}
.catalog-card:hover {
  box-shadow: var(--paap-shadow-lg);
  transform: translateY(-2px);
}
.catalog-card:focus-visible,
.catalog-card.selected {
  border-color: var(--paap-accent);
  box-shadow: 0 0 0 1px var(--paap-accent);
  outline: none;
}
.catalog-card-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.catalog-card-icon {
  font-size: 1.5rem;
  width: 2rem;
  text-align: center;
  flex-shrink: 0;
}
.catalog-card-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.catalog-card-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--paap-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.catalog-card-type {
  align-self: flex-start;
  font-size: 0.6875rem;
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
  padding: 0.0625rem 0.375rem;
  border-radius: var(--paap-radius-xs, 4px);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 10rem;
  margin-top: 0.0625rem;
}
.catalog-card-desc {
  font-size: 0.8125rem;
  color: var(--paap-muted);
  margin: 0;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.catalog-feature-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
.catalog-feature-chip {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 0.5rem;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1;
}
.catalog-feature-chip.disabled {
  opacity: 0.45;
}
.catalog-feature-chip--preinstalled {
  border-color: var(--paap-success-border, #24a148);
  background: var(--paap-success-soft, #defbe6);
  color: var(--paap-success-text, #0e6027);
}
.catalog-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 0.5rem;
  border-top: 1px solid var(--paap-border);
}
.catalog-versions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  min-width: 0;
  flex: 1;
}
.catalog-versions--empty {
  font-size: 0.75rem;
  color: var(--paap-border);
  font-style: italic;
}
.version-tag {
  display: inline-block;
  font-size: 0.6875rem;
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  padding: 0.125rem 0.375rem;
  border-radius: var(--paap-radius-xs, 4px);
  font-weight: 500;
}
.card-stats {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.card-stat {
  font-size: 0.75rem;
  color: var(--paap-muted);
  white-space: nowrap;
}
.card-stat-dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--paap-border-strong);
}
/* Detail moved to /catalog/:type detail page */
.no-data {
  padding: 2rem 0;
  color: var(--paap-muted);
  font-size: 0.875rem;
}
.catalog-empty-search {
  display: grid;
  gap: 0.5rem;
  padding: 1.25rem;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: 0.875rem;
  line-height: 1.45;
}
.catalog-empty-search strong {
  color: var(--paap-text);
  font-size: 0.9375rem;
  font-weight: 600;
}
.catalog-empty-clear {
  justify-self: start;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--paap-border-strong);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  color: var(--paap-text);
  font: inherit;
  font-size: 0.875rem;
  cursor: pointer;
}
.catalog-empty-clear:hover {
  background: var(--paap-accent-fill);
}

@media (max-width: 672px) {
  .rail-page {
    padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10);
  }
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .catalog-tabs {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
  .catalog-content {
    display: block;
  }
}
</style>
