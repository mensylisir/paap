<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">中间件目录</h1>
        <p class="page-desc">平台支持的中间件与工具一览<template v-if="hasCatalogItems">（共 {{ totalItems }} 个）</template></p>
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
        placeholder="搜索中间件或工具名称..."
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

    <!-- Tab panels -->
    <template v-for="group in catalogGroups" :key="group.category">
      <section v-show="activeTab === group.category" class="catalog-section">
        <div v-if="group.items.length === 0" class="no-data">暂无数据</div>
        <div v-else class="catalog-grid">
          <article v-for="item in group.items" :key="item.type" class="catalog-card">
            <div class="catalog-card-header">
              <span class="catalog-card-icon">{{ group.icon }}</span>
              <div class="catalog-card-info">
                <strong class="catalog-card-name">{{ item.name }}</strong>
                <code class="catalog-card-type">{{ item.type }}</code>
              </div>
            </div>
            <p class="catalog-card-desc">{{ item.description }}</p>
            <div class="catalog-card-footer">
              <span v-if="item.versions.length" class="catalog-versions">
                <span v-for="v in item.versions" :key="v" class="version-tag">v{{ stripV(v) }}</span>
              </span>
              <span v-else class="catalog-versions catalog-versions--empty">暂无版本</span>
            </div>
          </article>
        </div>
      </section>
    </template>

    <div v-if="!loading && hasCatalogItems && filterQuery.trim() && catalogGroups.length === 0" class="catalog-empty-search">
      <strong>没有匹配的中间件或工具</strong>
      <span>当前目录中没有包含“{{ filterQuery.trim() }}”的名称、类型或描述。</span>
      <button type="button" class="catalog-empty-clear" @click="clearCatalogSearch">清除搜索</button>
    </div>

    <p v-if="!loading && !hasCatalogItems && !pageError" class="no-data">没有找到中间件数据</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watchEffect, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { api } from '../api/client'

interface CatalogItem {
  type: string
  name: string
  description: string
  versions: string[]
}
interface CatalogGroup {
  category: string
  label: string
  icon: string
  items: CatalogItem[]
}

const loading = ref(false)
const pageError = ref('')
const templates = ref<any[]>([])
const activeTab = ref('')
const filterQuery = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)

const stripV = (v: string) => v.replace(/^v/i, '')

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
  if (!q) return templates.value
  return templates.value.filter((t: any) =>
    String(t.name || '').toLowerCase().includes(q) ||
    String(t.type || '').toLowerCase().includes(q) ||
    String(t.description || '').toLowerCase().includes(q)
  )
})
const hasCatalogItems = computed(() => templates.value.length > 0)
const totalItems = computed(() => filteredTemplates.value.length)

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
        versions: [],
      })
    }

    const item = items.get(key)!
    if (t.appVersion && !item.versions.includes(t.appVersion)) {
      item.versions.push(t.appVersion)
    }
  }

  for (const item of items.values()) {
    item.versions.sort()
    const t = source.find((x: any) => x.type === item.type)
    const cat = String(t?.category || 'other')

    if (!groups.has(cat)) {
      let label = cat
      let icon = '📦'
      if (cat === 'tool') { label = '工具类'; icon = '🔧' }
      else if (cat === 'infra') { label = '中间件 / 数据库'; icon = '🗄️' }
      else if (cat === 'middleware') { label = '中间件 / 数据库'; icon = '🗄️' }

      groups.set(cat, { category: cat, label, icon, items: [] })
    }
    groups.get(cat)!.items.push(item)
  }

  return Array.from(groups.values())
})

// Auto-select first tab when data loads, or if current tab becomes invalid
watchEffect(() => {
  const groups = catalogGroups.value
  if (groups.length > 0) {
    const current = activeTab.value
    if (!current || !groups.some(g => g.category === current)) {
      activeTab.value = groups[0].category
    }
  }
})

const clearCatalogSearch = () => {
  filterQuery.value = ''
  void nextTick(() => searchInputRef.value?.focus())
}

onMounted(async () => {
  loading.value = true
  pageError.value = ''
  await nextTick() // ensure skeleton renders before api call
  try {
    const data = await api.listServiceTemplates()
    templates.value = Array.isArray(data) ? data : (data?.data ? (Array.isArray(data.data) ? data.data : []) : [])
  } catch (e: any) {
    pageError.value = '加载中间件目录失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }

  // Press "/" to focus search input
  const handler = (e: KeyboardEvent) => {
    if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes((e.target as HTMLElement)?.tagName)) {
      e.preventDefault()
      searchInputRef.value?.focus()
    }
  }
  document.addEventListener('keydown', handler)
  onBeforeUnmount(() => document.removeEventListener('keydown', handler))
})
</script>

<style scoped>
/* ===== Page layout ===== */
.rail-page {
  padding: 20px 20px 36px;
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
  color: var(--cds-text-primary, #161616);
  letter-spacing: 0;
  line-height: 1.2;
}
.page-desc {
  font-size: 14px;
  color: var(--cds-text-secondary, #525252);
  line-height: 1.4;
}
.page-error {
  border: 1px solid var(--cds-red-60, #da1e28);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-red-60, #da1e28);
  border-radius: 0;
  padding: 10px 12px;
  font-size: 13px;
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
  border-radius: 0;
  background: linear-gradient(90deg, #f4f4f4 25%, #e8e8e8 50%, #f4f4f4 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}
.catalog-skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 0.75rem;
}
.catalog-skeleton-card {
  background: #fff;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
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
  border-radius: 0;
  flex-shrink: 0;
  background: linear-gradient(90deg, #f4f4f4 25%, #e8e8e8 50%, #f4f4f4 75%);
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
  border-radius: 0;
  background: linear-gradient(90deg, #f4f4f4 25%, #e8e8e8 50%, #f4f4f4 75%);
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
  border-radius: 3px;
  background: linear-gradient(90deg, #f4f4f4 25%, #e8e8e8 50%, #f4f4f4 75%);
  background-size: 200% 100%;
  animation: cds-skeleton-shimmer 1.5s ease-in-out infinite;
}

/* ===== Search bar ===== */
.catalog-search {
  display: flex;
  align-items: center;
  gap: 0;
  border: 1px solid #c6c6c6;
  border-radius: 0;
  padding: 0 0.75rem;
  margin-bottom: 0.75rem;
  background: #fff;
  transition: border-color 0.15s;
}
.catalog-search:focus-within {
  border-color: #0f62fe;
  outline: 1px solid #0f62fe;
}
.catalog-search-icon {
  display: flex;
  align-items: center;
  color: #6f6f6f;
  flex-shrink: 0;
}
.catalog-search-input {
  flex: 1;
  border: none;
  outline: none;
  background: none;
  font-size: 0.875rem;
  padding: 0.5rem 0.5rem;
  color: #161616;
  min-width: 0;
}
.catalog-search-input::placeholder {
  color: #a8a8a8;
}
.catalog-search-clear {
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  color: #6f6f6f;
  padding: 0.25rem;
  border-radius: 0;
  flex-shrink: 0;
}
.catalog-search-clear:hover {
  color: #161616;
}

/* ===== Tab bar ===== */
.catalog-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid #e0e0e0;
  margin-bottom: 1.25rem;
}
.catalog-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.625rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #525252;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
  white-space: nowrap;
}
.catalog-tab:hover {
  color: #161616;
  border-bottom-color: #a8a8a8;
}
.catalog-tab.active {
  color: #161616;
  border-bottom-color: #0f62fe;
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
  border-radius: 0.625rem;
  background: #e0e0e0;
  font-size: 0.6875rem;
  font-weight: 600;
  color: #525252;
  line-height: 1;
}
.catalog-tab.active .catalog-tab-count {
  background: #d0e2ff;
  color: #0f62fe;
}

/* ===== Section / Grid / Card ===== */
.catalog-section {
  margin-bottom: 2rem;
}
.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 0.75rem;
}
.catalog-card {
  background: #fff;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  transition: box-shadow 0.2s ease, transform 0.2s ease;
  will-change: transform;
  cursor: pointer;
}
.catalog-card:hover {
  box-shadow: 0 2px 6px rgba(0,0,0,0.1);
  transform: translateY(-2px);
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
  color: #161616;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.catalog-card-type {
  font-size: 0.75rem;
  color: #6f6f6f;
  background: #f4f4f4;
  padding: 0.125rem 0.375rem;
  border-radius: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 12rem;
  margin-top: 0.125rem;
}
.catalog-card-desc {
  font-size: 0.8125rem;
  color: #525252;
  margin: 0;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.catalog-card-footer {
  margin-top: auto;
  padding-top: 0.25rem;
}
.catalog-versions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
.catalog-versions--empty {
  font-size: 0.75rem;
  color: #c6c6c6;
  font-style: italic;
}
.version-tag {
  display: inline-block;
  font-size: 0.6875rem;
  background: #e8f0fe;
  color: #1967d2;
  padding: 0.125rem 0.375rem;
  border-radius: 3px;
  font-weight: 500;
}
.no-data {
  padding: 2rem 0;
  color: #6f6f6f;
  font-size: 0.875rem;
}
.catalog-empty-search {
  display: grid;
  gap: 0.5rem;
  padding: 1.25rem;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-secondary, #525252);
  font-size: 0.875rem;
  line-height: 1.45;
}
.catalog-empty-search strong {
  color: var(--cds-text-primary, #161616);
  font-size: 0.9375rem;
  font-weight: 600;
}
.catalog-empty-clear {
  justify-self: start;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--cds-border-strong-01, #8d8d8d);
  border-radius: 0;
  background: var(--cds-layer-01, #ffffff);
  color: var(--cds-text-primary, #161616);
  font: inherit;
  font-size: 0.875rem;
  cursor: pointer;
}
.catalog-empty-clear:hover {
  background: var(--cds-background-hover, rgba(141, 141, 141, 0.12));
}

@media (max-width: 672px) {
  .rail-page {
    padding: 20px 20px 32px;
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
}
</style>
