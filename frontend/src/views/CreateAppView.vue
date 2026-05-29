<template>
  <div class="page">
    <div class="form-card" style="max-width:640px">
      <nav class="bx--breadcrumb bx--breadcrumb--no-trailing-slash">
        <div class="bx--breadcrumb-item"><router-link to="/apps">我的应用</router-link></div>
        <div class="bx--breadcrumb-item bx--breadcrumb-item--current">创建应用</div>
      </nav>

      <h1 class="bx--type-productive-heading-04" style="margin:16px 0 24px">创建应用</h1>

      <form @submit.prevent="submit" class="bx--form">
        <div class="bx--form-item">
          <label class="bx--label">应用名称 <span class="required">*</span></label>
          <input v-model="form.name" class="bx--text-input" placeholder="例如：订单服务" required />
        </div>
        <div class="bx--form-item">
          <label class="bx--label">应用标识 <span class="required">*</span></label>
          <input v-model="form.identifier" class="bx--text-input" placeholder="例如：order-service" pattern="[a-z0-9-]+" required />
          <div class="bx--form__helper-text">唯一英文标识，用于生成命名空间。仅支持小写字母、数字、短横线。</div>
        </div>
        <div class="bx--form-item">
          <label class="bx--label">应用描述</label>
          <input v-model="form.description" class="bx--text-input" placeholder="简要描述应用的用途" />
        </div>

        <div class="bx--btn-set" style="margin-top:32px">
          <button type="button" class="bx--btn bx--btn--secondary" @click="$router.push('/apps')">取消</button>
          <button type="submit" class="bx--btn bx--btn--primary" :disabled="submitting">
            {{ submitting ? '创建中...' : '创建应用' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'

const router = useRouter()
const form = ref({ name: '', identifier: '', description: '' })
const submitting = ref(false)

const submit = async () => {
  if (!form.value.name || !form.value.identifier) return
  submitting.value = true
  try {
    const res = await api.createApp({
      name: form.value.name,
      identifier: form.value.identifier,
      description: form.value.description,
    })
    router.push(`/apps/${res.data.id}/overview`)
  } catch (e: any) {
    alert('创建失败：' + e.message)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.page {
  padding: 32px;
}
.form-card {
  background: #fff;
  border: 1px solid #e0e0e0;
  padding: 32px;
}
.required {
  color: #da1e28;
}
.bx--form-item {
  margin-bottom: 24px;
}
</style>
