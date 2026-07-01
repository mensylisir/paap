<template>
  <div class="login-page">
    <div class="login-card slide-up">
      <div class="brand-panel">
        <div class="brand-bg-pattern" />
        <div class="brand-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <h1 class="brand-title">PAAP</h1>
        <p class="brand-desc">应用管理平台</p>
      </div>
      <div class="form-panel">
        <h2 class="form-title">欢迎回来</h2>
        <p class="form-subtitle">登录以管理你的应用和环境</p>
        <form @submit.prevent="handleLogin" class="login-form">
          <div class="form-group">
            <label class="form-label">用户名</label>
            <input v-model.trim="username" type="text" class="form-input" placeholder="请输入用户名" required />
          </div>
          <div class="form-group">
            <label class="form-label">密码</label>
            <input v-model.trim="password" type="password" class="form-input" placeholder="请输入密码" required />
          </div>
          <p v-if="errorMessage" class="login-error" role="alert">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
            <span>{{ errorMessage }}</span>
          </p>
          <button type="submit" class="rail-btn rail-btn--primary login-btn" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>
          <button type="button" class="rail-btn rail-btn--secondary login-btn" :disabled="loading" @click="loginWithKeycloak">
            Keycloak 登录
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMessage = ref('')

const storeAuthenticatedUser = (user: any) => {
  localStorage.setItem('paap_user', JSON.stringify({
    id: user.id,
    username: user.username,
    roles: Array.isArray(user.roles) ? user.roles : [],
  }))
}

const handleLogin = async () => {
  errorMessage.value = ''
  loading.value = true
  try {
    const response = await api.login(username.value, password.value)
    const user = response?.data || {}
    if (!user.token) {
      throw new Error('登录响应缺少 token')
    }
    localStorage.setItem('paap_token', user.token)
    storeAuthenticatedUser(user)
    router.push('/apps?auto=true')
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : '请检查用户名和密码'
  } finally {
    loading.value = false
  }
}

const loginWithKeycloak = () => {
  location.href = '/api/v1/auth/keycloak/login'
}

const completeTokenLogin = async (token: string) => {
  errorMessage.value = ''
  loading.value = true
  try {
    localStorage.setItem('paap_token', token)
    const response = await api.me()
    storeAuthenticatedUser(response?.data || {})
    router.replace('/apps?auto=true')
  } catch (err) {
    localStorage.removeItem('paap_token')
    localStorage.removeItem('paap_user')
    errorMessage.value = err instanceof Error ? err.message : 'Keycloak 登录失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (token) {
    completeTokenLogin(token)
  }
})
</script>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: var(--paap-bg);
}

.login-card {
  display: flex;
  width: 840px;
  max-width: 95vw;
  min-height: 480px;
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  box-shadow: var(--paap-shadow-lg);
  overflow: hidden;
  animation: card-enter 0.45s ease-out;
}

@keyframes card-enter {
  from { opacity: 0; transform: translateY(16px); }
  to   { opacity: 1; transform: translateY(0); }
}

/* ── Brand panel: warm gradient ── */
.brand-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  padding: var(--paap-space-12);
  background: linear-gradient(135deg, var(--paap-accent) 0%, var(--paap-accent-secondary) 100%);
  color: #ffffff;
  position: relative;
  overflow: hidden;
  animation: fade-in 0.4s 0.1s ease-out both;
}

.brand-bg-pattern {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 20% 80%, rgba(255,255,255,0.10) 0%, transparent 50%),
    radial-gradient(circle at 80% 20%, rgba(255,255,255,0.06) 0%, transparent 50%);
  pointer-events: none;
}

.brand-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.15);
  border-radius: var(--paap-radius);
  color: #ffffff;
  margin-bottom: var(--paap-space-6);
  position: relative;
  backdrop-filter: blur(4px);
}

.brand-title {
  font-family: var(--paap-mono);
  font-size: 30px;
  font-weight: 500;
  letter-spacing: -0.5px;
  color: #ffffff;
  margin: 0 0 var(--paap-space-2);
  position: relative;
}

.brand-desc {
  font-size: 15px;
  color: rgba(255, 255, 255, 0.7);
  line-height: 1.5;
  position: relative;
}

/* ── Form panel ── */
.form-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: var(--paap-space-12);
  animation: fade-in 0.4s 0.15s ease-out both;
}

.form-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--paap-text);
  margin: 0 0 4px;
}

.form-subtitle {
  font-size: var(--paap-fs-body);
  color: var(--paap-muted);
  margin: 0 0 var(--paap-space-8);
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: var(--paap-space-5);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: var(--paap-fs-label);
  font-weight: 500;
  color: var(--paap-muted);
}

.form-input {
  width: 100%;
  padding: 10px 13px;
  font-size: var(--paap-fs-body);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
  color: var(--paap-text);
  outline: none;
  font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.form-input:focus {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-accent-ring);
}

.form-input::placeholder {
  color: var(--paap-muted);
}

.login-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 10px 12px;
  border: 1px solid var(--paap-danger);
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
  font-size: var(--paap-fs-compact);
  border-radius: var(--paap-radius-sm);
  line-height: 1.4;
}

.login-btn {
  margin-top: var(--paap-space-2);
  height: 40px;
  font-size: var(--paap-fs-body);
}

@media (max-width: 672px) {
  .login-card { flex-direction: column; min-height: auto; width: 100%; }
  .brand-panel { padding: var(--paap-space-8); align-items: center; text-align: center; }
  .form-panel { padding: var(--paap-space-8); }
}
</style>
