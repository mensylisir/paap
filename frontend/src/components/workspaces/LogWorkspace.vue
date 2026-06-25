<template>
  <ToolWorkspaceFrame :resources="resources">
    <template #default>
      <div class="ws-tabs">
        <button v-for="tab in availableTabs" :key="tab.key" class="ws-tab" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key">
          {{ tab.label }}
          <span v-if="tab.count" class="ws-tab-badge">{{ tab.count }}</span>
        </button>
      </div>

      <div v-if="activeTab === 'logs'" class="log-shell">
        <aside class="subject-rail">
          <div class="rail-head">
            <div class="rail-title">日志对象</div>
            <div class="rail-sub">当前查看：{{ selectedSubjectView?.name || '-' }}</div>
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

        <section class="log-main">
          <div v-if="grafanaLogFrames.length" class="loki-panel-list">
            <div v-for="frame in grafanaLogFrames" :key="frame.name" class="loki-frame-shell">
              <iframe class="loki-frame" :src="frame.url" :title="frame.name" loading="lazy" @load="compactGrafanaEmbed" />
            </div>
          </div>
        </section>
      </div>

      <div v-if="activeTab === 'streams'" class="tab-panel">
        <div v-if="streams.length" class="table-wrap">
          <table class="data-table">
            <thead><tr><th>名称</th><th>状态</th><th>Labels</th></tr></thead>
            <tbody>
              <tr v-for="stream in streams" :key="stream.name + stream.description">
                <td class="cell-name">{{ stream.name }}</td>
                <td><span class="badge" :class="statusBadge(stream.status)">{{ stream.status }}</span></td>
                <td class="cell-desc">{{ stream.description }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty-line">暂无日志流</div>
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
import { withEmbeddedProxyAuthToken, type WorkspaceResource } from '../../views/serviceWorkspace'

const props = defineProps<{
  resources: WorkspaceResource[]
  initialSubjectKey?: string
}>()

const streams = computed(() => props.resources.filter(r => r.type === 'Log Stream' || r.type === 'Log Streams' || r.type === 'Stream'))
const grafanaPanels = computed(() => props.resources.filter(r => r.type === 'Grafana Loki Panel'))
const subjects = computed(() => {
  const explicit = props.resources.filter(r => r.type === 'Log Subject')
  if (explicit.length) return sortLogSubjects(explicit)
  return [{ name: 'environment', type: 'Log Subject', status: 'Ready', description: '当前环境全部日志。', annotations: { subjectKind: 'environment' }, children: streams.value }]
})
const selectedSubject = ref<WorkspaceResource | null>(null)
const selectedSubjectView = computed(() => selectedSubject.value || subjects.value[0] || null)
const grafanaLogFrames = computed(() => {
  const subject = selectedSubjectView.value
  if (subject?.externalUrl) {
    return [{
      name: `${subject.name} · ${subjectKindLabel(subject)}`,
      url: toEmbeddedGrafanaURL(grafanaFrameSource(subject), subject),
    }].filter(item => item.url)
  }
  return grafanaPanels.value.map((panel) => ({
    name: panel.name,
    url: toEmbeddedGrafanaURL(grafanaFrameSource(panel), subject),
  })).filter(item => item.url)
})
const availableTabs = computed(() => [
  { key: 'logs', label: '日志', count: subjects.value.length },
  { key: 'streams', label: 'Streams', count: streams.value.length },
  { key: 'resources', label: '资源', count: props.resources.length },
])
const activeTab = ref('logs')

const selectSubject = (subject: WorkspaceResource) => {
  selectedSubject.value = subject
}
const numericAnnotation = (subject: WorkspaceResource, key: string) => Number(subject.annotations?.[key] || 0)
const sortLogSubjects = (items: WorkspaceResource[]) =>
  [...items].sort((a, b) =>
    numericAnnotation(b, 'entryCount') - numericAnnotation(a, 'entryCount') ||
    numericAnnotation(b, 'streamCount') - numericAnnotation(a, 'streamCount') ||
    a.name.localeCompare(b.name)
  )
const subjectKey = (subject: WorkspaceResource) =>
  `${subject.type}:${subject.annotations?.subjectKind || ''}:${subject.annotations?.namespace || ''}:${subject.name}`
const matchesInitialSubject = (subject: WorkspaceResource, targetKey: string) => {
  if (!targetKey) return false
  const servicePrefix = 'log:service:'
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
  ].map(item => String(item || '').trim()).filter(Boolean)
  if (expectedKind === 'pod-prefix') {
    return actualKind === 'pod' &&
      (!expectedNamespace || expectedNamespace === actualNamespace) &&
      names.some(name => name === expectedName || name.startsWith(`${expectedName}-`) || name.startsWith(expectedName))
  }
  return (!expectedKind || expectedKind === actualKind) &&
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
  if (kind === 'pod') return '运行实例'
  return '环境'
}
const grafanaFrameSource = (resource?: WorkspaceResource | null) =>
  String(resource?.annotations?.proxyURL || resource?.externalUrl || '').trim()
const toEmbeddedGrafanaURL = (url: string, subject?: WorkspaceResource | null) => {
  if (!url) return ''
  try {
    const parsed = new URL(url, window.location.origin)
    parsed.searchParams.set('kiosk', '')
    parsed.searchParams.set('embed', 'true')
    parsed.searchParams.set('paap_embed', '1')
    parsed.searchParams.set('theme', 'light')
    parsed.searchParams.set('orgId', '1')
    const namespace = String(subject?.annotations?.namespace || '')
    if (namespace) parsed.searchParams.set('var-namespace', namespace)
    const query = String(subject?.annotations?.logQuery || '')
    if (query && parsed.pathname.includes('/explore')) {
      const left = {
        datasource: 'Loki',
        queries: [{ refId: 'A', expr: query, queryType: 'range' }],
        range: { from: 'now-24h', to: 'now' },
      }
      parsed.searchParams.set('left', JSON.stringify(left))
    }
    return withEmbeddedProxyAuthToken(parsed.pathname + parsed.search + parsed.hash)
  } catch {
    return url
  }
}
const statusBadge = (s?: string) => {
  const v = String(s || '').toLowerCase()
  if (v.includes('ready') || v.includes('healthy') || v.includes('recent')) return 'green'
  if (v.includes('error') || v.includes('fail') || v.includes('degraded')) return 'red'
  if (v.includes('partial') || v.includes('empty')) return 'orange'
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
.log-shell {
  display: grid;
  grid-template-columns: minmax(240px, 280px) minmax(0, 1fr);
  gap: 8px;
  align-items: start;
}
.subject-rail {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  max-height: calc(100vh - 180px);
  overflow-y: auto;
  overflow-x: hidden;
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
.log-main { display: grid; gap: var(--paap-space-4); min-width: 0; }
.log-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-3);
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-5);
}
.log-title { color: var(--paap-text); font-size: 20px; font-weight: 600; word-break: break-word; }
.log-sub { color: var(--paap-muted); font-size: 13px; margin-top: var(--paap-space-1); }
.loki-panel-list { display: grid; gap: var(--paap-space-3); }
.loki-frame-shell {
  height: calc(100vh - 220px);
  min-height: 600px;
  overflow: hidden;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  background: #fff;
}
.loki-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
  background: #fff;
}
.stream-list {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-4);
}
.section-label { font-size: 11px; font-weight: 600; color: var(--paap-muted); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: var(--paap-space-2); }
.stream-table { display: grid; gap: var(--paap-space-2); }
.stream-row {
  display: grid;
  grid-template-columns: minmax(120px, 0.35fr) minmax(0, 1fr);
  gap: var(--paap-space-3);
  align-items: center;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel);
  padding: 10px 12px;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.1s;
}
.stream-row:hover { border-color: var(--paap-border-strong); }
.stream-row.selected { border-color: var(--paap-accent); background: var(--paap-accent-soft); }
.stream-row strong { color: var(--paap-text); font-size: 12px; word-break: break-word; }
.stream-row small { color: var(--paap-muted); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-row { display: flex; gap: var(--paap-space-3); padding: 2px 0; color: #d1d5db; }
.log-time { color: #6b7280; white-space: nowrap; flex-shrink: 0; width: 72px; font-family: var(--paap-mono); font-size: 11px; }
.log-level { width: 48px; flex-shrink: 0; font-weight: 600; text-align: center; border-radius: var(--paap-radius-xs); font-size: 10px; padding: 1px 0; }
.log-level.fatal { background: #7f1d1d; color: #fecaca; }
.log-level.error { background: #991b1b; color: #fecaca; }
.log-level.warn { background: #854d0e; color: #fef08a; }
.log-level.info { background: #14532d; color: #bbf7d0; }
.log-level.debug { background: #374151; color: #d1d5db; }
.log-level.log { background: #1f2937; color: #9ca3af; }
.log-msg { color: #e5e7eb; word-break: break-word; font-size: 12px; }
.terminal-empty { color: #9ca3af; padding: var(--paap-space-5) var(--paap-space-4); font-size: 12px; }
@media (max-width: 900px) {
  .log-shell { grid-template-columns: 1fr; }
  .subject-rail { position: static; max-height: 420px; }
  .loki-frame-shell { height: 900px; min-height: 720px; }
  .stream-row { grid-template-columns: 1fr; }
  .stream-row small { white-space: normal; }
}
</style>
