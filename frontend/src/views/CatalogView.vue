<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">中间件目录</h1>
        <p class="page-desc">平台支持的中间件与工具一览</p>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>
    <div v-if="loading" class="loading-text">加载中...</div>

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

    <p v-if="!loading && catalogGroups.length === 0 && !pageError" class="no-data">没有找到中间件数据</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watchEffect, onMounted } from 'vue'
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

const stripV = (v: string) => v.replace(/^v/i, '')

const availableTabs = computed(() =>
  catalogGroups.value.map(g => ({
    key: g.category,
    label: g.label,
    icon: g.icon,
    count: g.items.length,
  }))
)

const catalogGroups = computed<CatalogGroup[]>(() => {
  const groups = new Map<string, CatalogGroup>()
  const items = new Map<string, CatalogItem>()

  for (const t of templates.value) {
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
    const t = templates.value.find((x: any) => x.type === item.type)
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

onMounted(async () => {
  loading.value = true
  pageError.value = ''
  try {
    const data = await api.listServiceTemplates()
    templates.value = Array.isArray(data) ? data : (data?.data ? (Array.isArray(data.data) ? data.data : []) : [])
  } catch (e: any) {
    pageError.value = '加载中间件目录失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
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

.loading-text {
  padding: 2rem 0;
  color: #6f6f6f;
  font-size: 0.875rem;
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
