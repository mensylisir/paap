<template>
  <div class="rail-page">
    <header class="page-header">
      <div class="header-text">
        <h1 class="page-title">用户</h1>
        <p class="page-desc">管理所有平台用户的角色</p>
      </div>
    </header>

    <div v-if="pageError" class="page-error" role="alert">{{ pageError }}</div>

    <div class="users-card">
      <div v-if="loading" class="loading-text">加载中...</div>
      <div v-else-if="users.length === 0" class="loading-text">暂无用户</div>
      <table v-else class="users-table">
        <thead>
          <tr>
            <th>用户名</th>
            <th>邮箱</th>
            <th>角色</th>
            <th class="col-action">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td class="cell-username">{{ user.username }}</td>
            <td class="cell-email">{{ user.email || '-' }}</td>
            <td>
              <div class="role-tags">
                <span v-for="role in user.roles" :key="role" class="role-tag" :class="'role--' + role">{{ roleLabel(role) }}</span>
              </div>
            </td>
            <td class="cell-action">
              <div
                v-if="user.id !== currentUserId"
                class="role-checks"
                :aria-busy="updatingId === user.id"
              >
                <label v-for="option in roleOptions" :key="option.value" class="role-check">
                  <input
                    type="checkbox"
                    :checked="hasRole(user, option.value)"
                    :disabled="updatingId === user.id"
                    @change="toggleRole(user, option.value, eventChecked($event))"
                  />
                  <span>{{ option.label }}</span>
                </label>
              </div>
              <span v-else class="self-hint">当前用户</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

interface UserItem {
  id: number
  username: string
  email: string
  roles: string[]
}

const loading = ref(false)
const pageError = ref('')
const users = ref<UserItem[]>([])
const updatingId = ref<number | null>(null)
const currentUserId = ref(0)

const roleOptions = [
  { value: 'platform_admin', label: '平台管理员' },
  { value: 'app_admin', label: '应用管理员' },
  { value: 'user', label: '普通用户' },
]

const roleLabel = (role: string) => {
  const labels: Record<string, string> = {
    platform_admin: '平台管理员',
    app_admin: '应用管理员',
    user: '普通用户',
  }
  return labels[role] || role
}

const normalizeUser = (raw: any): UserItem => ({
  id: Number(raw?.id || 0),
  username: String(raw?.username || ''),
  email: String(raw?.email || ''),
  roles: Array.isArray(raw?.roles) ? raw.roles.map((role: any) => String(role)) : [],
})

const hasRole = (user: UserItem, role: string) => user.roles.includes(role)

const eventChecked = (event: Event) => Boolean((event.target as HTMLInputElement | null)?.checked)

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

onMounted(() => {
  try {
    const raw = localStorage.getItem('paap_user')
    if (raw) {
      const u = JSON.parse(raw)
      currentUserId.value = u.id || 0
    }
  } catch {}
  loadUsers()
})
</script>

<style scoped>
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
  line-height: 1.2;
}
.page-desc {
  font-size: 14px;
  color: var(--cds-text-secondary, #525252);
  line-height: 1.4;
}
.page-error {
  border: 1px solid #fecaca;
  border-radius: 4px;
  background: #fef2f2;
  color: #dc2626;
  padding: 10px 12px;
  font-size: 13px;
  margin-bottom: 16px;
}
.users-card {
  background: #fff;
  border: 1px solid var(--cds-border-subtle-01, #e0e0e0);
}
.loading-text {
  padding: 40px 20px;
  color: var(--cds-text-secondary, #525252);
  font-size: 14px;
  text-align: center;
}
.users-table {
  width: 100%;
  border-collapse: collapse;
}
.users-table th {
  text-align: left;
  padding: 12px 16px;
  font-size: 12px;
  font-weight: 600;
  color: var(--cds-text-secondary, #525252);
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  background: var(--cds-layer-02, #f4f4f4);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}
.users-table td {
  padding: 12px 16px;
  font-size: 14px;
  color: var(--cds-text-primary, #161616);
  border-bottom: 1px solid var(--cds-border-subtle-01, #e0e0e0);
  vertical-align: middle;
}
.col-action {
  width: 420px;
}
.cell-username {
  font-weight: 600;
}
.cell-email {
  color: var(--cds-text-secondary, #525252);
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
  border-radius: 3px;
  font-size: 12px;
  font-weight: 500;
}
.role--platform_admin {
  background: #e8f0fe;
  color: #1967d2;
}
.role--app_admin {
  background: #fef7e0;
  color: #e37400;
}
.role--user {
  background: var(--cds-layer-02, #f4f4f4);
  color: var(--cds-text-secondary, #525252);
}
.role-checks {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}
.role-check {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  color: var(--cds-text-primary, #161616);
  white-space: nowrap;
}
.role-check input {
  margin: 0;
}
.self-hint {
  font-size: 12px;
  color: var(--cds-text-secondary, #525252);
  font-style: italic;
}
</style>
