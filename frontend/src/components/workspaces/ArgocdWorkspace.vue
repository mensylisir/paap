<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default>
      <div class="ws-tabs">
        <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <!-- Applications view -->
      <div v-if="activeTab === 'apps'" class="argocd-shell">
        <aside class="argocd-sidebar">
          <div class="argocd-search">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8"/>
              <line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <input v-model="query" type="search" placeholder="搜索应用..." />
          </div>
          <div class="argocd-app-list">
            <button
              v-for="app in filteredApps"
              :key="app.name"
              class="argocd-app-item"
              :class="{ active: selectedApp?.name === app.name }"
              @click="selectApp(app)"
            >
              <div class="argocd-app-item-top">
                <span class="argocd-app-name">{{ app.name }}</span>
                <span class="status-pill" :class="healthClass(app.annotations?.health)">{{ app.annotations?.health || '-' }}</span>
              </div>
              <div class="argocd-app-item-bottom">
                <span class="argocd-app-path">{{ app.annotations?.path || '-' }}</span>
                <span class="status-pill" :class="syncClass(app.annotations?.syncStatus)">{{ app.annotations?.syncStatus || '-' }}</span>
              </div>
            </button>
          </div>
        </aside>

        <section class="argocd-main" v-if="selectedApp">
          <!-- App header -->
          <div class="argocd-app-header">
            <div class="argocd-app-header-left">
              <h3 class="argocd-app-title">{{ selectedApp.name }}</h3>
              <div class="argocd-app-source">
                <code class="argocd-app-repo">{{ appRepoDisplayURL(selectedApp) }}</code>
                <span class="argocd-app-revision">{{ selectedApp.annotations?.path || '-' }}</span>
                <code v-if="selectedApp.annotations?.revision" class="argocd-app-commit">{{ String(selectedApp.annotations.revision).slice(0, 7) }}</code>
              </div>
            </div>
            <div class="argocd-app-status">
              <span class="status-pill" :class="syncClass(selectedApp.annotations?.syncStatus)">Sync: {{ selectedApp.annotations?.syncStatus || '-' }}</span>
              <span class="status-pill" :class="healthClass(selectedApp.annotations?.health)">Health: {{ selectedApp.annotations?.health || '-' }}</span>
              <a v-if="selectedApp.externalUrl" :href="selectedApp.externalUrl" target="_blank" rel="noreferrer" class="argocd-open-link">打开 ArgoCD 拓扑</a>
            </div>
          </div>

          <!-- Actions -->
          <div class="argocd-actions">
            <button
              v-for="act in appActions(selectedApp)"
              :key="act.label"
              class="act-btn"
              :class="act.tone"
              @click="emit('action', act, act.target || selectedApp.name)"
            >
              {{ act.label }}
            </button>
          </div>

          <!-- Resource topology + detail -->
          <div class="argocd-content">
            <section class="argocd-topology">
              <div class="argocd-tree-header">
                <span>资源拓扑</span>
                <strong>{{ argocdTreeLayout.nodes.length }} 个节点</strong>
              </div>
              <div class="argocd-tree-canvas">
                <div
                  class="argocd-tree-stage topology-lanes"
                  :style="{ width: `${argocdTreeLayout.width}px`, height: `${argocdTreeLayout.height}px` }"
                >
                  <svg
                    class="argocd-tree-links"
                    :width="argocdTreeLayout.width"
                    :height="argocdTreeLayout.height"
                    :viewBox="`0 0 ${argocdTreeLayout.width} ${argocdTreeLayout.height}`"
                    aria-hidden="true"
                  >
                    <defs>
                      <marker id="argocd-tree-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                        <path d="M 0 0 L 10 5 L 0 10 z" class="argocd-tree-arrow-head" />
                      </marker>
                    </defs>
                    <path
                      v-for="edge in argocdTreeLayout.edges"
                      :key="edge.key"
                      class="argocd-tree-link"
                      :class="{ active: isSelected(edge.from.resource) || isSelected(edge.to.resource) }"
                      :d="argocdEdgePath(edge)"
                    />
                  </svg>
                  <button
                    v-for="node in argocdTreeLayout.nodes"
                    :key="node.key"
                    type="button"
                    class="argocd-resource-node topology-node"
                    :class="[healthClass(node.resource.annotations?.health || node.resource.status), { selected: isSelected(node.resource), root: node.depth === 0 }]"
                    :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${argocdNodeMetrics.width}px`, minHeight: `${argocdNodeMetrics.height}px` }"
                    @click="selectResource(node.resource)"
                  >
                    <span class="argocd-kind-chip">{{ kindShortLabel(node.resource.type) }}</span>
                    <span class="argocd-node-main">
                      <strong>{{ node.resource.name }}</strong>
                      <small>{{ node.resource.annotations?.namespace || node.resource.description || '-' }}</small>
                    </span>
                    <span class="argocd-node-health" :class="healthClass(node.resource.annotations?.health || node.resource.status)" />
                  </button>
                </div>
              </div>
            </section>

            <section class="argocd-detail">
              <div class="argocd-detail-header">
                <span class="ws-section-label">资源详情</span>
                <span class="status-pill" :class="healthClass(selectedResourceView?.annotations?.health || selectedResourceView?.status)">
                  {{ selectedResourceView?.status || selectedResourceView?.annotations?.health || '-' }}
                </span>
              </div>
              <div v-if="selectedResourceView" class="argocd-detail-body">
                <h4 class="argocd-detail-name">{{ selectedResourceView.name }}</h4>
                <span class="badge blue">{{ selectedResourceView.type }}</span>
                <p class="argocd-detail-desc">{{ resourceDescription(selectedResourceView) }}</p>
                <div class="argocd-detail-props">
                  <div v-for="row in resourceDetailRows" :key="row.label" class="argocd-detail-prop">
                    <span>{{ row.label }}</span>
                    <em>{{ row.value }}</em>
                  </div>
                  <div class="argocd-detail-prop">
                    <span>Application</span>
                    <em>{{ selectedApp.name }}</em>
                  </div>
                  <div class="argocd-detail-prop">
                    <span>环境</span>
                    <em>{{ selectedResourceView.annotations?.namespace || selectedResourceView.description || selectedApp.annotations?.namespace || '-' }}</em>
                  </div>
                  <div v-for="entry in selectedAnnotations" :key="entry.key" class="argocd-detail-prop">
                    <span>{{ entry.key }}</span>
                    <em>{{ entry.value }}</em>
                  </div>
                </div>
                <div class="argocd-detail-actions">
                  <span class="argocd-action-label">资源级操作</span>
                  <button
                    v-for="act in selectedResourceActions"
                    :key="act.label"
                    class="act-btn"
                    :class="act.tone"
                    @click="emit('action', act, act.target || selectedResourceView?.name)"
                  >
                    {{ act.label }}
                  </button>
                </div>
              </div>
            </section>
          </div>
        </section>

        <div v-else class="empty-state empty-state--compact"><p class="empty-state__title">暂无 Application 数据</p></div>
      </div>

      <!-- Resources view -->
      <div v-if="activeTab === 'resources'">
        <div v-if="allResources.length" class="table-wrap">
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
              <tr v-for="r in allResources" :key="r.name + r.type + (r.annotations?.key || '')">
                <td class="cell-name">{{ r.name }}</td>
                <td><span class="badge blue">{{ r.type }}</span></td>
                <td><span class="badge" :class="syncBadge(r.status)">{{ r.status }}</span></td>
                <td class="cell-desc">{{ resourceDescription(r) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-state empty-state--compact"><p class="empty-state__title">暂无资源数据</p></div>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'
import {
  argocdEdgePath,
  buildArgoCDResourceList,
  buildArgoCDTreeLayout,
  defaultArgoNodeMetrics,
} from './argocdTopology'

const props = defineProps<{
  resources: WorkspaceResource[]
}>()
const route = useRoute()

const emit = defineEmits<{
  (e: 'action', action: WorkspaceAction, target?: string): void
}>()

const query = ref('')
const requestedApplication = computed(() => String(route.query.application || '').trim())
const apps = computed(() => props.resources.filter(r => r.type === 'Application'))
const allResources = computed(() => buildArgoCDResourceList(props.resources))
const filteredApps = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return apps.value
  return apps.value.filter(app => `${app.name} ${appRepoDisplayURL(app)} ${app.annotations?.repoURL || ''} ${app.annotations?.path || ''}`.toLowerCase().includes(q))
})
const selectedApp = ref<WorkspaceResource | null>(null)
const selectedResource = ref<WorkspaceResource | null>(null)
const availableTabs = computed(() => [
  { key: 'apps', label: 'Applications', count: apps.value.length },
  { key: 'resources', label: '全部资源', count: allResources.value.length },
])
const activeTab = ref(apps.value.length ? 'apps' : 'resources')
const selectedResourceView = computed(() => selectedResource.value || selectedApp.value)
const argocdNodeMetrics = defaultArgoNodeMetrics
const argocdTreeLayout = computed(() => buildArgoCDTreeLayout(selectedApp.value, argocdNodeMetrics))
const kindShortLabel = (kind?: string) => {
  const labels: Record<string, string> = {
    Application: 'app',
    Deployment: 'deploy',
    ReplicaSet: 'rs',
    Pod: 'pod',
    Service: 'svc',
    Endpoints: 'ep',
    EndpointSlice: 'eps',
    ConfigMap: 'cm',
    Secret: 'sec',
    StatefulSet: 'sts',
    DaemonSet: 'ds',
    ControllerRevision: 'rev',
  }
  return labels[kind || ''] || String(kind || '-').slice(0, 4).toLowerCase()
}
const appRepoDisplayURL = (app?: WorkspaceResource | null) =>
  String(app?.annotations?.externalRepoURL || app?.annotations?.repoURL || '-')
const resourceDescription = (resource?: WorkspaceResource | null) => {
  if (!resource) return ''
  if (resource.type !== 'Application') return resource.description || ''
  const path = String(resource.annotations?.path || '').trim()
  const source = [appRepoDisplayURL(resource), path].filter(Boolean).join(' ')
  return source ? `Source: ${source}` : resource.description || ''
}
const selectedAnnotations = computed(() => {
  const annotations = selectedResourceView.value?.annotations || {}
  return Object.entries(annotations)
    .filter(([key]) => !['syncStatus', 'health', 'repoURL', 'externalRepoURL', 'server', 'proxyURL', 'treeNodes', 'treeEdges', 'parentRefs'].includes(key))
    .slice(0, 8)
    .map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))
})
const resourceDetailRows = computed(() => {
  const resource = selectedResourceView.value
  if (!resource) return []
  return [
    { label: '类型', value: resource.type },
    { label: '同步', value: String(resource.annotations?.syncStatus || resource.status || '-') },
    { label: '健康', value: String(resource.annotations?.health || resource.status || '-') },
  ]
})
const selectedResourceActions = computed<WorkspaceAction[]>(() => {
  const selected = selectedResourceView.value
  if (!selected) return []
  if (selected.type === 'Application') return appActions(selected)
  return [{ key: 'refresh', label: '刷新', description: `重新读取 ${selected.type} ${selected.name} 的状态。`, target: selected.name }]
})

const selectApp = (app: WorkspaceResource) => {
  selectedApp.value = app
  selectedResource.value = app
}
const selectResource = (resource: WorkspaceResource) => {
  selectedResource.value = resource
}
const isSelected = (resource: WorkspaceResource) =>
  selectedResourceView.value?.name === resource.name && selectedResourceView.value?.type === resource.type
const appActions = (app: WorkspaceResource): WorkspaceAction[] => app.actions?.length ? app.actions : [
  { key: 'sync_argocd_application', label: '同步', description: '触发该 ArgoCD Application 同步。', target: app.name },
  { key: 'delete_argocd_application', label: '删除', description: '删除该 ArgoCD Application。', tone: 'danger', target: app.name },
  { key: 'refresh', label: '刷新', description: '重新读取 Application 状态。', target: app.name },
]

watch(filteredApps, (items) => {
  if (!items.length) {
    selectedApp.value = null
    selectedResource.value = null
    return
  }
  const requested = requestedApplication.value
  const requestedApp = requested ? items.find(app => app.name === requested) : null
  if (requestedApp && selectedApp.value?.name !== requestedApp.name) {
    selectApp(requestedApp)
    return
  }
  if (!selectedApp.value || !items.some(app => app.name === selectedApp.value?.name)) selectApp(items[0])
}, { immediate: true })

const syncClass = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('synced')) return 'green'
  if (v.includes('outofsync')) return 'red'
  if (v.includes('progressing') || v.includes('pending')) return 'blue'
  return 'gray'
}
const healthClass = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('healthy')) return 'green'
  if (v.includes('degraded') || v.includes('error')) return 'red'
  if (v.includes('progressing') || v.includes('pending')) return 'blue'
  return 'gray'
}
const syncBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('synced') || v.includes('healthy')) return 'green'
  if (v.includes('outofsync') || v.includes('degraded') || v.includes('error')) return 'red'
  if (v.includes('progressing') || v.includes('pending')) return 'blue'
  return 'gray'
}
</script>

<style scoped>
.argocd-shell {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(720px, 1fr);
  gap: var(--paap-space-4);
  align-items: start;
  width: 100%;
  min-width: 0;
}

/* Sidebar */
.argocd-sidebar {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  position: sticky;
  top: 0;
}
.argocd-search {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  padding: var(--paap-space-3) var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
.argocd-search input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: var(--paap-fs-compact);
  color: var(--paap-text);
  outline: none;
  font-family: inherit;
}
.argocd-search input::placeholder {
  color: var(--paap-muted);
}
.argocd-app-list {
  max-height: 480px;
  overflow-y: auto;
}
.argocd-app-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  padding: 12px var(--paap-space-4);
  border: none;
  border-bottom: 1px solid var(--paap-panel-subtle);
  background: transparent;
  cursor: pointer;
  text-align: left;
  font-family: inherit;
  transition: background 0.1s;
}
.argocd-app-item:last-child { border-bottom: none; }
.argocd-app-item:hover { background: var(--paap-panel-subtle); }
.argocd-app-item.active { background: var(--paap-accent-soft); }
.argocd-app-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
}
.argocd-app-name {
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  color: var(--paap-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.argocd-app-item-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--paap-space-2);
}
.argocd-app-path {
  font-size: var(--paap-fs-small);
  color: var(--paap-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--paap-mono);
}

/* Main */
.argocd-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-4);
}

/* App header */
.argocd-app-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--paap-space-4);
  min-width: 0;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.argocd-app-header-left {
  min-width: 0;
}
.argocd-app-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--paap-text);
  margin: 0 0 var(--paap-space-2);
  word-break: break-all;
}
.argocd-app-source {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  min-width: 0;
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
}
.argocd-app-repo {
  max-width: 100%;
  color: var(--paap-accent);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
  word-break: break-all;
}
.argocd-app-revision {
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-small);
}
.argocd-app-commit {
  background: var(--paap-panel-subtle);
  border-radius: var(--paap-radius-xs);
  padding: 1px 6px;
  font-size: var(--paap-fs-small);
  font-family: var(--paap-mono);
}
.argocd-app-status {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  justify-content: flex-end;
  min-width: 0;
  flex-shrink: 0;
}
.argocd-open-link {
  display: inline-flex;
  align-items: center;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--paap-info-border, #bfdbfe);
  border-radius: var(--paap-radius-full);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
  transition: all var(--paap-transition-fast);
}
.argocd-open-link:hover {
  background: var(--paap-accent-soft);
}

/* Actions */
.argocd-actions {
  display: flex;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}

/* Content grid */
.argocd-content {
  display: grid;
  gap: var(--paap-space-4);
  align-items: start;
}

.argocd-topology {
  background: var(--paap-panel-subtle);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  min-width: 0;
}
.argocd-tree-canvas {
  min-height: 460px;
  max-height: 680px;
  overflow: auto;
  background: var(--paap-panel-subtle, #f3f4f6);
}
.argocd-tree-stage {
  position: relative;
  flex: 0 0 auto;
}
.argocd-tree-links {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.argocd-tree-link {
  fill: none;
  stroke: var(--paap-muted, #8aa0ad);
  stroke-width: 1.75;
  stroke-dasharray: 4 3;
  marker-end: url(#argocd-tree-arrow);
}
.argocd-tree-link.active {
  stroke: var(--paap-accent, #0f62fe);
  stroke-width: 2;
}
.argocd-tree-arrow-head {
  fill: var(--paap-muted, #8aa0ad);
}
.argocd-resource-node {
  position: absolute;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) 10px;
  gap: var(--paap-space-2);
  align-items: center;
  padding: 8px 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm, 6px);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-sm);
  color: var(--paap-text);
  text-align: left;
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.12s, box-shadow 0.12s, transform 0.12s;
}
.argocd-resource-node:hover,
.argocd-resource-node.selected {
  border-color: var(--paap-accent, #0f62fe);
  box-shadow: var(--paap-focus-ring);
  z-index: 2;
}
.argocd-resource-node.root {
  border-color: var(--paap-accent, #0f62fe);
}
.argocd-kind-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 24px;
  padding: 0 5px;
  border-radius: var(--paap-radius-xs, 4px);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}
.argocd-node-main {
  display: grid;
  min-width: 0;
}
.argocd-node-main strong {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-text);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.argocd-node-main small {
  min-width: 0;
  overflow: hidden;
  color: var(--paap-muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.argocd-node-health {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--paap-muted);
}
.argocd-node-health.green { background: var(--paap-success); }
.argocd-node-health.red { background: var(--paap-danger); }
.argocd-node-health.blue { background: var(--paap-accent); }

/* Resource tree */
.argocd-tree {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  min-width: 0;
}
.argocd-tree-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--paap-space-3) var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  color: var(--paap-muted);
}
.argocd-tree-header strong {
  color: var(--paap-text);
}
.argocd-tree-body {
  max-height: 480px;
  overflow-y: auto;
  padding: var(--paap-space-2);
}
.argocd-tree-node {
  width: 100%;
  display: grid;
  grid-template-columns: calc(var(--depth) * 20px) 10px minmax(80px, 100px) minmax(160px, 1fr) minmax(80px, auto) minmax(80px, auto);
  align-items: center;
  gap: var(--paap-space-2);
  min-height: 36px;
  border: 1px solid transparent;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  color: var(--paap-text);
  padding: 4px 8px;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.1s;
}
.argocd-tree-node:hover,
.argocd-tree-node.selected {
  background: var(--paap-accent-soft);
  border-color: var(--paap-info-border, #bfdbfe);
}
.argocd-tree-node.root {
  background: var(--paap-panel-subtle);
  border-color: var(--paap-border);
}
.argocd-tree-indent {
  display: block;
  height: 1px;
  border-bottom: 1px solid var(--paap-border);
}
.argocd-tree-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--paap-muted);
}
.argocd-tree-node.green .argocd-tree-dot { background: var(--paap-success); }
.argocd-tree-node.red .argocd-tree-dot { background: var(--paap-danger); }
.argocd-tree-node.blue .argocd-tree-dot { background: var(--paap-accent); }
.argocd-tree-kind {
  font-size: 10px;
  font-weight: 600;
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
  border-radius: var(--paap-radius-xs);
  padding: 1px 6px;
  text-transform: uppercase;
}
.argocd-tree-name {
  font-size: var(--paap-fs-compact);
  color: var(--paap-text);
  word-break: break-all;
}
.argocd-tree-sync,
.argocd-tree-health {
  font-size: var(--paap-fs-small);
  font-weight: 500;
  color: var(--paap-muted);
  background: var(--paap-panel-subtle);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  padding: 1px 8px;
  white-space: nowrap;
  text-align: center;
}

/* Detail */
.argocd-detail {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.argocd-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--paap-space-3);
}
.argocd-detail-body {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-3);
}
.argocd-detail-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--paap-text);
  margin: 0;
  word-break: break-all;
}
.argocd-detail-desc {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  line-height: 1.5;
  margin: 0;
}
.argocd-detail-props {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-2);
}
.argocd-detail-prop {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: var(--paap-space-3);
  padding: var(--paap-space-2) var(--paap-space-3);
  background: var(--paap-panel-subtle);
  border-radius: var(--paap-radius-xs);
}
.argocd-detail-prop span {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  flex-shrink: 0;
}
.argocd-detail-prop em {
  font-style: normal;
  font-size: var(--paap-fs-label);
  color: var(--paap-text);
  word-break: break-all;
  text-align: right;
}
.argocd-detail-actions {
  display: flex;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}

@media (max-width: 960px) {
  .argocd-shell { grid-template-columns: 1fr; }
  .argocd-sidebar { position: static; }
  .argocd-app-header {
    flex-direction: column;
    align-items: stretch;
  }
  .argocd-app-status {
    justify-content: flex-start;
  }
  .topology-lanes { grid-template-columns: 1fr; }
  .topology-lane:not(:last-child)::after { display: none; }
  .argocd-tree-node {
    grid-template-columns: calc(var(--depth) * 16px) 10px minmax(60px, 80px) minmax(120px, 1fr);
  }
  .argocd-tree-sync,
  .argocd-tree-health { display: none; }
}

@media (max-width: 672px) {
  .argocd-app-header {
    padding: var(--paap-space-4);
  }
  .argocd-app-title {
    font-size: var(--paap-fs-heading-lg);
  }
  .argocd-open-link {
    max-width: 100%;
    white-space: normal;
    text-align: center;
  }
}
</style>
