<template>
  <div :class="['rail-page', { 'rail-page--embedded': embedded }]">
    <header v-if="!embedded" class="page-header">
      <div class="header-text">
        <h1 class="page-title">用户与角色管理</h1>
        <p class="page-desc">管理平台用户和角色，为用户分配角色权限。</p>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div class="user-role-tabs" role="tablist" aria-label="用户角色视图">
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'users'"
        :class="{ active: activeTab === 'users' }"
        @click="setActiveTab('users')"
      >
        用户管理
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'roles'"
        :class="{ active: activeTab === 'roles' }"
        @click="setActiveTab('roles')"
      >
        角色管理
      </button>
    </div>

    <!-- ========== USERS TAB ========== -->
    <div v-show="activeTab === 'users'" class="tab-content" role="tabpanel">
      <div class="users-card">
        <div v-if="loading" class="loading-text">加载中...</div>
        <div v-else-if="users.length === 0" class="loading-text">暂无用户</div>
        <div v-else class="table-wrap">
          <table class="users-table">
            <thead>
              <tr>
                <th class="col-user">用户</th>
                <th>角色</th>
                <th class="col-action">角色分配</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in users" :key="user.id">
                <td>
                  <div class="user-cell">
                    <span class="user-avatar">{{ user.username.charAt(0).toUpperCase() }}</span>
                    <div class="user-info">
                      <div class="cell-username">{{ user.username }}</div>
                      <div v-if="user.email" class="cell-sub">{{ user.email }}</div>
                    </div>
                  </div>
                </td>
                <td>
                  <div class="role-tags">
                    <span v-for="role in user.roles" :key="role" class="role-tag" :class="'role--' + role">{{ roleLabel(role) }}</span>
                  </div>
                </td>
                <td class="cell-action">
                  <div
                    v-if="user.id !== currentUserId"
                    class="role-pills"
                    :aria-busy="updatingId === user.id"
                  >
                    <button
                      v-for="option in roleOptions"
                      :key="option.value"
                      type="button"
                      class="role-pill"
                      :class="{ active: hasRole(user, option.value) }"
                      :disabled="updatingId === user.id"
                      @click="toggleRole(user, option.value, !hasRole(user, option.value))"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                  <span v-else class="self-hint">当前用户</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ========== ROLES TAB ========== -->
    <div v-show="activeTab === 'roles'" class="tab-content" role="tabpanel">
      <div class="roles-toolbar">
        <label for="role-scope-filter">作用域</label>
        <select id="role-scope-filter" v-model="scopeFilter" class="rail-select">
          <option value="">全部</option>
          <option value="system">平台</option>
          <option value="app">应用</option>
          <option value="env">环境</option>
        </select>
        <cv-button kind="ghost" size="sm" :disabled="rolesLoading" @click="loadRoles">
          {{ rolesLoading ? '加载中...' : '刷新' }}
        </cv-button>
        <div class="toolbar-spacer"></div>
        <router-link to="/roles/new" custom v-slot="{ navigate }">
          <cv-button kind="primary" size="sm" @click="navigate">新建角色</cv-button>
        </router-link>
      </div>

      <section class="roles-panel slide-up">
        <div v-if="rolesLoading" class="loading-text">加载中...</div>
        <div v-else-if="filteredRoles.length === 0" class="loading-text">暂无角色</div>
        <div v-else class="table-wrap">
          <table class="roles-table">
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
                  <router-link :to="`/roles/${role.id}`" custom v-slot="{ navigate }">
                    <cv-button kind="ghost" size="sm" @click="navigate">
                      {{ role.editable && !role.builtin ? '编辑' : '查看' }}
                    </cv-button>
                  </router-link>
                  <cv-button
                    v-if="!role.builtin && role.editable"
                    kind="danger"
                    size="sm"
                    :disabled="deletingRoleId === role.id"
                    @click="promptDeleteRole(role)"
                  >
                    {{ deletingRoleId === role.id ? '删除中...' : '删除' }}
                  </cv-button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- Delete confirmation modal -->
      <cv-modal
        kind="danger"
        :visible="pendingDeleteRole !== null"
        @primary-click="confirmDeleteRole"
        @secondary-click="pendingDeleteRole = null"
        @modal-hidden="pendingDeleteRole = null"
        :primary-button-disabled="deletingRoleId !== null"
      >
        <template #title>删除角色</template>
        <template #content>
          <p>{{ pendingDeleteRole?.name }} 删除后无法继续分配给用户。</p>
        </template>
        <template #secondary-button>取消</template>
        <template #primary-button>{{ deletingRoleId !== null ? '删除中...' : '删除' }}</template>
      </cv-modal>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'

// ── Shared ──
const route = useRoute()
const router = useRouter()
const embedded = ref(false)
const pageError = ref('')
const currentUserId = ref(0)

// Tab state
const activeTab = ref<'users' | 'roles'>('users')
const normalizeUserRoleTab = (value: unknown): 'users' | 'roles' =>
  String(value || '') === 'roles' ? 'roles' : 'users'
const setActiveTab = (tab: 'users' | 'roles') => {
  activeTab.value = tab
  if (normalizeUserRoleTab(route.query.tab) !== tab) {
    void router.replace({ path: '/users', query: tab === 'roles' ? { tab: 'roles' } : {} })
  }
}

watch(() => route.query.tab, (tab) => {
  activeTab.value = normalizeUserRoleTab(tab)
}, { immediate: true })

// ── User Management ──
interface UserItem {
  id: number
  username: string
  email: string
  roles: string[]
}

interface RoleOption {
  value: string
  label: string
  builtin: boolean
}

const users = ref<UserItem[]>([])
const roleOptions = ref<RoleOption[]>([])
const loading = ref(false)
const updatingId = ref<number | null>(null)

const normalizeUser = (raw: any): UserItem => ({
  id: Number(raw?.id || 0),
  username: String(raw?.username || raw?.name || ''),
  email: String(raw?.email || ''),
  roles: Array.isArray(raw?.roles) ? raw.roles.map(String) : [],
})

const normalizeRoleOption = (raw: any): RoleOption => ({
  value: String(raw?.code || ''),
  label: String(raw?.name || raw?.code || ''),
  builtin: Boolean(raw?.builtin),
})

const roleLabel = (code: string) => {
  const found = roleOptions.value.find((r) => r.value === code)
  return found ? found.label : code
}

const hasRole = (user: UserItem, role: string) => user.roles.includes(role)

const loadRoleOptions = async () => {
  try {
    const res = await api.listAssignableRoles('system')
    roleOptions.value = Array.isArray(res.data)
      ? res.data.map(normalizeRoleOption).filter((role: RoleOption) => role.value)
      : []
  } catch (e: any) {
    pageError.value = '加载角色失败：' + (e?.message || '未知错误')
  }
}

const loadUsers = async () => {
  loading.value = true
  pageError.value = ''
  try {
    const res = await api.listUsers()
    users.value = Array.isArray(res.data) ? res.data.map(normalizeUser) : []
  } catch (e: any) {
    pageError.value = '加载用户失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

const toggleRole = async (user: UserItem, role: string, checked: boolean) => {
  const previousRoles = [...user.roles]
  const nextRoles = checked
    ? Array.from(new Set([...user.roles, role]))
    : user.roles.filter((item) => item !== role)
  if (nextRoles.length === 0) {
    pageError.value = '用户至少需要保留一个角色'
    return
  }

  user.roles = nextRoles
  updatingId.value = user.id
  pageError.value = ''
  try {
    await api.updateUserRoles(user.id, nextRoles)
  } catch (e: any) {
    user.roles = previousRoles
    pageError.value = '更新角色失败：' + (e?.message || '未知错误')
    await loadUsers()
  } finally {
    updatingId.value = null
  }
}

// ── Role Management ──
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
const rolesLoading = ref(false)
const deletingRoleId = ref<number | null>(null)
const pendingDeleteRole = ref<RoleItem | null>(null)

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
  rolesLoading.value = true
  pageError.value = ''
  try {
    const roleRes = await api.listRoles()
    roles.value = Array.isArray(roleRes.data) ? roleRes.data.map(normalizeRole) : []
  } catch (e: any) {
    pageError.value = '加载角色失败：' + (e?.message || '未知错误')
  } finally {
    rolesLoading.value = false
  }
}

const promptDeleteRole = (role: RoleItem) => {
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

// ── Lifecycle ──
onMounted(async () => {
  try {
    const raw = localStorage.getItem('paap_user')
    if (raw) {
      const u = JSON.parse(raw)
      currentUserId.value = u.id || 0
    }
  } catch {}
  await loadRoleOptions()
  await loadUsers()
  await loadRoles()
})
</script>

<style scoped>
/* ── Layout ── */
.rail-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
  max-width: none;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
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
  line-height: 1.2;
}

.page-desc {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  line-height: 1.4;
}

.page-error {
  border: 1px solid var(--paap-danger);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  padding: 10px 12px;
  font-size: var(--paap-fs-compact);
  margin-bottom: 16px;
}

.tab-content {
  min-height: 200px;
}

.user-role-tabs {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0 0 16px;
  padding: 4px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}

.user-role-tabs button {
  min-width: 96px;
  min-height: 32px;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: var(--paap-radius-xs);
  background: transparent;
  color: var(--paap-muted);
  font: inherit;
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  cursor: pointer;
}

.user-role-tabs button:hover {
  color: var(--paap-text);
  background: var(--paap-panel);
}

.user-role-tabs button.active {
  border-color: var(--paap-border);
  background: var(--paap-panel);
  color: var(--paap-accent);
  box-shadow: var(--paap-shadow-sm);
}

/* ── Users Card ── */
.users-card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
}

.loading-text {
  padding: 40px 20px;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  text-align: center;
}

.table-wrap {
  overflow-x: auto;
}

.users-table {
  width: 100%;
  border-collapse: collapse;
}

.users-table th {
  text-align: left;
  padding: 12px 16px;
  font-size: var(--paap-fs-label);
  font-weight: 600;
  color: var(--paap-muted);
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.users-table td {
  padding: 10px 16px;
  font-size: var(--paap-fs-body);
  color: var(--paap-text);
  border-bottom: 1px solid var(--paap-border);
  vertical-align: middle;
  transition: background var(--paap-transition-fast);
}

.users-table tbody tr:hover {
  background: var(--paap-accent-soft, rgba(15, 98, 254, 0.03));
}

.col-user {
  width: 240px;
}

.col-action {
  width: auto;
  min-width: 240px;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-avatar {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--paap-fs-body);
  font-weight: 700;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.cell-username {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-sub {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-email {
  color: var(--paap-muted);
}

.cell-action {
  text-align: right;
}

.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.role-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--paap-radius-xs, 4px);
  font-size: var(--paap-fs-label);
  font-weight: 500;
}

.role--platform_admin {
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
}

.role--app_admin {
  background: var(--paap-warning-soft);
  color: var(--paap-warning);
}

.role--user {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.role-pills {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
}

.role-pill {
  display: inline-flex;
  align-items: center;
  height: 26px;
  padding: 0 10px;
  border-radius: var(--paap-radius-full);
  border: 1px solid var(--paap-border);
  background: transparent;
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

.role-pill:hover:not(:disabled) {
  border-color: var(--paap-accent);
  color: var(--paap-accent);
  background: var(--paap-accent-soft);
}

.role-pill.active {
  border-color: var(--paap-accent);
  background: var(--paap-accent);
  color: #fff;
}

.role-pill:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.self-hint {
  font-size: var(--paap-fs-label);
  color: var(--paap-muted);
  font-style: italic;
}

/* ── Roles Tab ── */
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

.toolbar-spacer {
  flex: 1;
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
  transition: background var(--paap-transition-fast);
}

.roles-table th {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.roles-table tbody tr:hover {
  background: var(--paap-accent-soft, rgba(15, 98, 254, 0.04));
}

.role-name {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
}

.role-code {
  color: var(--paap-muted);
  font-family: var(--paap-mono, ui-monospace, monospace);
  font-size: var(--paap-fs-label);
}

.type-tag,
.status-pill {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: var(--paap-radius-xs, 4px);
  background: var(--paap-success-soft, rgba(0, 128, 0, 0.08));
  color: var(--paap-success, #008000);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.type-tag.builtin {
  background: var(--paap-accent-soft, rgba(15, 98, 254, 0.08));
  color: var(--paap-accent, #0f62fe);
}

.status-pill.disabled {
  background: var(--paap-panel-subtle, #f4f4f4);
  color: var(--paap-muted);
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

.cell-actions {
  display: flex;
  gap: 8px;
}

/* ── Responsive ── */
@media (max-width: 720px) {
  .page-header,
  .roles-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .users-card,
  .roles-panel {
    overflow-x: auto;
  }

  .users-table {
    min-width: 500px;
  }
  .roles-table {
    min-width: 660px;
  }
}
</style>
