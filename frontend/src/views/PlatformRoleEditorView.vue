<template>
  <div class="rail-page role-editor-page">
    <header class="page-header">
      <div>
        <h1 class="page-title">{{ pageTitle }}</h1>
        <p class="page-desc">{{ pageDesc }}</p>
      </div>
      <router-link class="rail-btn rail-btn--ghost" to="/users?tab=roles">返回用户角色</router-link>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <form class="role-form" @submit.prevent="saveRole">
      <section class="section-card">
        <div class="section-card-header">
          <h2>基本信息</h2>
        </div>
        <div class="section-card-body">
          <div class="form-grid">
            <label>
              <span>编码</span>
              <input v-model.trim="editor.code" class="rail-input mono" :disabled="!isCreate" placeholder="finance.operator" />
            </label>
            <label>
              <span>作用域</span>
              <select v-model="editor.scopeType" class="rail-select" :disabled="!isCreate">
                <option value="system">平台</option>
                <option value="app">应用</option>
                <option value="env">环境</option>
              </select>
            </label>
          </div>

          <label>
            <span>名称</span>
            <input v-model.trim="editor.name" class="rail-input" :disabled="editorReadonly" placeholder="财务操作员" />
          </label>

          <label>
            <span>描述</span>
            <textarea v-model.trim="editor.description" class="rail-textarea" :disabled="editorReadonly" rows="3" />
          </label>

          <label class="enable-row">
            <input v-model="editor.enabled" type="checkbox" :disabled="editorReadonly" />
            <span>启用角色</span>
          </label>
        </div>
      </section>

      <section class="section-card">
        <div class="section-card-header">
          <h2>权限点</h2>
          <span class="perm-count">{{ editor.permissionIds.length }} 已选</span>
        </div>
        <div class="section-card-body">
          <div v-if="loading" class="loading-text compact">加载中...</div>
          <div v-else-if="availablePermissionGroups.length === 0" class="loading-text compact">当前作用域没有可选权限</div>
          <div v-else class="permission-groups">
            <div v-for="group in availablePermissionGroups" :key="group.scopeType + group.group" class="permission-group">
              <div class="permission-group-header">
                <strong>{{ group.group || scopeLabel(group.scopeType) }}</strong>
                <span class="scope-tag">{{ scopeLabel(group.scopeType) }}</span>
              </div>
              <div class="permission-items">
                <label v-for="perm in group.permissions" :key="perm.id" class="permission-check">
                  <input
                    type="checkbox"
                    :checked="permissionChecked(perm.id)"
                    :disabled="editorReadonly"
                    @change="togglePermission(perm.id, eventChecked($event))"
                  />
                  <span>
                    <strong>{{ perm.name }}</strong>
                    <small>{{ perm.code }}</small>
                  </span>
                </label>
              </div>
            </div>
          </div>
        </div>
      </section>

      <div class="editor-actions">
        <router-link class="rail-btn rail-btn--ghost" to="/users?tab=roles">取消</router-link>
        <button type="submit" class="rail-btn rail-btn--primary" :disabled="editorReadonly || saving">
          {{ saving ? '保存中...' : '保存' }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
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

interface PermissionItem {
  id: number
  code: string
  name: string
  scopeType: ScopeType
}

interface PermissionGroup {
  group: string
  scopeType: ScopeType
  permissions: PermissionItem[]
}

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const pageError = ref('')
const loadedRole = ref<RoleItem | null>(null)
const permissionGroups = ref<PermissionGroup[]>([])
const editor = ref({
  id: 0,
  code: '',
  name: '',
  description: '',
  scopeType: 'app' as ScopeType,
  enabled: true,
  permissionIds: [] as number[],
})

const isCreate = computed(() => route.name === 'RoleCreate')
const editorReadonly = computed(() => Boolean(loadedRole.value?.builtin || loadedRole.value && !loadedRole.value.editable))
const pageTitle = computed(() => isCreate.value ? '新建角色' : loadedRole.value?.name || '角色详情')
const pageDesc = computed(() => {
  if (isCreate.value) return '为平台、应用或环境创建自定义角色，并勾选该角色拥有的权限点。'
  return editorReadonly.value ? '内置角色由系统维护，只能查看权限点。' : '修改自定义角色的名称、状态和权限点。'
})
const roleListRoute = { path: '/users', query: { tab: 'roles' } }

const availablePermissionGroups = computed(() =>
  permissionGroups.value
    .map((group) => ({
      ...group,
      permissions: group.permissions.filter((permission) => roleCanUsePermission(editor.value.scopeType, permission.scopeType)),
    }))
    .filter((group) => group.permissions.length > 0)
)

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

const normalizePermissionGroup = (raw: any): PermissionGroup => ({
  group: String(raw?.group || ''),
  scopeType: String(raw?.scopeType || 'system') as ScopeType,
  permissions: Array.isArray(raw?.permissions)
    ? raw.permissions.map((perm: any) => ({
        id: Number(perm?.id || 0),
        code: String(perm?.code || ''),
        name: String(perm?.name || perm?.code || ''),
        scopeType: String(perm?.scopeType || 'system') as ScopeType,
      })).filter((perm: PermissionItem) => perm.id > 0)
    : [],
})

const scopeLabel = (scopeType: string) => {
  const labels: Record<string, string> = { system: '平台', app: '应用', env: '环境' }
  return labels[scopeType] || scopeType
}

const roleCanUsePermission = (roleScopeType: ScopeType, permissionScopeType: ScopeType) => {
  if (roleScopeType === 'system') return permissionScopeType === 'system'
  if (roleScopeType === 'app') return permissionScopeType === 'app' || permissionScopeType === 'env'
  if (roleScopeType === 'env') return permissionScopeType === 'env'
  return false
}

const eventChecked = (event: Event) => Boolean((event.target as HTMLInputElement | null)?.checked)
const permissionChecked = (permissionID: number) => editor.value.permissionIds.includes(permissionID)

const togglePermission = (permissionID: number, checked: boolean) => {
  if (editorReadonly.value) return
  editor.value.permissionIds = checked
    ? Array.from(new Set([...editor.value.permissionIds, permissionID]))
    : editor.value.permissionIds.filter((id) => id !== permissionID)
}

const selectRole = (role: RoleItem) => {
  loadedRole.value = role
  editor.value = {
    id: role.id,
    code: role.code,
    name: role.name,
    description: role.description,
    scopeType: role.scopeType,
    enabled: role.enabled,
    permissionIds: [...role.permissionIds],
  }
}

const loadPage = async () => {
  loading.value = true
  pageError.value = ''
  try {
    const [roleRes, permissionRes] = await Promise.all([api.listRoles(), api.permissionTree()])
    permissionGroups.value = Array.isArray(permissionRes.data) ? permissionRes.data.map(normalizePermissionGroup) : []
    if (!isCreate.value) {
      const roleId = Number(route.params.roleId || 0)
      const role = Array.isArray(roleRes.data)
        ? roleRes.data.map(normalizeRole).find((item: RoleItem) => item.id === roleId)
        : null
      if (!role) throw new Error('角色不存在')
      selectRole(role)
    }
  } catch (e: any) {
    pageError.value = '加载角色失败：' + (e?.message || '未知错误')
  } finally {
    loading.value = false
  }
}

const saveRole = async () => {
  if (editorReadonly.value || saving.value) return
  if (!editor.value.name || (isCreate.value && !editor.value.code)) {
    pageError.value = '角色编码和名称不能为空'
    return
  }
  if (editor.value.permissionIds.length === 0) {
    pageError.value = '至少选择一个权限点'
    return
  }
  saving.value = true
  pageError.value = ''
  const payload = {
    code: editor.value.code,
    name: editor.value.name,
    description: editor.value.description,
    scopeType: editor.value.scopeType,
    enabled: editor.value.enabled,
    permissionIds: editor.value.permissionIds,
  }
  try {
    if (isCreate.value) {
      await api.createRole(payload)
    } else {
      await api.updateRole(editor.value.id, payload)
    }
    await router.push(roleListRoute)
  } catch (e: any) {
    pageError.value = '保存角色失败：' + (e?.message || '未知错误')
  } finally {
    saving.value = false
  }
}

onMounted(loadPage)
</script>

<style scoped>
.role-editor-page {
  padding: var(--paap-space-5) var(--paap-space-5) var(--paap-space-10);
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
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

/* ── Section Cards ── */
.section-card {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  overflow: hidden;
}

.section-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
}

.section-card-header h2 {
  color: var(--paap-text);
  font-size: var(--paap-fs-body);
  font-weight: 600;
  margin: 0;
}

.perm-count {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 500;
}

.section-card-body {
  padding: 18px;
  display: grid;
  gap: 14px;
}

/* ── Form Layout ── */
.role-form {
  display: grid;
  gap: 16px;
}

.role-form label {
  display: grid;
  gap: 6px;
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  font-weight: 600;
}

.form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 180px;
  gap: 12px;
}

.mono {
  font-family: var(--paap-mono);
}

.rail-textarea {
  width: 100%;
  min-height: 82px;
  resize: vertical;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  padding: 10px 12px;
  color: var(--paap-text);
  font: inherit;
}

.rail-textarea:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-focus-ring);
  outline: none;
}

.rail-input:disabled,
.rail-select:disabled,
.rail-textarea:disabled {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.enable-row {
  display: inline-flex;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 8px;
}

/* ── Permission Groups ── */
.permission-groups {
  display: grid;
  gap: 10px;
}

.permission-group {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  overflow: hidden;
}

.permission-group-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  border-bottom: 1px solid var(--paap-border);
}

.permission-group-header strong {
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
}

.scope-tag {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 7px;
  border-radius: var(--paap-radius-xs);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  font-size: var(--paap-fs-small);
  font-weight: 600;
}

.permission-items {
  display: grid;
}

.permission-check {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--paap-border);
  font-weight: 400;
  cursor: pointer;
  transition: background var(--paap-transition-fast);
}

.permission-check:last-child {
  border-bottom: 0;
}

.permission-check:hover {
  background: var(--paap-accent-soft, rgba(15, 98, 254, 0.03));
}

.permission-check span {
  display: grid;
  gap: 2px;
}

.permission-check small {
  color: var(--paap-muted);
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-label);
}

/* ── Actions ── */
.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.loading-text.compact {
  padding: 18px 12px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  text-align: center;
}

@media (max-width: 720px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
