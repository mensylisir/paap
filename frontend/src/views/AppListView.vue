<template>
  <div class="page">
    <div v-if="loading" style="padding:80px;text-align:center">
      <div class="bx--loading bx--loading--small"><svg class="bx--loading__svg" viewBox="-75 -75 150 150"><circle class="bx--loading__background" cx="0" cy="0" r="37.5" /><circle class="bx--loading__stroke" cx="0" cy="0" r="37.5" /></svg></div>
    </div>

    <div v-else-if="apps.length === 0" class="empty-state">
      <h1 class="bx--type-productive-heading-05">欢迎使用 PAAP</h1>
      <p class="bx--type-body-long-01">以应用为中心的自助式云原生平台</p>
      <button class="bx--btn bx--btn--primary" @click="handleCreate">创建第一个应用</button>
    </div>

    <div v-else>
      <div class="page-header">
        <h1 class="bx--type-productive-heading-04">我的应用</h1>
        <button class="bx--btn bx--btn--primary" @click="handleCreate">+ 创建应用</button>
      </div>
      <div class="app-grid">
        <div class="bx--tile app-card" v-for="app in apps" :key="app.id" @click="goToApp(app.id)">
          <div class="app-card-title">
            <h3>{{ app.name }}</h3>
            <!-- 空标签占位，保持布局稳定 -->
            <span></span>
          </div>
          <p class="bx--type-body-short-01" style="color:#525252">{{ app.description || '暂无描述' }}</p>
          <span class="bx--type-caption-01" style="color:#8d8d8d">{{ app.identifier }}</span>
        </div>
        <div class="bx--tile add-card" @click="handleCreate">
          <span style="font-size:32px">+</span>
          <p>创建新应用</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'

const router = useRouter()
const apps = ref<any[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await api.listApps()
    apps.value = res.data || []
  } catch (e) { console.error(e) }
  finally { loading.value = false }
})

const handleCreate = () => router.push('/apps/create')
const goToApp = (id: number) => router.push(`/apps/${id}/overview`)
</script>

<style scoped>
.page {
  padding: 32px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 48px);
  text-align: center;
  background: #f4f4f4;
}

.empty-state h1 {
  margin-bottom: 16px;
}

.empty-state p {
  color: #525252;
  margin-bottom: 32px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.app-card {
  padding: 24px;
  cursor: pointer;
  border: 1px solid #e0e0e0;
  transition: box-shadow 0.2s;
}

.app-card:hover {
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.app-card-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.app-card-title h3 {
  font-size: 18px;
  font-weight: 600;
}

.add-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  border: 1px dashed #c6c6c6;
  color: #525252;
}

.add-card:hover {
  border-color: #0f62fe;
  color: #0f62fe;
}
</style>
