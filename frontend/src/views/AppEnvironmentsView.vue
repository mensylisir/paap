<template>
  <div class="page">
    <div class="page-header">
      <h2 class="bx--type-productive-heading-04">环境管理</h2>
      <button class="bx--btn bx--btn--primary" @click="showModal=true">+ 创建环境</button>
    </div>
    <div v-if="environments.length===0" class="empty-state">
      <p class="bx--type-body-long-01">暂无环境，创建第一个环境来开始部署</p>
      <button class="bx--btn bx--btn--primary" @click="showModal=true">创建环境</button>
    </div>
    <div v-else class="env-list">
      <div class="env-row" v-for="env in environments" :key="env.id" @click="goToEnv(env.id)">
        <div>
          <div class="bx--type-productive-heading-02">{{ env.name }}</div>
          <div class="bx--type-caption-01" style="color:#8d8d8d">{{ env.identifier }}</div>
        </div>
        <div>
          <span :class="['bx--tag', env.status==='running'?'bx--tag--green':env.status==='empty'?'':'bx--tag--red']">{{ statusText(env.status) }}</span>
        </div>
        <div class="bx--type-body-short-01" style="color:#525252">
          <span v-if="components[env.id] && components[env.id].length">{{ components[env.id].length }} 个组件</span>
          <span v-else-if="services[env.id] && services[env.id].length">{{ services[env.id].length }} 个工具</span>
          <span v-else>空环境</span>
        </div>
        <div><button class="bx--btn bx--btn--ghost bx--btn--sm">进入环境</button></div>
      </div>
    </div>

    <!-- Modal -->
    <Teleport to="body">
      <div v-if="showModal" class="modal-overlay" @click.self="showModal=false">
        <div class="modal-container">
          <div class="modal-header">
            <div>
              <p class="modal-label">创建环境</p>
              <p class="modal-heading">新建环境</p>
            </div>
            <button class="modal-close" @click="showModal=false">✕</button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">环境名称 <span class="required">*</span></label>
              <input v-model="envForm.name" class="form-input" placeholder="例如：测试环境" />
            </div>
            <div class="form-group">
              <label class="form-label">环境标识 <span class="required">*</span></label>
              <input v-model="envForm.identifier" class="form-input" placeholder="例如：staging" />
              <p class="form-hint">纯英文，用于 K8s 命名空间。小写字母、数字、短横线。</p>
            </div>
            <div class="form-group">
              <label class="form-label">创建方式</label>
              <div class="radio-group">
                <label class="radio-item" :class="{active: envForm.mode==='template'}">
                  <input type="radio" value="template" v-model="envForm.mode" />
                  <span>从模板创建</span>
                </label>
                <label class="radio-item" :class="{active: envForm.mode==='empty'}">
                  <input type="radio" value="empty" v-model="envForm.mode" />
                  <span>创建空环境</span>
                </label>
              </div>
            </div>
            <div v-if="envForm.mode==='template'" class="form-group">
              <label class="form-label">选择模板</label>
              <select v-model="envForm.templateId" class="form-select">
                <option v-for="t in templates" :key="t.id" :value="String(t.id)">{{ t.name }}</option>
              </select>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn-secondary" @click="showModal=false">取消</button>
            <button class="btn-primary" @click="submitEnv" :disabled="!envForm.name || !envForm.identifier">创建</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)
const environments = ref<any[]>([])
const components = ref<Record<number,any[]>>({})
const services = ref<Record<number,any[]>>({})
const templates = ref<any[]>([])
const showModal = ref(false)
const envForm = ref({ name:'', identifier:'', mode:'template', templateId:'1' })

onMounted(async () => {
  await loadEnvs()
  try { templates.value = (await api.templates()).data || [] } catch(e){}
  // 从概览页"创建环境"跳转过来时自动打开弹窗
  if (route.query.create === 'true') {
    showModal.value = true
  }
})

async function loadEnvs() {
  try {
    const res = await api.listEnvs(appId)
    environments.value = res.data || []
    for(const env of environments.value) {
      try { components.value[env.id] = (await api.listComponents(env.id)).data || [] } catch(e){}
    }
  } catch(e){ console.error(e) }
}

const statusText = (s:string) => {
  const m: Record<string,string> = { running:'运行中', empty:'空环境', stopped:'已停止', creating:'创建中' }
  return m[s] || s
}

const submitEnv = async () => {
  try {
    const res = await api.createEnv(appId, { name:envForm.value.name, identifier:envForm.value.identifier, templateId:envForm.value.mode==='template'?Number(envForm.value.templateId):0, fromEmpty:envForm.value.mode==='empty' })
    showModal.value = false
    // 创建成功后直接进入环境详情
    const envId = res.data?.id
    if (envId) {
      router.push(`/apps/${appId}/environments/${envId}`)
    } else {
      await loadEnvs()
    }
  } catch(e:any) { alert('创建失败：'+e.message) }
}

const goToEnv = (envId:number) => router.push(`/apps/${appId}/environments/${envId}`)
</script>

<style scoped>
.page { padding:24px }
.page-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:24px }
.empty-state { padding:64px; text-align:center; background:#fff; border:1px solid #e0e0e0 }
.env-list { display:flex; flex-direction:column; gap:8px }
.env-row { display:grid; grid-template-columns:1fr auto auto auto; gap:16px; align-items:center; padding:16px; background:#fff; border:1px solid #e0e0e0; cursor:pointer }
.env-row:hover { box-shadow:0 2px 6px rgba(0,0,0,.06) }
</style>

<style>
/* Modal styles (unscoped for Teleport) */
.modal-overlay {
  position: fixed; inset: 0; z-index: 9000;
  background: rgba(0,0,0,.5);
  display: flex; align-items: center; justify-content: center;
}
.modal-container {
  background: #fff; width: 520px; max-height: 90vh; overflow-y: auto;
  box-shadow: 0 8px 32px rgba(0,0,0,.18); border-radius: 2px;
}
.modal-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding: 20px 24px 0; border-bottom: 1px solid #e0e0e0; padding-bottom: 16px;
}
.modal-label { font-size: 12px; color: #6f6f6f; margin-bottom: 4px; text-transform: uppercase; letter-spacing: .5px }
.modal-heading { font-size: 20px; font-weight: 600; color: #161616; margin: 0 }
.modal-close {
  background: none; border: none; font-size: 18px; color: #6f6f6f;
  cursor: pointer; padding: 4px 8px; line-height: 1;
}
.modal-close:hover { color: #161616; background: #e0e0e0 }
.modal-body { padding: 24px }
.form-group { margin-bottom: 20px }
.form-label { display: block; font-size: 14px; font-weight: 600; color: #161616; margin-bottom: 6px }
.required { color: #da1e28 }
.form-input {
  width: 100%; padding: 10px 12px; font-size: 14px; border: 1px solid #8d8d8d;
  background: #f4f4f4; color: #161616; outline: none; box-sizing: border-box;
}
.form-input:focus { border-color: #0f62fe; box-shadow: 0 0 0 1px #0f62fe }
.form-input::placeholder { color: #a8a8a8 }
.form-hint { font-size: 12px; color: #6f6f6f; margin-top: 4px }
.form-select {
  width: 100%; padding: 10px 12px; font-size: 14px; border: 1px solid #8d8d8d;
  background: #f4f4f4; color: #161616; outline: none; box-sizing: border-box;
}
.radio-group { display: flex; gap: 12px; margin-top: 4px }
.radio-item {
  display: flex; align-items: center; gap: 8px; padding: 10px 16px;
  border: 1px solid #c6c6c6; cursor: pointer; font-size: 14px; color: #161616;
}
.radio-item.active { border-color: #0f62fe; background: #edf5ff }
.radio-item input[type="radio"] { margin: 0 }
.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 16px 24px; border-top: 1px solid #e0e0e0;
}
.btn-primary {
  padding: 10px 24px; font-size: 14px; font-weight: 600;
  background: #0f62fe; color: #fff; border: none; cursor: pointer;
}
.btn-primary:hover { background: #0043ce }
.btn-primary:disabled { background: #c6c6c6; cursor: not-allowed }
.btn-secondary {
  padding: 10px 24px; font-size: 14px; font-weight: 600;
  background: #fff; color: #0f62fe; border: 1px solid #0f62fe; cursor: pointer;
}
.btn-secondary:hover { background: #edf5ff }
</style>
