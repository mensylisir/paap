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
            <span>应用</span>
          </router-link>
          <router-link to="/templates" class="nav-item" :class="{ active: $route.path === '/templates' }">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/>
              <polyline points="14 2 14 8 20 8"/>
              <line x1="16" y1="13" x2="8" y2="13"/>
              <line x1="16" y1="17" x2="8" y2="17"/>
              <polyline points="10 9 9 9 8 9"/>
            </svg>
            <span>模板</span>
          </router-link>
          <router-link to="/catalog" class="nav-item" :class="{ active: $route.path === '/catalog' }">
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
            <span>目录</span>
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
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const isAppsActive = computed(() =>
  route.path === '/apps' || route.path.startsWith('/apps/')
)

const userName = computed(() => {
  try {
    const raw = localStorage.getItem('paap_user')
    if (raw) {
      const u = JSON.parse(raw)
      return u.name || u.username || u.email || '用户'
    }
  } catch {}
  return '用户'
})

const userInitial = computed(() => {
  const name = userName.value
  return name.charAt(0).toUpperCase()
})

const logout = () => {
  localStorage.removeItem('paap_user')
  localStorage.removeItem('paap_token')
  location.href = '/login'
}
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
  z-index: 100;
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
  background: var(--cds-background-brand, var(--paap-accent));
  color: var(--cds-icon-on-color, #fff);
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
  font-size: 14px;
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
  background: var(--paap-panel-subtle);
  color: var(--cds-icon-interactive, var(--paap-accent));
  font-weight: 600;
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
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.user-name {
  font-size: 13px;
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
