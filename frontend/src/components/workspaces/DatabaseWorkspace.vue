<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="db-shell">
        <!-- Sidebar: Database tree -->
        <aside class="db-sidebar">
          <div class="db-sidebar-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <ellipse cx="12" cy="5" rx="9" ry="3"/>
              <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
              <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
            </svg>
            <span>{{ dbEngine }}</span>
          </div>
          <div class="db-tree">
            <div v-for="db in dbs" :key="db.name" class="db-tree-item" :class="{ expanded: expandedDb === db.name, selected: selectedResource?.name === db.name && selectedResource?.type === 'Database' }">
              <button class="db-tree-row" @click="inspectDatabase(db)">
                <svg class="db-tree-chevron" :class="{ open: expandedDb === db.name }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="9 18 15 12 9 6"/>
                </svg>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                  <ellipse cx="12" cy="5" rx="9" ry="3"/>
                  <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                  <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
                </svg>
                <span class="db-tree-name">{{ db.name }}</span>
              </button>
              <div v-if="expandedDb === db.name" class="db-tree-children">
                <button
                  v-for="t in tablesForDb(db.name)"
                  :key="t.name"
                  class="db-tree-row db-tree-leaf"
                  :class="{ selected: selectedResource?.name === t.name && selectedResource?.type === 'Table' }"
                  @click="inspectTable(t)"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="3" y="3" width="18" height="18" rx="2"/>
                    <line x1="3" y1="9" x2="21" y2="9"/>
                    <line x1="9" y1="3" x2="9" y2="21"/>
                  </svg>
                  <span class="db-tree-name">{{ t.name }}</span>
                </button>
                <div v-if="!tablesForDb(db.name).length" class="db-tree-empty">无表</div>
              </div>
            </div>
            <div v-if="!dbs.length" class="db-tree-empty">暂无数据库</div>
          </div>
        </aside>

        <!-- Main content area -->
        <main class="db-main">
          <section class="db-context-bar" aria-label="数据库快捷操作">
            <div class="db-context-summary">
              <span class="ws-section-label">当前对象</span>
              <strong>{{ selectedResource?.name || '未选择' }}</strong>
              <span v-if="selectedResource" class="badge blue">{{ selectedResource.type }}</span>
            </div>
            <div class="db-context-actions">
              <button
                v-for="action in visibleContextActions"
                :key="action.key + ':' + contextActionTarget(action)"
                type="button"
                class="act-btn db-context-action"
                :class="action.tone"
                :title="action.description"
                :disabled="actionRunning"
                @click="emit('action', action, contextActionTarget(action))"
              >
                {{ action.label }}
              </button>
            </div>
          </section>

          <!-- Tabs for data exploration -->
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
            title="数据库操作"
            @update-param="(payload) => emit('update-action-param', payload)"
            @submit="emit('submit-action')"
            @cancel="emit('cancel-action')"
          />

          <!-- Table data view -->
          <div v-if="activeTab === 'data'" class="db-data-panel">
            <div v-if="rows.length" class="table-wrap scroll">
              <table class="data-table">
                <thead>
                  <tr>
                    <th v-for="k in rowKeys" :key="k">{{ k }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, i) in rows" :key="i">
                    <td v-for="k in rowKeys" :key="k" class="mono">{{ row[k] }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="db-placeholder">
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
                <rect x="3" y="3" width="18" height="18" rx="2"/>
                <line x1="3" y1="9" x2="21" y2="9"/>
                <line x1="9" y1="3" x2="9" y2="21"/>
              </svg>
              <p>选择一张表查看数据预览</p>
            </div>
          </div>

          <!-- Columns view -->
          <div v-if="activeTab === 'columns'" class="db-data-panel">
            <div v-if="columns.length" class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th>字段名</th>
                    <th>类型</th>
                    <th>说明</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="c in columns" :key="c.name" :class="{ selected: selectedResource?.name === c.name }" @click="selectResource(c)">
                    <td class="cell-name mono">{{ c.name }}</td>
                    <td><span class="badge blue">{{ c.annotations?.columnType || '-' }}</span></td>
                    <td class="cell-desc">{{ c.description }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="db-placeholder">
              <p>选择一张表查看字段结构</p>
            </div>
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
            <div v-else class="empty-state empty-state--compact"><p class="empty-state__title">暂无资源数据</p></div>
          </div>

          <!-- Detail panel -->
          <aside v-if="selectedResource" class="db-detail">
            <div class="db-detail-header">
              <div>
                <span class="ws-section-label">对象详情</span>
                <h4 class="db-detail-name">{{ selectedResource.name }}</h4>
                <span class="badge blue">{{ selectedResource.type }}</span>
              </div>
              <span class="badge" :class="statusBadge(selectedResource.status)">{{ selectedResource.status }}</span>
            </div>
            <p class="db-detail-desc">{{ selectedResource.description }}</p>
            <div v-if="annotationItems(selectedResource).length" class="db-detail-props">
              <div v-for="item in annotationItems(selectedResource)" :key="item.key" class="db-detail-prop">
                <span class="db-detail-prop-key">{{ item.key }}</span>
                <span class="db-detail-prop-value mono">{{ item.value }}</span>
              </div>
            </div>
            <div v-if="(selectedResource.actions || []).length" class="db-detail-actions">
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
        </main>
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

const dbs = computed(() => props.resources.filter(r => r.type === 'Database'))
const tables = computed(() => props.resources.filter(r => r.type === 'Table'))
const columns = computed(() => props.resources.filter(r => r.type === 'Column'))
const rows = computed(() => {
  const r = props.resources.filter(x => x.type === 'Row')
  return r.map(x => {
    try { return JSON.parse(x.description || '{}') } catch { return { value: x.description } }
  })
})
const rowKeys = computed(() => {
  if (!rows.value.length) return []
  return Object.keys(rows.value[0])
})

const dbEngine = computed(() => {
  const conn = props.resources.find(r => r.type === 'Connection')
  return conn?.annotations?.engine || conn?.description || 'Database'
})

const expandedDb = ref<string | null>(null)
const firstSelectableResource = () => props.resources.find(r => r.type !== 'Connection') || props.resources[0] || null
const selectedResource = ref<WorkspaceResource | null>(firstSelectableResource())

const tablesForDb = (dbName: string) =>
  tables.value.filter(t => t.annotations?.database === dbName || !t.annotations?.database)

const selectResource = (resource: WorkspaceResource) => {
  selectedResource.value = resource
}

const findResourceAction = (resource: WorkspaceResource, key: string) =>
  (resource.actions || []).find((action) => action.key === key)

const runResourceAction = (resource: WorkspaceResource, key: string, fallbackTarget = resource.name) => {
  const action = findResourceAction(resource, key)
  if (!action) return
  const target = action.target || fallbackTarget
  emit('action', action, target)
}

const inspectDatabase = (db: WorkspaceResource) => {
  selectResource(db)
  if (expandedDb.value === db.name) {
    expandedDb.value = null
    return
  }
  expandedDb.value = db.name
  runResourceAction(db, 'list_database_tables', db.name)
}

const inspectTable = (table: WorkspaceResource) => {
  selectResource(table)
  runResourceAction(table, 'preview_table_rows', table.name)
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

const workspaceActions = computed(() => props.workspaceActions || [])
const findWorkspaceAction = (key: string) => workspaceActions.value.find((action) => action.key === key)
const actionWithFieldDefaults = (action: WorkspaceAction, defaults: Record<string, string>) => ({
  ...action,
  fields: (action.fields || []).map((field) => ({
    ...field,
    default: field.default || defaults[field.name] || '',
  })),
})
const databaseTableTarget = (databaseName: string, tableName: string) => `${databaseName}\t${tableName}`
const selectedTableDatabase = () => {
  const resource = selectedResource.value
  if (!resource || resource.type !== 'Table') return ''
  return String(resource.annotations?.database || expandedDb.value || '')
}
const selectedObjectDefaults = (): Record<string, string> => {
  const resource = selectedResource.value
  if (!resource) return {}
  if (resource.type === 'Database') return { database: resource.name }
  if (resource.type === 'Table') return { database: selectedTableDatabase(), table: resource.name }
  return {}
}
const contextActionTarget = (action: WorkspaceAction) => {
  if (action.target) return action.target
  const resource = selectedResource.value
  if (resource?.type === 'Database') return resource.name
  if (resource?.type === 'Table') return databaseTableTarget(selectedTableDatabase(), resource.name)
  return undefined
}
const visibleContextActions = computed(() => {
  const resource = selectedResource.value
  const actions: WorkspaceAction[] = []
  for (const key of ['check_database_connection', 'list_databases', 'create_database', 'create_database_backup']) {
    const action = findWorkspaceAction(key)
    if (action) actions.push(action)
  }
  if (resource?.type === 'Database') {
    const createTable = findWorkspaceAction('create_table')
    if (createTable) actions.push(actionWithFieldDefaults(createTable, selectedObjectDefaults()))
    actions.push(...(resource.actions || []))
  }
  if (resource?.type === 'Table') {
    const resourceActions = resource.actions || []
    actions.push(...resourceActions.map((action) => action.fields?.length ? actionWithFieldDefaults(action, selectedObjectDefaults()) : action))
  }
  const seen = new Set<string>()
  return actions.filter((action) => {
    const key = `${action.key || action.label}:${action.target || ''}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
})

const annotationItems = (resource: WorkspaceResource) =>
  Object.entries(resource.annotations || {})
    .filter(([key]) => !['database', 'columnType'].includes(key))
    .map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value) }))

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (rows.value.length) tabs.push({ key: 'data', label: '数据', count: rows.value.length })
  if (columns.value.length) tabs.push({ key: 'columns', label: '字段', count: columns.value.length })
  tabs.push({ key: 'resources', label: '全部资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(rows.value.length ? 'data' : (columns.value.length ? 'columns' : 'resources'))

const resourceKey = (resource?: WorkspaceResource | null) => resource ? `${resource.type}:${resource.name}` : ''
const firstDatabaseWithTables = () => {
  const table = tables.value.find(t => t.annotations?.database)
  return table ? String(table.annotations?.database || '') : ''
}
const syncWorkspaceState = () => {
  const currentKey = resourceKey(selectedResource.value)
  const refreshed = currentKey ? props.resources.find((resource) => resourceKey(resource) === currentKey) : null
  selectedResource.value = refreshed || firstSelectableResource()
  if (!tables.value.length) {
    expandedDb.value = null
  } else if (expandedDb.value && !dbs.value.some((db) => db.name === expandedDb.value)) {
    expandedDb.value = firstDatabaseWithTables() || null
  } else if (!expandedDb.value) {
    expandedDb.value = firstDatabaseWithTables() || null
  }
  if (rows.value.length) {
    activeTab.value = 'data'
    return
  }
  if (columns.value.length) {
    activeTab.value = 'columns'
    return
  }
  if (!availableTabs.value.some((tab) => tab.key === activeTab.value)) {
    activeTab.value = availableTabs.value[0]?.key || 'resources'
  }
}
watch(() => props.resources, syncWorkspaceState, { deep: true })
watch(availableTabs, syncWorkspaceState)

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('up')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('down')) return 'red'
  return 'gray'
}
</script>

<style scoped>
.db-shell {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: var(--paap-space-4);
  align-items: start;
}

/* Sidebar */
.db-sidebar {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  position: sticky;
  top: 0;
}
.db-sidebar-header {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  padding: var(--paap-space-3) var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  color: var(--paap-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  background: var(--paap-panel-subtle);
}
.db-tree {
  max-height: 480px;
  overflow-y: auto;
  padding: var(--paap-space-2);
}
.db-tree-item {
  margin-bottom: 1px;
}
.db-tree-row {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  width: 100%;
  padding: 6px 8px;
  border: none;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  cursor: pointer;
  transition: background 0.1s;
  text-align: left;
  font-family: inherit;
}
.db-tree-row:hover {
  background: var(--paap-panel-subtle);
}
.db-tree-row.selected {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}
.db-tree-chevron {
  flex-shrink: 0;
  transition: transform 0.15s;
  color: var(--paap-muted);
}
.db-tree-chevron.open {
  transform: rotate(90deg);
}
.db-tree-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.db-tree-leaf {
  padding-left: 28px;
}
.db-tree-children {
  padding-left: var(--paap-space-4);
}
.db-tree-empty {
  padding: var(--paap-space-4);
  text-align: center;
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
}

/* Main */
.db-main {
  min-width: 0;
}
.db-context-bar {
  display: grid;
  grid-template-columns: minmax(140px, 220px) minmax(0, 1fr);
  gap: var(--paap-space-4);
  align-items: center;
  margin-bottom: var(--paap-space-3);
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
}
.db-context-summary {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.db-context-summary .ws-section-label {
  margin-bottom: 0;
}
.db-context-summary strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.db-context-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  min-width: 0;
  flex-wrap: wrap;
}
.db-context-action {
  min-width: 64px;
}

/* Data panel */
.db-data-panel {
  min-height: 200px;
}
.db-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--paap-space-12);
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  gap: var(--paap-space-3);
}
.data-table tbody tr.selected td {
  background: var(--paap-accent-soft);
}

/* Detail panel */
.db-detail {
  margin-top: var(--paap-space-4);
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.db-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--paap-space-3);
  margin-bottom: var(--paap-space-3);
}
.db-detail-name {
  font-size: var(--paap-fs-heading-lg);
  font-weight: 600;
  color: var(--paap-text);
  margin-top: 2px;
  word-break: break-all;
}
.db-detail-desc {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.5;
  margin-bottom: var(--paap-space-4);
}
.db-detail-props {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: var(--paap-space-2);
  margin-bottom: var(--paap-space-4);
}
.db-detail-prop {
  padding: var(--paap-space-2) var(--paap-space-3);
  background: var(--paap-panel-subtle);
  border-radius: var(--paap-radius-xs);
}
.db-detail-prop-key {
  display: block;
  font-size: var(--paap-fs-small);
  color: var(--paap-muted);
  margin-bottom: 2px;
}
.db-detail-prop-value {
  font-size: var(--paap-fs-compact);
  color: var(--paap-text);
  word-break: break-all;
}
.db-detail-actions {
  display: flex;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
  padding-top: var(--paap-space-3);
  border-top: 1px solid var(--paap-border);
}

@media (max-width: 768px) {
  .db-shell {
    grid-template-columns: 1fr;
  }
  .db-context-bar {
    grid-template-columns: 1fr;
  }
  .db-context-actions {
    justify-content: flex-start;
  }
  .db-sidebar {
    position: static;
  }
}
</style>
