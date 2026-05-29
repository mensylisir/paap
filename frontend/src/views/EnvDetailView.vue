<template>
  <div class="page">
    <nav class="bx--breadcrumb bx--breadcrumb--no-trailing-slash">
      <div class="bx--breadcrumb-item"><router-link to="/apps">我的应用</router-link></div>
      <div class="bx--breadcrumb-item"><router-link :to="`/apps/${appId}/environments`">环境管理</router-link></div>
      <div class="bx--breadcrumb-item bx--breadcrumb-item--current">{{ env?.name || '环境详情' }}</div>
    </nav>
    <div class="env-header">
      <h2 class="bx--type-productive-heading-04">{{ env?.name }}</h2>
      <button class="bx--btn bx--btn--primary" @click="showComponentModal=true">+ 创建组件</button>
    </div>

    <div class="section">
      <div class="section-header"><h3 class="bx--type-productive-heading-02">工具</h3><button class="bx--btn bx--btn--ghost bx--btn--sm" @click="showServiceModal=true">+ 添加工具</button></div>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        <span v-for="svc in services" :key="svc.id" class="bx--tag bx--tag--blue">{{ svcLabel(svc.serviceType) }} <span style="cursor:pointer;margin-left:4px" @click="uninstallService(svc.id)">×</span></span>
        <span v-if="services.length===0" class="bx--type-body-short-01" style="color:#525252">当前未安装任何工具</span>
      </div>
    </div>

    <div class="section">
      <div class="section-header"><h3 class="bx--type-productive-heading-02">组件</h3></div>
      <div class="component-grid">
        <div class="bx--tile component-card" v-for="comp in components" :key="comp.id">
          <div class="bx--type-productive-heading-02">{{ comp.name }}</div>
          <div class="bx--type-caption-01" style="color:#8d8d8d">{{ compTypeText(comp.type) }}</div>
          <div style="display:flex;align-items:center;gap:6px;margin-top:8px">
            <span :class="['dot', comp.status]" style="width:8px;height:8px;display:inline-block"></span>
            <span class="bx--type-body-short-01">{{ comp.status==='running'?'运行中':'已停止' }}</span>
          </div>
          <div class="bx--type-caption-01" style="color:#8d8d8d;margin-top:4px">副本: {{ comp.replicas }}</div>
        </div>
        <div class="bx--tile add-card" @click="showComponentModal=true"><span>+ 创建组件</span></div>
      </div>
      <p v-if="components.length===0" class="bx--type-body-short-01" style="color:#525252;margin-top:8px">当前没有组件</p>
    </div>

    <div class="section">
      <div class="section-header"><h3 class="bx--type-productive-heading-02">基础设施</h3><button class="bx--btn bx--btn--ghost bx--btn--sm">+ 添加基础设施</button></div>
      <p class="bx--type-body-short-01" style="color:#525252">数据库、缓存、消息队列等基础设施将在此展示</p>
    </div>

    <!-- Create Component Modal -->
    <div v-if="showComponentModal" class="bx--modal" role="dialog" aria-modal="true" style="opacity:1;visibility:visible;z-index:9000">
      <div class="bx--modal-container" style="max-width:600px">
        <div class="bx--modal-header"><p class="bx--modal-header__label">创建组件</p><p class="bx--modal-header__heading">新建组件</p><button class="bx--modal-close" @click="showComponentModal=false"></button></div>
        <div class="bx--modal-content">
          <div class="bx--form-item"><label class="bx--label">组件名称</label><input v-model="compForm.name" class="bx--text-input" placeholder="例如：前端服务" /></div>
          <div class="bx--form-item"><label class="bx--label">组件类型</label>
            <div class="bx--select"><select v-model="compForm.type" class="bx--select-input"><option value="frontend">前端服务</option><option value="backend">后端服务</option><option value="database">数据库</option><option value="middleware">中间件</option><option value="custom">自定义</option></select></div>
          </div>
          <div class="bx--form-item"><label class="bx--label">镜像地址</label><input v-model="compForm.image" class="bx--text-input" placeholder="registry.example.com/app:latest" /></div>
          <div class="bx--form-item"><label class="bx--label">版本标签</label><input v-model="compForm.version" class="bx--text-input" placeholder="latest" /></div>
          <div class="bx--form-item"><label class="bx--label">副本数量</label><input type="number" v-model="compForm.replicas" class="bx--text-input" min="1" /></div>
        </div>
        <div class="bx--modal-footer">
          <button class="bx--btn bx--btn--secondary" @click="showComponentModal=false">取消</button>
          <button class="bx--btn bx--btn--primary" @click="submitComponent">创建</button>
        </div>
      </div>
    </div>

    <!-- Install Service Modal -->
    <div v-if="showServiceModal" class="bx--modal" role="dialog" aria-modal="true" style="opacity:1;visibility:visible;z-index:9000">
      <div class="bx--modal-container" style="max-width:600px">
        <div class="bx--modal-header"><p class="bx--modal-header__label">安装工具</p><p class="bx--modal-header__heading">选择要安装的工具</p><button class="bx--modal-close" @click="showServiceModal=false"></button></div>
        <div class="bx--modal-content">
          <div class="service-grid">
            <div v-for="svc in availableServices" :key="svc.type" :class="['bx--tile service-card', {selected: serviceForm.serviceType===svc.type}]" @click="serviceForm.serviceType=svc.type">
              <h4 class="bx--type-productive-heading-02">{{ svc.name }}</h4>
              <p class="bx--type-body-short-01" style="color:#525252">{{ svc.desc }}</p>
            </div>
          </div>
        </div>
        <div class="bx--modal-footer">
          <button class="bx--btn bx--btn--secondary" @click="showServiceModal=false">取消</button>
          <button class="bx--btn bx--btn--primary" @click="submitService">安装</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api/client'

const route = useRoute()
const appId = Number(route.params.id)
const envId = Number(route.params.envId)

const env = ref<any>(null)
const components = ref<any[]>([])
const services = ref<any[]>([])

const showComponentModal = ref(false)
const showServiceModal = ref(false)
const compForm = ref({ name:'', type:'backend', image:'', version:'latest', replicas:1 })
const serviceForm = ref({ serviceType:'deploy' })

const availableServices = [
  {type:'deploy',name:'部署服务',desc:'管理应用的部署、版本、回滚'},
  {type:'ci',name:'CI 服务',desc:'自动构建和测试代码'},
  {type:'monitor',name:'监控服务',desc:'监控资源使用和应用健康'},
  {type:'log',name:'日志服务',desc:'收集和查询应用日志'},
]

onMounted(async () => {
  try {
    const res = await api.getEnv(envId)
    env.value = res.data.environment
    components.value = res.data.components || []
    services.value = res.data.services || []
  } catch(e) { console.error(e) }
})

const svcLabel = (type:string) => {
  const m: Record<string,string> = { deploy:'部署服务', ci:'CI 服务', monitor:'监控服务', log:'日志服务' }
  return m[type] || type
}

const compTypeText = (type:string) => {
  const m: Record<string,string> = { frontend:'前端服务', backend:'后端服务', database:'数据库', middleware:'中间件', custom:'自定义' }
  return m[type] || type
}

const submitComponent = async () => {
  try {
    await api.createComponent(envId, compForm.value)
    const res = await api.getEnv(envId)
    components.value = res.data.components || []
    showComponentModal.value = false
    compForm.value = { name:'', type:'backend', image:'', version:'latest', replicas:1 }
  } catch(e:any) { alert('创建失败：'+e.message) }
}

const submitService = async () => {
  try {
    await api.installService(appId, { serviceType: serviceForm.value.serviceType })
    const res = await api.getEnv(envId)
    services.value = res.data.services || []
    showServiceModal.value = false
  } catch(e:any) { alert('安装失败：'+e.message) }
}

const uninstallService = async (id:number) => {
  services.value = services.value.filter(s=>s.id!==id)
}
</script>

<style scoped>
.page { padding:24px }
.env-header { display:flex; justify-content:space-between; align-items:center; margin:16px 0 24px }
.section { background:#fff; border:1px solid #e0e0e0; padding:20px; margin-bottom:16px }
.section-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:12px }
.component-grid { display:grid; grid-template-columns: repeat(auto-fill, minmax(180px,1fr)); gap:12px }
.component-card { padding:16px }
.dot.running { background:#24a148 }
.dot.stopped { background:#da1e28 }
.add-card { display:flex; align-items:center; justify-content:center; padding:16px; border:1px dashed #c6c6c6; color:#525252; min-height:100px }
.add-card:hover { border-color:#0f62fe; color:#0f62fe }
.service-grid { display:grid; grid-template-columns:1fr 1fr; gap:12px }
.service-card { padding:16px; cursor:pointer }
.service-card:hover { border-color:#0f62fe }
.service-card.selected { border-color:#0f62fe; background:#edf5ff }
.bx--modal { opacity:1; visibility:visible }
</style>
