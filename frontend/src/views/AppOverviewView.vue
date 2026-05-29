<template>
  <div class="page">
    <h2 class="bx--type-productive-heading-04">应用概览</h2>

    <!-- 环境状态面板 -->
    <div class="panel">
      <div class="panel-header">
        <h3 class="bx--type-productive-heading-02">环境状态</h3>
        <button class="bx--btn bx--btn--primary bx--btn--sm" @click="router.push(`/apps/${appId}/environments?create=true`)">+ 创建环境</button>
      </div>

      <div v-if="environments.length === 0" class="empty-panel">
        <div class="empty-content">
          <h4 class="bx--type-productive-heading-02">暂无环境</h4>
          <p class="bx--type-body-long-01">应用创建成功！接下来，创建一个环境来开始部署你的服务。</p>
          <p class="bx--type-body-short-01" style="color:#525252;margin-top:8px">你可以从模板快速创建（预置 ArgoCD、CI、监控等），或者创建空环境按需安装。</p>
          <button class="bx--btn bx--btn--primary" style="margin-top:16px" @click="router.push(`/apps/${appId}/environments?create=true`)">创建第一个环境</button>
        </div>
      </div>

      <div v-else class="env-grid">
        <div v-for="env in environments" :key="env.id" class="bx--tile env-card" @click="goToEnv(env.id)">
          <div class="env-header-row">
            <span class="bx--type-productive-heading-01">{{ env.name }}</span>
            <span :class="['status-badge', env.status]">{{ statusText(env.status) }}</span>
          </div>
          <div class="bx--type-caption-01" style="color:#8d8d8d;margin-top:4px">{{ env.identifier }}</div>
          <div class="env-tools" style="margin-top:12px">
            <span v-if="env.toolCount" class="bx--tag bx--tag--blue">{{ env.toolCount }} 个工具</span>
            <span v-if="env.componentCount" class="bx--tag bx--tag--green">{{ env.componentCount }} 个组件</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 资源概览 -->
    <div class="panel">
      <h3 class="bx--type-productive-heading-02">资源概览</h3>
      <div v-if="hasRunningEnv" class="resource-grid">
        <div class="bx--tile" style="padding:16px">
          <div class="bx--type-body-short-01" style="color:#525252">CPU 使用</div>
          <div class="bx--type-productive-heading-02">{{ resourceUsage.cpu }}%</div>
          <div style="height:6px;background:#e0e0e0;margin-top:8px"><div :style="{height:'100%',width:resourceUsage.cpu+'%',background:'#0f62fe'}"></div></div>
        </div>
        <div class="bx--tile" style="padding:16px">
          <div class="bx--type-body-short-01" style="color:#525252">内存使用</div>
          <div class="bx--type-productive-heading-02">{{ resourceUsage.memory }}%</div>
          <div style="height:6px;background:#e0e0e0;margin-top:8px"><div :style="{height:'100%',width:resourceUsage.memory+'%',background:'#0f62fe'}"></div></div>
        </div>
        <div class="bx--tile" style="padding:16px">
          <div class="bx--type-body-short-01" style="color:#525252">存储使用</div>
          <div class="bx--type-productive-heading-02">{{ resourceUsage.storage }}%</div>
          <div style="height:6px;background:#e0e0e0;margin-top:8px"><div :style="{height:'100%',width:resourceUsage.storage+'%',background:'#0f62fe'}"></div></div>
        </div>
      </div>
      <div v-else class="empty-panel">
        <p class="bx--type-body-long-01">暂无运行中的环境，资源数据不可用。</p>
      </div>
    </div>

    <!-- 最近事件 -->
    <div class="panel">
      <h3 class="bx--type-productive-heading-02">最近事件</h3>
      <ul v-if="recentEvents.length > 0" class="bx--list--unordered">
        <li v-for="(evt, i) in recentEvents" :key="i" class="bx--list__item">{{ evt }}</li>
      </ul>
      <div v-else class="empty-panel">
        <p class="bx--type-body-long-01">暂无事件记录。</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)
const environments = ref<any[]>([])
const resourceUsage = ref({ cpu: 0, memory: 0, storage: 0 })
const recentEvents = ref<string[]>([])

const hasRunningEnv = computed(() => environments.value.some(e => e.status === 'running'))

onMounted(async () => {
  await loadData()
})

async function loadData() {
  try {
    const res = await api.getApp(appId)
    environments.value = res.data.environments || []
  } catch (e) {
    console.error(e)
  }
}

const statusText = (s: string) => {
  const m: Record<string, string> = { running: '运行中', empty: '空环境', stopped: '已停止', creating: '创建中' }
  return m[s] || s
}

const goToEnv = (envId: number) => router.push(`/apps/${appId}/environments/${envId}`)
</script>

<style scoped>
.page { padding: 24px }
.page > h2 { margin-bottom: 24px }
.panel { background: #fff; border: 1px solid #e0e0e0; padding: 20px; margin-bottom: 16px }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px }
.panel-header h3 { margin: 0 }
.empty-panel { padding: 32px; background: #f4f4f4; border: 1px dashed #c6c6c6 }
.empty-content { max-width: 480px }
.empty-content h4 { margin-bottom: 8px }
.empty-content p { color: #525252 }
.env-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px }
.env-card { padding: 16px; cursor: pointer; transition: box-shadow .2s }
.env-card:hover { box-shadow: 0 2px 6px rgba(0,0,0,.08) }
.env-header-row { display: flex; justify-content: space-between; align-items: center }
.status-badge { font-size: 12px; padding: 2px 8px; color: #fff }
.status-badge.running { background: #24a148 }
.status-badge.empty { background: #8d8d8d }
.status-badge.stopped { background: #da1e28 }
.status-badge.creating { background: #8d8d8d }
.resource-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px }
</style>
