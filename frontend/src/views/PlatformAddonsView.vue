<template>
  <div class="rail-page platform-addons-page">
    <header class="page-header">
      <div>
        <h1 class="page-title">平台插件</h1>
        <p class="page-subtitle">集群级能力由管理员启用或停用，不跟随应用环境生命周期删除</p>
      </div>
      <div class="page-actions">
        <input ref="uploadInput" class="addon-upload-input" type="file" accept=".tar.gz,.tgz" @change="uploadAddon" />
        <button class="rail-btn rail-btn--secondary" type="button" :disabled="loading || actionLoading" @click="triggerUpload">
          上传
        </button>
        <button class="rail-btn rail-btn--secondary" type="button" :disabled="loading || actionLoading" @click="syncAddons">
          同步
        </button>
        <button class="rail-btn rail-btn--secondary" type="button" :disabled="loading" @click="loadAddons">
          {{ loading ? '刷新中...' : '刷新' }}
        </button>
      </div>
    </header>

    <div v-if="error" class="form-error" role="alert">{{ error }}</div>

    <section v-if="loading" class="section-card platform-addons-loading">
      <div class="loading-spinner" aria-hidden="true" />
      <span>正在加载平台插件...</span>
    </section>

    <div v-else-if="addons.length" class="platform-addons-layout">
      <section class="platform-addons-list slide-up" aria-label="平台插件列表">
        <button
          v-for="addon in addons"
          :key="addon.name"
          class="addon-card"
          :class="{ selected: selectedName === addon.name }"
          type="button"
          @click="selectAddon(addon.name)"
        >
          <span class="addon-card-topline">
            <span class="addon-name">{{ addon.displayName || addon.name }}</span>
            <span class="addon-status" :class="statusClass(addon.status)">
              {{ statusLabel(addon.status) }}
            </span>
          </span>
          <span class="addon-description">{{ addon.description || fallbackDescription(addon.name) }}</span>
          <span class="addon-meta-row">
            <code>{{ addon.name }}</code>
            <span>{{ addon.source === 'custom' ? '自定义' : '内置' }}</span>
            <span>{{ addon.version || '-' }}</span>
          </span>
          <span class="addon-deps">
            依赖：{{ dependencyLabel(addon) }}
          </span>
        </button>
      </section>

      <aside v-if="selectedAddon" class="addon-detail slide-up" aria-label="插件详情">
        <header class="addon-detail-header">
          <div>
            <p class="detail-kicker">插件详情</p>
            <h2>{{ selectedAddon.displayName || selectedAddon.name }}</h2>
            <p>{{ selectedAddon.description || fallbackDescription(selectedAddon.name) }}</p>
          </div>
          <span class="addon-status" :class="statusClass(selectedAddon.status)">
            {{ statusLabel(selectedAddon.status) }}
          </span>
        </header>

        <div class="addon-actions">
          <button
            class="rail-btn rail-btn--primary"
            type="button"
            :disabled="actionLoading || selectedAddon.desiredState === 'enabled'"
            @click="enableAddon(selectedAddon.name)"
          >
            启用
          </button>
          <button
            class="rail-btn rail-btn--secondary"
            type="button"
            :disabled="actionLoading || selectedAddon.desiredState === 'disabled'"
            @click="disableAddon(selectedAddon.name)"
          >
            停用
          </button>
          <button class="rail-btn rail-btn--ghost" type="button" :disabled="actionLoading" @click="checkAddon(selectedAddon.name)">
            检查
          </button>
        </div>

        <dl class="addon-summary-grid">
          <div>
            <dt>标识</dt>
            <dd><code>{{ selectedAddon.name }}</code></dd>
          </div>
          <div>
            <dt>来源</dt>
            <dd>{{ selectedAddon.source === 'custom' ? '自定义上传' : '内置包' }}</dd>
          </div>
          <div>
            <dt>命名空间</dt>
            <dd>{{ selectedAddon.namespace || '-' }}</dd>
          </div>
          <div>
            <dt>期望状态</dt>
            <dd>{{ desiredStateLabel(selectedAddon.desiredState) }}</dd>
          </div>
          <div>
            <dt>MinIO 对象</dt>
            <dd><code>{{ selectedAddon.s3Key || '-' }}</code></dd>
          </div>
          <div>
            <dt>最近检查</dt>
            <dd>{{ formatDate(selectedAddon.lastCheckedAt) }}</dd>
          </div>
        </dl>

        <section class="addon-detail-section">
          <h3>依赖</h3>
          <div class="addon-chip-row">
            <span v-for="item in parseList(selectedAddon.dependsOn)" :key="item" class="addon-chip">{{ item }}</span>
            <span v-if="!parseList(selectedAddon.dependsOn).length" class="addon-muted">无</span>
          </div>
        </section>

        <section class="addon-detail-section">
          <h3>能力</h3>
          <div class="addon-chip-row">
            <span v-for="item in parseList(selectedAddon.capabilities)" :key="item" class="addon-chip">{{ item }}</span>
            <span v-if="!parseList(selectedAddon.capabilities).length" class="addon-muted">未声明</span>
          </div>
        </section>

        <section class="addon-detail-section">
          <h3>检查项</h3>
          <div class="addon-checks">
            <div v-for="item in checkItems(selectedAddon)" :key="item.key" class="addon-check-item">
              <span>{{ item.label }}</span>
              <code>{{ item.value }}</code>
            </div>
            <p v-if="!checkItems(selectedAddon).length" class="addon-muted">未声明检查项</p>
          </div>
        </section>

        <section class="addon-detail-section">
          <h3>状态条件</h3>
          <div class="addon-conditions">
            <div v-for="condition in parseConditions(selectedAddon.conditions)" :key="condition.type + condition.message" class="addon-condition">
              <span class="condition-dot" :class="{ ready: condition.status === 'True' }" />
              <div>
                <strong>{{ condition.type }}</strong>
                <p>{{ condition.message }}</p>
              </div>
            </div>
            <p v-if="!parseConditions(selectedAddon.conditions).length" class="addon-muted">暂无检查结果</p>
          </div>
        </section>

        <section v-if="selectedAddon.errorMessage" class="addon-detail-section">
          <h3>错误</h3>
          <p class="addon-error">{{ selectedAddon.errorMessage }}</p>
        </section>

        <section class="addon-detail-section addon-readme">
          <h3>介绍</h3>
          <pre>{{ selectedAddon.readme || selectedAddon.description || fallbackDescription(selectedAddon.name) }}</pre>
        </section>
      </aside>
    </div>

    <section v-if="!loading && !addons.length" class="empty-state slide-up">
      <div class="empty-state-icon">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2L2 7l10 5 10-5-10-5z"/>
          <path d="M2 17l10 5 10-5"/>
          <path d="M2 12l10 5 10-5"/>
        </svg>
      </div>
      <div class="empty-state-text">暂无平台插件</div>
      <p class="empty-state-desc">metrics-server、kpack、KEDA、KubeVirt、CDI 插件包同步后会出现在这里。</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'

type ClusterAddon = {
  id: number
  name: string
  displayName: string
  category: string
  source: string
  namespace?: string
  version?: string
  installMode?: string
  s3Bucket?: string
  s3Key?: string
  dependsOn?: string
  capabilities?: string
  desiredState: string
  status: string
  config?: string
  conditions?: string
  errorMessage?: string
  description?: string
  readme?: string
  installedAt?: string
  lastCheckedAt?: string
}

type AddonCondition = {
  type: string
  status: string
  message: string
}

const addons = ref<ClusterAddon[]>([])
const selectedName = ref('')
const loading = ref(false)
const actionLoading = ref(false)
const error = ref('')
const uploadInput = ref<HTMLInputElement | null>(null)

const selectedAddon = computed(() => addons.value.find((addon) => addon.name === selectedName.value) || addons.value[0])

const loadAddons = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.platformAddons()
    addons.value = Array.isArray(response?.data) ? response.data : []
    if (!addons.value.some((addon) => addon.name === selectedName.value)) {
      selectedName.value = addons.value[0]?.name || ''
    }
  } catch (err: any) {
    error.value = err?.message || '平台插件加载失败'
  } finally {
    loading.value = false
  }
}

const selectAddon = (name: string) => {
  selectedName.value = name
}

const runAddonAction = async (action: () => Promise<any>) => {
  actionLoading.value = true
  error.value = ''
  try {
    const response = await action()
    const next = response?.data
    if (next?.name) {
      const index = addons.value.findIndex((addon) => addon.name === next.name)
      if (index >= 0) addons.value.splice(index, 1, next)
      else addons.value.push(next)
      selectedName.value = next.name
    } else {
      await loadAddons()
    }
  } catch (err: any) {
    error.value = err?.message || '平台插件操作失败'
  } finally {
    actionLoading.value = false
  }
}

const enableAddon = (name: string) => runAddonAction(() => api.enablePlatformAddon(name))
const disableAddon = (name: string) => runAddonAction(() => api.disablePlatformAddon(name))
const checkAddon = (name: string) => runAddonAction(() => api.checkPlatformAddon(name))

const syncAddons = async () => {
  await runAddonAction(() => api.syncPlatformAddons())
  await loadAddons()
}

const triggerUpload = () => {
  uploadInput.value?.click()
}

const uploadAddon = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const data = new FormData()
  data.append('file', file)
  await runAddonAction(() => api.uploadPlatformAddon(data))
  input.value = ''
}

const safeParse = (raw?: string) => {
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

const parseList = (raw?: string) => {
  const value = safeParse(raw)
  return Array.isArray(value) ? value.filter((item) => typeof item === 'string' && item.trim()) : []
}

const parseConditions = (raw?: string): AddonCondition[] => {
  const value = safeParse(raw)
  return Array.isArray(value) ? value : []
}

const checkItems = (addon: ClusterAddon) => {
  const config = safeParse(addon.config) || {}
  const rows: { key: string; label: string; value: string }[] = []
  const crds = Array.isArray(config.crds) ? config.crds : []
  const deployments = Array.isArray(config.deployments) ? config.deployments : []
  const daemonSets = Array.isArray(config.daemonSets) ? config.daemonSets : []
  crds.forEach((item: string) => rows.push({ key: `crd-${item}`, label: 'CRD', value: item }))
  deployments.forEach((item: any) => rows.push({ key: `deploy-${item.namespace}-${item.name}`, label: 'Deployment', value: `${item.namespace}/${item.name}` }))
  daemonSets.forEach((item: any) => rows.push({ key: `ds-${item.namespace}-${item.name}`, label: 'DaemonSet', value: `${item.namespace}/${item.name}` }))
  return rows
}

const dependencyLabel = (addon: ClusterAddon) => {
  const deps = parseList(addon.dependsOn)
  return deps.length ? deps.join(', ') : '无'
}

const statusLabel = (status?: string) => {
  const labels: Record<string, string> = {
    available: '可用',
    unavailable: '不可用',
    degraded: '异常',
    disabled: '已停用',
    installing: '启用中',
    failed: '失败',
    unknown: '未知',
  }
  return labels[status || 'unknown'] || status || '未知'
}

const statusClass = (status?: string) => ({
  available: status === 'available',
  warning: status === 'degraded' || status === 'installing',
  disabled: status === 'disabled' || status === 'unknown',
  failed: status === 'failed' || status === 'unavailable',
})

const desiredStateLabel = (state?: string) => state === 'enabled' ? '启用' : '停用'

const formatDate = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

const fallbackDescription = (name: string) => {
  const descriptions: Record<string, string> = {
    'metrics-server': 'Kubernetes metrics API，支撑 HPA 和资源用量读取。',
    kpack: 'Cloud Native Buildpacks 构建能力，用于源码到镜像交付。',
    keda: '事件驱动伸缩能力，用队列、Kafka、Prometheus 等外部指标触发扩缩容。',
    KubeVirt: 'KubeVirt 虚拟机运行时能力。',
    kubevirt: 'KubeVirt 虚拟机运行时能力。',
    cdi: 'KubeVirt 磁盘镜像导入和 DataVolume 管理能力。',
  }
  return descriptions[name] || '集群级平台插件。'
}

onMounted(loadAddons)
</script>

<style scoped>
.platform-addons-page {
  /* Block layout (not grid) — consistent spacing uses .page-header margin-bottom */
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--paap-space-4);
  /* margin-bottom comes from global style.scss (var(--paap-space-6) = 24px) */
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--paap-text);
  line-height: 1.2;
}

.page-subtitle {
  margin: var(--paap-space-1) 0 0;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
}

.page-actions {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}

.addon-upload-input {
  display: none;
}

.platform-addons-loading {
  display: flex;
  align-items: center;
  gap: var(--paap-space-3);
  min-height: 120px;
}

.section-card {
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  padding: var(--paap-space-5);
  animation: fade-slide-up 0.35s ease-out both;
}

.loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--paap-border);
  border-top-color: var(--paap-accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.platform-addons-layout {
  display: grid;
  grid-template-columns: minmax(280px, 420px) minmax(0, 1fr);
  gap: var(--paap-space-5);
  align-items: start;
  animation: fade-slide-up 0.3s ease-out both;
}

.platform-addons-list {
  display: grid;
  gap: var(--paap-space-3);
}

.addon-card {
  display: grid;
  gap: var(--paap-space-3);
  width: 100%;
  padding: var(--paap-space-4);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  color: var(--paap-text);
  text-align: left;
  cursor: pointer;
  transition: border-color var(--paap-transition-fast), background var(--paap-transition-fast), box-shadow var(--paap-transition-fast);
  animation: fade-slide-up 0.3s ease-out both;
}
.addon-card:nth-child(1)  { animation-delay: 0.00s; }
.addon-card:nth-child(2)  { animation-delay: 0.04s; }
.addon-card:nth-child(3)  { animation-delay: 0.08s; }
.addon-card:nth-child(4)  { animation-delay: 0.12s; }
.addon-card:nth-child(5)  { animation-delay: 0.16s; }
.addon-card:nth-child(6)  { animation-delay: 0.20s; }
.addon-card:nth-child(7)  { animation-delay: 0.24s; }
.addon-card:nth-child(8)  { animation-delay: 0.28s; }
.addon-card:nth-child(9)  { animation-delay: 0.32s; }
.addon-card:nth-child(10) { animation-delay: 0.36s; }

.addon-card:hover {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-shadow-sm), 0 2px 8px rgba(37, 99, 235, 0.06);
  background: var(--paap-accent-fill);
}

.addon-card.selected {
  border-color: var(--paap-accent);
  box-shadow: var(--paap-shadow-sm), inset 0 0 0 1px var(--paap-accent);
  background: var(--paap-accent-fill);
}

.addon-card-topline,
.addon-meta-row,
.addon-actions,
.addon-chip-row {
  display: flex;
  align-items: center;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}

.addon-name {
  font-weight: 700;
  font-size: var(--paap-fs-heading-lg);
}

.addon-description,
.addon-deps {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
  line-height: 1.45;
}

.addon-meta-row {
  color: var(--paap-text-soft);
  font-size: var(--paap-fs-label);
}

.addon-meta-row code,
.addon-summary-grid code,
.addon-check-item code {
  font-family: var(--paap-mono);
  font-size: var(--paap-fs-code);
  overflow-wrap: anywhere;
}

.addon-status {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: var(--paap-radius-full);
  background: var(--paap-border);
  color: var(--paap-text-soft);
  font-size: var(--paap-fs-label);
  font-weight: 600;
}

.addon-status.available {
  background: var(--paap-success-soft);
  color: var(--paap-success);
}

.addon-status.warning {
  background: var(--paap-warning-soft);
  color: #7a5b00;
}

.addon-status.failed {
  background: var(--paap-danger-soft);
  color: var(--paap-danger);
}

.addon-status.disabled {
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}

.addon-detail {
  display: grid;
  gap: var(--paap-space-5);
  padding: var(--paap-space-5);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: var(--paap-panel);
  animation: fade-slide-up 0.3s ease-out;
}

.addon-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  padding-bottom: var(--paap-space-4);
}

.addon-detail-header h2 {
  margin: 0;
  font-size: var(--paap-fs-heading-xl);
}

.addon-detail-header p {
  margin: 0;
  color: var(--paap-muted);
}

.detail-kicker {
  margin-bottom: var(--paap-space-1);
  color: var(--paap-text-soft);
  font-size: var(--paap-fs-label);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.addon-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--paap-border);
}

.addon-summary-grid div {
  display: grid;
  gap: 4px;
  min-height: 64px;
  padding: var(--paap-space-3);
  border-right: 1px solid var(--paap-border);
  border-bottom: 1px solid var(--paap-border);
  align-content: center;
  min-width: 0;
}

.addon-summary-grid div:nth-child(3n) {
  border-right: 0;
}

.addon-summary-grid div:nth-last-child(-n + 3) {
  border-bottom: 0;
}

.addon-summary-grid dt {
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
}

.addon-summary-grid dd {
  margin: 0;
  color: var(--paap-text);
  font-weight: 600;
  overflow-wrap: anywhere;
}

.addon-detail-section {
  display: grid;
  gap: var(--paap-space-3);
}

.addon-detail-section h3 {
  margin: 0;
  font-size: var(--paap-fs-heading-lg);
}

.addon-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-full);
  background: var(--paap-panel-subtle);
  color: var(--paap-text-soft);
  font-size: var(--paap-fs-label);
}

.addon-muted {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

.addon-checks,
.addon-conditions {
  display: grid;
  gap: var(--paap-space-2);
}

.addon-check-item,
.addon-condition {
  display: flex;
  align-items: flex-start;
  gap: var(--paap-space-3);
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-bg);
}

.addon-check-item {
  justify-content: space-between;
}

.condition-dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  background: var(--paap-danger);
  flex: 0 0 auto;
}

.condition-dot.ready {
  background: var(--paap-success);
}

.addon-condition strong,
.addon-condition p {
  margin: 0;
}

.addon-condition p {
  color: var(--paap-muted);
  font-size: var(--paap-fs-compact);
}

.addon-error {
  margin: 0;
  padding: var(--paap-space-3);
  border: 1px solid var(--paap-danger-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-danger-bg);
  color: var(--paap-danger-text);
  overflow-wrap: anywhere;
}

.addon-readme pre {
  max-height: 360px;
  overflow: auto;
  margin: 0;
  padding: var(--paap-space-4);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-bg);
  color: var(--paap-text);
  font-family: var(--paap-font);
  font-size: var(--paap-fs-compact);
  line-height: 1.55;
  white-space: pre-wrap;
}

@media (max-width: 1100px) {
  .platform-addons-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .rail-page { padding: var(--paap-space-6) var(--paap-space-4) var(--paap-space-10); }

  .page-header,
  .addon-detail-header {
    align-items: stretch;
    flex-direction: column;
  }

  .addon-summary-grid {
    grid-template-columns: 1fr;
  }

  .addon-summary-grid div,
  .addon-summary-grid div:nth-child(3n),
  .addon-summary-grid div:nth-last-child(-n + 3) {
    border-right: 0;
    border-bottom: 1px solid var(--paap-border);
  }
}
</style>
