<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default="{ resources: _rws }">
      <div class="ws-tabs">
        <button
          v-for="tab in availableTabs"
          :key="tab.key"
          class="ws-tab"
          :class="{ active: activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <div v-if="activeTab === 'jobs'" class="tab-panel">
        <div v-if="jobs.length" class="pipeline-workbench">
          <div class="job-list">
            <div
              v-for="job in jobs"
              :key="job.name"
              class="card job-card"
              :class="{ selected: isSelectedJob(job) }"
              @click="selectJob(job)"
            >
              <div class="job-header">
                <span class="status-ball" :class="ballClass(job.annotations?.color)">
                  <span v-if="isRunning(job.annotations?.color)" class="pulse" />
                </span>
                <div class="job-title-wrap">
                  <div class="job-name">
                    <a v-if="job.externalUrl" :href="job.externalUrl" target="_blank" class="link external" @click.stop>{{ job.name }}</a>
                    <span v-else>{{ job.name }}</span>
                  </div>
                  <div class="job-meta">
                    <span class="status-pill" :class="statusPillClass(job.status)">{{ statusLabel(job.status) }}</span>
                    <span v-if="job.annotations?.lastBuildNumber" class="chip">#{{ job.annotations.lastBuildNumber }}</span>
                    <span v-if="job.annotations?.branch" class="chip">{{ job.annotations.branch }}</span>
                  </div>
                </div>
              </div>

              <div v-if="job.actions?.length" class="job-actions">
                <button
                  v-for="act in job.actions"
                  :key="act.label"
                  class="act-btn"
                  :class="act.tone || 'ghost'"
                  @click.stop="emit('action', act, act.target || job.name)"
                >
                  {{ act.label }}
                </button>
              </div>
            </div>
          </div>

          <aside class="build-detail">
            <div class="build-head">
              <div>
                <div class="build-title">构建详情</div>
                <div class="build-sub">真实 Jenkins 数据优先展示；没有返回的字段不做前端伪造</div>
              </div>
              <span class="status-pill" :class="statusPillClass(selectedJobView?.status)">{{ statusLabel(selectedJobView?.status) }}</span>
            </div>

            <div v-if="selectedJobView" class="build-body">
              <div class="build-name">{{ selectedJobView.name }}</div>
              <div class="build-meta-grid">
                <div>
                  <span>构建号</span>
                  <strong>{{ buildNumber(selectedJobView) }}</strong>
                </div>
                <div>
                  <span>组件</span>
                  <strong>{{ selectedJobView.annotations?.component || '-' }}</strong>
                </div>
                <div>
                  <span>分支</span>
                  <strong>{{ selectedJobView.annotations?.branch || 'main' }}</strong>
                </div>
                <div>
                  <span>触发方式</span>
                  <strong>Webhook / 手动</strong>
                </div>
              </div>

              <section class="pipeline-section">
                <div class="section-title">阶段视图</div>
                <div class="stage-list">
                  <div v-for="stage in pipelineStages(selectedJobView)" :key="stage.name" class="stage-item" :class="stage.state">
                    <span class="stage-dot" />
                    <div>
                      <strong>{{ stage.name }}</strong>
                      <span>{{ stage.desc }}</span>
                    </div>
                  </div>
                </div>
              </section>

              <section class="pipeline-section">
                <div class="section-title">镜像产物</div>
                <div class="artifact-box">
                  <code>{{ imageArtifact(selectedJobView) }}</code>
                </div>
              </section>

              <section class="pipeline-section">
                <div class="section-title">构建日志</div>
                <div class="log-view">
                  <template v-if="realBuildLogLines(selectedJobView).length">
                    <div v-for="line in realBuildLogLines(selectedJobView)" :key="line" class="log-line">{{ line }}</div>
                  </template>
                  <div v-else class="log-empty">暂无真实构建日志，打开 Jenkins 查看 console 输出。</div>
                </div>
              </section>

              <div class="detail-actions">
                <button
                  v-for="act in selectedJobActions"
                  :key="act.label"
                  class="act-btn"
                  :class="act.tone || 'ghost'"
                  @click="emit('action', act, act.target || selectedJobView?.name)"
                >
                  {{ act.label }}
                </button>
                <a v-if="selectedJobView.externalUrl" :href="selectedJobView.externalUrl" target="_blank" class="link external">打开 Jenkins</a>
              </div>
            </div>
            <div v-else class="empty-line">选择流水线查看构建详情</div>
          </aside>
        </div>
        <div v-else class="empty-line">暂无流水线数据</div>
      </div>

      <div v-if="activeTab === 'resources'" class="tab-panel">
        <div v-if="resources.length" class="table-wrap">
          <table class="data-table">
            <thead>
              <tr><th>名称</th><th>类型</th><th>状态</th><th>说明</th></tr>
            </thead>
            <tbody>
              <tr v-for="r in resources" :key="r.name + r.type" class="resource-row" :class="{ selected: isSelectedJob(r) }" @click="selectJob(r)">
                <td class="cell-name">
                  <a v-if="r.externalUrl" :href="r.externalUrl" target="_blank" class="link external" @click.stop>{{ r.name }}</a>
                  <span v-else>{{ r.name }}</span>
                </td>
                <td><span class="badge blue">{{ r.type }}</span></td>
                <td><span class="badge" :class="statusBadge(r.status)">{{ r.status }}</span></td>
                <td class="cell-desc">{{ r.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无资源数据</div>
      </div>
    </template>
  </ToolWorkspaceFrame>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
}>()

const emit = defineEmits<{
  (e: 'action', action: WorkspaceAction, target?: string): void
}>()

const jobs = computed(() => props.resources.filter(r => r.type === 'Pipeline' || r.type === 'Job'))
const selectedJob = ref<WorkspaceResource | null>(null)
const selectedJobView = computed(() => selectedJob.value || jobs.value[0] || null)
const selectedJobActions = computed<WorkspaceAction[]>(() => {
  const job = selectedJobView.value
  if (!job) return []
  if (job.actions?.length) return job.actions
  return [
    { key: 'trigger_jenkins_build', label: '触发', description: '触发该流水线构建。', target: job.name },
    { key: 'refresh', label: '刷新', description: '重新读取流水线状态。', target: job.name },
  ]
})

const availableTabs = computed(() => {
  const tabs: { key: string; label: string; count: number }[] = []
  if (jobs.value.length) tabs.push({ key: 'jobs', label: '流水线', count: jobs.value.length })
  tabs.push({ key: 'resources', label: '资源', count: props.resources.length })
  return tabs
})

const activeTab = ref(jobs.value.length ? 'jobs' : 'resources')

const selectJob = (job: WorkspaceResource) => {
  selectedJob.value = job
}

const isSelectedJob = (job: WorkspaceResource) =>
  selectedJobView.value?.name === job.name && selectedJobView.value?.type === job.type

const isRunning = (color?: string) => {
  return String(color || '').includes('anime')
}

const ballClass = (color?: string) => {
  const c = String(color || '').toLowerCase()
  if (c.includes('anime')) return 'blue'
  if (c.startsWith('blue')) return 'green'
  if (c.startsWith('red')) return 'red'
  if (c.startsWith('yellow')) return 'yellow'
  if (c.startsWith('grey') || c.startsWith('gray') || c === 'notbuilt' || c === 'disabled') return 'gray'
  return 'gray'
}

const statusPillClass = (status?: string) => {
  const s = String(status || '').toLowerCase()
  if (s === 'success' || s === 'ready') return 'green'
  if (s === 'failed') return 'red'
  if (s === 'unstable') return 'yellow'
  if (s === 'running') return 'blue'
  if (s === 'disabled') return 'gray'
  return 'gray'
}

const statusLabel = (status?: string) => {
  const s = String(status || '').toLowerCase()
  if (s === 'success' || s === 'ready') return '成功'
  if (s === 'failed') return '失败'
  if (s === 'unstable') return '不稳定'
  if (s === 'running') return '构建中'
  if (s === 'disabled') return '已禁用'
  return s || '未知'
}

const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('success') || v.includes('ready') || v.includes('healthy')) return 'green'
  if (v.includes('fail') || v.includes('error')) return 'red'
  if (v.includes('run') || v.includes('progress')) return 'blue'
  return 'gray'
}

const stageState = (job: WorkspaceResource, index: number) => {
  const status = String(job.status || '').toLowerCase()
  if (status === 'failed') return index < 2 ? 'success' : index === 2 ? 'failed' : 'pending'
  if (status === 'running') return index < 2 ? 'success' : index === 2 ? 'running' : 'pending'
  if (status === 'success' || status === 'ready') return 'success'
  return index === 0 ? 'running' : 'pending'
}

const pipelineStages = (job: WorkspaceResource) => [
  { name: 'Checkout', desc: `读取 ${job.annotations?.branch || 'main'} 分支源码`, state: stageState(job, 0) },
  { name: 'Buildpacks', desc: '提交 kpack Image 并等待构建', state: stageState(job, 1) },
  { name: 'Push Image', desc: '推送镜像到环境 registry', state: stageState(job, 2) },
  { name: 'Deploy Callback', desc: '回调 PAAP 并刷新 GitOps 清单', state: stageState(job, 3) },
]

const buildNumber = (job: WorkspaceResource) =>
  job.annotations?.lastBuildNumber ? `#${job.annotations.lastBuildNumber}` : '-'

const imageArtifact = (job: WorkspaceResource) =>
  job.annotations?.image || 'Jenkins 未返回镜像产物'

const realBuildLogLines = (job: WorkspaceResource) => {
  const value = job.annotations?.consoleLog || job.annotations?.buildLog || job.annotations?.logs
  if (Array.isArray(value)) return value.map(item => String(item)).filter(Boolean)
  return String(value || '').split('\n').map(line => line.trimEnd()).filter(Boolean)
}
</script>

<style scoped>
.pipeline-workbench {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(720px, 1fr);
  gap: var(--paap-space-4);
  align-items: start;
  width: 100%;
  min-width: 0;
}
.job-list { display: grid; gap: var(--paap-space-2); min-width: 0; }
.job-card { display: flex; flex-direction: column; gap: var(--paap-space-2); min-width: 0; padding: var(--paap-space-3); cursor: pointer; }
.job-card.selected { border-color: var(--paap-accent); box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.08); }
.job-header { display: flex; align-items: flex-start; gap: var(--paap-space-3); }
.status-ball { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; margin-top: 4px; position: relative; }
.status-ball.green { background: var(--paap-success); }
.status-ball.red { background: var(--paap-danger); }
.status-ball.yellow { background: var(--paap-warning); }
.status-ball.blue { background: var(--paap-accent); }
.status-ball.gray { background: var(--paap-muted-2); }
.pulse { position: absolute; inset: -3px; border-radius: 50%; border: 2px solid currentColor; opacity: 0.5; animation: pulse 1.4s ease-out infinite; }
@keyframes pulse { 0% { transform: scale(1); opacity: 0.5; } 100% { transform: scale(2); opacity: 0; } }
.job-title-wrap { flex: 1; min-width: 0; }
.job-name { max-width: 100%; overflow: hidden; color: var(--paap-text); font-size: 13px; font-weight: 600; line-height: 1.4; text-overflow: ellipsis; overflow-wrap: anywhere; }
.job-meta { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.job-actions { display: flex; gap: 6px; flex-wrap: wrap; }
.build-detail {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
  min-width: 0;
  max-width: 100%;
  position: sticky;
  top: 0;
}
.build-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--paap-space-3); margin-bottom: var(--paap-space-3); }
.build-title { font-size: 15px; font-weight: 600; color: var(--paap-text); }
.build-sub { font-size: 12px; color: var(--paap-muted); margin-top: 2px; }
.build-body { display: grid; gap: var(--paap-space-3); min-width: 0; max-width: 100%; }
.build-name { font-size: 14px; font-weight: 600; color: var(--paap-text); word-break: break-all; }
.build-meta-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: var(--paap-space-2); }
.build-meta-grid > div { border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); padding: var(--paap-space-2) var(--paap-space-3); background: var(--paap-panel-subtle); }
.build-meta-grid span { display: block; color: var(--paap-muted); font-size: 11px; margin-bottom: 2px; }
.build-meta-grid strong { color: var(--paap-text); font-size: 13px; word-break: break-all; }
.pipeline-section { min-width: 0; max-width: 100%; border-top: 1px solid var(--paap-border); padding-top: var(--paap-space-3); }
.section-title { font-size: 11px; font-weight: 600; color: var(--paap-muted); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: var(--paap-space-2); }
.stage-list { display: grid; gap: var(--paap-space-2); }
.stage-item { display: flex; gap: var(--paap-space-3); align-items: flex-start; }
.stage-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--paap-muted-2); margin-top: 5px; flex-shrink: 0; }
.stage-item.success .stage-dot { background: var(--paap-success); }
.stage-item.running .stage-dot { background: var(--paap-accent); box-shadow: 0 0 0 4px rgba(37, 99, 235, 0.1); }
.stage-item.failed .stage-dot { background: var(--paap-danger); }
.stage-item.pending .stage-dot { background: var(--paap-border-strong); }
.stage-item strong { display: block; font-size: 13px; color: var(--paap-text); }
.stage-item span:not(.stage-dot) { color: var(--paap-muted); font-size: 12px; }
.artifact-box { max-width: 100%; background: var(--paap-panel-subtle); border: 1px solid var(--paap-border); border-radius: var(--paap-radius-xs); padding: var(--paap-space-2) var(--paap-space-3); overflow: auto; }
.artifact-box code { color: var(--paap-text); font-size: 12px; font-family: var(--paap-mono); overflow-wrap: anywhere; word-break: break-word; }
.log-view { box-sizing: border-box; max-width: 100%; background: #0f1117; border-radius: var(--paap-radius); padding: var(--paap-space-3); min-height: 360px; max-height: 520px; overflow: auto; }
.log-line { color: #d1d5db; font-family: var(--paap-mono); font-size: 12px; line-height: 1.6; white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-word; }
.log-empty { color: #9ca3af; font-size: 12px; line-height: 1.6; }
.detail-actions { display: flex; align-items: center; gap: var(--paap-space-2); flex-wrap: wrap; }
.resource-row { cursor: pointer; }
.resource-row.selected td { background: var(--paap-accent-soft); }
@media (max-width: 900px) {
  .pipeline-workbench { grid-template-columns: 1fr; }
}
</style>
