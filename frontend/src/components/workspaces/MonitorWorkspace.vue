<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default>
      <div class="ws-tabs">
        <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <div v-if="activeTab === 'dashboards'" class="monitor-shell">
        <aside class="subject-rail">
          <div class="rail-head">
            <div class="rail-title">监控对象</div>
            <div class="rail-sub">组件 / 工具 / 数据库中间件</div>
          </div>
          <button
            v-for="subject in subjects"
            :key="subjectKey(subject)"
            type="button"
            class="subject-row"
            :class="{ active: sameSubject(selectedSubjectView, subject) }"
            @click="selectSubject(subject)"
          >
            <span>{{ subjectKindLabel(subject) }}</span>
            <strong>{{ subject.name }}</strong>
            <small>{{ subject.description }}</small>
          </button>
        </aside>

        <section class="dashboard-stage">
          <div v-if="dashboardFrames.length" class="grafana-grid">
            <div v-for="frame in dashboardFrames" :key="frame.name" class="dashboard-frame-shell">
              <iframe class="grafana-frame" :src="frame.url" :title="frame.name" loading="lazy" @load="compactGrafanaEmbed" />
            </div>
          </div>
          <div v-else class="empty-line">暂无 Grafana Dashboard，点击上方“导入默认大盘”后刷新。</div>
        </section>
      </div>

      <div v-if="activeTab === 'targets'" class="tab-panel">
        <div v-if="targets.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="item in targets" :key="item.name + item.description">
                <td class="cell-name">{{ item.name }}</td>
                <td><span class="badge" :class="statusBadge(item.status)">{{ item.status }}</span></td>
                <td class="cell-desc">{{ item.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无目标数据</div>
      </div>

      <div v-if="activeTab === 'alerts'" class="tab-panel">
        <div v-if="alerts.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="item in alerts" :key="item.name + item.status">
                <td class="cell-name">{{ item.name }}</td>
                <td><span class="badge" :class="statusBadge(item.status)">{{ item.status }}</span></td>
                <td class="cell-desc">{{ item.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无告警数据</div>
      </div>

      <div v-if="activeTab === 'rules'" class="tab-panel">
        <div v-if="rules.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="item in rules" :key="item.name + item.description">
                <td class="cell-name">{{ item.name }}</td>
                <td><span class="badge" :class="statusBadge(item.status)">{{ item.status }}</span></td>
                <td class="cell-desc">{{ item.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无规则数据</div>
      </div>

      <div v-if="activeTab === 'resources'" class="tab-panel">
        <div v-if="resources.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>类型</th><th>状态</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="r in resources" :key="r.name + r.type">
                <td class="cell-name">{{ r.name }}</td>
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
import { computed, ref, watch } from 'vue'
import ToolWorkspaceFrame from './ToolWorkspaceFrame.vue'
import { compactGrafanaEmbed } from './grafanaEmbed'
import type { WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
  initialSubjectKey?: string
}>()

const dashboards = computed(() => props.resources.filter(r => r.type === 'Dashboard'))
const targets = computed(() => props.resources.filter(r => r.type === 'Prometheus Target' || r.type === 'Target'))
const alerts = computed(() => props.resources.filter(r => r.type === 'Alert'))
const rules = computed(() => props.resources.filter(r => r.type === 'Rule'))
const subjects = computed(() => {
  const explicit = props.resources.filter(r => r.type === 'Monitor Subject')
  if (explicit.length) return explicit
  return [{
    name: 'environment',
    type: 'Monitor Subject',
    status: targets.value.length ? 'Ready' : 'Pending',
    description: '当前环境 Grafana 监控面板。',
    annotations: { subjectKind: 'environment' },
  }]
})
const selectedSubject = ref<WorkspaceResource | null>(null)
const selectedSubjectView = computed(() => selectedSubject.value || subjects.value[0] || null)

const dashboardFrames = computed(() => {
  const subject = selectedSubjectView.value
  if (subject?.externalUrl) {
    return [{
      name: `${subject.name} · ${subjectKindLabel(subject)}`,
      url: dashboardFrameURL(grafanaFrameSource(subject), subject),
    }].filter(item => item.url)
  }
  const subjectKind = String(subject?.annotations?.subjectKind || '')
  return dashboards.value
    .filter((dashboard) => {
      const dashboardKind = String(dashboard.annotations?.subjectKind || '')
      if (!subjectKind || !dashboardKind) return true
      return dashboardKind === subjectKind || dashboardKind === 'environment'
    })
    .slice(0, 1)
    .map((dashboard) => ({
      name: dashboard.name,
      url: dashboardFrameURL(grafanaFrameSource(dashboard), subject),
    }))
    .filter(item => item.url)
})

const availableTabs = computed(() => [
  { key: 'dashboards', label: 'Grafana 面板', count: dashboards.value.length },
  { key: 'targets', label: 'Targets', count: targets.value.length },
  { key: 'alerts', label: 'Alerts', count: alerts.value.length },
  { key: 'rules', label: 'Rules', count: rules.value.length },
  { key: 'resources', label: '资源', count: props.resources.length },
])
const activeTab = ref('dashboards')

const selectSubject = (subject: WorkspaceResource) => {
  selectedSubject.value = subject
}
const subjectKey = (subject: WorkspaceResource) =>
  `${subject.type}:${subject.annotations?.subjectKind || ''}:${subject.annotations?.namespace || ''}:${subject.name}`
const subjectTargetKey = (subject: WorkspaceResource) =>
  `monitor:${subject.annotations?.subjectKind || ''}:${subject.annotations?.namespace || ''}:${subject.name}`
const matchesInitialSubject = (subject: WorkspaceResource, targetKey: string) => {
  if (!targetKey) return false
  const servicePrefix = 'monitor:service:'
  if (targetKey.startsWith(servicePrefix)) {
    return String(subject.annotations?.serviceId || '').trim() === targetKey.slice(servicePrefix.length)
  }
  const parts = targetKey.split(':')
  const expectedKind = parts[1] || ''
  const expectedNamespace = parts[2] || ''
  const expectedName = parts.slice(3).join(':')
  const actualKind = String(subject.annotations?.subjectKind || '')
  const actualNamespace = String(subject.annotations?.namespace || '')
  const names = [
    subject.name,
    subject.annotations?.selector,
    subject.annotations?.component,
    subject.annotations?.serviceType,
  ].map(item => String(item || '').trim()).filter(Boolean)
  return targetKey === subjectTargetKey(subject) ||
    (!expectedKind || expectedKind === actualKind) &&
    (!expectedNamespace || expectedNamespace === actualNamespace) &&
    names.includes(expectedName)
}
const sameSubject = (a?: WorkspaceResource | null, b?: WorkspaceResource | null) =>
  !!a && !!b && subjectKey(a) === subjectKey(b)
const subjectKindLabel = (subject?: WorkspaceResource | null) => {
  const kind = String(subject?.annotations?.subjectKind || '')
  if (kind === 'component') return '组件'
  if (kind === 'tool') return '工具'
  if (kind === 'middleware') return '数据库/中间件'
  return '环境'
}
const grafanaFrameSource = (resource?: WorkspaceResource | null) =>
  String(resource?.annotations?.proxyURL || resource?.externalUrl || '').trim()
const dashboardFrameURL = (url: string, subject?: WorkspaceResource | null) => {
  if (!url) return ''
  try {
    const parsed = new URL(url, window.location.origin)
    parsed.searchParams.set('kiosk', '')
    parsed.searchParams.set('embed', 'true')
    parsed.searchParams.set('paap_embed', '1')
    parsed.searchParams.set('theme', 'light')
    parsed.searchParams.set('orgId', '1')
    const namespace = String(subject?.annotations?.namespace || '')
    const selector = String(subject?.annotations?.selector || subject?.name || '')
    if (namespace) parsed.searchParams.set('var-namespace', namespace)
    if (selector) parsed.searchParams.set('var-workload', selector)
    return parsed.pathname + parsed.search + parsed.hash
  } catch {
    return url
  }
}
const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('synced') || v.includes('up') || v.includes('resolved')) return 'green'
  if (v.includes('partial') || v.includes('pending') || v.includes('warning')) return 'orange'
  if (v.includes('error') || v.includes('fail') || v.includes('down') || v.includes('degraded') || v.includes('firing')) return 'red'
  return 'gray'
}

watch([subjects, () => props.initialSubjectKey], ([items, targetKey]) => {
  if (!items.length) {
    selectedSubject.value = null
    return
  }
  const targeted = items.find(item => matchesInitialSubject(item, String(targetKey || '')))
  if (targeted) {
    selectedSubject.value = targeted
    return
  }
  if (!selectedSubject.value || !items.some(item => sameSubject(item, selectedSubject.value))) {
    selectedSubject.value = items[0]
  }
}, { immediate: true })
</script>

<style scoped>
.monitor-shell {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: 8px;
  align-items: start;
}
.subject-rail {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  max-height: calc(100vh - 180px);
  overflow: auto;
  position: sticky;
  top: 8px;
}
.rail-head {
  padding: var(--paap-space-4);
  border-bottom: 1px solid var(--paap-border);
  background: var(--paap-panel-subtle);
}
.rail-title { font-size: 14px; font-weight: 600; color: var(--paap-text); }
.rail-sub { font-size: 12px; color: var(--paap-muted); margin-top: 2px; }
.subject-row {
  width: 100%;
  display: grid;
  gap: 4px;
  padding: 12px var(--paap-space-4);
  border: none;
  border-bottom: 1px solid #f3f4f6;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.1s;
  font-family: inherit;
}
.subject-row:last-child { border-bottom: none; }
.subject-row:hover { background: var(--paap-panel-subtle); }
.subject-row.active { background: var(--paap-accent-soft); }
.subject-row span {
  justify-self: start;
  font-size: 10px;
  font-weight: 600;
  color: var(--paap-accent);
  background: var(--paap-accent-soft);
  border-radius: var(--paap-radius-full);
  padding: 1px 8px;
}
.subject-row strong { color: var(--paap-text); font-size: 13px; word-break: break-word; }
.subject-row small { color: var(--paap-muted); font-size: 12px; line-height: 1.4; }
.dashboard-stage { display: grid; gap: var(--paap-space-4); min-width: 0; }
.stage-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-3);
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.stage-title { color: var(--paap-text); font-size: 20px; font-weight: 600; word-break: break-word; }
.stage-sub { color: var(--paap-muted); font-size: 13px; margin-top: var(--paap-space-1); }
.grafana-grid { display: grid; gap: var(--paap-space-3); }
.dashboard-frame-shell {
  height: calc(100vh - 220px);
  min-height: 600px;
  overflow: hidden;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: #fff;
}
.grafana-frame {
  width: 100%;
  height: 100%;
  border: 0;
  display: block;
  background: #fff;
}
@media (max-width: 900px) {
  .monitor-shell { grid-template-columns: 1fr; }
  .subject-rail { position: static; max-height: 420px; }
  .dashboard-frame-shell { height: 1100px; min-height: 780px; }
}
</style>
