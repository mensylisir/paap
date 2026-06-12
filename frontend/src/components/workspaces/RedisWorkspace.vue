<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="redis-shell">
        <!-- Sidebar: Key browser -->
        <aside class="redis-sidebar">
          <div class="redis-sidebar-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/>
              <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
              <line x1="12" y1="22.08" x2="12" y2="12"/>
            </svg>
            <span>Key Browser</span>
          </div>
          <div class="redis-key-list">
            <button
              v-for="k in keys"
              :key="k.name"
              class="redis-key-item"
              :class="{ selected: selectedResource?.name === k.name }"
              @click="selectResource(k)"
            >
              <span class="redis-key-type">{{ k.annotations?.keyType || k.type }}</span>
              <span class="redis-key-name mono">{{ k.name }}</span>
            </button>
            <div v-if="!keys.length" class="redis-key-empty">暂无 Key 数据</div>
          </div>
        </aside>

        <!-- Main content -->
        <main class="redis-main">
          <div class="ws-tabs">
            <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
              {{ tab.label }}
              <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
            </button>
          </div>

          <!-- Key detail view -->
          <div v-if="activeTab === 'keys'" class="redis-content">
            <div v-if="selectedResource && selectedResource.type !== 'Connection'" class="redis-key-detail">
              <div class="redis-key-detail-header">
                <div class="detail-label">对象详情</div>
                <div class="redis-key-detail-meta">
                  <span class="badge blue">{{ selectedResource.annotations?.keyType || selectedResource.type }}</span>
                  <span class="badge" :class="selectedResource.status === 'Ready' ? 'green' : 'gray'">{{ selectedResource.status }}</span>
                </div>
                <h3 class="redis-key-detail-name mono">{{ selectedResource.name }}</h3>
              </div>
              <p class="redis-key-detail-desc">{{ selectedResource.description }}</p>
              <div v-if="annotationItems(selectedResource).length" class="redis-key-props">
                <div v-for="item in annotationItems(selectedResource)" :key="item.key" class="redis-key-prop">
                  <span class="redis-key-prop-key">{{ item.key }}</span>
                  <span class="redis-key-prop-value mono">{{ item.value }}</span>
                </div>
              </div>
              <div v-if="(selectedResource.actions || []).length" class="redis-key-actions">
                <div class="detail-label">对象级操作</div>
                <button
                  v-for="action in selectedResource.actions"
                  :key="action.label"
                  class="act-btn"
                  :class="action.tone"
                  @click="emit('action', action, action.target || selectedResource?.name)"
                >
                  {{ action.label }}
                </button>
              </div>
            </div>
            <div v-else class="redis-placeholder">
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/>
              </svg>
              <p>从左侧选择一个 Key 查看详情</p>
            </div>
          </div>

          <!-- Info view -->
          <div v-if="activeTab === 'info'" class="redis-content">
            <div v-if="info.length" class="redis-info-grid">
              <div v-for="item in info" :key="item.name" class="card redis-info-card" :class="{ selected: selectedResource?.name === item.name }" @click="selectResource(item)">
                <div class="card-title">{{ item.name }}</div>
                <div class="card-sub">{{ item.description }}</div>
              </div>
            </div>
            <div v-else class="empty-line">暂无实例信息</div>
          </div>

          <!-- Resources view -->
          <div v-if="activeTab === 'resources'">
            <div v-if="resources.length" class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>类型</th>
                    <th>状态</th>
                    <th>说明</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="r in resources" :key="r.name + r.type" :class="{ selected: selectedResource?.name === r.name && selectedResource?.type === r.type }" @click="selectResource(r)">
                    <td class="cell-name">{{ r.name }}</td>
                    <td><span class="badge blue">{{ r.type }}</span></td>
                    <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                    <td class="cell-desc">{{ r.description }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="empty-line">暂无资源数据</div>
          </div>
        </main>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
}>()

const emit = defineEmits<{
  (e: 'action', action: WorkspaceAction, target?: string): void
}>()

const keys = computed(() => props.resources.filter(r =>
  r.type === 'Key' || r.type === 'Keyspace' || r.type === 'Key Pattern' || String(r.type).toLowerCase() === 'key'
))
const info = computed(() => props.resources.filter(r =>
  r.type === 'Info' || r.type === 'Health' || r.type === 'health'
))
const selectedResource = ref<WorkspaceResource | null>(props.resources.find(r => r.type !== 'Connection') || props.resources[0] || null)

const selectResource = (resource: WorkspaceResource) => {
  selectedResource.value = resource
}

const annotationItems = (resource: WorkspaceResource) =>
  Object.entries(resource.annotations || {})
    .filter(([key]) => key !== 'keyType')
    .map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (keys.value.length) tabs.push({ key: 'keys', label: 'Key', count: keys.value.length })
  if (info.value.length) tabs.push({ key: 'info', label: '实例信息', count: info.value.length })
  tabs.push({ key: 'resources', label: '全部资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(keys.value.length ? 'keys' : (info.value.length ? 'info' : 'resources'))

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('up')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('down')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.redis-shell {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: var(--paap-space-4);
  align-items: start;
}

/* Sidebar */
.redis-sidebar {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  position: sticky;
  top: 0;
}
.redis-sidebar-header {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  padding: var(--paap-space-3) var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  font-size: 12px;
  font-weight: 600;
  color: var(--paap-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  background: var(--paap-panel-subtle);
}
.redis-key-list {
  max-height: 480px;
  overflow-y: auto;
  padding: var(--paap-space-2);
}
.redis-key-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  cursor: pointer;
  transition: background 0.1s;
  text-align: left;
  font-family: inherit;
}
.redis-key-item:hover {
  background: var(--paap-panel-subtle);
}
.redis-key-item.selected {
  background: var(--paap-accent-soft);
}
.redis-key-type {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: var(--paap-radius-full);
  background: #f3f4f6;
  color: var(--paap-muted);
  text-transform: uppercase;
}
.redis-key-item.selected .redis-key-type {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.redis-key-name {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--paap-text-soft);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.redis-key-empty {
  padding: var(--paap-space-6);
  text-align: center;
  font-size: 12px;
  color: var(--paap-muted-2);
}

/* Main */
.redis-main {
  min-width: 0;
}
.redis-content {
  min-height: 200px;
}

/* Key detail */
.redis-key-detail {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.redis-key-detail-header {
  margin-bottom: var(--paap-space-3);
}
.redis-key-detail-meta {
  display: flex;
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-2);
}
.redis-key-detail-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--paap-text);
  word-break: break-all;
}
.redis-key-detail-desc {
  color: var(--paap-muted);
  font-size: 13px;
  line-height: 1.5;
  margin-bottom: var(--paap-space-4);
}
.redis-key-props {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-4);
}
.redis-key-prop {
  padding: var(--paap-space-2) var(--paap-space-3);
  background: var(--paap-panel-subtle);
  border-radius: var(--paap-radius-xs);
}
.redis-key-prop-key {
  display: block;
  font-size: 11px;
  color: var(--paap-muted);
  margin-bottom: 2px;
}
.redis-key-prop-value {
  font-size: 13px;
  color: var(--paap-text);
  word-break: break-all;
}
.redis-key-actions {
  display: flex;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}

/* Info grid */
.redis-info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--paap-space-3);
}
.redis-info-card {
  cursor: pointer;
}
.redis-info-card.selected {
  border-color: var(--paap-accent);
}

/* Placeholder */
.redis-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--paap-space-12);
  color: var(--paap-muted-2);
  font-size: 14px;
  gap: var(--paap-space-3);
}

@media (max-width: 768px) {
  .redis-shell {
    grid-template-columns: 1fr;
  }
  .redis-sidebar {
    position: static;
  }
}
</style>
