<template>
  <div class="login-page">
    <div class="bx--tile login-card">
      <h2 class="bx--type-productive-heading-03">PAAP 平台登录</h2>
      <form @submit.prevent="handleLogin" class="bx--form">
        <div class="bx--form-item">
          <label class="bx--label">用户名</label>
          <input v-model="username" type="text" class="bx--text-input" placeholder="请输入用户名" />
        </div>
        <div class="bx--form-item">
          <label class="bx--label">密码</label>
          <input v-model="password" type="password" class="bx--text-input" placeholder="请输入密码" />
        </div>
        <button type="submit" class="bx--btn bx--btn--primary" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
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
    router.push('/apps')
  }, 500)
}
</script>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f4f4f4;
}

.login-card {
  padding: 48px;
  min-width: 400px;
}

.login-card h2 {
  margin-bottom: 32px;
  text-align: center;
}
</style>
