<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand-panel">
        <div class="brand-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <h1 class="brand-title">PAAP</h1>
        <p class="brand-desc">应用管理平台</p>
      </div>
      <div class="form-panel">
        <h2 class="form-title">登录</h2>
        <form @submit.prevent="handleLogin" class="login-form">
          <div class="form-group">
            <label class="form-label">用户名</label>
            <input v-model.trim="username" type="text" class="form-input" placeholder="请输入用户名" required />
          </div>
          <div class="form-group">
            <label class="form-label">密码</label>
            <input v-model.trim="password" type="password" class="form-input" placeholder="请输入密码" required />
          </div>
          <p v-if="errorMessage" class="login-error" role="alert">登录失败：{{ errorMessage }}</p>
          <button type="submit" class="btn btn--primary" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>
          <button type="button" class="btn btn--secondary" :disabled="loading" @click="loginWithKeycloak">
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
  width: 820px;
  max-width: 95vw;
  min-height: 460px;
  background: var(--cds-layer-01);
  border: 1px solid var(--cds-border-subtle-01);
  border-radius: 0;
  overflow: hidden;
}

.brand-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  padding: var(--paap-space-12);
  background: var(--cds-layer-accent-01);
  color: var(--cds-text-primary);
  border-right: 1px solid var(--cds-border-subtle-01);
}

.brand-icon {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--cds-interactive-01);
  border-radius: 0;
  color: var(--cds-text-on-color);
  margin-bottom: var(--paap-space-6);
}

.brand-title {
  font-family: var(--paap-mono);
  font-size: 28px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--cds-text-primary);
  margin: 0 0 var(--paap-space-2);
}

.brand-desc {
  font-size: 15px;
  color: var(--cds-text-secondary);
  line-height: 1.5;
}

.form-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: var(--paap-space-12);
}

.form-title {
  font-size: 24px;
  font-weight: 400;
  color: var(--cds-text-primary);
  letter-spacing: 0;
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
  font-size: 13px;
  font-weight: 400;
  color: var(--cds-text-secondary);
}
.form-input {
  width: 100%;
  padding: 10px 12px;
  font-size: 14px;
  border: 0;
  border-bottom: 1px solid var(--cds-border-strong-01);
  border-radius: 0;
  background: var(--cds-field-01);
  color: var(--cds-text-primary);
  outline: none;
  font-family: inherit;
  transition: box-shadow 0.15s;
}
.form-input:focus {
  box-shadow: inset 0 0 0 2px var(--cds-border-interactive);
}
.form-input::placeholder {
  color: var(--cds-text-placeholder);
}

.login-error {
  margin: 0;
  padding: 10px 12px;
  background: var(--cds-support-error-inverse, #da1e28);
  color: var(--cds-text-on-color);
  font-size: 13px;
  line-height: 1.4;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: inherit;
  font-size: 14px;
  font-weight: 400;
  cursor: pointer;
  outline: none;
  border: none;
  height: 44px;
  padding: 0 20px;
  border-radius: 0;
  transition: all 0.15s;
  margin-top: var(--paap-space-2);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn--primary {
  background: var(--cds-button-primary, var(--paap-accent));
  color: #ffffff;
}
.btn--primary:hover:not(:disabled) {
  background: var(--cds-button-primary-hover, var(--paap-accent-hover));
}

@media (max-width: 640px) {
  .login-card { flex-direction: column; min-height: auto; }
  .brand-panel { padding: var(--paap-space-8); align-items: center; text-align: center; }
  .form-panel { padding: var(--paap-space-8); }
}
</style>
