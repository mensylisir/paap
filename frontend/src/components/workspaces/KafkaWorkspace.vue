<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="ws-tabs">
        <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <div v-if="activeTab === 'topics'" class="tab-panel">
        <div v-if="topics.length" class="topic-grid">
          <div
            v-for="t in topics"
            :key="t.name"
            class="card topic-card selectable"
            :class="{ selected: selectedResource?.name === t.name && selectedResource?.type === t.type }"
            @click="selectResource(t)"
          >
            <div class="card-title">{{ t.name }}</div>
            <div class="card-sub">分区: {{ t.annotations?.partitions ?? '-' }} · 说明: {{ t.description }}</div>
            <span class="badge" :class="t.status === 'Ready' ? 'green' : 'gray'">{{ t.status }}</span>
          </div>
        </div>
        <div v-else class="empty-line">暂无 Topic 数据</div>
      </div>

      <div v-if="activeTab === 'resources'" class="tab-panel">
        <div v-if="resources.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>类型</th><th>状态</th><th>说明</th></tr></thead>
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

      <aside class="object-detail" v-if="selectedResource">
        <div class="detail-head">
          <div>
            <div class="detail-label">对象详情</div>
            <div class="detail-name">{{ selectedResource.name }}</div>
          </div>
          <span class="badge blue">{{ selectedResource.type }}</span>
        </div>
        <p class="detail-desc">{{ selectedResource.description }}</p>
        <div class="detail-grid">
          <div><span>状态</span><strong>{{ selectedResource.status }}</strong></div>
          <div v-for="item in annotationItems(selectedResource)" :key="item.key"><span>{{ item.key }}</span><strong>{{ item.value }}</strong></div>
        </div>
        <div class="object-actions">
          <div class="detail-label">对象级操作</div>
          <button
            v-for="action in selectedResource.actions || []"
            :key="action.label"
            class="act-btn"
            :class="action.tone"
            @click="emit('action', action, action.target || selectedResource?.name)"
          >
            {{ action.label }}
          </button>
          <span v-if="!(selectedResource.actions || []).length" class="empty-inline">暂无对象操作</span>
        </div>
      </aside>
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

const topics = computed(() => props.resources.filter(r => r.type === 'Topic'))
const selectedResource = ref<WorkspaceResource | null>(props.resources.find(r => r.type !== 'Connection') || props.resources[0] || null)

const selectResource = (resource: WorkspaceResource) => {
  selectedResource.value = resource
}

const annotationItems = (resource: WorkspaceResource) =>
  Object.entries(resource.annotations || {}).map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (topics.value.length) tabs.push({ key: 'topics', label: 'Topic', count: topics.value.length })
  tabs.push({ key: 'resources', label: '资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(topics.value.length ? 'topics' : 'resources')

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('synced') || v.includes('up')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('down') || v.includes('degraded')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.topic-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--paap-space-3); }
.selectable, .data-table tbody tr { cursor: pointer; }
.selectable.selected, .data-table tbody tr.selected { background: var(--paap-accent-soft); }
.object-detail {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
  margin-top: var(--paap-space-4);
  background: var(--paap-panel);
}
.detail-head { display: flex; justify-content: space-between; gap: var(--paap-space-3); align-items: flex-start; }
.detail-label { font-size: 11px; color: var(--paap-muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
.detail-name { font-size: 16px; color: var(--paap-text); font-weight: 600; margin-top: 2px; word-break: break-all; }
.detail-desc { color: var(--paap-muted); font-size: 13px; line-height: 1.5; }
.detail-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: var(--paap-space-2); margin-top: var(--paap-space-3); }
.detail-grid > div { border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); padding: var(--paap-space-2) var(--paap-space-3); min-width: 0; background: var(--paap-panel-subtle); }
.detail-grid span { display: block; color: var(--paap-muted); font-size: 11px; margin-bottom: 2px; }
.detail-grid strong { color: var(--paap-text); font-size: 13px; word-break: break-all; }
.object-actions { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; margin-top: var(--paap-space-3); }
.empty-inline { color: var(--paap-muted-2); font-size: 12px; }
</style>
