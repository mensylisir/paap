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
          <button type="submit" class="btn btn--primary" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)

const handleLogin = async () => {
  loading.value = true
  setTimeout(() => {
    loading.value = false
    router.push('/apps?auto=true')
  }, 500)
}
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
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
}

/* Brand panel */
.brand-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  padding: var(--paap-space-12);
  background: var(--paap-text);
  color: #ffffff;
}

.brand-icon {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.1);
  border-radius: var(--paap-radius-sm);
  color: #ffffff;
  margin-bottom: var(--paap-space-6);
}

.brand-title {
  font-family: var(--paap-mono);
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #ffffff;
  margin: 0 0 var(--paap-space-2);
}

.brand-desc {
  font-size: 15px;
  color: rgba(255, 255, 255, 0.6);
  line-height: 1.5;
}

/* Form panel */
.form-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: var(--paap-space-12);
}

.form-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--paap-text);
  letter-spacing: -0.02em;
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
  font-weight: 500;
  color: var(--paap-muted);
}
.form-input {
  width: 100%;
  padding: 10px 12px;
  font-size: 14px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  color: var(--paap-text);
  outline: none;
  font-family: inherit;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.form-input:focus {
  border-color: var(--paap-accent);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}
.form-input::placeholder {
  color: var(--paap-muted-2);
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: inherit;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  outline: none;
  border: none;
  height: 44px;
  padding: 0 20px;
  border-radius: var(--paap-radius-sm);
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
