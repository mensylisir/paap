<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-top">
        <router-link to="/apps" class="logo">
          <div class="logo-icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5z"/>
              <path d="M2 17l10 5 10-5"/>
              <path d="M2 12l10 5 10-5"/>
            </svg>
          </div>
          <span class="logo-text">PAAP</span>
        </router-link>
        <nav class="nav">
          <router-link to="/apps" class="nav-item" :class="{ active: isAppsActive }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="7" height="7" rx="1.5"/>
              <rect x="14" y="3" width="7" height="7" rx="1.5"/>
              <rect x="3" y="14" width="7" height="7" rx="1.5"/>
              <rect x="14" y="14" width="7" height="7" rx="1.5"/>
            </svg>
            <span>应用列表</span>
          </router-link>
          <router-link to="/templates" class="nav-item" :class="{ active: $route.path === '/templates' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
              <polyline points="14 2 14 8 20 8"/>
              <line x1="16" y1="13" x2="8" y2="13"/>
              <line x1="16" y1="17" x2="8" y2="17"/>
              <polyline points="10 9 9 9 8 9"/>
            </svg>
            <span>配置模板</span>
          </router-link>
          <router-link to="/catalog" class="nav-item" :class="{ active: $route.path === '/catalog' || $route.path.startsWith('/catalog/') }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="4" cy="4" r="2"/>
              <circle cx="12" cy="4" r="2"/>
              <circle cx="20" cy="4" r="2"/>
              <circle cx="4" cy="12" r="2"/>
              <circle cx="12" cy="12" r="2"/>
              <circle cx="20" cy="12" r="2"/>
              <circle cx="4" cy="20" r="2"/>
              <circle cx="12" cy="20" r="2"/>
              <circle cx="20" cy="20" r="2"/>
            </svg>
            <span>服务目录</span>
          </router-link>
          <router-link v-if="permissionStore.has('system.shared_pool.manage')" to="/shared-resources" class="nav-item" :class="{ active: $route.path === '/shared-resources' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 3l8 4.5-8 4.5-8-4.5L12 3z"/>
              <path d="M4 12l8 4.5 8-4.5"/>
              <path d="M4 16.5l8 4.5 8-4.5"/>
            </svg>
            <span>共享资源</span>
          </router-link>
          <router-link v-if="permissionStore.has('system.shared_pool.manage')" to="/platform/services" class="nav-item" :class="{ active: $route.path === '/platform/services' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 7h16"/>
              <path d="M4 12h16"/>
              <path d="M4 17h16"/>
              <circle cx="8" cy="7" r="1.5"/>
              <circle cx="13" cy="12" r="1.5"/>
              <circle cx="17" cy="17" r="1.5"/>
            </svg>
            <span>平台服务</span>
          </router-link>
          <router-link v-if="permissionStore.has('system.shared_pool.manage')" to="/platform/addons" class="nav-item" :class="{ active: $route.path === '/platform/addons' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2v20"/>
              <path d="M2 12h20"/>
              <path d="M4.9 4.9l14.2 14.2"/>
              <path d="M19.1 4.9L4.9 19.1"/>
            </svg>
            <span>平台插件</span>
          </router-link>
          <router-link v-if="canManageUsersOrRoles" to="/users" class="nav-item" :class="{ active: isUsersRolesActive }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M16 21v-2a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4v2"/>
              <circle cx="9.5" cy="7" r="4"/>
              <path d="M22 21v-2a4 4 0 0 0-3-3.9"/>
              <path d="M16 3.1a4 4 0 0 1 0 7.8"/>
            </svg>
            <span>用户角色</span>
          </router-link>
        </nav>
      </div>
      <div class="sidebar-bottom">
        <div class="user-section">
          <div class="user-avatar">{{ userInitial }}</div>
          <span class="user-name">{{ userName }}</span>
        </div>
        <button class="nav-item logout" @click="logout">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/>
            <polyline points="16 17 21 12 16 7"/>
            <line x1="21" y1="12" x2="9" y2="12"/>
          </svg>
          <span>退出</span>
        </button>
      </div>
    </aside>
    <main class="main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'
import { usePermissionStore } from '../stores/permission'

type StoredUser = {
  id?: number
  name?: string
  username?: string
  email?: string
  roles?: string[]
}

const route = useRoute()
const permissionStore = usePermissionStore()
const isAppsActive = computed(() =>
  route.path === '/apps' || route.path.startsWith('/apps/')
)
const isUsersRolesActive = computed(() =>
  route.path === '/users' || route.path === '/roles' || route.path.startsWith('/roles/')
)
const canManageUsersOrRoles = computed(() =>
  permissionStore.has('system.user.manage') || permissionStore.has('system.role.manage')
)
const readStoredUser = (): StoredUser | null => {
  try {
    const raw = localStorage.getItem('paap_user')
    if (raw) {
      return JSON.parse(raw)
    }
  } catch {}
  return null
}

const currentUser = ref<StoredUser | null>(readStoredUser())

const storeAuthenticatedUser = (user: any) => {
  const nextUser = {
    id: user.id,
    name: user.name,
    username: user.username,
    email: user.email,
    roles: Array.isArray(user.roles) ? user.roles : [],
  }
  localStorage.setItem('paap_user', JSON.stringify(nextUser))
  currentUser.value = nextUser
}

const refreshCurrentUser = async () => {
  if (!localStorage.getItem('paap_token')) return
  try {
    const response = await api.me()
    if (response?.data) storeAuthenticatedUser(response.data)
  } catch {}
}

const userName = computed(() => {
  const user = currentUser.value
  return user?.name || user?.username || user?.email || '用户'
})

const userInitial = computed(() => {
  const name = userName.value
  return name.charAt(0).toUpperCase()
})

const logout = () => {
  localStorage.removeItem('paap_user')
  localStorage.removeItem('paap_token')
  permissionStore.reset()
  location.href = '/login'
}

onMounted(() => {
  refreshCurrentUser()
})
</script>

<style scoped>
.layout {
  display: flex;
  min-height: 100vh;
  min-width: 0;
}

.sidebar {
  width: var(--paap-sidebar);
  flex-shrink: 0;
  background: var(--paap-panel);
  border-right: 1px solid var(--paap-border);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: var(--paap-space-5) 0;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: var(--paap-z-sticky);
}

.sidebar-top {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-2);
}

.logo {
  padding: 0 var(--paap-space-4) var(--paap-space-5);
  text-decoration: none;
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
}

.logo-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--paap-accent);
  color: #fff;
  border-radius: var(--paap-radius-sm);
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: var(--paap-accent);
  letter-spacing: -0.02em;
  font-family: var(--paap-mono);
}

.nav {
  display: flex;
  flex-direction: column;
  padding: 0 var(--paap-space-3);
  gap: 2px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  padding: var(--paap-space-2) var(--paap-space-3);
  border-radius: var(--paap-radius-sm);
  font-size: var(--paap-fs-body);
  font-weight: 500;
  color: var(--paap-muted);
  text-decoration: none;
  transition: all 0.15s ease;
  cursor: pointer;
  background: transparent;
  border: none;
  width: 100%;
  height: 40px;
}

.nav-item:hover {
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
}

.nav-item.active {
  background: var(--paap-accent-fill);
  color: var(--paap-accent);
  font-weight: 600;
  position: relative;
}
.nav-item.active::before {
  content: '';
  position: absolute;
  left: -12px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  border-radius: 0 2px 2px 0;
  background: var(--paap-accent);
}

.sidebar-bottom {
  padding: 0 var(--paap-space-3);
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-2);
}

.user-section {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  padding: var(--paap-space-2) var(--paap-space-3);
  margin-bottom: var(--paap-space-2);
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--paap-radius-full);
  background: var(--paap-accent-soft);
  color: var(--paap-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--paap-fs-compact);
  font-weight: 600;
  flex-shrink: 0;
}

.user-name {
  font-size: var(--paap-fs-compact);
  font-weight: 500;
  color: var(--paap-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nav-item.logout {
  color: var(--paap-muted);
}
.nav-item.logout:hover {
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
}

.main {
  flex: 1;
  margin-left: var(--paap-sidebar);
  width: calc(100% - var(--paap-sidebar));
  min-width: 0;
  background: var(--paap-bg);
  min-height: 100vh;
}

@media (max-width: 768px) {
  .layout {
    display: block;
  }

  .sidebar {
    position: sticky;
    top: 0;
    bottom: auto;
    width: 100%;
    min-width: 0;
    height: auto;
    padding: var(--paap-space-3) var(--paap-space-4);
    border-right: 0;
    border-bottom: 1px solid var(--paap-border);
  }

  .sidebar-top {
    flex-direction: row;
    align-items: center;
    gap: var(--paap-space-4);
    min-width: 0;
  }

  .logo {
    flex-shrink: 0;
    padding: 0;
  }

  .nav {
    flex: 1;
    flex-direction: row;
    gap: var(--paap-space-1);
    min-width: 0;
    overflow-x: auto;
    padding: 0;
  }

  .nav-item {
    width: auto;
    height: 36px;
    white-space: nowrap;
  }

  .sidebar-bottom {
    display: none;
  }

  .main {
    width: 100%;
    margin-left: 0;
  }
}
</style>
