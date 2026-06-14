<template>
  <section class="workspace-action-form" :class="{ 'workspace-action-form--danger': action.tone === 'danger' }">
    <header class="workspace-action-form__head">
      <div>
        <span>{{ title }}</span>
        <strong>{{ action.label }}</strong>
      </div>
      <button type="button" class="act-btn ghost" :disabled="running" @click="emit('cancel')">取消</button>
    </header>
    <p v-if="action.description" class="workspace-action-form__description">{{ action.description }}</p>
    <div v-if="(action.fields || []).length" class="workspace-action-form__fields">
      <label v-for="field in action.fields || []" :key="field.name" class="workspace-action-form__field">
        <span>{{ field.label }}</span>
        <textarea
          v-if="field.type === 'textarea'"
          class="workspace-action-form__input workspace-action-form__textarea"
          :value="fieldValue(field.name)"
          :placeholder="field.placeholder"
          :required="field.required"
          :disabled="running"
          @input="updateField(field.name, ($event.target as HTMLTextAreaElement).value)"
        />
        <input
          v-else-if="field.type === 'checkbox'"
          class="workspace-action-form__checkbox"
          type="checkbox"
          :checked="fieldValue(field.name) === 'true'"
          :disabled="running"
          @change="updateField(field.name, ($event.target as HTMLInputElement).checked ? 'true' : 'false')"
        />
        <input
          v-else
          class="workspace-action-form__input"
          :type="field.type === 'number' ? 'number' : 'text'"
          :value="fieldValue(field.name)"
          :placeholder="field.placeholder"
          :required="field.required"
          :disabled="running"
          @input="updateField(field.name, ($event.target as HTMLInputElement).value)"
        />
      </label>
    </div>
    <p v-if="error" class="workspace-action-form__error" role="alert">{{ error }}</p>
    <div class="workspace-action-form__footer">
      <button type="button" class="act-btn" :disabled="running" @click="emit('cancel')">取消</button>
      <button
        type="button"
        class="act-btn"
        :class="action.tone === 'danger' ? 'danger' : 'primary'"
        :disabled="running"
        @click="emit('submit')"
      >
        {{ running ? '执行中...' : '执行' }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { WorkspaceAction } from '../../views/serviceWorkspace'

const props = withDefaults(defineProps<{
  action: WorkspaceAction
  params?: Record<string, string>
  running?: boolean
  error?: string
  title?: string
}>(), {
  params: () => ({}),
  running: false,
  error: '',
  title: '当前操作',
})

const emit = defineEmits<{
  (event: 'update-param', payload: { name: string; value: string }): void
  (event: 'submit'): void
  (event: 'cancel'): void
}>()

const fieldValue = (name: string) => String(props.params?.[name] ?? '')
const updateField = (name: string, value: string) => emit('update-param', { name, value })
</script>

<style scoped>
.workspace-action-form {
  display: grid;
  gap: var(--paap-space-3);
  min-width: 0;
  margin-top: var(--paap-space-3);
  padding: var(--paap-space-4);
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-sm);
  background: var(--paap-panel-subtle);
}
.workspace-action-form--danger {
  border-color: var(--paap-danger-soft);
  background: #fff7f7;
}
.workspace-action-form__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--paap-space-3);
  min-width: 0;
}
.workspace-action-form__head > div {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.workspace-action-form__head span {
  color: var(--paap-muted);
  font-size: 11px;
  font-weight: 650;
}
.workspace-action-form__head strong {
  min-width: 0;
  color: var(--paap-text);
  font-size: 14px;
  font-weight: 700;
  overflow-wrap: anywhere;
}
.workspace-action-form__description {
  margin: 0;
  color: var(--paap-muted);
  font-size: 12px;
  line-height: 1.5;
}
.workspace-action-form__fields {
  display: grid;
  gap: var(--paap-space-2);
  min-width: 0;
}
.workspace-action-form__field {
  display: grid;
  gap: 6px;
  min-width: 0;
}
.workspace-action-form__field > span {
  color: var(--paap-muted);
  font-size: 12px;
  font-weight: 650;
}
.workspace-action-form__input {
  width: 100%;
  min-width: 0;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--paap-border);
  border-radius: var(--paap-radius-xs);
  background: var(--paap-panel);
  color: var(--paap-text);
  font: inherit;
  font-size: 12px;
}
.workspace-action-form__textarea {
  min-height: 86px;
  padding: 9px 10px;
  line-height: 1.5;
  resize: vertical;
}
.workspace-action-form__checkbox {
  width: 18px;
  height: 18px;
}
.workspace-action-form__error {
  margin: 0;
  color: var(--paap-danger);
  font-size: 12px;
}
.workspace-action-form__footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--paap-space-2);
  flex-wrap: wrap;
}
</style>
