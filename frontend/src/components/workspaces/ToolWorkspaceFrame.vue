<template>
  <div class="tool-workspace">
    <div v-if="!resources.length" class="ws-empty">
      <div class="ws-empty-icon">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <p>暂无数据，点击上方操作按钮获取最新状态。</p>
    </div>
    <template v-else>
      <slot :resources="resources" :run-action="runAction" />
    </template>
  </div>
</template>

<script setup lang="ts">
import type { WorkspaceAction, WorkspaceResource } from '../../views/serviceWorkspace'

defineProps<{
  resources: WorkspaceResource[]
  runAction?: (action: WorkspaceAction, target?: string) => void
}>()
</script>

<style scoped>
.tool-workspace {
  animation: ws-enter 0.2s ease-out;
}
.ws-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--paap-space-12) var(--paap-space-6);
  text-align: center;
  color: var(--paap-muted);
  font-size: var(--paap-fs-body);
  gap: var(--paap-space-3);
}
.ws-empty-icon {
  color: var(--paap-border-strong);
}
@keyframes ws-enter {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>

<style>
/* ===== Workspace Shared Design System ===== */

/* Panels */
.tab-panel {
  min-height: 80px;
  animation: fade-in 0.25s ease both;
}
/* Tables — clean & modern */
.table-wrap {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  overflow: hidden;
  max-width: 100%;
}
.table-wrap.scroll { overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--paap-fs-compact); }
.data-table thead { background: var(--paap-panel-subtle); }
.data-table th {
  text-align: left;
  padding: var(--paap-space-3) var(--paap-space-4);
  font-size: var(--paap-fs-small);
  font-weight: 600;
  color: var(--paap-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--paap-border);
  white-space: nowrap;
}
.data-table td {
  padding: 12px var(--paap-space-4);
  border-bottom: 1px solid var(--paap-panel-subtle);
  color: var(--paap-text);
  font-size: var(--paap-fs-compact);
  transition: background var(--paap-transition-fast);
}
.data-table tr:last-child td { border-bottom: none; }
.data-table tr:hover td { background: var(--paap-panel-subtle); }
.cell-name { font-weight: 500; }
.cell-desc { color: var(--paap-muted); font-size: var(--paap-fs-label); }

/* Badges */
.badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: var(--paap-radius-full);
  font-size: var(--paap-fs-small);
  font-weight: 500;
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
.badge.green { background: var(--paap-success-soft); color: var(--paap-success); }
.badge.red { background: var(--paap-danger-soft); color: var(--paap-danger); }
.badge.blue { background: var(--paap-accent-soft); color: var(--paap-accent); }
.badge.yellow { background: var(--paap-warning-soft); color: var(--paap-warning); }
.badge.gray { background: var(--paap-panel-subtle); color: var(--paap-muted); }
.badge.orange { background: var(--paap-warning-soft); color: var(--paap-warning); }

/* Action buttons */
.act-btn {
  height: 32px;
  padding: 0 12px;
  border-radius: var(--paap-radius-sm);
  border: 1px solid var(--paap-border);
  background: var(--paap-panel);
  color: var(--paap-muted);
  font-size: var(--paap-fs-label);
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  font-family: inherit;
}
.act-btn:hover { background: var(--paap-panel-subtle); border-color: var(--paap-border-strong); }
.act-btn.primary { background: var(--paap-accent); color: #fff; border-color: var(--paap-accent); }
.act-btn.primary:hover { background: var(--paap-accent-hover); border-color: var(--paap-accent-hover); }
.act-btn.danger { color: #fff; border-color: var(--paap-danger); background: var(--paap-danger); }
.act-btn.danger:hover { background: var(--paap-danger-hover); border-color: var(--paap-danger-hover); }
.act-btn.ghost { border-color: transparent; background: transparent; }
.act-btn.ghost:hover { background: var(--paap-panel-subtle); }

/* Cards */
.card {
  background: var(--paap-panel);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius);
  padding: var(--paap-space-4);
  transition: border-color 0.15s, box-shadow 0.15s, transform 0.12s;
}
.card:hover {
  border-color: var(--paap-border-strong);
  box-shadow: var(--paap-shadow-sm);
  transform: translateY(-1px);
}
.card-title { font-weight: 600; font-size: var(--paap-fs-body); color: var(--paap-text); margin-bottom: 2px; }
.card-sub { font-size: var(--paap-fs-label); color: var(--paap-muted); margin-bottom: var(--paap-space-2); }
.card-footer { display: flex; align-items: center; justify-content: space-between; margin-top: var(--paap-space-2); }

/* Links */
.link { color: var(--paap-accent); font-weight: 500; text-decoration: none; }
.link:hover { text-decoration: underline; }
.link.external { display: inline-flex; align-items: center; gap: 3px; }
.link.external::after { content: '↗'; font-size: 10px; opacity: 0.7; }

/* Monospace */
.mono { font-family: var(--paap-mono); font-size: var(--paap-fs-code); }

/* Status pills */
.status-pill {
  font-size: var(--paap-fs-small);
  padding: 2px 8px;
  border-radius: var(--paap-radius-full);
  font-weight: 500;
  background: var(--paap-panel-subtle);
  color: var(--paap-muted);
}
.status-pill.green { background: var(--paap-success-soft); color: var(--paap-success); }
.status-pill.red { background: var(--paap-danger-soft); color: var(--paap-danger); }
.status-pill.blue { background: var(--paap-accent-soft); color: var(--paap-accent); }
.status-pill.yellow { background: var(--paap-warning-soft); color: var(--paap-warning); }
.status-pill.gray { background: var(--paap-panel-subtle); color: var(--paap-muted); }

/* Grids */
.grid-260 { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 320px)); gap: var(--paap-space-3); justify-content: center; }
.grid-300 { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 360px)); gap: var(--paap-space-3); justify-content: center; }

/* Section label */
.ws-section-label {
  font-size: var(--paap-fs-small);
  font-weight: 600;
  color: var(--paap-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: var(--paap-space-3);
}
</style>
