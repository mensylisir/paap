<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="redis-workspace">
        <section class="redis-summary" aria-label="Redis 状态摘要">
          <div v-for="card in summaryCards" :key="card.key" class="redis-summary-card">
            <span>{{ card.label }}</span>
            <strong class="mono">{{ card.value }}</strong>
          </div>
        </section>

        <section class="redis-action-panel" aria-label="Redis Key 操作">
          <div>
            <span class="ws-section-label">Key 管理</span>
            <h3>搜索、读取和写入 Key</h3>
          </div>
          <div class="redis-action-list">
            <button
              v-for="action in primaryActions"
              :key="action.key"
              type="button"
              class="act-btn"
              :class="action.tone"
              :title="action.description"
              :disabled="actionRunning"
              @click="emit('action', action, action.target)"
            >
              {{ action.label }}
            </button>
          </div>
        </section>

        <WorkspaceActionForm
          v-if="workspaceActiveAction"
          :action="workspaceActiveAction"
          :params="actionParams"
          :running="actionRunning"
          :error="actionError"
          title="Redis 操作"
          @update-param="(payload) => emit('update-action-param', payload)"
          @submit="emit('submit-action')"
          @cancel="emit('cancel-action')"
        />

        <div class="ws-tabs">
          <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
            {{ tab.label }}
            <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
          </button>
        </div>

        <div v-if="activeTab === 'keys'" class="redis-grid">
          <div class="redis-key-panel">
            <div v-if="keys.length" class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>Key</th>
                    <th>类型</th>
                    <th>状态</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in keys"
                    :key="r.name + r.type"
                    :class="{ selected: selectedResource?.name === r.name && selectedResource?.type === r.type }"
                    @click="selectResource(r)"
                  >
                    <td class="cell-name mono">{{ r.name }}</td>
                    <td><span class="badge blue">{{ r.annotations?.keyType || r.type }}</span></td>
                    <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="empty-line">暂无 Key 数据，使用“查看 Key”按模式搜索。</div>
          </div>

          <aside v-if="selectedResource" class="redis-detail">
            <div class="redis-detail-head">
              <div>
                <span class="ws-section-label">对象详情</span>
                <h4 class="redis-detail-name mono">{{ selectedResource.name }}</h4>
              </div>
              <span class="badge" :class="statusBadge(selectedResource.status)">{{ selectedResource.status }}</span>
            </div>
            <p class="redis-detail-desc">{{ selectedResource.description }}</p>
            <div class="redis-detail-grid">
              <div>
                <span>类型</span>
                <strong>{{ selectedResource.annotations?.keyType || selectedResource.type }}</strong>
              </div>
              <div v-for="item in keyDetailItems" :key="item.key">
                <span>{{ item.key }}</span>
                <strong class="mono">{{ item.value }}</strong>
              </div>
              <div v-for="item in annotationItems(selectedResource)" :key="item.key">
                <span>{{ item.key }}</span>
                <strong class="mono">{{ item.value }}</strong>
              </div>
            </div>
            <div class="redis-object-actions">
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
            <WorkspaceActionForm
              v-if="selectedResourceActiveAction"
              :action="selectedResourceActiveAction"
              :params="actionParams"
              :running="actionRunning"
              :error="actionError"
              title="Key 操作"
              @update-param="(payload) => emit('update-action-param', payload)"
              @submit="emit('submit-action')"
              @cancel="emit('cancel-action')"
            />
          </aside>
        </div>

        <div v-if="activeTab === 'info'" class="redis-info-list">
          <div v-if="basicInfo.length" class="redis-info-grid">
            <button
              v-for="item in basicInfo"
              :key="item.name"
              type="button"
              class="card redis-info-card"
              :class="{ selected: selectedResource?.name === item.name && selectedResource?.type === item.type }"
              @click="selectResource(item)"
            >
              <span class="badge" :class="statusBadge(item.status)">{{ item.status }}</span>
              <strong>{{ infoLabel(item) }}</strong>
              <span class="mono">{{ item.description }}</span>
            </button>
          </div>
          <div v-else class="empty-line">暂无基本信息，使用“实例信息”刷新 Redis 状态。</div>
        </div>

        <details v-if="advancedResources.length" class="redis-advanced">
          <summary>高级资源信息</summary>
          <div class="table-wrap">
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
                <tr v-for="r in advancedResources" :key="r.name + r.type" @click="selectResource(r)">
                  <td class="cell-name">{{ r.name }}</td>
                  <td><span class="badge blue">{{ r.type }}</span></td>
                  <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                  <td class="cell-desc">{{ r.description }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </details>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import WorkspaceActionForm from './WorkspaceActionForm.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
  workspaceActions?: WorkspaceAction[]
  activeAction?: WorkspaceAction | null
  activeActionTarget?: string
  actionParams?: Record<string, string>
  actionRunning?: boolean
  actionError?: string
}>()

const emit = defineEmits<{
  (e: 'action', action: WorkspaceAction, target?: string): void
  (e: 'update-action-param', payload: { name: string; value: string }): void
  (e: 'submit-action'): void
  (e: 'cancel-action'): void
}>()

const keyTypes = new Set(['redis key', 'key'])
const infoTypes = new Set(['info', 'health', 'keyspace'])
const detailTypes = new Set(['ttl', 'value'])

const keys = computed(() => props.resources.filter(r =>
  r.type === 'Redis Key' || keyTypes.has(String(r.type).toLowerCase())
))
const basicInfo = computed(() => props.resources.filter(r =>
  infoTypes.has(String(r.type).toLowerCase())
))
const keyDetailResources = computed(() => props.resources.filter(r =>
  detailTypes.has(String(r.type).toLowerCase()) || ['ttl', 'value'].includes(String(r.name).toLowerCase())
))
const advancedResources = computed(() => props.resources.filter(r => {
  const type = String(r.type).toLowerCase()
  if (keyTypes.has(type) || infoTypes.has(type) || detailTypes.has(type)) return false
  if (String(r.name).toLowerCase() === 'value' || String(r.name).toLowerCase() === 'ttl') return false
  return type !== 'connection'
}))

const findResource = (...names: string[]) => {
  const wanted = new Set(names.map(name => name.toLowerCase()))
  return props.resources.find(resource => wanted.has(String(resource.name).toLowerCase()))
}

const summaryCards = computed(() => {
  const health = props.resources.find(resource => String(resource.type).toLowerCase() === 'health') || findResource('redis-ping')
  const keyspace = findResource('keys')
  const memory = findResource('used-memory')
  const hitRate = findResource('hit-rate')
  const version = findResource('redis-version')
  return [
    { key: 'health', label: '连接状态', value: health?.status === 'Ready' ? 'Ready' : valueOrFallback(health?.description, '-') },
    { key: 'keys', label: 'Key 数量', value: keyspace?.description || '-' },
    { key: 'memory', label: '内存使用', value: memory?.description || '-' },
    { key: 'hit-rate', label: '命中率', value: hitRate?.description || '-' },
    { key: 'version', label: '版本', value: version?.description || '-' },
  ]
})

const primaryActionKeys = new Set([
  'check_redis_health',
  'inspect_redis',
  'list_redis_keys',
  'get_redis_key',
  'set_redis_key',
  'delete_redis_key',
  'expire_redis_key',
])
const primaryActions = computed(() =>
  (props.workspaceActions || []).filter(action => primaryActionKeys.has(String(action.key || '')))
)

const firstSelectableResource = () => keys.value[0] || basicInfo.value[0] || props.resources.find(r => r.type !== 'Connection') || props.resources[0] || null
const selectedResource = ref<WorkspaceResource | null>(firstSelectableResource())

const selectResource = (resource: WorkspaceResource) => {
  selectedResource.value = resource
}

const actionTarget = (action: WorkspaceAction, fallback = '') => String(action.target || fallback || '')
const activeActionTarget = computed(() => actionTarget(props.activeAction || { label: '', description: '' }, props.activeActionTarget || ''))
const selectedResourceActiveAction = computed(() => {
  const action = props.activeAction
  const resource = selectedResource.value
  if (!action || !resource) return null
  const target = activeActionTarget.value
  const matches = (resource.actions || []).some((candidate) =>
    candidate.key === action.key && (!target || actionTarget(candidate, resource.name) === target)
  )
  return matches ? action : null
})
const workspaceActiveAction = computed(() =>
  props.activeAction && !selectedResourceActiveAction.value ? props.activeAction : null
)

const annotationItems = (resource: WorkspaceResource) =>
  Object.entries(resource.annotations || {})
    .filter(([key]) => key !== 'keyType')
    .map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))

const keyDetailItems = computed(() =>
  keyDetailResources.value.map(resource => ({
    key: infoLabel(resource),
    value: resource.description || resource.status || '-',
  }))
)

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  tabs.push({ key: 'keys', label: 'Key', count: keys.value.length })
  tabs.push({ key: 'info', label: '基本信息', count: basicInfo.value.length })
  return tabs
})

const activeTab = ref(keys.value.length ? 'keys' : 'info')

const resourceKey = (resource?: WorkspaceResource | null) => resource ? `${resource.type}:${resource.name}` : ''
const syncWorkspaceState = () => {
  const currentKey = resourceKey(selectedResource.value)
  const refreshed = currentKey ? props.resources.find((resource) => resourceKey(resource) === currentKey) : null
  selectedResource.value = refreshed || firstSelectableResource()
  if (!availableTabs.value.some((tab) => tab.key === activeTab.value)) {
    activeTab.value = availableTabs.value[0]?.key || 'info'
  }
}
watch(() => props.resources, syncWorkspaceState, { deep: true })
watch(availableTabs, syncWorkspaceState)

const infoLabel = (resource: WorkspaceResource) => {
  const name = String(resource.name || '')
  const labels: Record<string, string> = {
    'redis-ping': '连接状态',
    keys: 'Key 数量',
    'redis-version': '版本',
    'used-memory': '内存使用',
    'hit-rate': '命中率',
    'connected-clients': '客户端连接',
    ttl: 'TTL',
    value: 'Value',
  }
  return labels[name] || name || resource.type
}

const valueOrFallback = (value?: string, fallback = '-') => {
  const text = String(value || '').trim()
  return text || fallback
}

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('up')) return 'green'
  if (v.includes('partial') || v.includes('warn')) return 'yellow'
  if (v.includes('error') || v.includes('fail') || v.includes('down')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.redis-workspace {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-4);
}

.redis-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: var(--paap-space-3);
}

.redis-summary-card,
.redis-action-panel,
.redis-detail,
.redis-advanced {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
}

.redis-summary-card {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-2);
  padding: var(--paap-space-4);
  min-width: 0;
}

.redis-summary-card span {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.redis-summary-card strong {
  color: var(--paap-text);
  overflow-wrap: anywhere;
}

.redis-action-panel {
  display: flex;
  justify-content: space-between;
  gap: var(--paap-space-4);
  align-items: center;
  padding: var(--paap-space-4);
}

.redis-action-panel h3 {
  margin: 2px 0 0;
  font-size: var(--paap-fs-heading);
  color: var(--paap-text);
}

.redis-action-list,
.redis-object-actions {
  display: flex;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}

.redis-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
  gap: var(--paap-space-4);
  align-items: start;
}

.redis-key-panel {
  min-width: 0;
}

.data-table tbody tr {
  cursor: pointer;
}

.data-table tbody tr.selected td {
  background: var(--paap-accent-soft);
}

.redis-detail {
  padding: var(--paap-space-4);
}

.redis-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-3);
}

.redis-detail-name {
  margin: 2px 0 0;
  font-size: var(--paap-fs-heading);
  color: var(--paap-text);
  overflow-wrap: anywhere;
}

.redis-detail-desc {
  margin: var(--paap-space-3) 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
}

.redis-detail-grid,
.redis-info-grid {
  display: grid;
  gap: var(--paap-space-3);
}

.redis-detail-grid {
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  margin-bottom: var(--paap-space-4);
}

.redis-detail-grid div {
  min-width: 0;
  padding: var(--paap-space-3);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel-subtle);
}

.redis-detail-grid span {
  display: block;
  margin-bottom: 2px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-small);
}

.redis-detail-grid strong {
  color: var(--paap-text);
  overflow-wrap: anywhere;
}

.redis-object-actions {
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}

.redis-info-grid {
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.redis-info-card {
  cursor: pointer;
  text-align: left;
  font-family: inherit;
}

.redis-info-card strong,
.redis-info-card span:last-child {
  display: block;
  margin-top: var(--paap-space-2);
  overflow-wrap: anywhere;
}

.redis-info-card.selected {
  border-color: var(--paap-accent);
  background: var(--paap-accent-soft);
}

.redis-advanced {
  padding: var(--paap-space-4);
}

.redis-advanced summary {
  cursor: pointer;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.redis-advanced .table-wrap {
  margin-top: var(--paap-space-3);
}

@media (max-width: 900px) {
  .redis-action-panel,
  .redis-grid {
    grid-template-columns: 1fr;
  }

  .redis-action-panel {
    display: grid;
  }
}
</style>
