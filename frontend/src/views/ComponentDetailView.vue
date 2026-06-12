<template>
  <div class="rail-page">
    <div class="page-header">
      <div class="header-left">
        <button class="rail-btn rail-btn--ghost back-btn" @click="goBack">
          <svg width="16" height="16" viewBox="0 0 32 32" fill="currentColor"><path d="M14 26l1.4-1.4L7.8 17H28v-2H7.8l7.6-7.6L14 6 4 16z"/></svg>
          返回环境
        </button>
        <div class="title-block">
          <div class="title-row">
            <h1 class="page-title">{{ component?.name || '组件详情' }}</h1>
            <span class="status-badge" :class="component?.status || 'unknown'">
              <span class="rail-status-dot" :class="dotClass" />
              {{ componentStatusText(component?.status) }}
            </span>
          </div>
          <span class="title-meta">{{ componentTypeText(component?.type) }} · {{ component?.image || '-' }}</span>
        </div>
      </div>
      <div class="header-actions">
        <button v-if="component" type="button" class="rail-btn rail-btn--primary" :disabled="actionLoading" @click="deployCurrentComponent">
          部署
        </button>
        <button v-if="component?.argocdApp && deployServiceId" type="button" class="rail-btn rail-btn--primary" @click="goToArgoTopology">
          打开 ArgoCD 拓扑
        </button>
        <a v-if="componentRepoUrl" class="rail-btn rail-btn--ghost link-btn" :href="componentRepoUrl" target="_blank" rel="noreferrer">
          查看代码仓
        </a>
      </div>
    </div>

    <div v-if="loading" class="loading-mask">
      <div class="loading-spinner" />
    </div>

    <div v-else-if="!component" class="rail-empty">
      <h3 class="rail-empty-title">未找到组件</h3>
      <p class="rail-empty-desc">该组件可能已被删除或不属于当前环境。</p>
    </div>

    <template v-else>
      <div class="info-grid">
        <div v-for="item in summaryItems" :key="item.label" class="info-card">
          <div class="info-label">{{ item.label }}</div>
          <div class="info-value" :class="{ mono: item.mono }">{{ item.value }}</div>
        </div>
      </div>

      <section class="section-card">
        <div class="section-header">
          <div>
            <h2 class="rail-section-title">交付链路</h2>
            <p class="rail-section-desc">从源码、代码仓、构建、镜像、GitOps 到集群运行态追踪当前组件。</p>
          </div>
        </div>
        <div class="delivery-mode-tag" :class="deliveryModeClass">
          {{ deliveryModeLabel }}
        </div>
        <div class="delivery-graph">
          <template v-for="(step, index) in deliveryChain" :key="step.key">
            <div class="delivery-node" :class="step.status">
              <div class="node-index">{{ index + 1 }}</div>
              <div class="node-body">
                <div class="node-top">
                  <span class="node-label">{{ step.label }}</span>
                  <span class="node-status">{{ step.statusText }}</span>
                </div>
                <div class="node-value" :class="{ mono: step.mono }">{{ step.value }}</div>
                <p>{{ step.detail }}</p>
              </div>
            </div>
            <div v-if="index < deliveryChain.length - 1" class="delivery-edge" aria-hidden="true">
              <span />
            </div>
          </template>
        </div>
      </section>

      <section class="section-card">
        <div class="section-header">
          <div>
            <h2 class="rail-section-title">资源拓扑</h2>
            <p class="rail-section-desc">按当前组件配置和 GitOps 元数据展示资源关系；不会根据状态推断未返回的 Pod 或 ReplicaSet。</p>
          </div>
        </div>
        <div class="component-argocd-topology">
          <div class="component-resource-map">
            <div class="component-resource-links" aria-hidden="true">
              <span v-for="edge in resourceTopologyEdges" :key="edge.key">{{ edge.label }}</span>
            </div>
            <button
              v-for="node in resourceTopology"
              :key="node.key"
              type="button"
              class="resource-tree-node"
              :class="[
                `resource-tree-node--${node.status}`,
                { child: node.parentKey, selected: selectedResourceNode?.key === node.key },
              ]"
              @click="selectResourceNode(node)"
            >
              <span class="resource-node-status" />
              <div class="resource-node-content">
                <div class="resource-node-kind">{{ node.kind }}</div>
                <div class="resource-node-name" :class="{ mono: node.kind !== 'Component' && node.kind !== 'Cluster' }">{{ node.name }}</div>
                <p>{{ node.detail }}</p>
              </div>
            </button>
          </div>

          <aside class="component-resource-detail">
            <div class="resource-detail-head">
              <span class="resource-detail-label">资源详情</span>
              <span v-if="selectedResourceNode" class="resource-detail-status" :class="selectedResourceNode.status">
                {{ resourceNodeStatusText(selectedResourceNode.status) }}
              </span>
            </div>
            <div v-if="selectedResourceNode" class="resource-detail-body">
              <h3>{{ selectedResourceNode.name }}</h3>
              <span class="resource-detail-kind">{{ selectedResourceNode.kind }}</span>
              <p>{{ selectedResourceNode.detail }}</p>
              <div class="resource-detail-table">
                <div v-for="row in resourceDetailRows" :key="row.label" class="resource-detail-row">
                  <span>{{ row.label }}</span>
                  <strong :class="{ mono: row.mono }">{{ row.value }}</strong>
                </div>
              </div>
            </div>
            <div v-else class="empty-inline">暂无资源节点。</div>
          </aside>
        </div>
      </section>

      <section class="section-card">
        <div class="section-header">
          <div>
            <h2 class="rail-section-title">运行状态</h2>
            <p class="rail-section-desc">从业务可用性、发布版本和接入配置查看组件状态。</p>
          </div>
        </div>

        <div class="health-panel" :class="component.status || 'unknown'">
          <div>
            <div class="health-label">当前状态</div>
            <div class="health-title">{{ componentStatusText(component.status) }}</div>
            <p class="health-desc">{{ healthDescription }}</p>
          </div>
          <div class="health-scale">
            <span class="scale-number">{{ component.replicas || 0 }}</span>
            <span class="scale-label">实例</span>
          </div>
        </div>

        <div class="management-grid">
          <div class="management-card">
            <div class="management-label">发布配置</div>
            <div class="management-value mono">{{ component.image || '-' }}</div>
            <p>当前版本 {{ component.version || '-' }}，实例规模 {{ component.replicas || 0 }}。</p>
          </div>
          <div class="management-card">
            <div class="management-label">代码来源</div>
            <div class="management-value mono">{{ component.gitPath || component.gitRepoUrl || '-' }}</div>
            <p>用于生成组件源码和部署清单。</p>
          </div>
          <div class="management-card">
            <div class="management-label">持续交付</div>
            <div class="management-value mono">{{ component.argocdApp || '-' }}</div>
            <p>对应的发布应用和同步入口。</p>
          </div>
          <div class="management-card">
            <div class="management-label">资源规格</div>
            <div class="management-value">{{ component.cpu || '-' }} / {{ component.memory || '-' }}</div>
            <p>用于容量评估和运行成本判断。</p>
          </div>
        </div>
      </section>

      <section class="section-card">
        <div class="section-header config-header">
          <div>
            <h2 class="rail-section-title">运行配置</h2>
            <p class="rail-section-desc">保存环境变量、Secret 引用、ConfigMap 引用和组件依赖。保存配置不会部署，点击部署后才写入 GitOps 和运行态。</p>
          </div>
          <button type="button" class="rail-btn rail-btn--primary" :disabled="actionLoading" @click="() => saveComponentConfig()">
            保存配置
          </button>
        </div>
        <div v-if="actionMessage" class="config-message" :class="{ error: actionError }">{{ actionMessage }}</div>
        <div class="runtime-grid">
          <label class="config-field">
            <span>实例数</span>
            <input id="component-runtime-replicas" name="component-runtime-replicas" v-model.number="runtimeForm.replicas" class="rail-input" type="number" min="1" />
          </label>
          <label class="config-field">
            <span>CPU</span>
            <input id="component-runtime-cpu" name="component-runtime-cpu" v-model.trim="runtimeForm.cpu" class="rail-input mono" placeholder="250m" />
          </label>
          <label class="config-field">
            <span>内存</span>
            <input id="component-runtime-memory" name="component-runtime-memory" v-model.trim="runtimeForm.memory" class="rail-input mono" placeholder="256Mi" />
          </label>
        </div>

        <div class="env-editor">
          <div class="env-editor-head">
            <div>
              <h3>环境变量</h3>
              <p>普通值、SecretKeyRef 和 ConfigMapKeyRef 会在部署时进入 Deployment。</p>
            </div>
            <button type="button" class="rail-btn rail-btn--ghost" @click="addEnvRow">添加变量</button>
          </div>
          <div class="env-table">
            <div class="env-row head">
              <span>名称</span>
              <span>来源</span>
              <span>值 / 名称</span>
              <span>Key</span>
              <span></span>
            </div>
            <div v-for="(item, index) in envRows" :key="index" class="env-row">
              <input v-model.trim="item.name" class="rail-input mono" placeholder="DATABASE_URL" />
              <select v-model="item.source" class="rail-select">
                <option value="value">值</option>
                <option value="secret">Secret</option>
                <option value="configmap">ConfigMap</option>
              </select>
              <input v-if="item.source === 'value'" v-model.trim="item.value" class="rail-input mono" placeholder="postgres://..." />
              <input v-else-if="item.source === 'secret'" v-model.trim="item.secretName" class="rail-input mono" placeholder="secret-name" />
              <input v-else v-model.trim="item.configMapName" class="rail-input mono" placeholder="configmap-name" />
              <input v-if="item.source === 'value'" class="rail-input mono" disabled value="-" />
              <input v-else-if="item.source === 'secret'" v-model.trim="item.secretKey" class="rail-input mono" placeholder="password" />
              <input v-else v-model.trim="item.configMapKey" class="rail-input mono" placeholder="host" />
              <button type="button" class="icon-action danger" title="删除变量" @click="removeEnvRow(index)">×</button>
            </div>
            <div v-if="envRows.length === 0" class="empty-inline">暂无环境变量。</div>
          </div>
        </div>

        <label class="config-field dependency-field">
          <span>依赖组件 / 中间件</span>
          <input id="component-runtime-dependencies" name="component-runtime-dependencies" v-model.trim="dependencyText" class="rail-input mono" placeholder="redis, mysql" />
        </label>
      </section>

      <section class="section-card" style="margin-bottom:0">
        <div class="section-header">
          <div>
            <h2 class="rail-section-title">生命周期事件</h2>
            <p class="rail-section-desc">组件发布、运行和异常状态的业务事件。</p>
          </div>
        </div>
        <div class="event-table">
          <div class="event-row head">
            <span>时间</span>
            <span>类型</span>
            <span>原因</span>
            <span>消息</span>
          </div>
          <div v-for="(evt, i) in events" :key="i" class="event-row">
            <span class="mono">{{ evt.time }}</span>
            <span class="rail-tag" :class="evt.type === 'Warning' ? 'rail-tag--red' : 'rail-tag--blue'">{{ evt.type }}</span>
            <span>{{ evt.reason }}</span>
            <span class="evt-msg">{{ evt.message }}</span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import {
  buildComponentDeliveryChain,
  buildComponentEvents,
  buildComponentResourceTopology,
  buildComponentSummary,
  componentHealthDescription,
  componentStatusText,
  componentTypeText,
} from './componentWorkspace'
import type { ComponentResourceTopologyNode } from './componentWorkspace'
import { serviceProxyUrl } from './serviceWorkspace'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)
const envId = Number(route.params.envId)
const compId = Number(route.params.compId)

const component = ref<any>(null)
const deployServiceId = ref<number | null>(null)
const gitServiceId = ref<number | null>(null)
const loading = ref(true)
const actionLoading = ref(false)
const actionMessage = ref('')
const actionError = ref(false)
const runtimeForm = ref({ replicas: 1, cpu: '', memory: '' })
const dependencyText = ref('')
const selectedResourceKey = ref('')

type EnvSource = 'value' | 'secret' | 'configmap'
type EnvRow = {
  name: string
  source: EnvSource
  value: string
  secretName: string
  secretKey: string
  configMapName: string
  configMapKey: string
}
const envRows = ref<EnvRow[]>([])

const summaryItems = computed(() => buildComponentSummary(component.value))
const deliveryChain = computed(() => buildComponentDeliveryChain(component.value))
const resourceTopology = computed(() => buildComponentResourceTopology(component.value))
const resourceTopologyEdges = computed(() => resourceTopology.value
  .filter(node => node.parentKey)
  .map(node => ({
    key: `${node.parentKey}->${node.key}`,
    label: `${resourceTopology.value.find(parent => parent.key === node.parentKey)?.kind || 'Parent'} -> ${node.kind}`,
  })))
const selectedResourceNode = computed(() =>
  resourceTopology.value.find(node => node.key === selectedResourceKey.value) || resourceTopology.value[0] || null
)
const resourceDetailRows = computed(() => {
  const node = selectedResourceNode.value
  if (!node) return []
  const parent = node.parentKey ? resourceTopology.value.find(item => item.key === node.parentKey) : null
  return [
    { label: '类型', value: node.kind },
    { label: '名称', value: node.name, mono: node.kind !== 'Component' && node.kind !== 'Cluster' },
    { label: '状态', value: resourceNodeStatusText(node.status) },
    { label: '上级资源', value: parent ? `${parent.kind} / ${parent.name}` : '-', mono: !!parent },
    { label: '说明', value: node.detail },
  ]
})
const events = computed(() => buildComponentEvents(component.value))
const healthDescription = computed(() => componentHealthDescription(component.value))
const isSourceDelivery = computed(() => component.value?.deliveryMode === 'source' || !!component.value?.sourceRepoUrl || !!component.value?.sourceMirrorRepoUrl || !!component.value?.jenkinsJob)
const deliveryModeLabel = computed(() => isSourceDelivery.value ? '源码交付链路' : '镜像交付链路')
const deliveryModeClass = computed(() => isSourceDelivery.value ? 'source' : 'image')
const repoProxyPath = computed(() => {
  const repoURL = String(component.value?.gitRepoUrl || '').trim().replace(/\.git$/, '')
  if (!repoURL) return ''
  const parts = repoURL.split('/').filter(Boolean)
  if (parts.length < 2) return ''
  const repo = parts[parts.length - 1]
  const owner = parts[parts.length - 2]
  return `${owner}/${repo}`
})
const componentRepoUrl = computed(() =>
  gitServiceId.value && repoProxyPath.value
    ? serviceProxyUrl(envId, gitServiceId.value, repoProxyPath.value)
    : ''
)

const dotClass = computed(() => {
  const s = component.value?.status
  if (s === 'running') return 'rail-status-dot--running'
  if (s === 'error') return 'rail-status-dot--error'
  if (s === 'pending') return 'rail-status-dot--creating'
  return 'rail-status-dot--empty'
})

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push(`/apps/${appId}/environments/${envId}?tab=components`)
}
const goToArgoTopology = () => {
  if (!deployServiceId.value || !component.value?.argocdApp) return
  router.push(`/apps/${appId}/environments/${envId}?tab=continuous-deployment&application=${encodeURIComponent(component.value.argocdApp)}`)
}
const resourceNodeStatusText = (status?: string) => status === 'ready' ? '已配置' : '待配置'
const selectResourceNode = (node: ComponentResourceTopologyNode) => {
  selectedResourceKey.value = node.key
}

const emptyEnvRow = (): EnvRow => ({
  name: '',
  source: 'value',
  value: '',
  secretName: '',
  secretKey: '',
  configMapName: '',
  configMapKey: '',
})

const parseComponentConfig = (raw: any) => {
  if (!raw) return { env: [], dependencies: [] }
  if (typeof raw === 'object') return raw
  try {
    return JSON.parse(String(raw))
  } catch {
    return { env: [], dependencies: [] }
  }
}

const hydrateConfigForm = () => {
  runtimeForm.value = {
    replicas: Number(component.value?.replicas || 1),
    cpu: component.value?.cpu || '',
    memory: component.value?.memory || '',
  }
  const cfg = parseComponentConfig(component.value?.config)
  envRows.value = (cfg.env || []).map((item: any) => {
    const row = emptyEnvRow()
    row.name = item.name || ''
    row.value = item.value || ''
    row.secretName = item.secretName || ''
    row.secretKey = item.secretKey || ''
    row.configMapName = item.configMapName || ''
    row.configMapKey = item.configMapKey || ''
    row.source = row.secretName || row.secretKey ? 'secret' : row.configMapName || row.configMapKey ? 'configmap' : 'value'
    return row
  })
  dependencyText.value = (cfg.dependencies || []).join(', ')
}

const addEnvRow = () => envRows.value.push(emptyEnvRow())
const removeEnvRow = (index: number) => envRows.value.splice(index, 1)

const buildRuntimeConfig = () => ({
  env: envRows.value
    .filter(item => item.name.trim())
    .map(item => ({
      name: item.name.trim(),
      value: item.source === 'value' ? item.value.trim() : '',
      secretName: item.source === 'secret' ? item.secretName.trim() : '',
      secretKey: item.source === 'secret' ? item.secretKey.trim() : '',
      configMapName: item.source === 'configmap' ? item.configMapName.trim() : '',
      configMapKey: item.source === 'configmap' ? item.configMapKey.trim() : '',
    })),
  dependencies: dependencyText.value
    .split(',')
    .map(item => item.trim())
    .filter(Boolean),
})

const imageTag = (image?: string) => {
  const last = String(image || '').split('/').pop() || ''
  const colon = last.lastIndexOf(':')
  return colon >= 0 ? last.slice(colon + 1) : ''
}

const saveComponentConfig = async (options: { quiet?: boolean } = {}) => {
  if (!component.value?.id || actionLoading.value) return false
  actionLoading.value = true
  actionError.value = false
  if (!options.quiet) actionMessage.value = ''
  try {
    const res = await api.updateComponent(Number(component.value.id), {
      replicas: Number(runtimeForm.value.replicas || 1),
      cpu: runtimeForm.value.cpu,
      memory: runtimeForm.value.memory,
      config: buildRuntimeConfig(),
    })
    component.value = res.data
    hydrateConfigForm()
    if (!options.quiet) actionMessage.value = '配置已保存'
    return true
  } catch (e: any) {
    actionError.value = true
    actionMessage.value = '保存失败：' + (e?.message || '未知错误')
    return false
  } finally {
    actionLoading.value = false
  }
}

const deployCurrentComponent = async () => {
  if (!component.value?.id || actionLoading.value) return
  const saved = await saveComponentConfig({ quiet: true })
  if (!saved) return
  const version = String(component.value?.version || imageTag(component.value?.image) || '').trim()
  if (!version || version.toLowerCase() === 'latest') {
    actionError.value = true
    actionMessage.value = '部署前需要填写明确版本，不能使用 latest。'
    return
  }
  actionLoading.value = true
  actionError.value = false
  actionMessage.value = ''
  try {
    const res = await api.deployComponent(Number(component.value.id), { version })
    component.value = res.data
    hydrateConfigForm()
    actionMessage.value = '部署已触发'
  } catch (e: any) {
    actionError.value = true
    actionMessage.value = '部署失败：' + (e?.message || '未知错误')
  } finally {
    actionLoading.value = false
  }
}

const loadComponent = async () => {
  try {
    const res = await api.getEnv(envId)
    const comps = res.data.components || []
    component.value = comps.find((c: any) => c.id === compId) || null
    const deploy = (res.data.services || []).find((svc: any) => svc.serviceType === 'deploy')
    const git = (res.data.services || []).find((svc: any) => svc.serviceType === 'git')
    deployServiceId.value = deploy?.id || null
    gitServiceId.value = git?.id || null
    hydrateConfigForm()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

onMounted(loadComponent)
</script>

<style scoped>
.rail-page { padding: 20px 20px 36px; max-width: none; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; gap: 16px; }
.header-left { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.back-btn { align-self: flex-start; }
.title-block { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.title-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.page-title { font-size: 24px; font-weight: 600; color: #11181c; line-height: 1.2; letter-spacing: 0; margin: 0; overflow-wrap: anywhere; }
.title-meta { font-family: 'IBM Plex Mono', monospace; font-size: 12px; color: #9ba1a6; overflow-wrap: anywhere; }
.header-actions { flex-shrink: 0; }
.link-btn { text-decoration: none; }

.status-badge {
  display: inline-flex; align-items: center; gap: 5px;
  font-size: 11px; font-weight: 500; padding: 3px 8px; border-radius: 4px;
  background: #f1f3f5; color: #687076;
}
.status-badge.running { background: #f0fdf4; color: #16a34a; }
.status-badge.stopped { background: #fef2f2; color: #dc2626; }
.status-badge.pending { background: #eff6ff; color: #2563eb; }
.status-badge.error { background: #fef2f2; color: #dc2626; }

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.info-card {
  background: #ffffff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.info-label { font-size: 12px; color: #687076; font-weight: 500; }
.info-value { font-size: 14px; color: #11181c; font-weight: 500; line-height: 1.4; overflow-wrap: anywhere; }
.mono { font-family: 'IBM Plex Mono', monospace; font-size: 12px; }

.section-card {
  background: #ffffff;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 16px;
}
.section-header { margin-bottom: 20px; }
.config-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 14px; }
.runtime-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 18px; }
.config-field { display: flex; flex-direction: column; gap: 7px; min-width: 0; }
.config-field > span { font-size: 12px; color: #687076; font-weight: 600; }
.rail-input, .rail-select {
  width: 100%;
  min-height: 36px;
  border: 1px solid #d7dbdf;
  border-radius: 6px;
  background: #ffffff;
  color: #11181c;
  padding: 8px 10px;
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}
.rail-input:focus, .rail-select:focus { border-color: #11181c; box-shadow: 0 0 0 2px rgba(17,24,28,0.08); }
.rail-input:disabled { background: #f7f8fa; color: #9ba1a6; }
.config-message { border: 1px solid #bbf7d0; background: #f0fdf4; color: #166534; border-radius: 6px; padding: 9px 11px; font-size: 13px; margin-bottom: 14px; }
.config-message.error { border-color: #fecaca; background: #fef2f2; color: #b91c1c; }
.env-editor { border: 1px solid #e6e8eb; border-radius: 8px; overflow: hidden; margin-bottom: 16px; }
.env-editor-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; border-bottom: 1px solid #e6e8eb; background: #f9fafb; }
.env-editor-head h3 { margin: 0; font-size: 14px; color: #11181c; }
.env-editor-head p { margin: 4px 0 0; font-size: 12px; color: #687076; line-height: 1.4; }
.env-table { display: flex; flex-direction: column; }
.env-row { display: grid; grid-template-columns: minmax(150px, 1fr) 118px minmax(180px, 1.2fr) minmax(150px, 1fr) 36px; gap: 8px; padding: 10px 12px; align-items: center; border-bottom: 1px solid #f1f3f5; }
.env-row:last-child { border-bottom: none; }
.env-row.head { color: #687076; font-size: 11px; font-weight: 700; text-transform: uppercase; background: #ffffff; }
.icon-action {
  width: 32px;
  height: 32px;
  border: 1px solid #e6e8eb;
  border-radius: 6px;
  background: #ffffff;
  color: #687076;
  cursor: pointer;
}
.icon-action.danger:hover { color: #dc2626; border-color: #fecaca; background: #fef2f2; }
.empty-inline { padding: 16px; font-size: 13px; color: #687076; }
.dependency-field { margin-top: 6px; }

.delivery-mode-tag {
  display: inline-flex;
  align-items: center;
  height: 26px;
  border-radius: 999px;
  padding: 0 10px;
  margin-bottom: 14px;
  font-size: 12px;
  font-weight: 700;
}
.delivery-mode-tag.source { background: #eff6ff; color: #1d4ed8; }
.delivery-mode-tag.image { background: #f0fdf4; color: #166534; }

.delivery-graph {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr) 34px);
  gap: 0;
  align-items: center;
}
.delivery-graph > :last-child { display: none; }
.delivery-node {
  position: relative;
  display: flex;
  gap: 10px;
  min-width: 0;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 12px;
  background: #f9fafb;
}
.delivery-edge {
  height: 100%;
  min-height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.delivery-edge span {
  width: 100%;
  height: 2px;
  background: #cbd5e1;
  position: relative;
}
.delivery-edge span::after {
  content: '';
  position: absolute;
  right: -1px;
  top: 50%;
  width: 8px;
  height: 8px;
  border-right: 2px solid #cbd5e1;
  border-top: 2px solid #cbd5e1;
  transform: translateY(-50%) rotate(45deg);
}
.delivery-node.ready { border-color: #bbf7d0; background: #f0fdf4; }
.delivery-node.running { border-color: #bfdbfe; background: #eff6ff; }
.delivery-node.pending { border-color: #e6e8eb; background: #ffffff; }
.delivery-node.error { border-color: #fecaca; background: #fef2f2; }
.component-argocd-topology {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(280px, 360px);
  gap: 16px;
  align-items: stretch;
}
.component-resource-map {
  position: relative;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 12px;
  min-width: 0;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 16px;
  background: #f9fafb;
}
.component-resource-links {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}
.component-resource-links span {
  border: 1px solid #e6e8eb;
  border-radius: 999px;
  background: #ffffff;
  color: #687076;
  padding: 3px 8px;
  font-size: 11px;
  line-height: 1.35;
}
.resource-tree-node {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 10px;
  text-align: left;
  font-family: inherit;
  cursor: pointer;
  min-width: 0;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 14px;
  background: #ffffff;
  color: inherit;
}
.resource-tree-node:hover,
.resource-tree-node.selected { border-color: #93c5fd; background: #eff6ff; }
.resource-tree-node.child { border-left: 3px solid #cbd5e1; }
.resource-tree-node--ready { background: #ffffff; }
.resource-tree-node--pending {
  background: #ffffff;
  color: #687076;
}
.resource-node-status {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 4px;
  background: #cbd5e1;
}
.resource-tree-node--ready .resource-node-status { background: #16a34a; }
.resource-tree-node.selected .resource-node-status { background: #2563eb; }
.resource-node-content { min-width: 0; }
.resource-node-kind {
  margin-bottom: 8px;
  color: #687076;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}
.resource-node-name {
  min-width: 0;
  color: #11181c;
  font-size: 14px;
  font-weight: 650;
  overflow-wrap: anywhere;
}
.resource-node-body p {
  margin: 7px 0 0;
  color: #687076;
  font-size: 12px;
  line-height: 1.45;
}
.component-resource-detail {
  min-width: 0;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #ffffff;
  overflow: hidden;
}
.resource-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border-bottom: 1px solid #e6e8eb;
  background: #ffffff;
}
.resource-detail-label {
  color: #11181c;
  font-size: 13px;
  font-weight: 700;
}
.resource-detail-status {
  border-radius: 999px;
  padding: 3px 8px;
  background: #f1f3f5;
  color: #687076;
  font-size: 11px;
  font-weight: 700;
}
.resource-detail-status.ready { background: #f0fdf4; color: #166534; }
.resource-detail-status.pending { background: #eff6ff; color: #1d4ed8; }
.resource-detail-body { padding: 16px; }
.resource-detail-body h3 {
  margin: 0 0 8px;
  color: #11181c;
  font-size: 16px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}
.resource-detail-kind {
  display: inline-flex;
  border-radius: 4px;
  background: #eff6ff;
  color: #1d4ed8;
  padding: 3px 7px;
  font-size: 11px;
  font-weight: 700;
}
.resource-detail-body p {
  margin: 12px 0;
  color: #687076;
  font-size: 13px;
  line-height: 1.5;
}
.resource-detail-table {
  display: grid;
  border-top: 1px solid #f1f3f5;
}
.resource-detail-row {
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid #f1f3f5;
  font-size: 12px;
}
.resource-detail-row span { color: #687076; }
.resource-detail-row strong {
  min-width: 0;
  color: #11181c;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.node-index {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #ffffff;
  border: 1px solid rgba(0,0,0,0.08);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 700;
  color: #687076;
}
.node-body { min-width: 0; flex: 1; }
.node-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 6px; }
.node-label { font-size: 12px; font-weight: 700; color: #11181c; }
.node-status { font-size: 11px; color: #687076; white-space: nowrap; }
.delivery-node.ready .node-status { color: #16a34a; }
.delivery-node.running .node-status { color: #2563eb; }
.delivery-node.error .node-status { color: #dc2626; }
.node-value { font-size: 12px; font-weight: 600; color: #11181c; line-height: 1.35; overflow-wrap: anywhere; }
.node-body p { margin: 7px 0 0; color: #687076; font-size: 12px; line-height: 1.45; }

.health-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  padding: 18px;
  margin-bottom: 16px;
  background: #f9fafb;
}
.health-panel.running { border-color: #bbf7d0; background: #f0fdf4; }
.health-panel.pending { border-color: #bfdbfe; background: #eff6ff; }
.health-panel.error { border-color: #fecaca; background: #fef2f2; }
.health-label { font-size: 12px; font-weight: 600; color: #687076; margin-bottom: 4px; }
.health-title { font-size: 20px; font-weight: 600; color: #11181c; }
.health-desc { margin: 6px 0 0; color: #687076; font-size: 14px; line-height: 1.5; }
.health-scale {
  width: 92px;
  height: 72px;
  border-radius: 8px;
  background: #ffffff;
  border: 1px solid rgba(0,0,0,0.06);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.scale-number { font-size: 24px; font-weight: 700; color: #11181c; line-height: 1; }
.scale-label { font-size: 12px; color: #687076; margin-top: 4px; }

.management-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}
.management-card {
  border: 1px solid #f1f3f5;
  border-radius: 8px;
  padding: 14px;
  min-width: 0;
}
.management-label { font-size: 12px; font-weight: 600; color: #687076; margin-bottom: 6px; }
.management-value { font-size: 14px; font-weight: 600; color: #11181c; overflow-wrap: anywhere; }
.management-card p { margin: 8px 0 0; color: #687076; font-size: 13px; line-height: 1.45; }

.event-table { display: flex; flex-direction: column; }
.event-row {
  display: grid;
  grid-template-columns: 120px 90px 140px 1fr;
  gap: 16px;
  padding: 12px 16px;
  border-bottom: 1px solid #f1f3f5;
  align-items: center;
  font-size: 13px;
  color: #11181c;
}
.event-row:last-child { border-bottom: none; }
.event-row.head {
  font-size: 11px;
  font-weight: 600;
  color: #687076;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid #e6e8eb;
  padding-bottom: 10px;
}
.evt-msg { color: #687076; overflow-wrap: anywhere; }

.loading-mask { display: flex; align-items: center; justify-content: center; padding: 64px; }
.loading-spinner { width: 24px; height: 24px; border: 2px solid #e6e8eb; border-top-color: #11181c; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 900px) {
  .rail-page { padding: 20px 20px 32px; }
  .delivery-graph { grid-template-columns: 1fr; gap: 8px; }
  .delivery-edge { min-height: 26px; }
  .delivery-edge span { width: 2px; height: 24px; }
  .delivery-edge span::after { right: 50%; top: auto; bottom: -1px; transform: translateX(50%) rotate(135deg); }
  .event-row { grid-template-columns: 100px 80px 100px 1fr; }
  .env-row { grid-template-columns: 1fr 120px; }
  .env-row.head { display: none; }
  .env-row .icon-action { justify-self: end; }
}
@media (max-width: 672px) {
  .page-header { flex-direction: column; }
  .header-actions, .config-header, .env-editor-head { flex-direction: column; align-items: stretch; width: 100%; }
  .info-grid { grid-template-columns: 1fr; }
  .delivery-graph { grid-template-columns: 1fr; }
  .health-panel { align-items: flex-start; flex-direction: column; }
  .event-row { grid-template-columns: 1fr; gap: 4px; padding: 14px 0; }
  .event-row.head { display: none; }
  .env-row { grid-template-columns: 1fr; }
}
</style>
