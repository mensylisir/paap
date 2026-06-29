<template>
  <div class="rail-page roles-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">角色</h1>
        <p class="page-desc">查看系统内置角色和自定义角色，权限编辑在独立页面完成。</p>
      </div>
      <router-link class="rail-btn rail-btn--primary" to="/roles/new">新建角色</router-link>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div class="roles-toolbar">
      <label for="role-scope-filter">作用域</label>
      <select id="role-scope-filter" v-model="scopeFilter" class="rail-select">
        <option value="">全部</option>
        <option value="system">平台</option>
        <option value="app">应用</option>
        <option value="env">环境</option>
      </select>
      <button type="button" class="rail-btn rail-btn--ghost rail-btn--sm" :disabled="loading" @click="loadRoles">
        {{ loading ? '加载中...' : '刷新' }}
      </button>
    </div>

    <section class="roles-panel">
      <div v-if="loading" class="loading-text">加载中...</div>
      <div v-else-if="filteredRoles.length === 0" class="loading-text">暂无角色</div>
      <table v-else class="roles-table">
        <thead>
          <tr>
            <th>角色</th>
            <th>作用域</th>
            <th>类型</th>
            <th>状态</th>
            <th>权限</th>
            <th class="col-action">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="role in filteredRoles" :key="role.id">
            <td>
              <div class="role-name">{{ role.name }}</div>
              <div class="role-code">{{ role.code }}</div>
            </td>
            <td>{{ scopeLabel(role.scopeType) }}</td>
            <td>
              <span class="type-tag" :class="{ builtin: role.builtin }">{{ role.builtin ? '内置' : '自定义' }}</span>
            </td>
            <td>
              <span class="status-pill" :class="{ disabled: !role.enabled }">{{ role.enabled ? '启用' : '停用' }}</span>
            </td>
            <td><span class="perm-badge">{{ role.permissionIds.length }}</span></td>
            <td class="cell-actions">
              <router-link class="rail-btn rail-btn--ghost rail-btn--sm" :to="`/roles/${role.id}`">
                {{ role.editable && !role.builtin ? '编辑' : '查看' }}
              </router-link>
              <button
                v-if="!role.builtin && role.editable"
                type="button"
                class="rail-btn rail-btn--danger rail-btn--sm"
                :disabled="deletingRoleId === role.id"
                @click="deleteRole(role)"
              >
                {{ deletingRoleId === role.id ? '删除中...' : '删除' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <div v-if="pendingDeleteRole" class="modal-backdrop" role="presentation" @click.self="pendingDeleteRole = null">
      <div class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="delete-role-title">
        <h2 id="delete-role-title">删除角色</h2>
        <p>{{ pendingDeleteRole.name }} 删除后无法继续分配给用户。</p>
        <div class="confirm-actions">
          <button type="button" class="rail-btn rail-btn--ghost" :disabled="deletingRoleId !== null" @click="pendingDeleteRole = null">取消</button>
          <button type="button" class="rail-btn rail-btn--danger" :disabled="deletingRoleId !== null" @click="confirmDeleteRole">
            {{ deletingRoleId !== null ? '删除中...' : '删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'

type ScopeType = 'system' | 'app' | 'env'

interface RoleItem {
  id: number
  code: string
  name: string
  description: string
  scopeType: ScopeType
  builtin: boolean
  editable: boolean
  enabled: boolean
  permissionIds: number[]
}

const roles = ref<RoleItem[]>([])
const scopeFilter = ref('')
const loading = ref(false)
const deletingRoleId = ref<number | null>(null)
const pendingDeleteRole = ref<RoleItem | null>(null)
const pageError = ref('')

const filteredRoles = computed(() => {
  if (!scopeFilter.value) return roles.value
  return roles.value.filter((role) => role.scopeType === scopeFilter.value)
})

const normalizeRole = (raw: any): RoleItem => ({
  id: Number(raw?.id || 0),
  code: String(raw?.code || ''),
  name: String(raw?.name || raw?.code || ''),
  description: String(raw?.description || ''),
  scopeType: String(raw?.scopeType || 'app') as ScopeType,
  builtin: Boolean(raw?.builtin),
  editable: Boolean(raw?.editable),
  enabled: raw?.enabled !== false,
  permissionIds: Array.isArray(raw?.permissionIds) ? raw.permissionIds.map((id: any) => Number(id)).filter(Boolean) : [],
})

const scopeLabel = (scopeType: string) => {
  const labels: Record<string, string> = { system: '平台', app: '应用', env: '环境' }
  return labels[scopeType] || scopeType
}

const loadRoles = async () => {
  loading.value = true
  pageError.value = ''
  try {
    const roleRes = await api.listRoles()
    roles.value = Array.isArray(roleRes.data) ? roleRes.data.map(normalizeRole) : []
  } catch (e: any) {
    pageError.value = '加载角色失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

const deleteRole = async (role: RoleItem) => {
  if (role.builtin || !role.editable || deletingRoleId.value !== null) return
  pendingDeleteRole.value = role
}

const confirmDeleteRole = async () => {
  const role = pendingDeleteRole.value
  if (!role || deletingRoleId.value !== null) return
  deletingRoleId.value = role.id
  pageError.value = ''
  try {
    await api.deleteRole(role.id)
    await loadRoles()
    pendingDeleteRole.value = null
  } catch (e: any) {
    pageError.value = '删除角色失败：' + (e?.message || '未知错误')
  } finally {
    deletingRoleId.value = null
  }
}

onMounted(loadRoles)
</script>

<style scoped>
.roles-page {
  padding: 20px 20px 36px;
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.header-text {
  display: grid;
  gap: 2px;
}

.page-title {
  color: var(--paap-text);
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.page-desc {
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
}

.page-error {
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  font-size: var(--paap-fs-compact);
}

.roles-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.roles-toolbar label {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

.roles-toolbar .rail-select {
  width: 160px;
}

.roles-panel {
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
  border-radius: var(--paap-radius);
  overflow: hidden;
}

.roles-table {
  width: 100%;
  border-collapse: collapse;
}

.roles-table th,
.roles-table td {
  border-bottom: 1px solid var(--paap-border);
  padding: 12px 14px;
  text-align: left;
  vertical-align: middle;
}

.roles-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.roles-table tbody tr:hover {
  background: var(--paap-accent-soft);
}

.perm-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 22px;
  padding: 0 6px;
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.role-name {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
}

.role-code {
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
}

.type-tag,
.status-pill {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: var(--paap-radius-xs);
  background: var(--paap-success-soft);
  color: var(--paap-success);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.type-tag.builtin {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}

.status-pill.disabled {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.col-action {
  width: 160px;
}

.cell-actions {
  display: flex;
  gap: 8px;
}

.loading-text {
  padding: 34px 20px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  text-align: center;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: var(--paap-z-dropdown);
  display: grid;
  place-items: center;
  padding: 24px;
  background: var(--paap-overlay);
}

.confirm-dialog {
  width: min(420px, 100%);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  box-shadow: var(--paap-shadow-lg);
  padding: 20px;
}

.confirm-dialog h2 {
  color: var(--paap-text);
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
}

.confirm-dialog p {
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  margin-bottom: 18px;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 720px) {
  .page-header,
  .roles-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .roles-toolbar .rail-select {
    width: 100%;
  }

  .roles-panel {
    overflow-x: auto;
  }

  .roles-table {
    min-width: 720px;
  }
}
</style>
