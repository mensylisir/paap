<template>
  <div class="rail-page shared-resource-page">
    <section class="section-card shared-resource-loading">
      <div class="loading-spinner" aria-hidden="true" />
      <div>
        <h1 class="rail-section-title">共享资源</h1>
        <p class="rail-section-desc">{{ error || '正在打开共享资源池画布...' }}</p>
      </div>
      <button v-if="error" type="button" class="rail-btn rail-btn--secondary" @click="openSharedResourcePool">
        重试
      </button>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'

const router = useRouter()
const error = ref('')

const openSharedResourcePool = async () => {
  error.value = ''
  try {
    const res = await api.getSharedResourcePool()
    const appId = Number(res.data?.application?.id || 0)
    const envId = Number(res.data?.environment?.id || 0)
    if (!appId || !envId) throw new Error('共享资源池尚未初始化')
    await router.replace(`/apps/${appId}/environments/${envId}`)
  } catch (e: any) {
    error.value = '打开共享资源池失败：' + (e?.message || '未知错误')
  }
}

onMounted(() => {
  void openSharedResourcePool()
})
</script>

<style scoped>
.shared-resource-page {
  display: flex;
  align-items: center;
  min-height: 100vh;
}

.shared-resource-loading {
  display: flex;
  align-items: center;
  gap: var(--paap-space-4);
  width: min(560px, 100%);
  margin: 0 auto;
}

.shared-resource-loading h1,
.shared-resource-loading p {
  margin: 0;
}
</style>
