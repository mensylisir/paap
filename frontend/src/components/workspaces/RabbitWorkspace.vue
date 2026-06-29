<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="ws-tabs">
        <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>
      <WorkspaceActionForm
        v-if="workspaceActiveAction"
        :action="workspaceActiveAction"
        :params="actionParams"
        :running="actionRunning"
        :error="actionError"
        title="消息队列操作"
        @update-param="(payload) => emit('update-action-param', payload)"
        @submit="emit('submit-action')"
        @cancel="emit('cancel-action')"
      />

      <div v-if="activeTab === 'queues'" class="tab-panel">
        <div v-if="queues.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>队列</th><th>VHost</th><th>消息数</th><th>状态</th></tr></thead>
            <tbody>
              <tr v-for="q in queues" :key="q.name" :class="{ selected: selectedResource?.name === q.name && selectedResource?.type === q.type }" @click="selectResource(q)">
                <td class="cell-name">{{ q.name }}</td>
                <td>{{ q.annotations?.vhost || '/' }}</td>
                <td>{{ q.annotations?.messages ?? '-' }}</td>
                <td><span class="badge" :class="q.status === 'Ready' ? 'green' : 'gray'">{{ q.status }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无队列数据</div>
      </div>

      <div v-if="activeTab === 'exchanges'" class="tab-panel">
        <div v-if="exchanges.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>交换机</th><th>类型</th><th>状态</th></tr></thead>
            <tbody>
              <tr v-for="e in exchanges" :key="e.name" :class="{ selected: selectedResource?.name === e.name && selectedResource?.type === e.type }" @click="selectResource(e)">
                <td class="cell-name">{{ e.name }}</td>
                <td>{{ e.annotations?.exchangeType || e.annotations?.type || '-' }}</td>
                <td><span class="badge" :class="e.status === 'Ready' ? 'green' : 'gray'">{{ e.status }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无交换机数据</div>
      </div>

      <div v-if="activeTab === 'bindings'" class="tab-panel">
        <div v-if="bindings.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>源</th><th>目标</th><th>Routing key</th><th>VHost</th></tr></thead>
            <tbody>
              <tr v-for="b in bindings" :key="bindingKey(b)" :class="{ selected: selectedResource?.name === b.name && selectedResource?.type === b.type }" @click="selectResource(b)">
                <td class="cell-name">{{ b.annotations?.source || '-' }}</td>
                <td>{{ b.annotations?.destination || b.name }}</td>
                <td>{{ b.annotations?.routingKey || '-' }}</td>
                <td>{{ b.annotations?.vhost || '/' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无绑定数据</div>
      </div>

      <div v-if="activeTab === 'vhosts'" class="tab-panel">
        <div v-if="vhosts.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>VHost</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="v in vhosts" :key="v.name" :class="{ selected: selectedResource?.name === v.name && selectedResource?.type === v.type }" @click="selectResource(v)">
                <td class="cell-name">{{ v.name }}</td>
                <td><span class="badge" :class="statusBadge(v.status)">{{ v.status }}</span></td>
                <td class="cell-desc">{{ v.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无 VHost 数据</div>
      </div>

      <div v-if="activeTab === 'messages'" class="tab-panel">
        <div v-if="messages.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>消息</th><th>Routing key</th><th>剩余</th><th>内容</th></tr></thead>
            <tbody>
              <tr v-for="m in messages" :key="m.name" :class="{ selected: selectedResource?.name === m.name && selectedResource?.type === m.type }" @click="selectResource(m)">
                <td class="cell-name">{{ m.name }}</td>
                <td>{{ m.annotations?.routingKey || '-' }}</td>
                <td>{{ m.annotations?.remaining ?? '-' }}</td>
                <td class="cell-desc">{{ m.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无消息数据</div>
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
        <WorkspaceActionForm
          v-if="selectedResourceActiveAction"
          :action="selectedResourceActiveAction"
          :params="actionParams"
          :running="actionRunning"
          :error="actionError"
          title="对象操作"
          @update-param="(payload) => emit('update-action-param', payload)"
          @submit="emit('submit-action')"
          @cancel="emit('cancel-action')"
        />
      </aside>
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

const queues = computed(() => props.resources.filter(r => r.type === 'Queue'))
const exchanges = computed(() => props.resources.filter(r => r.type === 'Exchange'))
const bindings = computed(() => props.resources.filter(r => r.type === 'Binding'))
const vhosts = computed(() => props.resources.filter(r => r.type === 'VHost'))
const messages = computed(() => props.resources.filter(r => r.type === 'Message'))
const firstSelectableResource = () => props.resources.find(r => r.type !== 'Connection') || props.resources[0] || null
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
  Object.entries(resource.annotations || {}).map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))

const bindingKey = (resource: WorkspaceResource) =>
  `${resource.annotations?.vhost || '/'}:${resource.annotations?.source || ''}:${resource.annotations?.destination || resource.name}:${resource.annotations?.propertiesKey || ''}`

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (queues.value.length) tabs.push({ key: 'queues', label: '队列', count: queues.value.length })
  if (exchanges.value.length) tabs.push({ key: 'exchanges', label: '交换机', count: exchanges.value.length })
  if (bindings.value.length) tabs.push({ key: 'bindings', label: '绑定', count: bindings.value.length })
  if (vhosts.value.length) tabs.push({ key: 'vhosts', label: 'VHost', count: vhosts.value.length })
  if (messages.value.length) tabs.push({ key: 'messages', label: '消息', count: messages.value.length })
  tabs.push({ key: 'resources', label: '资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(queues.value.length ? 'queues' : (exchanges.value.length ? 'exchanges' : 'resources'))

const resourceKey = (resource?: WorkspaceResource | null) => resource ? `${resource.type}:${resource.name}` : ''
const syncWorkspaceState = () => {
  const currentKey = resourceKey(selectedResource.value)
  const refreshed = currentKey ? props.resources.find((resource) => resourceKey(resource) === currentKey) : null
  selectedResource.value = refreshed || firstSelectableResource()
  if (!availableTabs.value.some((tab) => tab.key === activeTab.value)) {
    activeTab.value = availableTabs.value[0]?.key || 'resources'
  }
}
watch(() => props.resources, syncWorkspaceState, { deep: true })
watch(availableTabs, syncWorkspaceState)

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('synced') || v.includes('up')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('down') || v.includes('degraded')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.data-table tbody tr { cursor: pointer; }
.data-table tbody tr.selected { background: var(--paap-accent-soft); }
.object-detail {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
  margin-top: var(--paap-space-4);
  background: var(--paap-panel);
}
.detail-head { display: flex; justify-content: space-between; gap: var(--paap-space-3); align-items: flex-start; }
.detail-label { font-size: var(--paap-fs-small); color: var(--paap-muted); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
.detail-name { font-size: var(--paap-fs-heading-lg); color: var(--paap-text); font-weight: 600; margin-top: 2px; word-break: break-all; }
.detail-desc { color: var(--paap-muted); font-size: var(--paap-fs-compact); line-height: 1.5; }
.detail-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: var(--paap-space-2); margin-top: var(--paap-space-3); }
.detail-grid > div { border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); padding: var(--paap-space-2) var(--paap-space-3); min-width: 0; background: var(--paap-panel-subtle); }
.detail-grid span { display: block; color: var(--paap-muted); font-size: var(--paap-fs-small); margin-bottom: 2px; }
.detail-grid strong { color: var(--paap-text); font-size: var(--paap-fs-compact); word-break: break-all; }
.object-actions { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; margin-top: var(--paap-space-3); }
.empty-inline { color: var(--paap-muted); font-size: var(--paap-fs-label); }
</style>
